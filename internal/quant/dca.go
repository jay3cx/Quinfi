package quant

import "fmt"

// DCAStrategy 定投策略类型
type DCAStrategy string

const (
	DCAFixed DCAStrategy = "fixed"
	DCAValue DCAStrategy = "value"
	DCASmart DCAStrategy = "smart"
)

// DCARequest 定投模拟请求
type DCARequest struct {
	FundCode  string      `json:"fund_code"`
	Strategy  DCAStrategy `json:"strategy"`
	Amount    float64     `json:"amount"`
	Frequency string      `json:"frequency"`
}

// DCAResult 定投模拟结果
type DCAResult struct {
	Strategy      DCAStrategy      `json:"strategy"`
	TotalInvested float64          `json:"total_invested"`
	FinalValue    float64          `json:"final_value"`
	TotalReturn   float64          `json:"total_return"`
	AnnualReturn  float64          `json:"annual_return"`
	AvgCost       float64          `json:"avg_cost"`
	LumpSumReturn float64          `json:"lump_sum_return"`
	ExcessReturn  float64          `json:"excess_return"`
	InvestCurve   []CurvePoint     `json:"invest_curve"`
	ValueCurve    []CurvePoint     `json:"value_curve"`
	CostCurve     []CurvePoint     `json:"cost_curve"`
	Transactions  []DCATransaction `json:"transactions"`
}

// DCATransaction 单次定投交易
type DCATransaction struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	NAV    float64 `json:"nav"`
	Shares float64 `json:"shares"`
}

// RunDCA 执行定投模拟
func RunDCA(req *DCARequest, nav *NavSeries) (*DCAResult, error) {
	if len(nav.Points) < 2 {
		return nil, fmt.Errorf("净值数据不足")
	}

	investDates := selectInvestDates(nav.Points, req.Frequency)
	if len(investDates) == 0 {
		return nil, fmt.Errorf("无有效定投日期")
	}

	var totalInvested, totalShares float64
	transactions := make([]DCATransaction, 0, len(investDates))
	investCurve := make([]CurvePoint, 0, len(investDates))
	valueCurve := make([]CurvePoint, 0, len(investDates))
	costCurve := make([]CurvePoint, 0, len(investDates))

	maMap := buildMAMap(nav.Points, 250)

	for i, idx := range investDates {
		p := nav.Points[idx]
		amount := req.Amount

		switch req.Strategy {
		case DCAValue:
			targetValue := float64(i+1) * req.Amount
			currentValue := totalShares * p.NAV
			amount = targetValue - currentValue
			if amount < 0 {
				amount = 0
			}
		case DCASmart:
			if ma, ok := maMap[p.Date]; ok && ma > 0 {
				deviation := (p.NAV - ma) / ma
				multiplier := 1.0 - deviation*2.5
				if multiplier < 0.5 {
					multiplier = 0.5
				}
				if multiplier > 1.5 {
					multiplier = 1.5
				}
				amount = req.Amount * multiplier
			}
		}

		shares := amount / p.NAV
		totalInvested += amount
		totalShares += shares

		transactions = append(transactions, DCATransaction{
			Date: p.Date, Amount: amount, NAV: p.NAV, Shares: shares,
		})

		currentValue := totalShares * p.NAV
		avgCost := 0.0
		if totalShares > 0 {
			avgCost = totalInvested / totalShares
		}
		investCurve = append(investCurve, CurvePoint{Date: p.Date, Value: totalInvested})
		valueCurve = append(valueCurve, CurvePoint{Date: p.Date, Value: currentValue})
		costCurve = append(costCurve, CurvePoint{Date: p.Date, Value: avgCost})
	}

	lastNAV := nav.Points[len(nav.Points)-1].NAV
	finalValue := totalShares * lastNAV
	avgCost := 0.0
	if totalShares > 0 {
		avgCost = totalInvested / totalShares
	}
	totalReturn := 0.0
	if totalInvested > 0 {
		totalReturn = (finalValue - totalInvested) / totalInvested
	}

	firstNAV := nav.Points[0].NAV
	lumpSumReturn := 0.0
	if firstNAV > 0 {
		lumpSumReturn = (lastNAV - firstNAV) / firstNAV
	}

	return &DCAResult{
		Strategy:      req.Strategy,
		TotalInvested: totalInvested,
		FinalValue:    finalValue,
		TotalReturn:   totalReturn,
		AnnualReturn:  AnnualizedReturn(totalInvested, finalValue, len(nav.Points)-1),
		AvgCost:       avgCost,
		LumpSumReturn: lumpSumReturn,
		ExcessReturn:  totalReturn - lumpSumReturn,
		InvestCurve:   investCurve,
		ValueCurve:    valueCurve,
		CostCurve:     costCurve,
		Transactions:  transactions,
	}, nil
}

func selectInvestDates(points []NavPoint, freq string) []int {
	if len(points) == 0 {
		return nil
	}
	var indices []int
	lastMonth := ""
	for i, p := range points {
		if len(p.Date) < 7 {
			continue
		}
		month := p.Date[:7]
		if month != lastMonth {
			indices = append(indices, i)
			lastMonth = month
		}
	}
	return indices
}

func buildMAMap(points []NavPoint, window int) map[string]float64 {
	m := make(map[string]float64, len(points))
	for i, p := range points {
		start := 0
		if i-window+1 > 0 {
			start = i - window + 1
		}
		sum := 0.0
		count := 0
		for j := start; j <= i; j++ {
			sum += points[j].NAV
			count++
		}
		m[p.Date] = sum / float64(count)
	}
	return m
}
