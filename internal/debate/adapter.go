// Package debate 提供工具适配器
package debate

import (
	"context"
	"encoding/json"
)

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
// 可选 onPhase 回调，每个阶段完成后推送 JSON 格式的阶段数据
func (a *ToolAdapter) RunDebate(ctx context.Context, fundCode string, onPhase ...func(phaseJSON string)) (string, error) {
	var debateCb DebateProgressFunc
	if len(onPhase) > 0 && onPhase[0] != nil {
		cb := onPhase[0]
		debateCb = func(phase Phase, arg *Argument, verdict *Verdict) {
			data := map[string]any{
				"type":  "debate_phase",
				"phase": string(phase),
			}
			if arg != nil {
				data["argument"] = arg
			}
			if verdict != nil {
				data["verdict"] = verdict
			}
			metaJSON, _ := json.Marshal(data)
			cb(string(metaJSON))
		}
	}

	result, err := a.orch.RunDebate(ctx, fundCode, debateCb)
	if err != nil {
		return "", err
	}

	// 辩论完成后推送系统置信度门控结果
	if len(onPhase) > 0 && onPhase[0] != nil && result.SystemConfidence > 0 {
		data := map[string]any{
			"type":              "debate_phase",
			"phase":             "confidence_gate",
			"system_confidence": result.SystemConfidence,
			"decision_gate":     string(result.DecisionGate),
		}
		metaJSON, _ := json.Marshal(data)
		onPhase[0](string(metaJSON))
	}

	return result.FormatAsMarkdown(), nil
}
