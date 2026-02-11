// Package debate 提供工具适配器
package debate

import "context"

// ToolAdapter 将 Orchestrator 适配为 agent.debateRunner 接口
// 返回格式化的 Markdown 文本，供 FundAgent 的工具系统使用
type ToolAdapter struct {
	orch *Orchestrator
}

// NewToolAdapter 创建工具适配器
func NewToolAdapter(orch *Orchestrator) *ToolAdapter {
	return &ToolAdapter{orch: orch}
}

// RunDebate 执行辩论并返回 Markdown 格式的结果文本
func (a *ToolAdapter) RunDebate(ctx context.Context, fundCode string) (string, error) {
	result, err := a.orch.RunDebate(ctx, fundCode)
	if err != nil {
		return "", err
	}
	return result.FormatAsMarkdown(), nil
}
