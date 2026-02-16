// Package task 提供异步任务管理器
package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// 任务状态常量
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Task 异步任务
type Task struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Payload     string     `json:"payload"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	Progress    int        `json:"progress"`
	ProgressMsg string     `json:"progress_msg,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Update 进度更新事件（SSE 推送）
type Update struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	ProgressMsg string `json:"progress_msg,omitempty"`
	Metadata    string `json:"metadata,omitempty"` // JSON 格式的结构化附加数据（如辩论阶段详情）
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	Done        bool   `json:"done"`
}

// Manager 异步任务管理器
type Manager struct {
	mu        sync.RWMutex
	tasks     map[string]*Task          // 内存热缓存
	listeners map[string][]chan Update   // SSE 订阅者
	db        *sql.DB
	sem       chan struct{}              // 并发限制
}

// NewManager 创建任务管理器
func NewManager(db *sql.DB, maxConcurrent int) *Manager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Manager{
		tasks:     make(map[string]*Task),
		listeners: make(map[string][]chan Update),
		db:        db,
		sem:       make(chan struct{}, maxConcurrent),
	}
}

// Submit 提交任务，返回任务 ID
func (m *Manager) Submit(taskType string, payload interface{}) (string, error) {
	id := uuid.New().String()
	now := time.Now()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	task := &Task{
		ID:        id,
		Type:      taskType,
		Status:    StatusPending,
		Payload:   string(payloadBytes),
		CreatedAt: now,
	}

	// 存 DB
	_, err = m.db.Exec(
		`INSERT INTO tasks (id, type, status, payload, created_at) VALUES (?, ?, ?, ?, ?)`,
		task.ID, task.Type, task.Status, task.Payload, task.CreatedAt,
	)
	if err != nil {
		return "", err
	}

	// 存内存
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	return id, nil
}

// Execute 在后台执行任务
func (m *Manager) Execute(taskID string, fn func(ctx context.Context, reportProgress func(int, string, ...string)) (string, error)) {
	go func() {
		// 获取信号量，控制并发
		m.sem <- struct{}{}
		defer func() { <-m.sem }()

		// 标记为 running
		m.updateStatus(taskID, StatusRunning, 0, "")

		ctx := context.Background()

		progressFn := func(progress int, msg string, metadata ...string) {
			m.ReportProgress(taskID, progress, msg, metadata...)
		}

		result, err := fn(ctx, progressFn)

		if err != nil {
			m.fail(taskID, err.Error())
		} else {
			m.complete(taskID, result)
		}
	}()
}

// Get 查询任务（先查内存，miss 查 DB）
func (m *Manager) Get(taskID string) (*Task, error) {
	// 先查内存
	m.mu.RLock()
	if t, ok := m.tasks[taskID]; ok {
		m.mu.RUnlock()
		return t, nil
	}
	m.mu.RUnlock()

	// 查 DB
	row := m.db.QueryRow(
		`SELECT id, type, status, payload, result, error, progress, progress_msg, created_at, completed_at FROM tasks WHERE id = ?`,
		taskID,
	)

	t := &Task{}
	var completedAt sql.NullTime
	var result, errStr, progressMsg sql.NullString

	err := row.Scan(&t.ID, &t.Type, &t.Status, &t.Payload, &result, &errStr, &t.Progress, &progressMsg, &t.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}

	if result.Valid {
		t.Result = result.String
	}
	if errStr.Valid {
		t.Error = errStr.String
	}
	if progressMsg.Valid {
		t.ProgressMsg = progressMsg.String
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}

	// 缓存到内存
	m.mu.Lock()
	m.tasks[taskID] = t
	m.mu.Unlock()

	return t, nil
}

// List 列出最近任务
func (m *Manager) List(taskType string, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if taskType != "" {
		rows, err = m.db.Query(
			`SELECT id, type, status, payload, result, error, progress, progress_msg, created_at, completed_at
			 FROM tasks WHERE type = ? ORDER BY created_at DESC LIMIT ?`,
			taskType, limit,
		)
	} else {
		rows, err = m.db.Query(
			`SELECT id, type, status, payload, result, error, progress, progress_msg, created_at, completed_at
			 FROM tasks ORDER BY created_at DESC LIMIT ?`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var completedAt sql.NullTime
		var result, errStr, progressMsg sql.NullString

		if err := rows.Scan(&t.ID, &t.Type, &t.Status, &t.Payload, &result, &errStr, &t.Progress, &progressMsg, &t.CreatedAt, &completedAt); err != nil {
			continue
		}
		if result.Valid {
			t.Result = result.String
		}
		if errStr.Valid {
			t.Error = errStr.String
		}
		if progressMsg.Valid {
			t.ProgressMsg = progressMsg.String
		}
		if completedAt.Valid {
			t.CompletedAt = &completedAt.Time
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

// Subscribe 订阅任务进度更新，返回更新 channel 和取消函数
func (m *Manager) Subscribe(taskID string) (<-chan Update, func()) {
	ch := make(chan Update, 16)

	m.mu.Lock()
	m.listeners[taskID] = append(m.listeners[taskID], ch)
	m.mu.Unlock()

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		subs := m.listeners[taskID]
		for i, s := range subs {
			if s == ch {
				m.listeners[taskID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}

	return ch, unsubscribe
}

// ReportProgress 报告任务进度，可选附加 metadata（JSON 格式）
func (m *Manager) ReportProgress(taskID string, progress int, msg string, metadata ...string) {
	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.Progress = progress
		t.ProgressMsg = msg
	}
	m.mu.Unlock()

	// 更新 DB
	_, err := m.db.Exec(
		`UPDATE tasks SET progress = ?, progress_msg = ? WHERE id = ?`,
		progress, msg, taskID,
	)
	if err != nil {
		logger.Warn("更新任务进度失败", zap.Error(err))
	}

	// 通知订阅者
	update := Update{
		TaskID:      taskID,
		Status:      StatusRunning,
		Progress:    progress,
		ProgressMsg: msg,
	}
	if len(metadata) > 0 && metadata[0] != "" {
		update.Metadata = metadata[0]
	}
	m.broadcast(taskID, update)
}

// updateStatus 更新任务状态
func (m *Manager) updateStatus(taskID, status string, progress int, msg string) {
	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.Status = status
		t.Progress = progress
		t.ProgressMsg = msg
	}
	m.mu.Unlock()

	_, err := m.db.Exec(
		`UPDATE tasks SET status = ?, progress = ?, progress_msg = ? WHERE id = ?`,
		status, progress, msg, taskID,
	)
	if err != nil {
		logger.Warn("更新任务状态失败", zap.Error(err))
	}

	m.broadcast(taskID, Update{
		TaskID:      taskID,
		Status:      status,
		Progress:    progress,
		ProgressMsg: msg,
	})
}

// complete 标记任务完成
func (m *Manager) complete(taskID, result string) {
	now := time.Now()

	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.Status = StatusCompleted
		t.Result = result
		t.Progress = 100
		t.ProgressMsg = "完成"
		t.CompletedAt = &now
	}
	m.mu.Unlock()

	_, err := m.db.Exec(
		`UPDATE tasks SET status = ?, result = ?, progress = 100, progress_msg = '完成', completed_at = ? WHERE id = ?`,
		StatusCompleted, result, now, taskID,
	)
	if err != nil {
		logger.Warn("更新任务完成状态失败", zap.Error(err))
	}

	m.broadcast(taskID, Update{
		TaskID:   taskID,
		Status:   StatusCompleted,
		Progress: 100,
		Result:   result,
		Done:     true,
	})
}

// fail 标记任务失败
func (m *Manager) fail(taskID, errMsg string) {
	now := time.Now()

	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.Status = StatusFailed
		t.Error = errMsg
		t.CompletedAt = &now
	}
	m.mu.Unlock()

	_, err := m.db.Exec(
		`UPDATE tasks SET status = ?, error = ?, completed_at = ? WHERE id = ?`,
		StatusFailed, errMsg, now, taskID,
	)
	if err != nil {
		logger.Warn("更新任务失败状态失败", zap.Error(err))
	}

	m.broadcast(taskID, Update{
		TaskID: taskID,
		Status: StatusFailed,
		Error:  errMsg,
		Done:   true,
	})
}

// broadcast 向所有订阅者发送更新
func (m *Manager) broadcast(taskID string, update Update) {
	m.mu.RLock()
	subs := make([]chan Update, len(m.listeners[taskID]))
	copy(subs, m.listeners[taskID])
	m.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- update:
		default:
			// channel 满了，跳过（避免阻塞）
		}
	}
}
