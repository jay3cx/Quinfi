// Package agent 提供 Agent 抽象层
package agent

import "context"

// Agent AI Agent 接口
type Agent interface {
	// Run 同步执行，返回完整响应
	Run(ctx context.Context, input *AgentInput) (*AgentResponse, error)

	// RunStream 流式执行，返回响应块 channel
	RunStream(ctx context.Context, input *AgentInput) (<-chan StreamChunk, error)

	// Name 返回 Agent 名称
	Name() string
}

// AgentInput Agent 输入
type AgentInput struct {
	Query    string            `json:"query"`    // 用户查询
	Context  *AgentContext     `json:"context"`  // 会话上下文
	Metadata map[string]string `json:"metadata"` // 扩展元数据
	Images   []string          `json:"images"`   // 附带的图片（base64）
}

// AgentContext Agent 上下文
type AgentContext struct {
	SessionID string            `json:"session_id"` // 会话标识
	UserID    string            `json:"user_id"`    // 用户标识
	History   []Message         `json:"history"`    // 历史消息
	Variables map[string]string `json:"variables"`  // 上下文变量
}

// Message 对话消息
type Message struct {
	Role     string `json:"role"`               // system/user/assistant/tool_calls
	Content  string `json:"content"`            // 消息内容
	Metadata string `json:"metadata,omitempty"` // 元数据（JSON，工具调用记录等）
}

// AgentResponse Agent 响应
type AgentResponse struct {
	Content     string            `json:"content"`      // 响应内容
	Metadata    map[string]string `json:"metadata"`     // 响应元数据
	TokensUsed  TokenUsage        `json:"tokens_used"`  // Token 使用量
	GeneratedAt string            `json:"generated_at"` // 生成时间
}

// TokenUsage Token 使用量
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ChunkType 流式响应块类型
type ChunkType string

const (
	ChunkText       ChunkType = "text"        // 文本内容
	ChunkToolStart  ChunkType = "tool_start"  // 工具调用开始
	ChunkToolResult ChunkType = "tool_result" // 工具调用结果
	ChunkThinking   ChunkType = "thinking"    // 思考中间状态
)

// StreamChunk 流式响应块
type StreamChunk struct {
	Type     ChunkType // 块类型，默认为 text
	Content  string    // 内容片段
	ToolName string    // 工具名称（Type 为 tool_* 时有值）
	Done     bool      // 是否完成
	Error    error     // 错误信息
}

// NewAgentContext 创建新的 Agent 上下文
func NewAgentContext(sessionID, userID string) *AgentContext {
	return &AgentContext{
		SessionID: sessionID,
		UserID:    userID,
		History:   make([]Message, 0),
		Variables: make(map[string]string),
	}
}

// AddMessage 添加消息到历史
func (c *AgentContext) AddMessage(role, content string) {
	c.History = append(c.History, Message{Role: role, Content: content})
}

// SetVariable 设置上下文变量
func (c *AgentContext) SetVariable(key, value string) {
	c.Variables[key] = value
}

// GetVariable 获取上下文变量
func (c *AgentContext) GetVariable(key string) string {
	return c.Variables[key]
}
