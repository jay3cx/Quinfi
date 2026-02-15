package quant

import (
	"math"
	"testing"
)

func TestNavSeriesToReturns(t *testing.T) {
	series := &NavSeries{
		FundCode: "000001",
		Points: []NavPoint{
			{Date: "2026-01-10", NAV: 1.0000},
			{Date: "2026-01-11", NAV: 1.0100},
			{Date: "2026-01-12", NAV: 0.9900},
			{Date: "2026-01-13", NAV: 1.0200},
		},
	}

	returns := series.ToReturns()

	if returns.FundCode != "000001" {
		t.Errorf("expected fund code 000001, got %s", returns.FundCode)
	}
	if len(returns.Returns) != 3 {
		t.Fatalf("expected 3 returns, got %d", len(returns.Returns))
	}
	if math.Abs(returns.Returns[0]-0.01) > 1e-9 {
		t.Errorf("expected return[0] ≈ 0.01, got %f", returns.Returns[0])
	}
	if math.Abs(returns.Returns[1]-(-0.0198019801980198)) > 1e-9 {
		t.Errorf("expected return[1] ≈ -0.0198, got %f", returns.Returns[1])
	}
}

func TestNavSeries_Empty(t *testing.T) {
	series := &NavSeries{FundCode: "000001", Points: []NavPoint{}}
	returns := series.ToReturns()
	if len(returns.Returns) != 0 {
		t.Errorf("expected 0 returns for empty series, got %d", len(returns.Returns))
	}
}

func TestNavSeries_SinglePoint(t *testing.T) {
	series := &NavSeries{FundCode: "000001", Points: []NavPoint{{Date: "2026-01-10", NAV: 1.0}}}
	returns := series.ToReturns()
	if len(returns.Returns) != 0 {
		t.Errorf("expected 0 returns for single point, got %d", len(returns.Returns))
	}
}
