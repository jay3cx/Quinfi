package quant

import (
	"testing"
)

func makeMonthlyNAV() *NavSeries {
	return &NavSeries{
		FundCode: "000001",
		Points: []NavPoint{
			{Date: "2025-01-15", NAV: 1.00},
			{Date: "2025-02-15", NAV: 0.95},
			{Date: "2025-03-15", NAV: 1.05},
			{Date: "2025-04-15", NAV: 0.90},
			{Date: "2025-05-15", NAV: 1.00},
			{Date: "2025-06-15", NAV: 1.10},
			{Date: "2025-07-15", NAV: 1.05},
			{Date: "2025-08-15", NAV: 0.98},
			{Date: "2025-09-15", NAV: 1.02},
			{Date: "2025-10-15", NAV: 1.08},
			{Date: "2025-11-15", NAV: 1.12},
			{Date: "2025-12-15", NAV: 1.15},
		},
	}
}

func TestDCA_FixedStrategy(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCAFixed,
		Amount:    1000,
		Frequency: "monthly",
	}
	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}
	if result.TotalInvested != 12000 {
		t.Errorf("expected total invested 12000, got %f", result.TotalInvested)
	}
	if result.FinalValue <= 0 {
		t.Errorf("expected positive final value, got %f", result.FinalValue)
	}
	if len(result.Transactions) != 12 {
		t.Errorf("expected 12 transactions, got %d", len(result.Transactions))
	}
	if !almostEqual(result.Transactions[0].Shares, 1000, 0.01) {
		t.Errorf("expected first shares ≈ 1000, got %f", result.Transactions[0].Shares)
	}
	if result.AvgCost <= 0 {
		t.Errorf("expected positive avg cost, got %f", result.AvgCost)
	}
}

func TestDCA_LumpSumComparison(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCAFixed,
		Amount:    1000,
		Frequency: "monthly",
	}
	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}
	expectedLumpSum := (1.15/1.00 - 1)
	if !almostEqual(result.LumpSumReturn, expectedLumpSum, 0.001) {
		t.Errorf("expected lump sum return ≈ %f, got %f", expectedLumpSum, result.LumpSumReturn)
	}
}

func TestDCA_SmartStrategy(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCASmart,
		Amount:    1000,
		Frequency: "monthly",
	}
	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}
	if result.TotalInvested == 12000 {
		t.Error("smart DCA should vary investment amounts")
	}
	if len(result.Transactions) != 12 {
		t.Errorf("expected 12 transactions, got %d", len(result.Transactions))
	}
}

func TestDCA_EmptyNAV(t *testing.T) {
	nav := &NavSeries{FundCode: "000001", Points: []NavPoint{}}
	req := &DCARequest{FundCode: "000001", Strategy: DCAFixed, Amount: 1000, Frequency: "monthly"}
	_, err := RunDCA(req, nav)
	if err == nil {
		t.Error("expected error for empty NAV")
	}
}
