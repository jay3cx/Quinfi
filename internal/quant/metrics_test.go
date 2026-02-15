package quant

import (
	"math"
	"testing"
)

const tolerance = 1e-6

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func makeTestReturns() *ReturnSeries {
	return &ReturnSeries{
		FundCode: "TEST",
		Dates:    []string{"d1", "d2", "d3", "d4"},
		Returns:  []float64{0.05, -0.02857142857, 0.05882352941, 0.01851851852},
	}
}

func TestAnnualizedReturn(t *testing.T) {
	r := AnnualizedReturn(1.0, 1.10, 4)
	if r <= 0 {
		t.Errorf("expected positive annualized return, got %f", r)
	}
	r2 := AnnualizedReturn(1.0, 1.0, 100)
	if !almostEqual(r2, 0, tolerance) {
		t.Errorf("expected 0 for no change, got %f", r2)
	}
}

func TestVolatility(t *testing.T) {
	rs := makeTestReturns()
	vol := Volatility(rs.Returns)
	if vol <= 0 {
		t.Errorf("expected positive volatility, got %f", vol)
	}
}

func TestMaxDrawdown(t *testing.T) {
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 0.9},
		{Date: "d4", NAV: 1.05},
	}
	mdd := MaxDrawdown(navs)
	expected := (1.1 - 0.9) / 1.1
	if !almostEqual(mdd, expected, tolerance) {
		t.Errorf("expected max drawdown ≈ %.6f, got %.6f", expected, mdd)
	}
}

func TestMaxDrawdown_NoDrawdown(t *testing.T) {
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 1.2},
	}
	mdd := MaxDrawdown(navs)
	if !almostEqual(mdd, 0, tolerance) {
		t.Errorf("expected 0 drawdown, got %f", mdd)
	}
}

func TestSharpeRatio(t *testing.T) {
	rs := makeTestReturns()
	sr := SharpeRatio(rs.Returns, 0.02)
	if sr <= 0 {
		t.Errorf("expected positive Sharpe ratio, got %f", sr)
	}
}

func TestSortinoRatio(t *testing.T) {
	rs := makeTestReturns()
	sortino := SortinoRatio(rs.Returns, 0.02)
	if sortino <= 0 {
		t.Errorf("expected positive Sortino ratio, got %f", sortino)
	}
}

func TestCalmarRatio(t *testing.T) {
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 0.9},
		{Date: "d4", NAV: 1.2},
	}
	calmar := CalmarRatio(navs)
	if calmar <= 0 {
		t.Errorf("expected positive Calmar ratio, got %f", calmar)
	}
}

func TestMetrics_EmptyInput(t *testing.T) {
	if vol := Volatility(nil); vol != 0 {
		t.Errorf("Volatility(nil) should be 0, got %f", vol)
	}
	if mdd := MaxDrawdown(nil); mdd != 0 {
		t.Errorf("MaxDrawdown(nil) should be 0, got %f", mdd)
	}
	if sr := SharpeRatio(nil, 0.02); sr != 0 {
		t.Errorf("SharpeRatio(nil) should be 0, got %f", sr)
	}
	if sor := SortinoRatio(nil, 0.02); sor != 0 {
		t.Errorf("SortinoRatio(nil) should be 0, got %f", sor)
	}
	if cal := CalmarRatio(nil); cal != 0 {
		t.Errorf("CalmarRatio(nil) should be 0, got %f", cal)
	}
}
