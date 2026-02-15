package quant

import (
	"testing"
)

func TestCompare_TwoFunds(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "2025-01-01", NAV: 1.0}, {Date: "2025-02-01", NAV: 1.1},
			{Date: "2025-03-01", NAV: 1.05}, {Date: "2025-04-01", NAV: 1.2},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "2025-01-01", NAV: 2.0}, {Date: "2025-02-01", NAV: 2.1},
			{Date: "2025-03-01", NAV: 1.9}, {Date: "2025-04-01", NAV: 2.3},
		}},
	}
	fundNames := map[string]string{"A": "基金A", "B": "基金B"}
	req := &CompareRequest{FundCodes: []string{"A", "B"}, Period: "max"}
	result, err := RunCompare(req, navData, fundNames)
	if err != nil {
		t.Fatalf("RunCompare failed: %v", err)
	}
	if len(result.Funds) != 2 {
		t.Fatalf("expected 2 fund profiles, got %d", len(result.Funds))
	}
	for _, f := range result.Funds {
		if len(f.NormalizedNAV) == 0 {
			t.Errorf("fund %s has no normalized NAV", f.Code)
			continue
		}
		if !almostEqual(f.NormalizedNAV[0].Value, 1.0, 0.001) {
			t.Errorf("fund %s first normalized NAV should be 1.0, got %f", f.Code, f.NormalizedNAV[0].Value)
		}
	}
	if len(result.Matrix.Values) != 2 || len(result.Matrix.Values[0]) != 2 {
		t.Errorf("expected 2x2 correlation matrix")
	}
	if !almostEqual(result.Matrix.Values[0][0], 1.0, 0.001) {
		t.Errorf("expected self-correlation = 1.0, got %f", result.Matrix.Values[0][0])
	}
}

func TestCompare_TooFewFunds(t *testing.T) {
	navData := map[string]*NavSeries{}
	req := &CompareRequest{FundCodes: []string{"A"}, Period: "1y"}
	_, err := RunCompare(req, navData, nil)
	if err == nil {
		t.Error("expected error for single fund")
	}
}

func TestCompare_Correlation(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.1}, {Date: "d3", NAV: 1.2}, {Date: "d4", NAV: 1.3},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "d1", NAV: 2.0}, {Date: "d2", NAV: 2.2}, {Date: "d3", NAV: 2.4}, {Date: "d4", NAV: 2.6},
		}},
	}
	req := &CompareRequest{FundCodes: []string{"A", "B"}, Period: "max"}
	result, err := RunCompare(req, navData, nil)
	if err != nil {
		t.Fatalf("RunCompare failed: %v", err)
	}
	corr := result.Matrix.Values[0][1]
	if !almostEqual(corr, 1.0, 0.01) {
		t.Errorf("expected correlation ≈ 1.0, got %f", corr)
	}
}
