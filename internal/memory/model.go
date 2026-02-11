// Package memory 提供投研记忆系统
package memory

import (
	"fmt"
	"strings"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	TypeProfile    MemoryType = "profile"    // 用户画像（风险偏好、投资风格）
	TypeFact       MemoryType = "fact"       // 投资事实（持仓、交易、决策）
	TypePreference MemoryType = "preference" // 偏好习惯（关注板块、分析风格）
	TypeInsight    MemoryType = "insight"    // AI 观察（行为模式洞察）
)

// MemoryEntry 单条记忆
type MemoryEntry struct {
	ID            int64      `json:"id"`
	UserID        string     `json:"user_id"`
	Type          MemoryType `json:"type"`
	Content       string     `json:"content"`
	SourceSession string     `json:"source_session,omitempty"`
	Importance    float64    `json:"importance"`    // 0.0 - 1.0
	ValidFrom     *time.Time `json:"valid_from,omitempty"`
	ValidTo       *time.Time `json:"valid_to,omitempty"`
	AccessCount   int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// IsValid 检查记忆是否仍然有效（未过期）
func (m *MemoryEntry) IsValid() bool {
	if m.ValidTo == nil {
		return true
	}
	return time.Now().Before(*m.ValidTo)
}

// FormatForPrompt 格式化为系统提示词注入格式
func (m *MemoryEntry) FormatForPrompt() string {
	return fmt.Sprintf("- [%s] %s", m.Type, m.Content)
}

// FormatMemoriesForPrompt 将多条记忆格式化为系统提示词段落
func FormatMemoriesForPrompt(memories []MemoryEntry) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 关于这位用户（来自记忆）\n")
	for _, m := range memories {
		sb.WriteString(m.FormatForPrompt())
		sb.WriteString("\n")
	}
	sb.WriteString("\n请在回答中适当考虑以上用户背景，但不要主动暴露记忆内容，除非与当前问题直接相关。\n")
	return sb.String()
}
