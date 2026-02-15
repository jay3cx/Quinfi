package quant

import (
	"math"
	"testing"
)

func TestBacktest_SingleFund_NoRebalance(t *testing.T) {
	navData := map[string]*NavSeries{
		"000001": {
			FundCode: "000001",
			Points: []NavPoint{
				{Date: "2026-01-10", NAV: 1.0},
				{Date: "2026-01-11", NAV: 1.05},
				{Date: "2026-01-12", NAV: 1.02},
				{Date: "2026-01-13", NAV: 1.10},
			},
		},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "000001", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	if !almostEqual(result.TotalReturn, 0.10, 0.001) {
		t.Errorf("expected total return ≈ 0.10, got %f", result.TotalReturn)
	}
	if len(result.EquityCurve) != 4 {
		t.Errorf("expected 4 equity curve points, got %d", len(result.EquityCurve))
	}
	if !almostEqual(result.EquityCurve[0].Value, 100000, 1) {
		t.Errorf("expected first equity ≈ 100000, got %f", result.EquityCurve[0].Value)
	}
	if !almostEqual(result.EquityCurve[3].Value, 110000, 1) {
		t.Errorf("expected last equity ≈ 110000, got %f", result.EquityCurve[3].Value)
	}
}

func TestBacktest_TwoFunds_EqualWeight(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.2},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 0.8},
		}},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "A", Weight: 0.5}, {FundCode: "B", Weight: 0.5}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	if !almostEqual(result.TotalReturn, 0.0, 0.001) {
		t.Errorf("expected total return ≈ 0, got %f", result.TotalReturn)
	}
}

func TestBacktest_WithBenchmark(t *testing.T) {
	navData := map[string]*NavSeries{
		"000001": {FundCode: "000001", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.1}, {Date: "d3", NAV: 1.15},
		}},
		"000300": {FundCode: "000300", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.05}, {Date: "d3", NAV: 1.08},
		}},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "000001", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
		Benchmark:   "000300",
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	if len(result.BenchmarkCurve) != 3 {
		t.Errorf("expected 3 benchmark points, got %d", len(result.BenchmarkCurve))
	}
}

func TestBacktest_InvalidWeights(t *testing.T) {
	navData := map[string]*NavSeries{}
	req := &BacktestRequest{
		Holdings: []HoldingWeight{{FundCode: "A", Weight: 0.3}},
	}
	_, err := RunBacktest(req, navData)
	if err == nil {
		t.Error("expected error for invalid weights, got nil")
	}
}

func TestBacktest_MetricsPresent(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: func() []NavPoint {
			pts := make([]NavPoint, 30)
			nav := 1.0
			for i := range pts {
				pts[i] = NavPoint{Date: "d" + string(rune('A'+i)), NAV: nav}
				nav *= 1 + (float64(i%5)-2)*0.01
			}
			return pts
		}()},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "A", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	if math.IsNaN(result.SharpeRatio) {
		t.Error("SharpeRatio is NaN")
	}
	if math.IsNaN(result.Volatility) {
		t.Error("Volatility is NaN")
	}
}
