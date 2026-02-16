package debate

import "testing"

func TestConfidenceEngine_Evaluate_HardRulesDegrade(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())

	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"收益增长12%", "夏普比率1.4"},
		},
		BearCase: &Argument{
			Points: []string{"最大回撤6%", "估值压力"},
		},
		BullRebuttal: &Argument{
			Points: []string{"仓位维持80%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"板块集中度35%"},
		},
		Verdict: &Verdict{
			Confidence: 88,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      false, // 命中硬规则：缺 NAV
		},
		Phases: []PhaseRecord{
			{Phase: PhaseDataGather, ParseAttempted: true, ParseOK: false}, // data_gather 不参与硬规则 parse 判断
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: false},   // 命中硬规则
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBullRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseJudgeVerdict, ParseAttempted: true, ParseOK: true},
		},
		Error: "judge timeout", // 命中硬规则：结果错误
	}

	assessment := engine.Evaluate(result)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}

	if assessment.Decision != DecisionGateDegrade {
		t.Fatalf("expected decision degrade, got %s", assessment.Decision)
	}

	if assessment.BaseScore <= DefaultConfidenceConfig().HardCap {
		t.Fatalf("expected base score > hard cap to verify cap behavior, got %d", assessment.BaseScore)
	}

	if assessment.FinalScore != DefaultConfidenceConfig().HardCap {
		t.Fatalf("expected final score capped to %d, got %d", DefaultConfidenceConfig().HardCap, assessment.FinalScore)
	}

	if len(assessment.HardRuleHits) == 0 {
		t.Fatal("expected hard rule hits")
	}
	if !containsString(assessment.HardRuleHits, "missing_nav") {
		t.Fatalf("expected hard rule hit missing_nav, got %v", assessment.HardRuleHits)
	}
	if !containsString(assessment.HardRuleHits, "parse_failure") {
		t.Fatalf("expected hard rule hit parse_failure, got %v", assessment.HardRuleHits)
	}
	if !containsString(assessment.HardRuleHits, "phase_error") {
		t.Fatalf("expected hard rule hit phase_error, got %v", assessment.HardRuleHits)
	}
	if containsString(assessment.HardRuleHits, "missing_phase_record") {
		t.Fatalf("did not expect missing_phase_record when all phase records exist, got %v", assessment.HardRuleHits)
	}
	if len(assessment.Reasons) == 0 {
		t.Fatal("expected reasons on hard rule path")
	}
}

func TestConfidenceEngine_Evaluate_PassWhenStrong(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())

	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"近12个月收益18.6%", "规模120亿"},
		},
		BearCase: &Argument{
			Points: []string{"波动率14%", "行业集中度31%"},
		},
		BullRebuttal: &Argument{
			Points: []string{"持仓周转率22%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"历史最大回撤9%"},
		},
		Verdict: &Verdict{
			Confidence: 92,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseDataGather, ParseAttempted: true, ParseOK: false}, // 非 data_gather 才检查
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBullRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseJudgeVerdict, ParseAttempted: true, ParseOK: true},
		},
	}

	assessment := engine.Evaluate(result)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}

	if assessment.Decision != DecisionGatePass {
		t.Fatalf("expected decision pass, got %s", assessment.Decision)
	}
	if assessment.FinalScore != assessment.BaseScore {
		t.Fatalf("expected final score equals base score on non-hard path, base=%d final=%d", assessment.BaseScore, assessment.FinalScore)
	}
	if assessment.BaseScore < DefaultConfidenceConfig().PassThreshold {
		t.Fatalf("expected strong case base score >= pass threshold %d, got %d", DefaultConfidenceConfig().PassThreshold, assessment.BaseScore)
	}
	if len(assessment.HardRuleHits) != 0 {
		t.Fatalf("expected no hard rule hits, got %v", assessment.HardRuleHits)
	}
	if len(assessment.Reasons) != 0 {
		t.Fatalf("expected no reasons on pass path, got %v", assessment.Reasons)
	}
}

func TestConfidenceEngine_Evaluate_ReviewWhenBelowReviewThreshold(t *testing.T) {
	engine := NewConfidenceEngine(ConfidenceConfig{
		ReviewThreshold: 75,
		PassThreshold:   80,
		HardCap:         60,
	})

	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"缺少量化数据支撑"},
		},
		BearCase: &Argument{
			Points: []string{"估值中性"},
		},
		BullRebuttal: &Argument{
			Points: []string{"长期逻辑仍在"},
		},
		BearRebuttal: &Argument{
			Points: []string{"短期波动偏大"},
		},
		Verdict: &Verdict{
			Confidence: 60,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBullRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseJudgeVerdict, ParseAttempted: true, ParseOK: true},
		},
	}

	assessment := engine.Evaluate(result)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.Decision != DecisionGateReview {
		t.Fatalf("expected review, got %s", assessment.Decision)
	}
}

func TestConfidenceEngine_Evaluate_DegradeWhenBetweenThresholds(t *testing.T) {
	engine := NewConfidenceEngine(ConfidenceConfig{
		ReviewThreshold: 60,
		PassThreshold:   80,
		HardCap:         60,
	})

	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"上涨趋势延续", "估值偏低"},
		},
		BearCase: &Argument{
			Points: []string{"风险未消除", "行业轮动"},
		},
		BullRebuttal: &Argument{
			Points: []string{"仓位30%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"回撤8%"},
		},
		Verdict: &Verdict{
			Confidence: 72,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBullRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseJudgeVerdict, ParseAttempted: true, ParseOK: true},
		},
	}

	assessment := engine.Evaluate(result)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.Decision != DecisionGateDegrade {
		t.Fatalf("expected degrade in gray zone, got %s", assessment.Decision)
	}
}

func TestConfidenceEngine_Evaluate_NilResultHardRule(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())
	assessment := engine.Evaluate(nil)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if assessment.Decision != DecisionGateDegrade {
		t.Fatalf("expected degrade for nil input, got %s", assessment.Decision)
	}
	if !containsString(assessment.HardRuleHits, "nil_result") {
		t.Fatalf("expected nil_result hard rule, got %v", assessment.HardRuleHits)
	}
}

func TestConfidenceEngine_Evaluate_MissingFundInfoHardRule(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())
	result := &DebateResult{
		DataAvailability: DataAvailability{
			HasFundInfo: false,
			HasNAV:      true,
		},
		Verdict: &Verdict{
			Confidence: 55,
		},
	}
	assessment := engine.Evaluate(result)
	if assessment == nil {
		t.Fatal("assessment is nil")
	}
	if !containsString(assessment.HardRuleHits, "missing_fund_info") {
		t.Fatalf("expected missing_fund_info, got %v", assessment.HardRuleHits)
	}
}

func TestConfidenceEngine_Evaluate_MissingVerdictHardRule(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())
	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"近12个月收益18.6%"},
		},
		BearCase: &Argument{
			Points: []string{"波动率14%"},
		},
		BullRebuttal: &Argument{
			Points: []string{"持仓周转率22%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"历史最大回撤9%"},
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBullRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearRebuttal, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseJudgeVerdict, ParseAttempted: true, ParseOK: true},
		},
	}
	assessment := engine.Evaluate(result)
	if assessment.Decision != DecisionGateDegrade {
		t.Fatalf("expected degrade when verdict missing, got %s", assessment.Decision)
	}
	if !containsString(assessment.HardRuleHits, "missing_verdict") {
		t.Fatalf("expected missing_verdict, got %v", assessment.HardRuleHits)
	}
}

func TestConfidenceEngine_Evaluate_MissingPhaseRecordHardRule(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())
	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"近12个月收益18.6%"},
		},
		BearCase: &Argument{
			Points: []string{"波动率14%"},
		},
		BullRebuttal: &Argument{
			Points: []string{"持仓周转率22%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"历史最大回撤9%"},
		},
		Verdict: &Verdict{
			Confidence: 92,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseBullCase, ParseAttempted: true, ParseOK: true},
			{Phase: PhaseBearCase, ParseAttempted: true, ParseOK: true},
			// 缺失 BullRebuttal/BearRebuttal/Judge 记录
		},
	}
	assessment := engine.Evaluate(result)
	if assessment.Decision != DecisionGateDegrade {
		t.Fatalf("expected degrade when phase records are missing, got %s", assessment.Decision)
	}
	if !containsString(assessment.HardRuleHits, "missing_phase_record") {
		t.Fatalf("expected missing_phase_record, got %v", assessment.HardRuleHits)
	}
}

func TestConfidenceEngine_Evaluate_DoesNotTreatUnsetParseOKAsFailure(t *testing.T) {
	engine := NewConfidenceEngine(DefaultConfidenceConfig())
	result := &DebateResult{
		BullCase: &Argument{
			Points: []string{"近12个月收益18.6%"},
		},
		BearCase: &Argument{
			Points: []string{"波动率14%"},
		},
		BullRebuttal: &Argument{
			Points: []string{"持仓周转率22%"},
		},
		BearRebuttal: &Argument{
			Points: []string{"历史最大回撤9%"},
		},
		Verdict: &Verdict{
			Confidence: 90,
		},
		DataAvailability: DataAvailability{
			HasFundInfo: true,
			HasNAV:      true,
		},
		Phases: []PhaseRecord{
			{Phase: PhaseBullCase},
			{Phase: PhaseBearCase},
			{Phase: PhaseBullRebuttal},
			{Phase: PhaseBearRebuttal},
			{Phase: PhaseJudgeVerdict},
		},
	}

	assessment := engine.Evaluate(result)
	if containsString(assessment.HardRuleHits, "parse_failure") {
		t.Fatalf("did not expect parse_failure when ParseOK is unset for all phases, got %v", assessment.HardRuleHits)
	}
}

func TestNormalizeConfidenceConfig_AdjustsInvalidThresholdOrder(t *testing.T) {
	cfg := normalizeConfidenceConfig(ConfidenceConfig{
		ReviewThreshold: 75,
		PassThreshold:   70,
		HardCap:         60,
	})
	if cfg.ReviewThreshold >= cfg.PassThreshold {
		t.Fatalf("expected review threshold < pass threshold, got review=%d pass=%d", cfg.ReviewThreshold, cfg.PassThreshold)
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
