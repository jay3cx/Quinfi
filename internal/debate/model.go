// Package debate 提供多空辩论系统
package debate

import "time"

// Phase 辩论阶段
type Phase string

const (
	PhaseDataGather  Phase = "data_gather"  // 数据收集
	PhaseBullCase    Phase = "bull_case"    // Bull 立论
	PhaseBearCase    Phase = "bear_case"    // Bear 立论
	PhaseBullRebuttal Phase = "bull_rebuttal" // Bull 反驳
	PhaseBearRebuttal Phase = "bear_rebuttal" // Bear 反驳
	PhaseJudgeVerdict Phase = "judge_verdict" // Judge 裁决
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
	BullStrength string   `json:"bull_strength"`  // 看多方最强论点
	BearStrength string   `json:"bear_strength"`  // 看空方最强论点
	Suggestion   string   `json:"suggestion"`     // 投资参考建议
	RiskWarnings []string `json:"risk_warnings"`  // 风险提示
	Confidence   int      `json:"confidence"`     // 综合置信度 0-100
}

// DebateResult 完整辩论结果
type DebateResult struct {
	FundCode string `json:"fund_code"` // 基金代码
	FundName string `json:"fund_name"` // 基金名称

	BullCase     *Argument `json:"bull_case"`      // Bull 立论
	BearCase     *Argument `json:"bear_case"`      // Bear 立论
	BullRebuttal *Argument `json:"bull_rebuttal"`  // Bull 反驳
	BearRebuttal *Argument `json:"bear_rebuttal"`  // Bear 反驳
	Verdict      *Verdict  `json:"verdict"`        // 裁判结论

	Phases      []PhaseRecord `json:"phases"`       // 各阶段记录
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Error       string        `json:"error,omitempty"` // 如有错误
}

// PhaseRecord 阶段执行记录
type PhaseRecord struct {
	Phase       Phase         `json:"phase"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	TokensUsed  int           `json:"tokens_used"`
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
		s += "### 📈 看多方观点（置信度: " + itoa(r.BullCase.Confidence) + "/100）\n"
		s += "**立场**: " + r.BullCase.Position + "\n"
		for i, p := range r.BullCase.Points {
			s += itoa(i+1) + ". " + p + "\n"
		}
		s += "\n"
	}

	if r.BearCase != nil {
		s += "### 📉 看空方观点（置信度: " + itoa(r.BearCase.Confidence) + "/100）\n"
		s += "**立场**: " + r.BearCase.Position + "\n"
		for i, p := range r.BearCase.Points {
			s += itoa(i+1) + ". " + p + "\n"
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
		s += "### ⚖️ 裁判结论（置信度: " + itoa(r.Verdict.Confidence) + "/100）\n"
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

// itoa 简单整数转字符串
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
