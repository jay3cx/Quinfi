package quant

import "fmt"

// CompareRequest 跨基金对比请求
type CompareRequest struct {
	FundCodes []string `json:"fund_codes"`
	Period    string   `json:"period"`
}

// CompareResult 对比结果
type CompareResult struct {
	Period string            `json:"period"`
	Funds  []FundProfile     `json:"funds"`
	Matrix CorrelationMatrix `json:"correlation_matrix"`
}

// FundProfile 单只基金画像
type FundProfile struct {
	Code          string       `json:"code"`
	Name          string       `json:"name"`
	TotalReturn   float64      `json:"total_return"`
	AnnualReturn  float64      `json:"annual_return"`
	Volatility    float64      `json:"volatility"`
	SharpeRatio   float64      `json:"sharpe_ratio"`
	MaxDrawdown   float64      `json:"max_drawdown"`
	SortinoRatio  float64      `json:"sortino_ratio"`
	NormalizedNAV []CurvePoint `json:"normalized_nav"`
}

// CorrelationMatrix 相关性矩阵
type CorrelationMatrix struct {
	Codes  []string    `json:"codes"`
	Values [][]float64 `json:"values"`
}

// RunCompare 执行跨基金对比分析
func RunCompare(req *CompareRequest, navData map[string]*NavSeries, fundNames map[string]string) (*CompareResult, error) {
	if len(req.FundCodes) < 2 {
		return nil, fmt.Errorf("至少需要 2 只基金进行对比")
	}

	holdings := make([]HoldingWeight, len(req.FundCodes))
	for i, code := range req.FundCodes {
		holdings[i] = HoldingWeight{FundCode: code, Weight: 1}
	}
	dates := commonDates(holdings, navData)
	if len(dates) < 2 {
		return nil, fmt.Errorf("公共交易日不足")
	}

	navIndex := buildNavIndex(navData)
	profiles := make([]FundProfile, len(req.FundCodes))
	returnSeriesMap := make(map[string][]float64)

	for i, code := range req.FundCodes {
		startNAV := navIndex[code][dates[0]]
		endNAV := navIndex[code][dates[len(dates)-1]]

		normalized := make([]CurvePoint, len(dates))
		navPoints := make([]NavPoint, len(dates))
		for j, d := range dates {
			nav := navIndex[code][d]
			normVal := 1.0
			if startNAV > 0 {
				normVal = nav / startNAV
			}
			normalized[j] = CurvePoint{Date: d, Value: normVal}
			navPoints[j] = NavPoint{Date: d, NAV: nav}
		}

		series := &NavSeries{FundCode: code, Points: navPoints}
		rs := series.ToReturns()
		returnSeriesMap[code] = rs.Returns

		totalReturn := 0.0
		if startNAV > 0 {
			totalReturn = (endNAV - startNAV) / startNAV
		}

		name := code
		if fundNames != nil {
			if n, ok := fundNames[code]; ok {
				name = n
			}
		}

		profiles[i] = FundProfile{
			Code:          code,
			Name:          name,
			TotalReturn:   totalReturn,
			AnnualReturn:  AnnualizedReturn(startNAV, endNAV, len(dates)-1),
			Volatility:    Volatility(rs.Returns),
			SharpeRatio:   SharpeRatio(rs.Returns, 0.02),
			MaxDrawdown:   MaxDrawdown(navPoints),
			SortinoRatio:  SortinoRatio(rs.Returns, 0.02),
			NormalizedNAV: normalized,
		}
	}

	n := len(req.FundCodes)
	matrix := CorrelationMatrix{
		Codes:  req.FundCodes,
		Values: make([][]float64, n),
	}
	for i := 0; i < n; i++ {
		matrix.Values[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix.Values[i][j] = 1.0
			} else {
				matrix.Values[i][j] = Correlation(
					returnSeriesMap[req.FundCodes[i]],
					returnSeriesMap[req.FundCodes[j]],
				)
			}
		}
	}

	return &CompareResult{
		Period: req.Period,
		Funds:  profiles,
		Matrix: matrix,
	}, nil
}
