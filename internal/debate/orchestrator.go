// Package debate 提供辩论编排器
package debate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Orchestrator 多空辩论编排器
// 用同一个 LLM 客户端，通过 prompt 切换角色，编排 6 个辩论阶段
type Orchestrator struct {
	client llm.Client        // LLM 客户端（支持 Tool Calling）
	tools  *agent.ToolRegistry // 复用 Agent 的工具（获取基金数据）
}

// NewOrchestrator 创建辩论编排器
func NewOrchestrator(client llm.Client, tools *agent.ToolRegistry) *Orchestrator {
	return &Orchestrator{
		client: client,
		tools:  tools,
	}
}

// RunDebate 执行完整辩论流程
// 6 个阶段：数据收集 → Bull立论 → Bear立论 → Bull反驳 → Bear反驳 → Judge裁决
func (o *Orchestrator) RunDebate(ctx context.Context, fundCode string) (*DebateResult, error) {
	result := &DebateResult{
		FundCode:  fundCode,
		StartedAt: time.Now(),
		Phases:    make([]PhaseRecord, 0, 6),
	}

	logger.Info("开始多空辩论", zap.String("fund_code", fundCode))

	// Phase 1: 数据收集
	fundCtx, err := o.gatherData(ctx, fundCode, result)
	if err != nil {
		result.Error = fmt.Sprintf("数据收集失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, err
	}
	result.FundName = fundCtx.FundName

	dataText := fundCtx.FormatForLLM()

	debateModel := llm.ModelGemini3ProHigh // 辩论统一使用 Gemini 3 Pro

	// Phase 2: Bull 立论
	bullCase, bullRaw, err := o.runPhase(ctx, result, PhaseBullCase,
		BullSystemPrompt, buildBullCasePrompt(dataText), debateModel)
	if err != nil {
		result.Error = fmt.Sprintf("Bull 立论失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, nil // 返回部分结果，不报错
	}
	result.BullCase = bullCase

	// Phase 3: Bear 立论
	bearCase, bearRaw, err := o.runPhase(ctx, result, PhaseBearCase,
		BearSystemPrompt, buildBearCasePrompt(dataText), debateModel)
	if err != nil {
		result.Error = fmt.Sprintf("Bear 立论失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.BearCase = bearCase

	// Phase 4: Bull 反驳
	bullRebuttal, bullRebutRaw, err := o.runPhase(ctx, result, PhaseBullRebuttal,
		BullSystemPrompt, buildBullRebuttalPrompt(dataText, bearRaw), debateModel)
	if err != nil {
		result.Error = fmt.Sprintf("Bull 反驳失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.BullRebuttal = bullRebuttal

	// Phase 5: Bear 反驳
	bearRebuttal, bearRebutRaw, err := o.runPhase(ctx, result, PhaseBearRebuttal,
		BearSystemPrompt, buildBearRebuttalPrompt(dataText, bullRaw), debateModel)
	if err != nil {
		result.Error = fmt.Sprintf("Bear 反驳失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.BearRebuttal = bearRebuttal

	// Phase 6: Judge 裁决（同样使用 Gemini 3 Pro）
	verdict, err := o.runJudgePhase(ctx, result,
		buildJudgePrompt(dataText, bullRaw, bearRaw, bullRebutRaw, bearRebutRaw))
	if err != nil {
		result.Error = fmt.Sprintf("Judge 裁决失败: %s", err.Error())
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.Verdict = verdict

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
		// 尝试从 JSON 中提取基金名称
		fundCtx.FundName = extractFundName(info)
	} else {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	// 获取净值走势
	navArgs := map[string]any{"code": fundCode, "days": float64(30)}
	if nav, err := o.tools.Execute(ctx, "get_nav_history", navArgs); err == nil {
		fundCtx.NAV = nav
	}

	// 获取持仓
	if holdings, err := o.tools.Execute(ctx, "get_fund_holdings", args); err == nil {
		fundCtx.Holdings = holdings
	}

	// 获取新闻
	newsArgs := map[string]any{"keyword": fundCtx.FundName, "limit": float64(5)}
	if news, err := o.tools.Execute(ctx, "search_news", newsArgs); err == nil {
		fundCtx.News = news
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
) (*Argument, string, error) {
	phaseStart := time.Now()

	logger.Info("辩论阶段开始", zap.String("phase", string(phase)))

	resp, err := o.client.Chat(ctx, &llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userPrompt},
		},
		MaxTokens:   2048,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	result.Phases = append(result.Phases, PhaseRecord{
		Phase:       phase,
		StartedAt:   phaseStart,
		CompletedAt: time.Now(),
		TokensUsed:  resp.InputTokens + resp.OutputTokens,
	})

	// 解析 JSON 响应
	arg, err := parseArgument(resp.Content)
	if err != nil {
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

	logger.Info("辩论阶段完成",
		zap.String("phase", string(phase)),
		zap.Int("confidence", arg.Confidence),
	)

	return arg, resp.Content, nil
}

// runJudgePhase 执行 Judge 裁决阶段（使用深度推理模型）
func (o *Orchestrator) runJudgePhase(ctx context.Context, result *DebateResult, prompt string) (*Verdict, error) {
	phaseStart := time.Now()

	logger.Info("辩论裁决阶段开始")

	resp, err := o.client.Chat(ctx, &llm.ChatRequest{
		Model: llm.ModelGemini3ProHigh, // Gemini 3 Pro 裁决
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: JudgeSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.3, // 裁决需要更确定性
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	result.Phases = append(result.Phases, PhaseRecord{
		Phase:       PhaseJudgeVerdict,
		StartedAt:   phaseStart,
		CompletedAt: time.Now(),
		TokensUsed:  resp.InputTokens + resp.OutputTokens,
	})

	verdict, err := parseVerdict(resp.Content)
	if err != nil {
		logger.Warn("裁决 JSON 解析失败，使用原始文本", zap.Error(err))
		verdict = &Verdict{
			Summary:      resp.Content,
			Suggestion:   "请参考上述分析自行判断",
			RiskWarnings: []string{"裁决结果解析异常，请谨慎参考"},
			Confidence:   30,
		}
	}

	logger.Info("辩论裁决完成", zap.Int("confidence", verdict.Confidence))
	return verdict, nil
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
