package agent

import (
	"context"
	"testing"

	"github.com/jay3cx/fundmind/pkg/llm"
)

// === 辅助函数 ===

func newTestAgent(mock *llm.MockClient) *FundAgent {
	tools := NewToolRegistry()
	// 不注册真实工具，避免外部依赖
	return NewFundAgent(mock, tools)
}

func newTestContext() *AgentContext {
	return NewAgentContext("test-session", "test-user")
}

// === 测试用例 ===

func TestFundAgent_Name(t *testing.T) {
	mock := llm.NewMockClient()
	agent := newTestAgent(mock)

	if agent.Name() != "小基" {
		t.Errorf("expected '小基', got '%s'", agent.Name())
	}
}

func TestFundAgent_Run_SimpleResponse(t *testing.T) {
	// Mock LLM 直接返回文本（无工具调用）
	mock := llm.NewMockClient().WithResponse("这是一个测试回复")

	agent := newTestAgent(mock)

	input := &AgentInput{
		Query:   "你好",
		Context: newTestContext(),
	}

	resp, err := agent.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "这是一个测试回复" {
		t.Errorf("expected '这是一个测试回复', got '%s'", resp.Content)
	}

	// 验证 LLM 被调用了 1 次
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", mock.CallCount())
	}

	// 验证系统提示词被注入
	lastCall := mock.LastCall()
	if len(lastCall.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(lastCall.Messages))
	}
	if lastCall.Messages[0].Role != llm.RoleSystem {
		t.Errorf("expected first message role 'system', got '%s'", lastCall.Messages[0].Role)
	}
}

func TestFundAgent_Run_WithToolCall(t *testing.T) {
	// 模拟 ReAct 循环：第一次返回工具调用，第二次返回文本
	callCount := 0
	mock := llm.NewMockClient()
	mock.ChatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			// 第一轮：返回工具调用
			return &llm.ChatResponse{
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call_001",
						Name:      "unknown_tool",
						Arguments: `{"code": "005827"}`,
					},
				},
				Model: "test-model",
			}, nil
		}
		// 第二轮：返回最终文本
		return &llm.ChatResponse{
			Content: "基于查询结果，这只基金表现良好。",
			Model:   "test-model",
		}, nil
	}

	agent := newTestAgent(mock)

	input := &AgentInput{
		Query:   "分析 005827",
		Context: newTestContext(),
	}

	resp, err := agent.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "基于查询结果，这只基金表现良好。" {
		t.Errorf("unexpected content: %s", resp.Content)
	}

	// 验证 ReAct 循环执行了 2 轮
	if callCount != 2 {
		t.Errorf("expected 2 rounds, got %d", callCount)
	}

	if resp.Metadata["rounds"] != "2" {
		t.Errorf("expected rounds=2, got %s", resp.Metadata["rounds"])
	}
}

func TestFundAgent_Run_WithHistory(t *testing.T) {
	mock := llm.NewMockClient().WithResponse("好的，继续分析")

	agent := newTestAgent(mock)

	ctx := newTestContext()
	ctx.AddMessage("user", "之前的问题")
	ctx.AddMessage("assistant", "之前的回答")

	input := &AgentInput{
		Query:   "继续",
		Context: ctx,
	}

	_, err := agent.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证历史消息被包含在请求中
	lastCall := mock.LastCall()
	// system + 2 history + 1 current = 4
	if len(lastCall.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(lastCall.Messages))
	}
}

func TestFundAgent_Run_ToolsPassedToLLM(t *testing.T) {
	mock := llm.NewMockClient().WithResponse("ok")

	tools := NewToolRegistry()
	tools.Register(&dummyTool{name: "test_tool"})

	agent := NewFundAgent(mock, tools)

	input := &AgentInput{
		Query:   "测试",
		Context: newTestContext(),
	}

	_, err := agent.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证工具定义被传递给 LLM
	lastCall := mock.LastCall()
	if len(lastCall.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(lastCall.Tools))
	}
	if lastCall.Tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got '%s'", lastCall.Tools[0].Name)
	}
}

func TestFundAgent_RunStream_Basic(t *testing.T) {
	mock := llm.NewMockClient().WithResponse("流式测试内容")

	agent := newTestAgent(mock)

	input := &AgentInput{
		Query:   "你好",
		Context: newTestContext(),
	}

	chunks, err := agent.RunStream(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var gotContent string
	var gotDone bool
	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("unexpected chunk error: %v", chunk.Error)
		}
		if chunk.Done {
			gotDone = true
			break
		}
		gotContent += chunk.Content
	}

	if !gotDone {
		t.Error("expected done signal")
	}
	if gotContent != "流式测试内容" {
		t.Errorf("expected '流式测试内容', got '%s'", gotContent)
	}
}

func TestToolRegistry_RegisterAndExecute(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&dummyTool{name: "greet", result: "Hello!"})

	// List
	tools := registry.List()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	// Get
	tool, ok := registry.Get("greet")
	if !ok {
		t.Fatal("expected to find tool 'greet'")
	}
	if tool.Name() != "greet" {
		t.Errorf("expected name 'greet', got '%s'", tool.Name())
	}

	// Execute
	result, err := registry.Execute(context.Background(), "greet", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", result)
	}

	// Unknown tool
	_, err = registry.Execute(context.Background(), "unknown", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestToolRegistry_ToLLMTools(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(&dummyTool{name: "tool_a"})
	registry.Register(&dummyTool{name: "tool_b"})

	defs := registry.ToLLMTools()
	if len(defs) != 2 {
		t.Errorf("expected 2 tool defs, got %d", len(defs))
	}
}

// === Dummy Tool for testing ===

type dummyTool struct {
	name   string
	result string
}

func (d *dummyTool) Name() string        { return d.name }
func (d *dummyTool) Description() string  { return "A dummy tool for testing" }
func (d *dummyTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (d *dummyTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if d.result != "" {
		return d.result, nil
	}
	return "dummy result", nil
}
