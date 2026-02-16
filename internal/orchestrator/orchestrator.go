package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jay3cx/Quinfi/internal/analyzer"
	"github.com/jay3cx/Quinfi/internal/debate"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Orchestrator 主调度器
type Orchestrator struct {
	fundAnalyzer    analyzer.FundAnalyzer
	managerAnalyzer analyzer.ManagerAnalyzer
	macroAnalyzer   analyzer.MacroAnalyzer
	debater         *debate.Orchestrator
	newsFunc        func(ctx context.Context, keyword string, limit int) ([]string, error)
}

// NewOrchestrator 创建主调度器
func NewOrchestrator(
	fund analyzer.FundAnalyzer,
	manager analyzer.ManagerAnalyzer,
	macro analyzer.MacroAnalyzer,
	debater *debate.Orchestrator,
	newsFunc func(ctx context.Context, keyword string, limit int) ([]string, error),
) *Orchestrator {
	return &Orchestrator{
		fundAnalyzer:    fund,
		managerAnalyzer: manager,
		macroAnalyzer:   macro,
		debater:         debater,
		newsFunc:        newsFunc,
	}
}

// ProgressFunc 进度回调函数，可选附加 metadata（JSON 格式）
type ProgressFunc func(progress int, msg string, metadata ...string)

// DeepAnalysis 深度分析 — 并行调用多个 Agent 后汇总
// onProgress 可为 nil（兼容同步调用）
func (o *Orchestrator) DeepAnalysis(ctx context.Context, code string, onProgress ProgressFunc) (*DeepReport, error) {
	logger.Info("开始深度分析", zap.String("code", code))

	report := &DeepReport{
		FundCode:    code,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

	progress := func(p int, msg string, metadata ...string) {
		if onProgress != nil {
			onProgress(p, msg, metadata...)
		}
	}

	progress(5, "开始深度分析...")

	// Phase 1: 并行调用基金分析、经理分析、获取新闻
	g, gCtx := errgroup.WithContext(ctx)

	// 基金综合分析
	g.Go(func() error {
		result, err := o.fundAnalyzer.Analyze(gCtx, code, false)
		if err != nil {
			logger.Warn("基金分析失败", zap.Error(err))
			return nil // 非致命错误，继续
		}
		report.FundAnalysis = result
		report.FundName = result.FundName
		progress(20, "基金分析完成")
		return nil
	})

	// 基金经理分析
	g.Go(func() error {
		result, err := o.managerAnalyzer.AnalyzeManager(gCtx, code)
		if err != nil {
			logger.Warn("经理分析失败", zap.Error(err))
			return nil
		}
		report.ManagerReport = result
		progress(35, "经理分析完成")
		return nil
	})

	// 调仓检测
	g.Go(func() error {
		result, err := o.fundAnalyzer.DetectRebalance(gCtx, code)
		if err != nil {
			logger.Warn("调仓检测失败", zap.Error(err))
			return nil
		}
		report.RebalanceResult = result
		progress(45, "调仓检测完成")
		return nil
	})

	// 获取新闻
	var news []string
	g.Go(func() error {
		if o.newsFunc != nil {
			var err error
			news, err = o.newsFunc(gCtx, code, 10)
			if err != nil {
				logger.Warn("获取新闻失败", zap.Error(err))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("并行分析失败: %w", err)
	}

	progress(50, "Phase 1 完成，开始宏观研判...")

	// 获取基金名称用于宏观分析
	fundName := report.FundName
	if fundName == "" {
		fundName = code
	}

	// Phase 2: 宏观研判（依赖新闻数据）
	if macroReport, err := o.macroAnalyzer.AnalyzeMacro(ctx, fundName, news); err == nil {
		report.MacroReport = macroReport
	} else {
		logger.Warn("宏观研判失败", zap.Error(err))
	}
	progress(75, "宏观研判完成，开始多空辩论...")

	// Phase 3: 多空辩论（依赖前面的分析结果）
	if o.debater != nil {
		// 辩论阶段进度 → 进度百分比映射（75-98 范围）
		phaseProgress := map[debate.Phase]int{
			debate.PhaseDataGather:    77,
			debate.PhaseBullCase:      80,
			debate.PhaseBearCase:      85,
			debate.PhaseBullRebuttal:  88,
			debate.PhaseBearRebuttal:  92,
			debate.PhaseJudgeVerdict:  98,
		}
		phaseLabels := map[debate.Phase]string{
			debate.PhaseDataGather:    "辩论数据收集完成",
			debate.PhaseBullCase:      "Bull 立论完成",
			debate.PhaseBearCase:      "Bear 立论完成",
			debate.PhaseBullRebuttal:  "Bull 反驳完成",
			debate.PhaseBearRebuttal:  "Bear 反驳完成",
			debate.PhaseJudgeVerdict:  "裁判裁决完成",
		}

		debateResult, err := o.debater.RunDebate(ctx, code, func(phase debate.Phase, arg *debate.Argument, verdict *debate.Verdict) {
			phaseData := map[string]any{
				"type":  "debate_phase",
				"phase": string(phase),
			}
			if arg != nil {
				phaseData["argument"] = arg
			}
			if verdict != nil {
				phaseData["verdict"] = verdict
			}
			metaJSON, _ := json.Marshal(phaseData)

			p := phaseProgress[phase]
			label := phaseLabels[phase]
			progress(p, label, string(metaJSON))
		})
		if err != nil {
			logger.Warn("多空辩论失败", zap.Error(err))
		} else {
			report.DebateResult = debateResult
		}
	}

	progress(100, "深度分析完成")

	logger.Info("深度分析完成",
		zap.String("code", code),
		zap.String("fund_name", report.FundName),
	)

	return report, nil
}
