package debate

import "testing"

func TestComputeConsistencyScore(t *testing.T) {
	base := &Verdict{
		Suggestion: "建议分批买入并中长期持有，注意控制仓位。",
		RiskWarnings: []string{
			"持仓集中度较高",
			"行业政策变化风险",
		},
		Confidence: 78,
	}
	review := &Verdict{
		Suggestion: "可考虑分批买入并中长期持有，同时做好仓位控制。",
		RiskWarnings: []string{
			"持仓集中度较高",
			"行业政策变化风险",
		},
		Confidence: 75,
	}

	score := ComputeConsistencyScore(base, review)
	if score < 88 || score > 90 {
		t.Fatalf("expected stable score around 89 (+/-1), got %d", score)
	}
	if score > 100 || score < 0 {
		t.Fatalf("expected score in [0,100], got %d", score)
	}
}

func TestComputeConsistencyScore_LowConsistency(t *testing.T) {
	base := &Verdict{
		Suggestion: "建议分批买入并中长期持有，注意控制仓位。",
		RiskWarnings: []string{
			"持仓集中度较高",
			"行业政策变化风险",
		},
		Confidence: 78,
	}
	review := &Verdict{
		Suggestion: "建议立即减仓并回避该基金，短期卖出。",
		RiskWarnings: []string{
			"流动性风险",
			"信用违约风险",
		},
		Confidence: 25,
	}

	score := ComputeConsistencyScore(base, review)
	if score > 30 {
		t.Fatalf("expected low consistency score <= 30 for conflicting verdicts, got %d", score)
	}
}

func TestComputeConsistencyScore_NilInput(t *testing.T) {
	if got := ComputeConsistencyScore(nil, &Verdict{}); got != 0 {
		t.Fatalf("expected nil base score 0, got %d", got)
	}
	if got := ComputeConsistencyScore(&Verdict{}, nil); got != 0 {
		t.Fatalf("expected nil review score 0, got %d", got)
	}
	if got := ComputeConsistencyScore(nil, nil); got != 0 {
		t.Fatalf("expected nil input score 0, got %d", got)
	}
}

func TestDetectSuggestionIntent_AvoidsEnglishSubstringFalsePositive(t *testing.T) {
	if got := detectSuggestionIntent("set threshold and rebalance by volatility"); got != suggestionIntentUnknown {
		t.Fatalf("expected unknown intent for non-intent english text, got %v", got)
	}
}

func TestDetectSuggestionIntent_CautiousHoldNotSell(t *testing.T) {
	if got := detectSuggestionIntent("建议谨慎持有，等待更清晰信号"); got != suggestionIntentHold {
		t.Fatalf("expected hold intent for cautious-hold text, got %v", got)
	}
}

func TestDetectSuggestionIntent_PunctuationSeparatedEnglishTokens(t *testing.T) {
	if got := detectSuggestionIntent("buy,sell signal mixed"); got != suggestionIntentUnknown {
		t.Fatalf("expected unknown when buy/sell counts are balanced, got %v", got)
	}
	if got := detectSuggestionIntent("buy/hold then rebalance"); got != suggestionIntentBuy {
		t.Fatalf("expected buy intent when buy dominates hold fallback, got %v", got)
	}
}
