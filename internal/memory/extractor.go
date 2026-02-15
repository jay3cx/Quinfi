// Package memory 提供 LLM 驱动的记忆提取
package memory

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

const extractionPrompt = `你是一个记忆提取器。从以下用户与 AI 助手的对话中，提取值得长期记忆的用户信息。

## 提取规则
1. 只提取关于**用户**的新信息，不提取通用知识或 AI 的分析内容
2. 每条记忆必须是一个独立的、可在未来对话中使用的事实
3. 如果对话中没有值得记忆的新信息，返回空数组 []
4. 重要性(importance)评分标准：
   - 0.9-1.0: 明确的持仓变动、重大投资决策
   - 0.7-0.8: 风险偏好、投资风格等画像信息
   - 0.5-0.6: 关注的板块、偏好的分析方式
   - 0.3-0.4: 一般性偏好

## 记忆类型
- profile: 用户画像（风险偏好、投资经验、资金规模等）
- fact: 投资事实（持有某基金、某日买入/卖出、亏损/盈利等）
- preference: 偏好习惯（关注的板块、喜欢的分析方式等）
- insight: 行为模式观察（追涨杀跌倾向、定投习惯等）

## 输出格式
返回 JSON 数组，每个元素包含 type, content, importance：
[
  {"type": "fact", "content": "用户持有 005827 易方达蓝筹精选", "importance": 0.9},
  {"type": "preference", "content": "用户关注新能源板块", "importance": 0.6}
]

如果没有值得提取的信息，返回：[]

只输出 JSON 数组，不要输出其他内容。`

// Extractor LLM 驱动的记忆提取器
type Extractor struct {
	client llm.Client
	model  llm.ModelID
}

// NewExtractor 创建记忆提取器（默认使用 Gemini 3 Pro）
func NewExtractor(client llm.Client) *Extractor {
	return &Extractor{
		client: client,
		model:  llm.ModelGemini3ProHigh,
	}
}

// NewExtractorWithModel 创建指定模型的记忆提取器
func NewExtractorWithModel(client llm.Client, model llm.ModelID) *Extractor {
	return &Extractor{
		client: client,
		model:  model,
	}
}

// Extract 从对话中提取值得长期记忆的信息
func (e *Extractor) Extract(ctx context.Context, userMessage, assistantResponse, sessionID string) []MemoryEntry {
	conversation := "用户: " + userMessage + "\nAI助手: " + assistantResponse

	resp, err := e.client.Chat(ctx, &llm.ChatRequest{
		Model: e.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: extractionPrompt},
			{Role: llm.RoleUser, Content: conversation},
		},
		MaxTokens:   500,
		Temperature: 0.1, // 低温度确保输出稳定
	})
	if err != nil {
		logger.Warn("记忆提取 LLM 调用失败", zap.Error(err))
		return nil
	}

	entries := parseExtractedMemories(resp.Content, sessionID)
	if len(entries) > 0 {
		logger.Info("提取到新记忆",
			zap.Int("count", len(entries)),
			zap.String("session", sessionID),
		)
	}
	return entries
}

// parseExtractedMemories 解析 LLM 返回的记忆 JSON
func parseExtractedMemories(content, sessionID string) []MemoryEntry {
	content = strings.TrimSpace(content)

	// 移除 markdown 代码块标记
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// 尝试找到 JSON 数组
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var raw []struct {
		Type       string  `json:"type"`
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
	}

	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		// 空数组 "[]" 也是合法的
		if content == "[]" {
			return nil
		}
		logger.Warn("记忆提取 JSON 解析失败",
			zap.String("content", content[:min(len(content), 200)]),
			zap.Error(err),
		)
		return nil
	}

	var entries []MemoryEntry
	for _, r := range raw {
		if r.Content == "" {
			continue
		}

		// 验证类型
		memType := MemoryType(r.Type)
		switch memType {
		case TypeProfile, TypeFact, TypePreference, TypeInsight:
			// valid
		default:
			memType = TypePreference // 未知类型降级为 preference
		}

		// 规范化重要性
		importance := r.Importance
		if importance <= 0 {
			importance = 0.5
		}
		if importance > 1.0 {
			importance = 1.0
		}

		entries = append(entries, MemoryEntry{
			UserID:        "default",
			Type:          memType,
			Content:       r.Content,
			SourceSession: sessionID,
			Importance:    importance,
		})
	}

	return entries
}
