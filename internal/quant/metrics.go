package quant

import "math"

const tradingDaysPerYear = 252

// AnnualizedReturn 年化收益率
func AnnualizedReturn(startNAV, endNAV float64, tradingDays int) float64 {
	if startNAV <= 0 || tradingDays <= 0 {
		return 0
	}
	totalReturn := endNAV / startNAV
	return math.Pow(totalReturn, float64(tradingDaysPerYear)/float64(tradingDays)) - 1
}

// Volatility 年化波动率
func Volatility(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	return stddev(returns) * math.Sqrt(float64(tradingDaysPerYear))
}

// MaxDrawdown 最大回撤
func MaxDrawdown(navs []NavPoint) float64 {
	if len(navs) < 2 {
		return 0
	}
	peak := navs[0].NAV
	maxDD := 0.0
	for _, p := range navs {
		if p.NAV > peak {
			peak = p.NAV
		}
		dd := (peak - p.NAV) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// SharpeRatio 夏普比率
func SharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	avgDaily := mean(returns)
	annualReturn := avgDaily * float64(tradingDaysPerYear)
	vol := Volatility(returns)
	if vol == 0 {
		return 0
	}
	return (annualReturn - riskFreeRate) / vol
}

// SortinoRatio 仅计算下行波动率
func SortinoRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	avgDaily := mean(returns)
	annualReturn := avgDaily * float64(tradingDaysPerYear)
	downside := downsideDeviation(returns) * math.Sqrt(float64(tradingDaysPerYear))
	if downside == 0 {
		return 0
	}
	return (annualReturn - riskFreeRate) / downside
}

// CalmarRatio 年化收益 / 最大回撤
func CalmarRatio(navs []NavPoint) float64 {
	if len(navs) < 2 {
		return 0
	}
	annRet := AnnualizedReturn(navs[0].NAV, navs[len(navs)-1].NAV, len(navs)-1)
	mdd := MaxDrawdown(navs)
	if mdd == 0 {
		return 0
	}
	return annRet / mdd
}

// DrawdownSeries 回撤曲线
func DrawdownSeries(navs []NavPoint) []CurvePoint {
	if len(navs) == 0 {
		return nil
	}
	result := make([]CurvePoint, len(navs))
	peak := navs[0].NAV
	for i, p := range navs {
		if p.NAV > peak {
			peak = p.NAV
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - p.NAV) / peak
		}
		result[i] = CurvePoint{Date: p.Date, Value: -dd}
	}
	return result
}

// Correlation 皮尔逊相关系数
func Correlation(a, b []float64) float64 {
	n := min(len(a), len(b))
	if n < 2 {
		return 0
	}
	meanA := mean(a[:n])
	meanB := mean(b[:n])
	var sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		sumAB += da * db
		sumA2 += da * da
		sumB2 += db * db
	}
	denom := math.Sqrt(sumA2 * sumB2)
	if denom == 0 {
		return 0
	}
	return sumAB / denom
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func stddev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := mean(data)
	sumSq := 0.0
	for _, v := range data {
		d := v - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(data)-1))
}

func downsideDeviation(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	sumSq := 0.0
	count := 0
	for _, r := range returns {
		if r < 0 {
			sumSq += r * r
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(sumSq / float64(count))
}
