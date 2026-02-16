// Package agent 提供 Agent 工具系统
package agent

import (
	"context"
	"fmt"

	"github.com/jay3cx/Quinfi/pkg/llm"
)

// Tool Agent 可用工具接口
type Tool interface {
	// Name 工具名称（唯一标识，如 "get_fund_info"）
	Name() string

	// Description 工具描述（供 LLM 理解用途）
	Description() string

	// Parameters 工具参数定义（JSON Schema 格式）
	Parameters() map[string]any

	// Execute 执行工具，返回文本结果
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ToolRegistry 工具注册中心
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry 创建工具注册中心
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get 获取工具
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List 列出所有工具
func (r *ToolRegistry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ToLLMTools 转换为 LLM 工具定义列表
func (r *ToolRegistry) ToLLMTools() []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// Execute 执行指定工具
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return tool.Execute(ctx, args)
}

// getStringArg 从参数 map 中安全地获取字符串参数
func getStringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

// getIntArg 从参数 map 中安全地获取整数参数
func getIntArg(args map[string]any, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}
