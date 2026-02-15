package orchestrator

import (
	"github.com/jay3cx/fundmind/internal/analyzer"
	"github.com/jay3cx/fundmind/internal/debate"
)

// DeepReport 深度分析报告（汇总多 Agent 结果）
type DeepReport struct {
	FundCode      string                   `json:"fund_code"`
	FundName      string                   `json:"fund_name"`
	FundAnalysis  *analyzer.AnalysisReport `json:"fund_analysis"`
	ManagerReport *analyzer.ManagerReport  `json:"manager_report,omitempty"`
	MacroReport   *analyzer.MacroReport    `json:"macro_report,omitempty"`
	DebateResult  *debate.DebateResult     `json:"debate_result,omitempty"`
	GeneratedAt   string                   `json:"generated_at"`
}
