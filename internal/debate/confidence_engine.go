package debate

import (
	"math"
	"strings"
	"unicode"
)

// ConfidenceConfig 系统置信度评估配置。
type ConfidenceConfig struct {
	ReviewThreshold int
	PassThreshold   int
	HardCap         int
}

// DefaultConfidenceConfig 返回默认门控阈值。
func DefaultConfidenceConfig() ConfidenceConfig {
	return ConfidenceConfig{
		ReviewThreshold: 70,
		PassThreshold:   75,
		HardCap:         60,
	}
}

// ConfidenceEngine 计算辩论结果的系统置信度与门控决策。
type ConfidenceEngine struct {
	cfg ConfidenceConfig
}

// NewConfidenceEngine 创建置信度引擎。
func NewConfidenceEngine(cfg ConfidenceConfig) *ConfidenceEngine {
	return &ConfidenceEngine{cfg: normalizeConfidenceConfig(cfg)}
}

// Evaluate 对辩论结果执行 S0 评分与硬规则判定。
func (e *ConfidenceEngine) Evaluate(r *DebateResult) *ConfidenceAssessment {
	cfg := normalizeConfidenceConfig(DefaultConfidenceConfig())
	if e != nil {
		cfg = normalizeConfidenceConfig(e.cfg)
	}

	assessment := &ConfidenceAssessment{
		EvidenceScore:  calculateEvidenceScore(r),
		IntegrityScore: calculateIntegrityScore(r),
		SelfScore:      calculateSelfScore(r),
	}
	assessment.BaseScore = int(math.Round(
		0.5*float64(assessment.EvidenceScore) +
			0.4*float64(assessment.IntegrityScore) +
			0.1*float64(assessment.SelfScore),
	))
	assessment.FinalScore = assessment.BaseScore

	hardRuleHits, reasons := evaluateHardRules(r)
	if len(hardRuleHits) > 0 {
		assessment.HardRuleHits = hardRuleHits
		assessment.Reasons = reasons
		assessment.FinalScore = minInt(assessment.FinalScore, cfg.HardCap)
		assessment.Decision = DecisionGateDegrade
		return assessment
	}

	if assessment.BaseScore < cfg.ReviewThreshold {
		assessment.Decision = DecisionGateReview
	} else if assessment.BaseScore >= cfg.PassThreshold {
		assessment.Decision = DecisionGatePass
	} else {
		assessment.Decision = DecisionGateDegrade
	}

	return assessment
}

func normalizeConfidenceConfig(cfg ConfidenceConfig) ConfidenceConfig {
	defaultCfg := DefaultConfidenceConfig()
	if cfg.ReviewThreshold <= 0 {
		cfg.ReviewThreshold = defaultCfg.ReviewThreshold
	}
	if cfg.PassThreshold <= 0 {
		cfg.PassThreshold = defaultCfg.PassThreshold
	}
	if cfg.HardCap <= 0 {
		cfg.HardCap = defaultCfg.HardCap
	}

	cfg.ReviewThreshold = clampThreshold(cfg.ReviewThreshold)
	cfg.PassThreshold = clampThreshold(cfg.PassThreshold)
	cfg.HardCap = clampThreshold(cfg.HardCap)

	// 保证 review/pass 形成可区分区间，避免分支退化。
	if cfg.ReviewThreshold >= cfg.PassThreshold {
		if cfg.ReviewThreshold >= 100 {
			cfg.ReviewThreshold = 99
			cfg.PassThreshold = 100
		} else {
			cfg.PassThreshold = cfg.ReviewThreshold + 1
		}
	}
	return cfg
}

func calculateEvidenceScore(r *DebateResult) int {
	if r == nil {
		return 0
	}

	totalPoints := 0
	pointsWithDigit := 0
	sources := []*Argument{r.BullCase, r.BearCase, r.BullRebuttal, r.BearRebuttal}
	for _, arg := range sources {
		if arg == nil {
			continue
		}
		for _, point := range arg.Points {
			if strings.TrimSpace(point) == "" {
				continue
			}
			totalPoints++
			if containsDigit(point) {
				pointsWithDigit++
			}
		}
	}

	if totalPoints == 0 {
		return 0
	}

	return int(math.Round(float64(pointsWithDigit) * 100 / float64(totalPoints)))
}

func calculateIntegrityScore(r *DebateResult) int {
	if r == nil {
		return 0
	}

	available := 0
	if r.BullCase != nil {
		available++
	}
	if r.BearCase != nil {
		available++
	}
	if r.BullRebuttal != nil {
		available++
	}
	if r.BearRebuttal != nil {
		available++
	}
	if r.Verdict != nil {
		available++
	}
	if r.DataAvailability.HasFundInfo {
		available++
	}
	if r.DataAvailability.HasNAV {
		available++
	}

	const totalChecks = 7
	return int(math.Round(float64(available) * 100 / float64(totalChecks)))
}

func calculateSelfScore(r *DebateResult) int {
	if r != nil && r.Verdict != nil {
		return clampScore(r.Verdict.Confidence)
	}
	return 0
}

func evaluateHardRules(r *DebateResult) ([]string, []string) {
	if r == nil {
		return []string{"nil_result"}, []string{"nil_result"}
	}

	hits := make([]string, 0, 4)
	reasons := make([]string, 0, 4)
	appendHardRule := func(code string) {
		hits = append(hits, code)
		reasons = append(reasons, code)
	}

	if !r.DataAvailability.HasFundInfo {
		appendHardRule("missing_fund_info")
	}
	if !r.DataAvailability.HasNAV {
		appendHardRule("missing_nav")
	}
	if r.Verdict == nil {
		appendHardRule("missing_verdict")
	}

	seen := map[Phase]bool{
		PhaseBullCase:      false,
		PhaseBearCase:      false,
		PhaseBullRebuttal:  false,
		PhaseBearRebuttal:  false,
		PhaseJudgeVerdict:  false,
	}
	parseFailed := false
	parseStatusObserved := false
	for _, phase := range r.Phases {
		if phase.Phase == PhaseDataGather {
			continue
		}
		_, trackedPhase := seen[phase.Phase]
		if trackedPhase {
			seen[phase.Phase] = true
			if phase.ParseAttempted {
				parseStatusObserved = true
				if !phase.ParseOK {
					parseFailed = true
				}
			}
		}
	}
	if parseStatusObserved && parseFailed {
		appendHardRule("parse_failure")
	}
	for _, found := range seen {
		if !found {
			appendHardRule("missing_phase_record")
			break
		}
	}

	if strings.TrimSpace(r.Error) != "" {
		appendHardRule("phase_error")
	}

	return hits, reasons
}

func containsDigit(s string) bool {
	for _, ch := range s {
		if unicode.IsDigit(ch) {
			return true
		}
	}
	return false
}

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func clampThreshold(v int) int {
	if v < 1 {
		return 1
	}
	if v > 100 {
		return 100
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
