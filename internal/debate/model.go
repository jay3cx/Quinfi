// Package debate 提供多空辩论系统
package debate

import (
	"strconv"
	"time"
)

// Phase 辩论阶段
type Phase string

const (
	PhaseDataGather   Phase = "data_gather"   // 数据收集
	PhaseBullCase     Phase = "bull_case"     // Bull 立论
	PhaseBearCase     Phase = "bear_case"     // Bear 立论
	PhaseBullRebuttal Phase = "bull_rebuttal" // Bull 反驳
	PhaseBearRebuttal Phase = "bear_rebuttal" // Bear 反驳
	PhaseJudgeVerdict Phase = "judge_verdict" // Judge 裁决
)

// DecisionGate 系统决策门控
type DecisionGate string

const (
	DecisionGateUnknown DecisionGate = "unknown"
	DecisionGatePass    DecisionGate = "pass"
	DecisionGateReview  DecisionGate = "review"
	DecisionGateDegrade DecisionGate = "degrade"
)

// Argument 结构化论点
type Argument struct {
	Role       string   `json:"role"`       // bull / bear
	Position   string   `json:"position"`   // 核心立场（一句话）
	Points     []string `json:"points"`     // 论据列表
	Confidence int      `json:"confidence"` // 置信度 0-100
}

// Verdict 裁判结论
type Verdict struct {
	Summary      string   `json:"summary"`       // 综合结论
	BullStrength string   `json:"bull_strength"` // 看多方最强论点
	BearStrength string   `json:"bear_strength"` // 看空方最强论点
	Suggestion   string   `json:"suggestion"`    // 投资参考建议
	RiskWarnings []string `json:"risk_warnings"` // 风险提示
	Confidence   int      `json:"confidence"`    // 综合置信度 0-100
}

// DataAvailability 辩论输入数据可用性
type DataAvailability struct {
	HasFundInfo bool `json:"has_fund_info"`
	HasNAV      bool `json:"has_nav"`
	HasHoldings bool `json:"has_holdings"`
	HasNews     bool `json:"has_news"`
}

// ConfidenceAssessment 系统置信度评估细节
type ConfidenceAssessment struct {
	EvidenceScore    int          `json:"evidence_score"`
	IntegrityScore   int          `json:"integrity_score"`
	SelfScore        int          `json:"self_score"`
	ConsistencyScore int          `json:"consistency_score,omitempty"`
	BaseScore        int          `json:"s0"` // 初判分（未复核）
	FinalScore       int          `json:"s1"` // 终判分（复核后）
	Decision         DecisionGate `json:"decision"`
	Reasons          []string     `json:"reasons,omitempty"`
	HardRuleHits     []string     `json:"hard_rule_hits,omitempty"`
}

// DebateResult 完整辩论结果
type DebateResult struct {
	FundCode string `json:"fund_code"` // 基金代码
	FundName string `json:"fund_name"` // 基金名称

	BullCase          *Argument             `json:"bull_case"`                    // Bull 立论
	BearCase          *Argument             `json:"bear_case"`                    // Bear 立论
	BullRebuttal      *Argument             `json:"bull_rebuttal"`                // Bull 反驳
	BearRebuttal      *Argument             `json:"bear_rebuttal"`                // Bear 反驳
	Verdict           *Verdict              `json:"verdict"`                      // 裁判结论
	DataAvailability  DataAvailability      `json:"data_availability"`            // 数据可用性
	SystemConfidence  int                   `json:"system_confidence,omitempty"`  // 系统置信度 0-100
	DecisionGate      DecisionGate          `json:"decision_gate,omitempty"`      // 系统门控决策
	ConfidenceReasons []string              `json:"confidence_reasons,omitempty"` // 门控原因
	ConfidenceDetail  *ConfidenceAssessment `json:"confidence_detail,omitempty"`  // 门控评估细节
	ReviewAttempted   bool                  `json:"review_attempted"`             // 是否触发过复审

	Phases      []PhaseRecord `json:"phases"` // 各阶段记录
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Error       string        `json:"error,omitempty"` // 如有错误
}

// PhaseRecord 阶段执行记录
type PhaseRecord struct {
	Phase          Phase     `json:"phase"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	TokensUsed     int       `json:"tokens_used"`
	ParseAttempted bool      `json:"parse_attempted,omitempty"`
	ParseOK        bool      `json:"parse_ok"`
}

// FundContext 辩论用的基金数据上下文
// 数据收集阶段产出，传给 Bull/Bear/Judge 共享同一份事实基础
type FundContext struct {
	FundCode string `json:"fund_code"`
	FundName string `json:"fund_name"`
	Info     string `json:"info"`     // 基金基本信息
	NAV      string `json:"nav"`      // 近期净值走势
	Holdings string `json:"holdings"` // 持仓数据
	News     string `json:"news"`     // 相关资讯
}

// FormatForLLM 将基金上下文格式化为 LLM 可读文本
func (fc *FundContext) FormatForLLM() string {
	var s string
	s += "## 基金基本信息\n" + fc.Info + "\n\n"
	s += "## 近期净值走势\n" + fc.NAV + "\n\n"
	s += "## 持仓数据\n" + fc.Holdings + "\n\n"
	if fc.News != "" {
		s += "## 相关资讯\n" + fc.News + "\n\n"
	}
	return s
}

// FormatAsMarkdown 将辩论结果格式化为 Markdown（供 FundAgent 引用）
func (r *DebateResult) FormatAsMarkdown() string {
	var s string
	s += "## 多空辩论结果：" + r.FundCode + " " + r.FundName + "\n\n"

	if r.BullCase != nil {
		s += "### 📈 看多方观点（置信度: " + strconv.Itoa(r.BullCase.Confidence) + "/100）\n"
		s += "**立场**: " + r.BullCase.Position + "\n"
		for i, p := range r.BullCase.Points {
			s += strconv.Itoa(i+1) + ". " + p + "\n"
		}
		s += "\n"
	}

	if r.BearCase != nil {
		s += "### 📉 看空方观点（置信度: " + strconv.Itoa(r.BearCase.Confidence) + "/100）\n"
		s += "**立场**: " + r.BearCase.Position + "\n"
		for i, p := range r.BearCase.Points {
			s += strconv.Itoa(i+1) + ". " + p + "\n"
		}
		s += "\n"
	}

	if r.BullRebuttal != nil {
		s += "### 看多方反驳\n"
		for _, p := range r.BullRebuttal.Points {
			s += "- " + p + "\n"
		}
		s += "\n"
	}

	if r.BearRebuttal != nil {
		s += "### 看空方反驳\n"
		for _, p := range r.BearRebuttal.Points {
			s += "- " + p + "\n"
		}
		s += "\n"
	}

	if r.Verdict != nil {
		s += "### ⚖️ 裁判结论\n"
		s += r.Verdict.Summary + "\n\n"
		s += "**看多方最强论点**: " + r.Verdict.BullStrength + "\n"
		s += "**看空方最强论点**: " + r.Verdict.BearStrength + "\n"
		s += "**参考建议**: " + r.Verdict.Suggestion + "\n\n"
		if len(r.Verdict.RiskWarnings) > 0 {
			s += "**风险提示**:\n"
			for _, w := range r.Verdict.RiskWarnings {
				s += "- ⚠️ " + w + "\n"
			}
		}
		s += "\n> 以上仅为多角度分析参考，不构成投资建议。\n"
	}

	if r.Error != "" {
		s += "\n> ⚠️ 辩论未完整完成: " + r.Error + "\n"
	}

	return s
}
