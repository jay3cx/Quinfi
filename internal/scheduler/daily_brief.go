// Package scheduler 提供每日投资简报任务
package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/internal/memory"
	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// DailyBriefTask 每日投资简报生成任务
// 小基自主执行：扫描持仓净值 + 相关资讯 + 市场概况 → 生成简报
type DailyBriefTask struct {
	client      llm.Client
	tools       *agent.ToolRegistry
	memoryStore *memory.Store
	outputFunc  func(ctx context.Context, brief string) error // 输出回调（写入 Obsidian / 打印 / 存 DB）
}

// NewDailyBriefTask 创建每日简报任务
func NewDailyBriefTask(
	client llm.Client,
	tools *agent.ToolRegistry,
	memStore *memory.Store,
	outputFunc func(ctx context.Context, brief string) error,
) *DailyBriefTask {
	return &DailyBriefTask{
		client:      client,
		tools:       tools,
		memoryStore: memStore,
		outputFunc:  outputFunc,
	}
}

func (t *DailyBriefTask) Name() string { return "daily_brief" }

func (t *DailyBriefTask) Run(ctx context.Context) error {
	logger.Info("开始生成每日投资简报")

	// 1. 收集数据
	var dataBuilder strings.Builder

	// 从记忆中获取用户持仓信息
	holdings := t.getHoldingsFromMemory(ctx)
	if holdings != "" {
		dataBuilder.WriteString("## 用户持仓\n")
		dataBuilder.WriteString(holdings)
		dataBuilder.WriteString("\n\n")
	}

	// 获取持仓基金的最新净值
	fundCodes := t.extractFundCodes(holdings)
	for _, code := range fundCodes {
		if navData, err := t.tools.Execute(ctx, "get_nav_history", map[string]any{"code": code, "days": float64(5)}); err == nil {
			dataBuilder.WriteString(fmt.Sprintf("## 基金 %s 近期净值\n%s\n\n", code, navData))
		}
	}

	// 获取最新资讯
	if news, err := t.tools.Execute(ctx, "search_news", map[string]any{"limit": float64(10)}); err == nil {
		dataBuilder.WriteString("## 最新市场资讯\n")
		dataBuilder.WriteString(news)
		dataBuilder.WriteString("\n\n")
	}

	data := dataBuilder.String()
	if data == "" {
		logger.Info("无数据可用，跳过简报生成")
		return nil
	}

	// 2. 让小基生成简报
	briefPrompt := fmt.Sprintf(`你是首席投研官小基。请基于以下数据生成今日投资简报。

%s

## 输出格式
# 每日投资简报 (%s)

## 持仓概况
用表格展示各持仓基金今日表现：基金代码、基金名称、最新净值、日涨跌、近5日表现。然后逐一点评。

## 市场要闻
与持仓相关的重要新闻，分为利空信息、利好信息、中性信息三类。每条标注事件、影响、与持仓的关联度。

## 关注事项
需要用户关注或操作的事项，如有则标注 [需要关注] 或 [建议操作]。

## 一句话总结
今日市场一句话概括。

## 注意
- 禁止使用任何 emoji 符号
- 持仓表格使用标准 Markdown 表格格式
- 表格内换行使用 HTML <br> 标签`, data, time.Now().Format("2006-01-02"))

	resp, err := t.client.Chat(ctx, &llm.ChatRequest{
		Model:       llm.ModelClaudeSonnet45,
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: briefPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return fmt.Errorf("LLM 生成简报失败: %w", err)
	}

	logger.Info("每日简报生成完成", zap.Int("length", len(resp.Content)))

	// 3. 输出简报
	if t.outputFunc != nil {
		return t.outputFunc(ctx, resp.Content)
	}

	// 默认打印到日志
	logger.Info("每日投资简报:\n" + resp.Content)
	return nil
}

// getHoldingsFromMemory 从记忆系统获取用户持仓信息
func (t *DailyBriefTask) getHoldingsFromMemory(ctx context.Context) string {
	if t.memoryStore == nil {
		return ""
	}

	memories, err := t.memoryStore.Recall(ctx, "default", "持有 基金 持仓", 10)
	if err != nil {
		return ""
	}

	var holdings []string
	for _, m := range memories {
		if m.Type == memory.TypeFact && strings.Contains(m.Content, "持有") {
			holdings = append(holdings, m.Content)
		}
	}

	if len(holdings) == 0 {
		return ""
	}

	return strings.Join(holdings, "\n")
}

// extractFundCodes 从持仓文本中提取基金代码（6位数字）
func (t *DailyBriefTask) extractFundCodes(text string) []string {
	var codes []string
	words := strings.Fields(text)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) == 6 && isDigits(w) {
			codes = append(codes, w)
		}
	}
	return codes
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
