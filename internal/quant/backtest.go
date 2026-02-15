package quant

import (
	"fmt"
	"math"
)

// RebalanceType 再平衡策略
type RebalanceType string

const (
	RebalanceNone      RebalanceType = "none"
	RebalanceMonthly   RebalanceType = "monthly"
	RebalanceQuarterly RebalanceType = "quarterly"
)

// BacktestRequest 回测请求
type BacktestRequest struct {
	Holdings    []HoldingWeight
	InitialCash float64
	Rebalance   RebalanceType
	Benchmark   string
}

// HoldingWeight 持仓权重
type HoldingWeight struct {
	FundCode string  `json:"fund_code"`
	Weight   float64 `json:"weight"`
}

// BacktestResult 回测结果
type BacktestResult struct {
	TotalReturn  float64 `json:"total_return"`
	AnnualReturn float64 `json:"annual_return"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	Volatility   float64 `json:"volatility"`
	SortinoRatio float64 `json:"sortino_ratio"`
	CalmarRatio  float64 `json:"calmar_ratio"`

	EquityCurve    []CurvePoint    `json:"equity_curve"`
	DrawdownCurve  []CurvePoint    `json:"drawdown_curve"`
	BenchmarkCurve []CurvePoint    `json:"benchmark_curve,omitempty"`
	FundMetrics    []FundMetricRow `json:"fund_metrics"`
}

// FundMetricRow 单只基金在回测期内的表现
type FundMetricRow struct {
	FundCode    string  `json:"fund_code"`
	Weight      float64 `json:"weight"`
	TotalReturn float64 `json:"total_return"`
}

// RunBacktest 执行组合回测
func RunBacktest(req *BacktestRequest, navData map[string]*NavSeries) (*BacktestResult, error) {
	if req.InitialCash <= 0 {
		req.InitialCash = 100000
	}

	totalWeight := 0.0
	for _, h := range req.Holdings {
		totalWeight += h.Weight
	}
	if math.Abs(totalWeight-1.0) > 0.01 {
		return nil, fmt.Errorf("权重之和必须为 1.0，当前为 %.4f", totalWeight)
	}

	dates := commonDates(req.Holdings, navData)
	if len(dates) < 2 {
		return nil, fmt.Errorf("公共交易日不足（至少需要2天），当前 %d 天", len(dates))
	}

	navIndex := buildNavIndex(navData)

	equityCurve := make([]CurvePoint, len(dates))
	portfolioNAVs := make([]NavPoint, len(dates))

	for i, date := range dates {
		portfolioReturn := 0.0
		for _, h := range req.Holdings {
			navStart := navIndex[h.FundCode][dates[0]]
			navCurr := navIndex[h.FundCode][date]
			if navStart > 0 {
				portfolioReturn += h.Weight * (navCurr / navStart)
			}
		}
		value := req.InitialCash * portfolioReturn
		equityCurve[i] = CurvePoint{Date: date, Value: value}
		portfolioNAVs[i] = NavPoint{Date: date, NAV: portfolioReturn}
	}

	pSeries := &NavSeries{FundCode: "portfolio", Points: portfolioNAVs}
	pReturns := pSeries.ToReturns()

	result := &BacktestResult{
		TotalReturn:   portfolioNAVs[len(portfolioNAVs)-1].NAV - 1.0,
		AnnualReturn:  AnnualizedReturn(1.0, portfolioNAVs[len(portfolioNAVs)-1].NAV, len(dates)-1),
		MaxDrawdown:   MaxDrawdown(portfolioNAVs),
		SharpeRatio:   SharpeRatio(pReturns.Returns, 0.02),
		Volatility:    Volatility(pReturns.Returns),
		SortinoRatio:  SortinoRatio(pReturns.Returns, 0.02),
		CalmarRatio:   CalmarRatio(portfolioNAVs),
		EquityCurve:   equityCurve,
		DrawdownCurve: DrawdownSeries(portfolioNAVs),
	}

	for _, h := range req.Holdings {
		navStart := navIndex[h.FundCode][dates[0]]
		navEnd := navIndex[h.FundCode][dates[len(dates)-1]]
		ret := 0.0
		if navStart > 0 {
			ret = (navEnd - navStart) / navStart
		}
		result.FundMetrics = append(result.FundMetrics, FundMetricRow{
			FundCode: h.FundCode, Weight: h.Weight, TotalReturn: ret,
		})
	}

	if req.Benchmark != "" {
		if benchSeries, ok := navData[req.Benchmark]; ok {
			bIndex := make(map[string]float64)
			for _, p := range benchSeries.Points {
				bIndex[p.Date] = p.NAV
			}
			startNAV := bIndex[dates[0]]
			if startNAV > 0 {
				benchCurve := make([]CurvePoint, 0, len(dates))
				for _, date := range dates {
					if nav, ok := bIndex[date]; ok {
						benchCurve = append(benchCurve, CurvePoint{
							Date: date, Value: req.InitialCash * nav / startNAV,
						})
					}
				}
				result.BenchmarkCurve = benchCurve
			}
		}
	}

	return result, nil
}

// commonDates 所有基金都有数据的公共交易日（升序）
func commonDates(holdings []HoldingWeight, navData map[string]*NavSeries) []string {
	if len(holdings) == 0 {
		return nil
	}
	first := navData[holdings[0].FundCode]
	if first == nil {
		return nil
	}
	dateSet := make(map[string]bool)
	for _, p := range first.Points {
		dateSet[p.Date] = true
	}
	for i := 1; i < len(holdings); i++ {
		series := navData[holdings[i].FundCode]
		if series == nil {
			return nil
		}
		otherDates := make(map[string]bool)
		for _, p := range series.Points {
			otherDates[p.Date] = true
		}
		for d := range dateSet {
			if !otherDates[d] {
				delete(dateSet, d)
			}
		}
	}
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sortStrings(dates)
	return dates
}

// buildNavIndex 构建 fundCode → date → NAV 索引
func buildNavIndex(navData map[string]*NavSeries) map[string]map[string]float64 {
	index := make(map[string]map[string]float64)
	for code, series := range navData {
		m := make(map[string]float64, len(series.Points))
		for _, p := range series.Points {
			m[p.Date] = p.NAV
		}
		index[code] = m
	}
	return index
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
