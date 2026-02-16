package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jay3cx/Quinfi/internal/quant"
)

// QuantDataLoader 量化数据加载接口
type QuantDataLoader interface {
	LoadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error)
	GetFundName(ctx context.Context, code string) string
}

// ===== BacktestPortfolioTool =====

type BacktestPortfolioTool struct {
	loader QuantDataLoader
}

func NewBacktestPortfolioTool(loader QuantDataLoader) *BacktestPortfolioTool {
	return &BacktestPortfolioTool{loader: loader}
}

func (t *BacktestPortfolioTool) Name() string { return "backtest_portfolio" }

func (t *BacktestPortfolioTool) Description() string {
	return "回测基金组合的历史表现。输入一组基金代码和权重比例，返回年化收益、最大回撤、夏普比率等风控指标。适合用户想要评估组合配置效果时使用。"
}

func (t *BacktestPortfolioTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"holdings": map[string]any{
				"type":        "string",
				"description": "基金组合，格式: '代码1:权重1,代码2:权重2'，如 '005827:0.6,110011:0.4'。权重之和须为1。",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "回测天数，默认365天（1年）",
			},
		},
		"required": []string{"holdings"},
	}
}

func (t *BacktestPortfolioTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	holdingsStr := getStringArg(args, "holdings")
	days := getIntArg(args, "days", 365)

	holdings, err := parseHoldings(holdingsStr)
	if err != nil {
		return "", err
	}

	navData := make(map[string]*quant.NavSeries)
	for _, h := range holdings {
		series, err := t.loader.LoadNAVSeries(ctx, h.FundCode, days)
		if err != nil {
			return "", fmt.Errorf("加载 %s 净值失败: %w", h.FundCode, err)
		}
		navData[h.FundCode] = series
	}

	req := &quant.BacktestRequest{
		Holdings:    holdings,
		InitialCash: 100000,
		Rebalance:   quant.RebalanceNone,
	}
	result, err := quant.RunBacktest(req, navData)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("## 组合回测结果\n\n### 组合配置\n")
	for _, h := range holdings {
		name := t.loader.GetFundName(ctx, h.FundCode)
		sb.WriteString(fmt.Sprintf("- %s（%s）: %.0f%%\n", name, h.FundCode, h.Weight*100))
	}
	sb.WriteString("\n### 风控指标\n")
	sb.WriteString(fmt.Sprintf("- 累计收益率: %.2f%%\n", result.TotalReturn*100))
	sb.WriteString(fmt.Sprintf("- 年化收益率: %.2f%%\n", result.AnnualReturn*100))
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", result.MaxDrawdown*100))
	sb.WriteString(fmt.Sprintf("- 夏普比率: %.3f\n", result.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- 年化波动率: %.2f%%\n", result.Volatility*100))
	sb.WriteString(fmt.Sprintf("- Sortino 比率: %.3f\n", result.SortinoRatio))
	sb.WriteString(fmt.Sprintf("- Calmar 比率: %.3f\n", result.CalmarRatio))
	return sb.String(), nil
}

// ===== SimulateDCATool =====

type SimulateDCATool struct {
	loader QuantDataLoader
}

func NewSimulateDCATool(loader QuantDataLoader) *SimulateDCATool {
	return &SimulateDCATool{loader: loader}
}

func (t *SimulateDCATool) Name() string { return "simulate_dca" }

func (t *SimulateDCATool) Description() string {
	return "模拟定投策略的历史收益。输入基金代码、每期金额和策略类型，返回累计投入、终值、年化收益、对比一次性投入的超额收益。适合用户想要比较定投方案时使用。"
}

func (t *SimulateDCATool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码",
			},
			"amount": map[string]any{
				"type":        "number",
				"description": "每期定投金额（元），默认1000",
			},
			"strategy": map[string]any{
				"type":        "string",
				"description": "定投策略: fixed(固定金额) / value(目标价值) / smart(智能定投)，默认 fixed",
				"enum":        []string{"fixed", "value", "smart"},
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "模拟天数，默认1095天（3年）",
			},
		},
		"required": []string{"code"},
	}
}

func (t *SimulateDCATool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}
	amount := getFloatArg(args, "amount", 1000)
	strategy := getStringArg(args, "strategy")
	if strategy == "" {
		strategy = "fixed"
	}
	days := getIntArg(args, "days", 1095)

	series, err := t.loader.LoadNAVSeries(ctx, code, days)
	if err != nil {
		return "", fmt.Errorf("加载净值失败: %w", err)
	}

	req := &quant.DCARequest{
		FundCode:  code,
		Strategy:  quant.DCAStrategy(strategy),
		Amount:    amount,
		Frequency: "monthly",
	}
	result, err := quant.RunDCA(req, series)
	if err != nil {
		return "", err
	}

	name := t.loader.GetFundName(ctx, code)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（%s）定投模拟\n\n", name, code))
	sb.WriteString(fmt.Sprintf("- 策略: %s\n", result.Strategy))
	sb.WriteString(fmt.Sprintf("- 定投次数: %d 期\n", len(result.Transactions)))
	sb.WriteString(fmt.Sprintf("- 累计投入: %.0f 元\n", result.TotalInvested))
	sb.WriteString(fmt.Sprintf("- 当前市值: %.0f 元\n", result.FinalValue))
	sb.WriteString(fmt.Sprintf("- 累计收益率: %.2f%%\n", result.TotalReturn*100))
	sb.WriteString(fmt.Sprintf("- 平均成本: %.4f\n", result.AvgCost))
	sb.WriteString("\n### 对比一次性投入\n")
	sb.WriteString(fmt.Sprintf("- 一次性投入收益率: %.2f%%\n", result.LumpSumReturn*100))
	sb.WriteString(fmt.Sprintf("- 定投超额收益: %.2f%%\n", result.ExcessReturn*100))
	return sb.String(), nil
}

// ===== CompareFundsTool =====

type CompareFundsTool struct {
	loader QuantDataLoader
}

func NewCompareFundsTool(loader QuantDataLoader) *CompareFundsTool {
	return &CompareFundsTool{loader: loader}
}

func (t *CompareFundsTool) Name() string { return "compare_funds" }

func (t *CompareFundsTool) Description() string {
	return "对比多只基金的表现。输入2-5只基金代码，返回各基金收益率、风险指标、相关性矩阵的对比。适合用户想要横向对比筛选基金时使用。"
}

func (t *CompareFundsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"codes": map[string]any{
				"type":        "string",
				"description": "基金代码列表，逗号分隔，如 '005827,110011,161725'",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "对比天数，默认365天",
			},
		},
		"required": []string{"codes"},
	}
}

func (t *CompareFundsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	codesStr := getStringArg(args, "codes")
	days := getIntArg(args, "days", 365)

	codes := strings.Split(codesStr, ",")
	for i := range codes {
		codes[i] = strings.TrimSpace(codes[i])
	}
	if len(codes) < 2 || len(codes) > 5 {
		return "", fmt.Errorf("请提供 2-5 只基金代码")
	}

	navData := make(map[string]*quant.NavSeries)
	fundNames := make(map[string]string)
	for _, code := range codes {
		series, err := t.loader.LoadNAVSeries(ctx, code, days)
		if err != nil {
			return "", fmt.Errorf("加载 %s 净值失败: %w", code, err)
		}
		navData[code] = series
		fundNames[code] = t.loader.GetFundName(ctx, code)
	}

	req := &quant.CompareRequest{FundCodes: codes, Period: "1y"}
	result, err := quant.RunCompare(req, navData, fundNames)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("## 基金对比分析\n\n### 业绩与风险指标\n")
	sb.WriteString("| 基金 | 累计收益 | 年化收益 | 最大回撤 | 夏普比率 | 波动率 |\n")
	sb.WriteString("|------|---------|---------|---------|---------|--------|\n")
	for _, f := range result.Funds {
		sb.WriteString(fmt.Sprintf("| %s（%s）| %.2f%% | %.2f%% | %.2f%% | %.3f | %.2f%% |\n",
			f.Name, f.Code, f.TotalReturn*100, f.AnnualReturn*100,
			f.MaxDrawdown*100, f.SharpeRatio, f.Volatility*100))
	}
	sb.WriteString("\n### 相关性矩阵\n")
	n := len(result.Matrix.Codes)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			corr := result.Matrix.Values[i][j]
			label := "低相关"
			if corr > 0.7 {
				label = "高相关"
			} else if corr > 0.3 {
				label = "中等相关"
			}
			sb.WriteString(fmt.Sprintf("- %s ↔ %s: %.3f（%s）\n",
				result.Matrix.Codes[i], result.Matrix.Codes[j], corr, label))
		}
	}
	return sb.String(), nil
}

// ===== 辅助函数 =====

func parseHoldings(s string) ([]quant.HoldingWeight, error) {
	parts := strings.Split(s, ",")
	var holdings []quant.HoldingWeight
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("格式错误: '%s'，应为 '代码:权重'", part)
		}
		code := strings.TrimSpace(kv[0])
		weight, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("权重解析失败: '%s'", kv[1])
		}
		holdings = append(holdings, quant.HoldingWeight{FundCode: code, Weight: weight})
	}
	return holdings, nil
}

func getFloatArg(args map[string]any, key string, defaultVal float64) float64 {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return defaultVal
}
