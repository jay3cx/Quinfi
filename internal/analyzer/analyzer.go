// Package analyzer 提供基金分析服务
package analyzer

import "context"

// FundAnalyzer 基金分析器接口
type FundAnalyzer interface {
	// Analyze 综合分析，返回完整报告
	Analyze(ctx context.Context, code string, forceRefresh bool) (*AnalysisReport, error)

	// AnalyzeHoldings 持仓分析
	AnalyzeHoldings(ctx context.Context, code string) (*HoldingAnalysis, error)

	// DetectRebalance 调仓检测
	DetectRebalance(ctx context.Context, code string) (*RebalanceResult, error)
}

// AnalysisReport 综合分析报告
type AnalysisReport struct {
	FundCode       string           `json:"fund_code"`       // 基金代码
	FundName       string           `json:"fund_name"`       // 基金名称
	Summary        string           `json:"summary"`         // 基金摘要
	HoldingAnalysis *HoldingAnalysis `json:"holding_analysis"` // 持仓分析
	RiskAssessment *RiskAssessment  `json:"risk_assessment"`  // 风险评估
	Recommendation *Recommendation  `json:"recommendation"`   // 投资建议
	GeneratedAt    string           `json:"generated_at"`     // 生成时间
}

// AnalyzeOptions 分析选项
type AnalyzeOptions struct {
	ForceRefresh bool // 强制刷新，忽略缓存
}
