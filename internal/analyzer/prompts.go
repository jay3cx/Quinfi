// Package analyzer 提供 LLM Prompt 模板
package analyzer

import (
	"bytes"
	"text/template"

	"github.com/jay3cx/fundmind/internal/datasource"
)

// PromptTemplate Prompt 模板类型
type PromptTemplate string

const (
	// PromptAnalyze 综合分析 Prompt
	PromptAnalyze PromptTemplate = `你是一位专业的基金分析师。请根据以下基金数据，生成一份结构化的分析报告。

## 基金信息
- 基金代码：{{.Fund.Code}}
- 基金名称：{{.Fund.Name}}
- 基金类型：{{.Fund.Type}}
- 基金规模：{{.Fund.Scale}}亿元
- 成立日期：{{.Fund.EstablishAt.Format "2006-01-02"}}
- 基金公司：{{.Fund.Company}}
{{if .Fund.Manager}}
## 基金经理
- 姓名：{{.Fund.Manager.Name}}
- 任职年限：{{.Fund.Manager.Years}}年
- 管理规模：{{.Fund.Manager.TotalScale}}亿元
{{end}}

## 前十大持仓
{{range .Holdings}}
- {{.StockName}}（{{.StockCode}}）：{{.Ratio}}%
{{end}}

## 近期净值走势
{{range .NAVList}}
- {{.Date}}：单位净值 {{.UnitNAV}}，日涨幅 {{.DailyReturn}}%
{{end}}

请以 JSON 格式输出分析报告，包含以下字段：
{
  "summary": "基金整体评价（2-3句话）",
  "holding_analysis": {
    "industry_distribution": [{"industry": "行业名称", "weight": 占比数值}],
    "concentration": {"top3_ratio": 数值, "top5_ratio": 数值, "top10_ratio": 数值, "level": "高/中/低"},
    "top_holdings": [{"stock_code": "代码", "stock_name": "名称", "ratio": 占比, "comment": "简评"}],
    "analysis_text": "持仓分析文本"
  },
  "risk_assessment": {
    "risk_level": "低/中/高",
    "volatility": "波动率评估",
    "max_drawdown": "最大回撤估算",
    "risk_warnings": ["风险提示1", "风险提示2"],
    "assessment_text": "风险评估文本"
  },
  "recommendation": {
    "action": "买入/持有/观望/减持",
    "confidence": "高/中/低",
    "reasons": ["理由1", "理由2"],
    "caveats": ["注意事项1"],
    "text": "投资建议文本"
  }
}

只输出 JSON，不要输出其他内容。`

	// PromptHoldings 持仓分析 Prompt
	PromptHoldings PromptTemplate = `你是一位专业的基金分析师。请分析以下基金的持仓情况。

## 基金信息
- 基金代码：{{.Fund.Code}}
- 基金名称：{{.Fund.Name}}
- 基金类型：{{.Fund.Type}}

## 前十大持仓
{{range .Holdings}}
- {{.StockName}}（{{.StockCode}}）：{{.Ratio}}%，持仓市值 {{.MarketValue}}万元
{{end}}

请以 JSON 格式输出持仓分析，包含以下字段：
{
  "industry_distribution": [{"industry": "行业名称", "weight": 占比数值}],
  "concentration": {"top3_ratio": 数值, "top5_ratio": 数值, "top10_ratio": 数值, "level": "高/中/低"},
  "top_holdings": [{"stock_code": "代码", "stock_name": "名称", "ratio": 占比, "comment": "对该股票的简评"}],
  "analysis_text": "整体持仓分析（100字左右）"
}

只输出 JSON，不要输出其他内容。`

	// PromptRebalance 调仓检测 Prompt
	PromptRebalance PromptTemplate = `你是一位专业的基金分析师。请对比以下两期持仓数据，分析基金的调仓动向。

## 基金信息
- 基金代码：{{.FundCode}}
- 基金名称：{{.FundName}}

## 上期持仓（{{.PrevDate}}）
{{range .PrevHoldings}}
- {{.StockName}}（{{.StockCode}}）：{{.Ratio}}%
{{end}}

## 本期持仓（{{.CurrDate}}）
{{range .CurrHoldings}}
- {{.StockName}}（{{.StockCode}}）：{{.Ratio}}%
{{end}}

请以 JSON 格式输出调仓分析，包含以下字段：
{
  "changes": [
    {
      "stock_code": "股票代码",
      "stock_name": "股票名称",
      "action": "新增/增持/减持/清仓",
      "prev_ratio": 上期占比,
      "curr_ratio": 本期占比,
      "change_ratio": 变动幅度,
      "comment": "简评"
    }
  ],
  "summary": "调仓摘要（分析基金经理的操作思路，100字左右）"
}

只输出 JSON，不要输出其他内容。`
)

// AnalyzeData 综合分析数据
type AnalyzeData struct {
	Fund     *datasource.Fund
	Holdings []datasource.Holding
	NAVList  []datasource.NAV
}

// HoldingsData 持仓分析数据
type HoldingsData struct {
	Fund     *datasource.Fund
	Holdings []datasource.Holding
}

// RebalanceData 调仓检测数据
type RebalanceData struct {
	FundCode     string
	FundName     string
	PrevDate     string
	CurrDate     string
	PrevHoldings []datasource.Holding
	CurrHoldings []datasource.Holding
}

// RenderPrompt 渲染 Prompt 模板
func RenderPrompt(tmpl PromptTemplate, data interface{}) (string, error) {
	t, err := template.New("prompt").Parse(string(tmpl))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
