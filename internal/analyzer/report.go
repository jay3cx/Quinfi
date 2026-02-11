// Package analyzer 提供基金分析报告模型定义
package analyzer

// HoldingAnalysis 持仓分析
type HoldingAnalysis struct {
	IndustryDistribution []IndustryWeight `json:"industry_distribution"` // 行业分布
	Concentration        Concentration    `json:"concentration"`         // 持仓集中度
	TopHoldings          []StockAnalysis  `json:"top_holdings"`          // 重仓股分析
	AnalysisText         string           `json:"analysis_text"`         // 分析文本
}

// IndustryWeight 行业权重
type IndustryWeight struct {
	Industry string  `json:"industry"` // 行业名称
	Weight   float64 `json:"weight"`   // 权重（%）
}

// Concentration 持仓集中度
type Concentration struct {
	Top3Ratio  float64 `json:"top3_ratio"`  // 前3大持仓占比（%）
	Top5Ratio  float64 `json:"top5_ratio"`  // 前5大持仓占比（%）
	Top10Ratio float64 `json:"top10_ratio"` // 前10大持仓占比（%）
	Level      string  `json:"level"`       // 集中度等级：高/中/低
}

// StockAnalysis 单只股票分析
type StockAnalysis struct {
	StockCode string  `json:"stock_code"` // 股票代码
	StockName string  `json:"stock_name"` // 股票名称
	Ratio     float64 `json:"ratio"`      // 持仓占比（%）
	Comment   string  `json:"comment"`    // 简评
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	RiskLevel     string   `json:"risk_level"`     // 风险等级：低/中/高
	Volatility    string   `json:"volatility"`     // 波动率评估
	MaxDrawdown   string   `json:"max_drawdown"`   // 最大回撤估算
	RiskWarnings  []string `json:"risk_warnings"`  // 风险提示
	AssessmentText string  `json:"assessment_text"` // 评估文本
}

// Recommendation 投资建议
type Recommendation struct {
	Action     string   `json:"action"`      // 建议操作：买入/持有/观望/减持
	Confidence string   `json:"confidence"`  // 置信度：高/中/低
	Reasons    []string `json:"reasons"`     // 理由
	Caveats    []string `json:"caveats"`     // 注意事项
	Text       string   `json:"text"`        // 建议文本
}

// RebalanceResult 调仓检测结果
type RebalanceResult struct {
	FundCode    string          `json:"fund_code"`    // 基金代码
	ReportDate  string          `json:"report_date"`  // 报告日期
	Changes     []RebalanceInfo `json:"changes"`      // 调仓变动
	Summary     string          `json:"summary"`      // 调仓摘要
	GeneratedAt string          `json:"generated_at"` // 生成时间
}

// RebalanceInfo 单条调仓信息
type RebalanceInfo struct {
	StockCode   string  `json:"stock_code"`   // 股票代码
	StockName   string  `json:"stock_name"`   // 股票名称
	Action      string  `json:"action"`       // 操作：新增/增持/减持/清仓
	PrevRatio   float64 `json:"prev_ratio"`   // 上期占比（%）
	CurrRatio   float64 `json:"curr_ratio"`   // 本期占比（%）
	ChangeRatio float64 `json:"change_ratio"` // 变动幅度（%）
	Comment     string  `json:"comment"`      // 简评
}

// RebalanceAction 调仓操作类型
type RebalanceAction string

const (
	RebalanceActionNew      RebalanceAction = "新增"
	RebalanceActionIncrease RebalanceAction = "增持"
	RebalanceActionDecrease RebalanceAction = "减持"
	RebalanceActionClear    RebalanceAction = "清仓"
)
