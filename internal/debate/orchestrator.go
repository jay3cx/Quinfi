// Package debate 提供辩论编排器
package debate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/jay3cx/Quinfi/internal/agent"
	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Orchestrator 多空辩论编排器
// 用同一个 LLM 客户端，通过 prompt 切换角色，编排 6 个辩论阶段
type Orchestrator struct {
	client     llm.Client          // LLM 客户端（支持 Tool Calling）
	tools      *agent.ToolRegistry // 复用 Agent 的工具（获取基金数据）
	confidence *ConfidenceEngine
	reviewer   reviewJudgeRunner
}

// NewOrchestrator 创建辩论编排器
func NewOrchestrator(client llm.Client, tools *agent.ToolRegistry) *Orchestrator {
	return NewOrchestratorWithConfidence(client, tools, DefaultConfidenceConfig())
}

// NewOrchestratorWithConfidence 创建带置信度门控配置的辩论编排器。
func NewOrchestratorWithConfidence(client llm.Client, tools *agent.ToolRegistry, cfg ConfidenceConfig) *Orchestrator {
	return &Orchestrator{
		client:     client,
		tools:      tools,
		confidence: NewConfidenceEngine(cfg),
		reviewer:   NewLLMReviewer(client),
	}
}

// DebateProgressFunc 辩论阶段进度回调
// phase: 当前完成的阶段，arg: 论点结果（Bull/Bear 阶段），verdict: 裁决结果（Judge 阶段）
type DebateProgressFunc func(phase Phase, arg *Argument, verdict *Verdict)

// phaseOut 并发阶段的返回结果
type phaseOut struct {
	arg *Argument
	raw string
	err error
}

type reviewJudgeRunner interface {
	ReviewJudge(ctx context.Context, prompt string) (*Verdict, error)
}

// RunDebate 执行完整辩论流程
// 6 个阶段：数据收集 → Bull立论‖Bear立论 → Bull反驳‖Bear反驳 → Judge裁决
// Bull/Bear 立论和反驳分别并发执行，缩短总耗时约 40%
// 可选 onPhase 回调，每个阶段完成后通知调用方
func (o *Orchestrator) RunDebate(ctx context.Context, fundCode string, onPhase ...DebateProgressFunc) (*DebateResult, error) {
	result := &DebateResult{
		FundCode:  fundCode,
		StartedAt: time.Now(),
		Phases:    make([]PhaseRecord, 0, 6),
	}
	var phasesMu sync.Mutex // 保护 result.Phases 的并发 append

	logger.Info("开始多空辩论", zap.String("fund_code", fundCode))

	// 阶段进度通知
	notifyPhase := func(phase Phase, arg *Argument, verdict *Verdict) {
		if len(onPhase) > 0 && onPhase[0] != nil {
			onPhase[0](phase, arg, verdict)
		}
	}

	// Phase 1: 数据收集
	fundCtx, err := o.gatherData(ctx, fundCode, result)
	if err != nil {
		result.Error = fmt.Sprintf("数据收集失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, err
	}
	result.FundName = fundCtx.FundName
	notifyPhase(PhaseDataGather, nil, nil)

	dataText := fundCtx.FormatForLLM()

	debateModel := llm.ModelClaudeOpus46

	// Phase 2 & 3: Bull 立论 + Bear 立论（并发执行）
	bullCaseCh := make(chan phaseOut, 1)
	bearCaseCh := make(chan phaseOut, 1)

	go func() {
		arg, raw, err := o.runPhase(ctx, result, PhaseBullCase,
			BullSystemPrompt, buildBullCasePrompt(dataText), debateModel, &phasesMu)
		bullCaseCh <- phaseOut{arg, raw, err}
	}()
	go func() {
		arg, raw, err := o.runPhase(ctx, result, PhaseBearCase,
			BearSystemPrompt, buildBearCasePrompt(dataText), debateModel, &phasesMu)
		bearCaseCh <- phaseOut{arg, raw, err}
	}()

	bullCaseOut := <-bullCaseCh
	bearCaseOut := <-bearCaseCh

	// 处理 Bull 立论结果
	if bullCaseOut.err != nil {
		result.Error = fmt.Sprintf("Bull 立论失败: %s", bullCaseOut.err.Error())
		result.CompletedAt = time.Now()
		return result, bullCaseOut.err
	}
	result.BullCase = bullCaseOut.arg
	notifyPhase(PhaseBullCase, bullCaseOut.arg, nil)

	// 处理 Bear 立论结果
	if bearCaseOut.err != nil {
		result.Error = fmt.Sprintf("Bear 立论失败: %s", bearCaseOut.err.Error())
		result.CompletedAt = time.Now()
		return result, bearCaseOut.err
	}
	result.BearCase = bearCaseOut.arg
	notifyPhase(PhaseBearCase, bearCaseOut.arg, nil)

	bullRaw := bullCaseOut.raw
	bearRaw := bearCaseOut.raw

	// Phase 4 & 5: Bull 反驳 + Bear 反驳（并发执行）
	bullRebutCh := make(chan phaseOut, 1)
	bearRebutCh := make(chan phaseOut, 1)

	go func() {
		arg, raw, err := o.runPhase(ctx, result, PhaseBullRebuttal,
			BullSystemPrompt, buildBullRebuttalPrompt(dataText, bearRaw), debateModel, &phasesMu)
		bullRebutCh <- phaseOut{arg, raw, err}
	}()
	go func() {
		arg, raw, err := o.runPhase(ctx, result, PhaseBearRebuttal,
			BearSystemPrompt, buildBearRebuttalPrompt(dataText, bullRaw), debateModel, &phasesMu)
		bearRebutCh <- phaseOut{arg, raw, err}
	}()

	bullRebutOut := <-bullRebutCh
	bearRebutOut := <-bearRebutCh

	// 处理 Bull 反驳结果
	if bullRebutOut.err != nil {
		result.Error = fmt.Sprintf("Bull 反驳失败: %s", bullRebutOut.err.Error())
		result.CompletedAt = time.Now()
		return result, bullRebutOut.err
	}
	result.BullRebuttal = bullRebutOut.arg
	notifyPhase(PhaseBullRebuttal, bullRebutOut.arg, nil)

	// 处理 Bear 反驳结果
	if bearRebutOut.err != nil {
		result.Error = fmt.Sprintf("Bear 反驳失败: %s", bearRebutOut.err.Error())
		result.CompletedAt = time.Now()
		return result, bearRebutOut.err
	}
	result.BearRebuttal = bearRebutOut.arg
	notifyPhase(PhaseBearRebuttal, bearRebutOut.arg, nil)

	bullRebutRaw := bullRebutOut.raw
	bearRebutRaw := bearRebutOut.raw

	// Phase 6: Judge 裁决
	verdict, err := o.runJudgePhase(ctx, result,
		buildJudgePrompt(dataText, bullRaw, bearRaw, bullRebutRaw, bearRebutRaw), &phasesMu)
	if err != nil {
		result.Error = fmt.Sprintf("Judge 裁决失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, err
	}
	result.Verdict = verdict
	o.applyConfidenceGate(ctx, result, buildJudgePrompt(dataText, bullRaw, bearRaw, bullRebutRaw, bearRebutRaw))
	notifyPhase(PhaseJudgeVerdict, nil, result.Verdict)

	result.CompletedAt = time.Now()
	logger.Info("辩论完成",
		zap.String("fund_code", fundCode),
		zap.Duration("duration", result.CompletedAt.Sub(result.StartedAt)),
		zap.Int("phases", len(result.Phases)),
	)

	return result, nil
}

// gatherData 数据收集阶段：复用现有工具获取基金数据
func (o *Orchestrator) gatherData(ctx context.Context, fundCode string, result *DebateResult) (*FundContext, error) {
	phaseStart := time.Now()
	defer func() {
		result.Phases = append(result.Phases, PhaseRecord{
			Phase:       PhaseDataGather,
			StartedAt:   phaseStart,
			CompletedAt: time.Now(),
		})
	}()

	logger.Info("辩论数据收集", zap.String("fund_code", fundCode))

	fundCtx := &FundContext{FundCode: fundCode}
	args := map[string]any{"code": fundCode}

	// 获取基金信息
	if info, err := o.tools.Execute(ctx, "get_fund_info", args); err == nil {
		fundCtx.Info = info
		result.DataAvailability.HasFundInfo = strings.TrimSpace(info) != ""
		// 尝试从 JSON 中提取基金名称
		fundCtx.FundName = extractFundName(info)
	} else {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	// 获取净值走势
	navArgs := map[string]any{"code": fundCode, "days": float64(30)}
	if nav, err := o.tools.Execute(ctx, "get_nav_history", navArgs); err == nil {
		fundCtx.NAV = nav
		result.DataAvailability.HasNAV = strings.TrimSpace(nav) != ""
	}

	// 获取持仓
	if holdings, err := o.tools.Execute(ctx, "get_fund_holdings", args); err == nil {
		fundCtx.Holdings = holdings
		result.DataAvailability.HasHoldings = strings.TrimSpace(holdings) != ""
	}

	// 获取新闻
	newsArgs := map[string]any{"keyword": fundCtx.FundName, "limit": float64(5)}
	if news, err := o.tools.Execute(ctx, "search_news", newsArgs); err == nil {
		fundCtx.News = news
		result.DataAvailability.HasNews = strings.TrimSpace(news) != ""
	}

	return fundCtx, nil
}

// runPhase 执行 Bull/Bear 辩论阶段
// 返回解析后的 Argument + 原始 JSON 文本（供下一阶段引用）
func (o *Orchestrator) runPhase(
	ctx context.Context,
	result *DebateResult,
	phase Phase,
	systemPrompt string,
	userPrompt string,
	model llm.ModelID,
	phasesMu *sync.Mutex,
) (*Argument, string, error) {
	phaseStart := time.Now()

	logger.Info("辩论阶段开始", zap.String("phase", string(phase)))

	resp, err := o.client.Chat(ctx, &llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userPrompt},
		},
		MaxTokens:   0,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	// 解析 JSON 响应
	parseOK := true
	arg, err := parseArgument(resp.Content)
	if err != nil {
		parseOK = false
		logger.Warn("论点 JSON 解析失败，使用原始文本",
			zap.String("phase", string(phase)),
			zap.Error(err),
		)
		// 降级：用原始文本构建 Argument
		role := "bull"
		if phase == PhaseBearCase || phase == PhaseBearRebuttal {
			role = "bear"
		}
		arg = &Argument{
			Role:       role,
			Position:   "（论点解析失败，以下为原始分析）",
			Points:     []string{resp.Content},
			Confidence: 50,
		}
	}

	phasesMu.Lock()
	result.Phases = append(result.Phases, PhaseRecord{
		Phase:          phase,
		StartedAt:      phaseStart,
		CompletedAt:    time.Now(),
		TokensUsed:     resp.InputTokens + resp.OutputTokens,
		ParseAttempted: true,
		ParseOK:        parseOK,
	})
	phasesMu.Unlock()

	logger.Info("辩论阶段完成",
		zap.String("phase", string(phase)),
		zap.Int("confidence", arg.Confidence),
	)

	return arg, resp.Content, nil
}

// runJudgePhase 执行 Judge 裁决阶段（使用深度推理模型）
func (o *Orchestrator) runJudgePhase(ctx context.Context, result *DebateResult, prompt string, phasesMu *sync.Mutex) (*Verdict, error) {
	phaseStart := time.Now()

	logger.Info("辩论裁决阶段开始")

	resp, err := o.client.Chat(ctx, &llm.ChatRequest{
		Model: llm.ModelClaudeOpus46,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: JudgeSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   0,
		Temperature: 0.3, // 裁决需要更确定性
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	parseOK := true
	verdict, err := parseVerdict(resp.Content)
	if err != nil {
		parseOK = false
		logger.Warn("裁决 JSON 解析失败，使用原始文本", zap.Error(err))
		verdict = &Verdict{
			Summary:      resp.Content,
			Suggestion:   "请参考上述分析自行判断",
			RiskWarnings: []string{"裁决结果解析异常，请谨慎参考"},
			Confidence:   30,
		}
	}

	phasesMu.Lock()
	result.Phases = append(result.Phases, PhaseRecord{
		Phase:          PhaseJudgeVerdict,
		StartedAt:      phaseStart,
		CompletedAt:    time.Now(),
		TokensUsed:     resp.InputTokens + resp.OutputTokens,
		ParseAttempted: true,
		ParseOK:        parseOK,
	})
	phasesMu.Unlock()

	logger.Info("辩论裁决完成", zap.Int("confidence", verdict.Confidence))
	return verdict, nil
}

func (o *Orchestrator) applyConfidenceGate(ctx context.Context, result *DebateResult, reviewPrompt string) {
	if o == nil || result == nil {
		return
	}
	if o.confidence == nil {
		o.confidence = NewConfidenceEngine(DefaultConfidenceConfig())
	}

	assessment := o.confidence.Evaluate(result)
	if assessment.Decision == DecisionGateReview {
		result.ReviewAttempted = true
		reviewStart := time.Now()
		consistency := 0

		if o.reviewer == nil {
			assessment.Reasons = append(assessment.Reasons, "review_unavailable")
		} else {
			reviewVerdict, err := o.reviewer.ReviewJudge(ctx, reviewPrompt)
			if err != nil {
				assessment.Reasons = append(assessment.Reasons, "review_failed")
			} else {
				consistency = ComputeConsistencyScore(result.Verdict, reviewVerdict)
			}
		}

		assessment.ConsistencyScore = consistency
		assessment.FinalScore = clampScore(int(math.Round(
			0.7*float64(assessment.BaseScore) + 0.3*float64(assessment.ConsistencyScore),
		)))

		cfg := normalizeConfidenceConfig(o.confidence.cfg)
		if len(assessment.HardRuleHits) == 0 && assessment.FinalScore >= cfg.PassThreshold {
			assessment.Decision = DecisionGatePass
		} else {
			assessment.Decision = DecisionGateDegrade
		}

		logger.Info("辩论复核完成",
			zap.Duration("review_latency", time.Since(reviewStart)),
			zap.Int("consistency_score", assessment.ConsistencyScore),
		)
	}

	result.SystemConfidence = assessment.FinalScore
	result.DecisionGate = assessment.Decision
	result.ConfidenceReasons = append([]string(nil), assessment.Reasons...)
	result.ConfidenceDetail = assessment

	if result.DecisionGate == DecisionGateDegrade && result.Verdict != nil {
		result.Verdict.Suggestion = "证据不足，建议观望"
	}

	logger.Info("辩论置信度门控",
		zap.Int("s0", assessment.BaseScore),
		zap.Int("s1", assessment.FinalScore),
		zap.String("decision", string(assessment.Decision)),
		zap.Strings("hard_rule_hits", assessment.HardRuleHits),
		zap.Bool("review_attempted", result.ReviewAttempted),
	)
}

// === JSON 解析辅助函数 ===

func parseArgument(content string) (*Argument, error) {
	jsonStr := extractJSON(content)
	var arg Argument
	if err := json.Unmarshal([]byte(jsonStr), &arg); err != nil {
		return nil, fmt.Errorf("Argument 解析失败: %w", err)
	}
	if arg.Position == "" && len(arg.Points) == 0 {
		return nil, fmt.Errorf("Argument 内容为空")
	}
	return &arg, nil
}

func parseVerdict(content string) (*Verdict, error) {
	jsonStr := extractJSON(content)
	var v Verdict
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return nil, fmt.Errorf("Verdict 解析失败: %w", err)
	}
	if v.Summary == "" {
		return nil, fmt.Errorf("Verdict 内容为空")
	}
	return &v, nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// 移除 markdown 代码块
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	s = strings.TrimSpace(s)

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func extractFundName(info string) string {
	// 简单尝试从 JSON 中提取 name 字段
	var fund struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(info), &fund); err == nil && fund.Name != "" {
		return fund.Name
	}
	return ""
}
