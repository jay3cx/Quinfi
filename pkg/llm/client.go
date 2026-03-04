// Package llm 提供 LLM 客户端接口和实现
package llm

import "context"

// ModelID 模型标识
type ModelID string

const (
	// Claude 模型
	ModelClaudeSonnet46 ModelID = "claude-sonnet-4-6"         // Sonnet 4.6 (Thinking)
	ModelClaudeOpus46   ModelID = "claude-opus-4-6-thinking"  // Opus 4.6 (Thinking)

	// Gemini 模型
	ModelGemini3Flash          ModelID = "gemini-3-flash-preview"
	ModelGemini25Flash         ModelID = "gemini-2.5-flash"
	ModelGemini3ProHigh        ModelID = "gemini-3-pro-high"
	ModelGemini25FlashThinking ModelID = "gemini-2.5-flash-thinking"
	ModelGemini3ProPreview     ModelID = "gemini-3-pro-preview"

	// OpenAI 模型（通过 OpenAI 兼容协议访问）
	ModelGPT4o     ModelID = "gpt-4o"
	ModelGPT4oMini ModelID = "gpt-4o-mini"
	ModelO1        ModelID = "o1"
	ModelO1Mini    ModelID = "o1-mini"
)

// Role 消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentPart 多模态消息内容部分（用于 Vision 等场景）
type ContentPart struct {
	Type     string    `json:"type"`                // "text" 或 "image_url"
	Text     string    `json:"text,omitempty"`      // type=text 时的文本内容
	ImageURL *ImageURL `json:"image_url,omitempty"` // type=image_url 时的图片
}

// ImageURL 图片 URL（支持 base64 data URI 和 HTTP URL）
type ImageURL struct {
	URL string `json:"url"` // "data:image/png;base64,..." 或 "https://..."
}

// Message 对话消息
type Message struct {
	Role         Role          `json:"role"`
	Content      string        `json:"content"`
	MultiContent []ContentPart `json:"multi_content,omitempty"` // 多模态内容（优先于 Content）
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`    // assistant 消息可包含工具调用
	ToolCallID   string        `json:"tool_call_id,omitempty"`  // tool 角色消息的调用 ID
}

// NewVisionMessage 创建包含文本和图片的多模态消息
func NewVisionMessage(role Role, text string, imageBase64 ...string) Message {
	parts := []ContentPart{{Type: "text", Text: text}}
	for _, img := range imageBase64 {
		url := img
		// 自动添加 data URI 前缀
		if len(img) > 0 && img[0] != 'h' && img[0] != 'd' {
			url = "data:image/png;base64," + img
		}
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: url},
		})
	}
	return Message{Role: role, MultiContent: parts}
}

const (
	RoleTool Role = "tool" // 工具结果角色
)

// ToolDef 工具定义（供 LLM 使用 function calling）
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema 格式
}

// ToolCall LLM 发起的工具调用
type ToolCall struct {
	ID        string `json:"id"`        // 调用标识
	Name      string `json:"name"`      // 工具名称
	Arguments string `json:"arguments"` // JSON 格式的参数
}

// ToolMessage 工具执行结果消息
type ToolMessage struct {
	ToolCallID string `json:"tool_call_id"` // 对应的 ToolCall.ID
	Content    string `json:"content"`      // 执行结果
}

// ChatRequest 对话请求
type ChatRequest struct {
	Model        ModelID       `json:"model"`
	Messages     []Message     `json:"messages"`
	MaxTokens    int           `json:"max_tokens,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
	Stream       bool          `json:"stream,omitempty"`
	Tools        []ToolDef     `json:"tools,omitempty"`         // 工具定义列表
	ToolMessages []ToolMessage `json:"tool_messages,omitempty"` // 工具执行结果
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"` // LLM 请求的工具调用
}

// HasToolCalls 检查响应是否包含工具调用
func (r *ChatResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Content   string
	ToolCalls []ToolCall // 流式工具调用（流结束时汇总完整的工具调用列表）
	Done      bool
	Error     error
}

// Client LLM 客户端接口
type Client interface {
	// Chat 同步对话调用
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// ChatStream 流式对话调用，返回响应块 channel
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
}

// TaskType 任务类型
type TaskType string

const (
	TaskLight  TaskType = "light"  // 轻量任务：RSS摘要、情绪识别
	TaskDaily  TaskType = "daily"  // 日常分析：基金分析、笔记生成
	TaskDeep   TaskType = "deep"   // 深度推理：多空辩论、复杂研报
)

// GetDefaultModel 根据任务类型返回默认模型
func GetDefaultModel(task TaskType) ModelID {
	switch task {
	case TaskLight:
		return ModelClaudeSonnet46
	case TaskDaily:
		return ModelClaudeSonnet46
	case TaskDeep:
		return ModelClaudeOpus46
	default:
		return ModelClaudeSonnet46
	}
}
