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

// ManagerAnalyzer 基金经理分析器接口
type ManagerAnalyzer interface {
	// AnalyzeManager 分析基金经理能力
	AnalyzeManager(ctx context.Context, code string) (*ManagerReport, error)
}

// ManagerReport 基金经理分析报告
type ManagerReport struct {
	ManagerName     string   `json:"manager_name"`
	Years           float64  `json:"years"`
	Style           string   `json:"style"`            // 投资风格
	Strengths       []string `json:"strengths"`         // 优势
	Weaknesses      []string `json:"weaknesses"`        // 劣势
	BestPerformance string   `json:"best_performance"`  // 最佳业绩
	AnalysisText    string   `json:"analysis_text"`     // 综合分析
}

// MacroAnalyzer 宏观环境分析器接口
type MacroAnalyzer interface {
	// AnalyzeMacro 基于新闻分析宏观环境对基金的影响
	AnalyzeMacro(ctx context.Context, fundName string, news []string) (*MacroReport, error)
}

// MacroReport 宏观研判报告
type MacroReport struct {
	MarketSentiment string   `json:"market_sentiment"` // 市场情绪：乐观/中性/悲观
	KeyEvents       []string `json:"key_events"`       // 关键宏观事件
	Impact          string   `json:"impact"`           // 对目标基金的影响分析
	RiskFactors     []string `json:"risk_factors"`     // 宏观风险因素
	AnalysisText    string   `json:"analysis_text"`    // 综合分析
}
