// Package llm 提供测试用 Mock 客户端
package llm

import "context"

// MockClient 测试用 LLM 客户端
// 通过注入自定义函数控制返回行为
type MockClient struct {
	ChatFunc       func(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStreamFunc func(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)
	calls          []ChatRequest // 记录所有调用
}

// NewMockClient 创建 Mock 客户端
func NewMockClient() *MockClient {
	return &MockClient{
		calls: make([]ChatRequest, 0),
	}
}

// Chat 同步调用（委托给 ChatFunc）
func (m *MockClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	m.calls = append(m.calls, *req)
	if m.ChatFunc != nil {
		return m.ChatFunc(ctx, req)
	}
	return &ChatResponse{
		Content: "Mock response",
		Model:   string(req.Model),
	}, nil
}

// ChatStream 流式调用（委托给 ChatStreamFunc）
func (m *MockClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	m.calls = append(m.calls, *req)
	if m.ChatStreamFunc != nil {
		return m.ChatStreamFunc(ctx, req)
	}

	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Content: "Mock stream"}
		ch <- StreamChunk{Done: true}
	}()
	return ch, nil
}

// Calls 返回所有调用记录
func (m *MockClient) Calls() []ChatRequest {
	return m.calls
}

// LastCall 返回最后一次调用
func (m *MockClient) LastCall() *ChatRequest {
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

// CallCount 返回调用次数
func (m *MockClient) CallCount() int {
	return len(m.calls)
}

// Reset 重置调用记录
func (m *MockClient) Reset() {
	m.calls = m.calls[:0]
}

// WithResponse 设置固定文本响应（同时设置 Chat 和 ChatStream）
func (m *MockClient) WithResponse(content string) *MockClient {
	m.ChatFunc = func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
		return &ChatResponse{
			Content: content,
			Model:   string(req.Model),
		}, nil
	}
	m.ChatStreamFunc = func(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
		ch := make(chan StreamChunk, 2)
		go func() {
			defer close(ch)
			ch <- StreamChunk{Content: content}
			ch <- StreamChunk{Done: true}
		}()
		return ch, nil
	}
	return m
}

// WithToolCalls 设置工具调用响应
func (m *MockClient) WithToolCalls(toolCalls []ToolCall) *MockClient {
	m.ChatFunc = func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
		return &ChatResponse{
			ToolCalls: toolCalls,
			Model:     string(req.Model),
		}, nil
	}
	return m
}

// WithSequence 设置按顺序返回的响应序列
// 每次调用返回序列中的下一个，超出后返回最后一个
func (m *MockClient) WithSequence(responses []*ChatResponse) *MockClient {
	idx := 0
	m.ChatFunc = func(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
		resp := responses[idx]
		if idx < len(responses)-1 {
			idx++
		}
		return resp, nil
	}
	return m
}

// 确保 MockClient 实现 Client 接口
var _ Client = (*MockClient)(nil)
