// Package api 提供会话管理（支持 SQLite 持久化）
package api

import (
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jay3cx/Quinfi/internal/agent"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Session 会话
type Session struct {
	ID           string          `json:"id"`
	History      []agent.Message `json:"history"`
	CreatedAt    time.Time       `json:"created_at"`
	LastActiveAt time.Time       `json:"last_active_at"`
}

// SessionManager 会话管理器（Write-Through: 内存缓存 + DB 持久化）
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
	db       *sql.DB // 可选，为 nil 时纯内存模式
}

// NewSessionManager 创建会话管理器
func NewSessionManager(ttl time.Duration, db *sql.DB) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
		db:       db,
	}

	// 从 DB 恢复会话
	if db != nil {
		sm.loadFromDB()
	}

	// 启动清理协程
	go sm.cleanupLoop()

	return sm
}

// Create 创建新会话
func (sm *SessionManager) Create() *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	session := &Session{
		ID:           uuid.New().String(),
		History:      make([]agent.Message, 0),
		CreatedAt:    now,
		LastActiveAt: now,
	}

	sm.sessions[session.ID] = session

	// 持久化
	if sm.db != nil {
		sm.dbInsertSession(session)
	}

	return session
}

// Get 获取会话
func (sm *SessionManager) Get(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[id]
	if !ok {
		return nil, false
	}

	if time.Since(session.LastActiveAt) > sm.ttl {
		return nil, false
	}

	return session, true
}

// Update 更新会话（追加消息）
func (sm *SessionManager) Update(id string, msg agent.Message) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return false
	}

	session.History = append(session.History, msg)
	session.LastActiveAt = time.Now()

	// 持久化
	if sm.db != nil {
		sm.dbInsertMessage(id, msg)
		sm.dbUpdateSessionActive(id, session.LastActiveAt)
	}

	return true
}

// UpsertLastAssistant 更新或追加最后一条 assistant 消息（仅内存）
// 用于流式处理期间保存中间状态，让刷新后的轮询客户端能看到中间进度。
// 不写数据库，最终持久化由 FinalizeAssistant 完成。
func (sm *SessionManager) UpsertLastAssistant(id string, msg agent.Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return
	}

	n := len(session.History)
	if n > 0 && session.History[n-1].Role == "assistant" {
		session.History[n-1] = msg // 就地替换
	} else {
		session.History = append(session.History, msg)
	}
	session.LastActiveAt = time.Now()
}

// FinalizeAssistant 完成流式处理，最终保存 assistant 消息到内存 + DB
func (sm *SessionManager) FinalizeAssistant(id string, msg agent.Message) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[id]
	if !ok {
		return false
	}

	n := len(session.History)
	if n > 0 && session.History[n-1].Role == "assistant" {
		session.History[n-1] = msg
	} else {
		session.History = append(session.History, msg)
	}
	session.LastActiveAt = time.Now()

	if sm.db != nil {
		sm.dbInsertMessage(id, msg)
		sm.dbUpdateSessionActive(id, session.LastActiveAt)
	}

	return true
}

// Delete 删除会话
func (sm *SessionManager) Delete(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)

	if sm.db != nil {
		sm.dbDeleteSession(id)
	}
}

// ListSessions 返回所有活跃会话（按最后活跃时间降序）
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	now := time.Now()
	var list []*Session
	for _, s := range sm.sessions {
		if now.Sub(s.LastActiveAt) <= sm.ttl && len(s.History) > 0 {
			list = append(list, s)
		}
	}

	// 按最后活跃时间降序
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastActiveAt.After(list[j].LastActiveAt)
	})

	return list
}

// GetOrCreate 获取或创建会话
func (sm *SessionManager) GetOrCreate(id string) *Session {
	if id != "" {
		if session, ok := sm.Get(id); ok {
			return session
		}
	}
	return sm.Create()
}

// === DB 操作（内部方法）===

func (sm *SessionManager) loadFromDB() {
	// 先收集所有 session，关闭 rows 后再查 messages（避免 SQLite 单连接死锁）
	rows, err := sm.db.Query(`
		SELECT id, created_at, last_active_at FROM sessions
		WHERE last_active_at > datetime('now', ?)
		ORDER BY last_active_at DESC
	`, fmt.Sprintf("-%.0f seconds", sm.ttl.Seconds()))
	if err != nil {
		logger.Error("加载会话失败", zap.Error(err))
		return
	}

	var sessions []*Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.LastActiveAt); err != nil {
			logger.Error("解析会话失败", zap.Error(err))
			continue
		}
		s.History = make([]agent.Message, 0)
		sessions = append(sessions, &s)
	}
	rows.Close() // 必须先关闭，释放连接

	// 再逐个加载消息
	for _, s := range sessions {
		s.History = sm.dbLoadMessages(s.ID)
		sm.sessions[s.ID] = s
	}

	logger.Info("从数据库恢复会话", zap.Int("count", len(sessions)))
}

func (sm *SessionManager) dbLoadMessages(sessionID string) []agent.Message {
	rows, err := sm.db.Query(
		`SELECT role, content, metadata FROM messages WHERE session_id = ? ORDER BY id ASC`,
		sessionID,
	)
	if err != nil {
		logger.Error("加载消息失败", zap.String("session_id", sessionID), zap.Error(err))
		return nil
	}
	defer rows.Close()

	var messages []agent.Message
	for rows.Next() {
		var m agent.Message
		if err := rows.Scan(&m.Role, &m.Content, &m.Metadata); err != nil {
			continue
		}
		messages = append(messages, m)
	}
	return messages
}

func (sm *SessionManager) dbInsertSession(s *Session) {
	_, err := sm.db.Exec(
		`INSERT OR IGNORE INTO sessions (id, created_at, last_active_at) VALUES (?, ?, ?)`,
		s.ID, s.CreatedAt, s.LastActiveAt,
	)
	if err != nil {
		logger.Error("持久化会话失败", zap.String("id", s.ID), zap.Error(err))
	}
}

func (sm *SessionManager) dbInsertMessage(sessionID string, msg agent.Message) {
	_, err := sm.db.Exec(
		`INSERT INTO messages (session_id, role, content, metadata) VALUES (?, ?, ?, ?)`,
		sessionID, msg.Role, msg.Content, msg.Metadata,
	)
	if err != nil {
		logger.Error("持久化消息失败", zap.String("session_id", sessionID), zap.Error(err))
	}
}

func (sm *SessionManager) dbUpdateSessionActive(id string, t time.Time) {
	_, err := sm.db.Exec(
		`UPDATE sessions SET last_active_at = ? WHERE id = ?`,
		t, id,
	)
	if err != nil {
		logger.Error("更新会话活跃时间失败", zap.String("id", id), zap.Error(err))
	}
}

func (sm *SessionManager) dbDeleteSession(id string) {
	_, _ = sm.db.Exec(`DELETE FROM messages WHERE session_id = ?`, id)
	_, _ = sm.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
}

// === 清理协程 ===

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanup()
	}
}

func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if now.Sub(session.LastActiveAt) > sm.ttl {
			delete(sm.sessions, id)
		}
	}

	// DB 也清理过期数据
	if sm.db != nil {
		sm.db.Exec(`DELETE FROM messages WHERE session_id IN (SELECT id FROM sessions WHERE last_active_at < datetime('now', ?))`,
			fmt.Sprintf("-%.0f seconds", sm.ttl.Seconds()))
		sm.db.Exec(`DELETE FROM sessions WHERE last_active_at < datetime('now', ?)`,
			fmt.Sprintf("-%.0f seconds", sm.ttl.Seconds()))
	}
}
