// Package memory 提供记忆存储与召回
package memory

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Store 记忆存储（基于 SQLite）
type Store struct {
	db *sql.DB
}

// NewStore 创建记忆存储
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Store 存储新记忆（去重：相同 type + 相似 content 不重复存）
func (s *Store) Save(ctx context.Context, entries []MemoryEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO memories (user_id, type, content, source_session, importance, valid_from, valid_to)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	saved := 0
	for _, e := range entries {
		// 去重检查：同 user + type 下是否有高度相似的 content
		if s.isDuplicate(ctx, tx, e) {
			continue
		}

		userID := e.UserID
		if userID == "" {
			userID = "default"
		}

		_, err := stmt.ExecContext(ctx, userID, e.Type, e.Content, e.SourceSession, e.Importance, e.ValidFrom, e.ValidTo)
		if err != nil {
			logger.Error("存储记忆失败", zap.String("content", e.Content), zap.Error(err))
			continue
		}
		saved++
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if saved > 0 {
		logger.Info("存储新记忆", zap.Int("count", saved))
	}
	return nil
}

// Recall 召回与查询相关的记忆
// 策略：加载所有有效记忆 → 复合评分 → 返回 Top-N
func (s *Store) Recall(ctx context.Context, userID, query string, limit int) ([]MemoryEntry, error) {
	if userID == "" {
		userID = "default"
	}
	if limit <= 0 {
		limit = 8
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, type, content, source_session, importance,
		       valid_from, valid_to, access_count, last_accessed_at,
		       created_at, updated_at
		FROM memories
		WHERE user_id = ?
		  AND (valid_to IS NULL OR valid_to > datetime('now'))
		ORDER BY created_at DESC
		LIMIT 200
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []MemoryEntry
	for rows.Next() {
		var m MemoryEntry
		var validFrom, validTo, lastAccessed sql.NullTime
		var sourceSession sql.NullString

		err := rows.Scan(
			&m.ID, &m.UserID, &m.Type, &m.Content, &sourceSession, &m.Importance,
			&validFrom, &validTo, &m.AccessCount, &lastAccessed,
			&m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if sourceSession.Valid {
			m.SourceSession = sourceSession.String
		}
		if validFrom.Valid {
			m.ValidFrom = &validFrom.Time
		}
		if validTo.Valid {
			m.ValidTo = &validTo.Time
		}
		if lastAccessed.Valid {
			m.LastAccessedAt = &lastAccessed.Time
		}
		all = append(all, m)
	}

	if len(all) == 0 {
		return nil, nil
	}

	// 复合评分
	scored := scoreMemories(all, query)

	// 取 Top-N
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// 更新 access_count
	s.updateAccessCounts(ctx, scored)

	return scored, nil
}

// Invalidate 使某条记忆失效
func (s *Store) Invalidate(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE memories SET valid_to = datetime('now'), updated_at = datetime('now') WHERE id = ?`,
		id,
	)
	return err
}

// GetByUser 获取用户所有有效记忆
func (s *Store) GetByUser(ctx context.Context, userID string) ([]MemoryEntry, error) {
	return s.Recall(ctx, userID, "", 100)
}

// === 内部方法 ===

// isDuplicate 检查是否有高度相似的现有记忆
// 如果找到已失效的同内容记忆，自动重新激活而非插入新记录
func (s *Store) isDuplicate(ctx context.Context, tx *sql.Tx, entry MemoryEntry) bool {
	userID := entry.UserID
	if userID == "" {
		userID = "default"
	}

	// 检查是否有有效的同内容记忆
	var activeCount int
	tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories WHERE user_id = ? AND type = ? AND content = ? AND (valid_to IS NULL OR valid_to > datetime('now'))`,
		userID, entry.Type, entry.Content,
	).Scan(&activeCount)

	if activeCount > 0 {
		return true // 已有有效记忆，跳过
	}

	// 检查是否有已失效的同内容记忆 → 重新激活
	var expiredID int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM memories WHERE user_id = ? AND type = ? AND content = ? AND valid_to IS NOT NULL AND valid_to <= datetime('now') LIMIT 1`,
		userID, entry.Type, entry.Content,
	).Scan(&expiredID)

	if err == nil && expiredID > 0 {
		// 重新激活：清除 valid_to，更新 updated_at 和 importance
		tx.ExecContext(ctx,
			`UPDATE memories SET valid_to = NULL, importance = ?, updated_at = datetime('now') WHERE id = ?`,
			entry.Importance, expiredID,
		)
		logger.Info("重新激活已失效记忆",
			zap.Int64("id", expiredID),
			zap.String("content", entry.Content),
		)
		return true // 已处理（通过激活而非插入）
	}

	return false // 无重复，允许插入
}

// scoreMemories 对记忆列表进行复合评分并排序
func scoreMemories(memories []MemoryEntry, query string) []MemoryEntry {
	type scored struct {
		entry MemoryEntry
		score float64
	}

	queryWords := extractKeywords(query)
	now := time.Now()

	var results []scored
	for _, m := range memories {
		// 1. 关键词匹配 (0.4 权重)
		keywordScore := 0.0
		if len(queryWords) > 0 {
			contentLower := strings.ToLower(m.Content)
			matched := 0
			for _, w := range queryWords {
				if strings.Contains(contentLower, w) {
					matched++
				}
			}
			keywordScore = float64(matched) / float64(len(queryWords))
		}

		// 2. 重要性 (0.3 权重)
		importanceScore := m.Importance

		// 3. 时效性 (0.2 权重) — 越近越高，指数衰减
		daysSinceCreated := now.Sub(m.CreatedAt).Hours() / 24
		recencyScore := math.Exp(-daysSinceCreated / 30) // 30 天半衰期

		// 4. 类型加权 (0.1 权重) — profile 和 insight 始终相关
		typeBoost := 0.0
		switch m.Type {
		case TypeProfile:
			typeBoost = 1.0
		case TypeInsight:
			typeBoost = 0.8
		case TypeFact:
			typeBoost = 0.5
		case TypePreference:
			typeBoost = 0.6
		}

		// 无查询时（空字符串），退化为 importance + recency + type
		var total float64
		if len(queryWords) == 0 {
			total = importanceScore*0.4 + recencyScore*0.3 + typeBoost*0.3
		} else {
			total = keywordScore*0.4 + importanceScore*0.3 + recencyScore*0.2 + typeBoost*0.1
		}

		results = append(results, scored{entry: m, score: total})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	out := make([]MemoryEntry, len(results))
	for i, r := range results {
		out[i] = r.entry
	}
	return out
}

// extractKeywords 从查询中提取关键词（简单分词）
func extractKeywords(query string) []string {
	if query == "" {
		return nil
	}

	query = strings.ToLower(query)
	// 简单按空格和标点分词
	replacer := strings.NewReplacer(
		"，", " ", "。", " ", "？", " ", "！", " ",
		"、", " ", "：", " ", "；", " ",
		",", " ", ".", " ", "?", " ", "!", " ",
	)
	query = replacer.Replace(query)

	words := strings.Fields(query)
	// 过滤过短的词
	var keywords []string
	for _, w := range words {
		if len(w) >= 2 { // UTF-8 中文至少 3 字节，但保留2字节以上
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// updateAccessCounts 更新被召回记忆的访问计数
func (s *Store) updateAccessCounts(ctx context.Context, memories []MemoryEntry) {
	for _, m := range memories {
		s.db.ExecContext(ctx,
			`UPDATE memories SET access_count = access_count + 1, last_accessed_at = datetime('now') WHERE id = ?`,
			m.ID,
		)
	}
}
