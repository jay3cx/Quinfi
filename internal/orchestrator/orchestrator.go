package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/jay3cx/fundmind/internal/analyzer"
	"github.com/jay3cx/fundmind/internal/debate"
	"github.com/jay3cx/fundmind/pkg/logger"
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

// DeepAnalysis 深度分析 — 并行调用多个 Agent 后汇总
func (o *Orchestrator) DeepAnalysis(ctx context.Context, code string) (*DeepReport, error) {
	logger.Info("开始深度分析", zap.String("code", code))

	report := &DeepReport{
		FundCode:    code,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

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

	// Phase 3: 多空辩论（依赖前面的分析结果）
	if o.debater != nil {
		debateResult, err := o.debater.RunDebate(ctx, code)
		if err != nil {
			logger.Warn("多空辩论失败", zap.Error(err))
		} else {
			report.DebateResult = debateResult
		}
	}

	logger.Info("深度分析完成",
		zap.String("code", code),
		zap.String("fund_name", report.FundName),
	)

	return report, nil
}
