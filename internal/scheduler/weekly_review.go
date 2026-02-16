// Package scheduler 提供周度复盘任务
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/jay3cx/Quinfi/internal/memory"
	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// WeeklyReviewTask 周度复盘任务
// Quinfi 自主执行：回顾本周所有对话和决策 → 评估建议准确率 → 总结经验
type WeeklyReviewTask struct {
	client      llm.Client
	memoryStore *memory.Store
	outputFunc  func(ctx context.Context, review string) error
}

// NewWeeklyReviewTask 创建周度复盘任务
func NewWeeklyReviewTask(
	client llm.Client,
	memStore *memory.Store,
	outputFunc func(ctx context.Context, review string) error,
) *WeeklyReviewTask {
	return &WeeklyReviewTask{
		client:      client,
		memoryStore: memStore,
		outputFunc:  outputFunc,
	}
}

func (t *WeeklyReviewTask) Name() string { return "weekly_review" }

func (t *WeeklyReviewTask) Run(ctx context.Context) error {
	logger.Info("开始生成周度复盘")

	if t.memoryStore == nil {
		logger.Info("无记忆存储，跳过复盘")
		return nil
	}

	// 获取本周所有记忆
	memories, err := t.memoryStore.Recall(ctx, "default", "", 50)
	if err != nil {
		return fmt.Errorf("获取记忆失败: %w", err)
	}

	if len(memories) == 0 {
		logger.Info("无记忆数据，跳过复盘")
		return nil
	}

	// 构建复盘数据
	var memoryText string
	for _, m := range memories {
		memoryText += fmt.Sprintf("- [%s] %s (创建于: %s)\n", m.Type, m.Content, m.CreatedAt.Format("2006-01-02"))
	}

	reviewPrompt := fmt.Sprintf(`你是首席投研官 Quinfi。请基于以下用户的投资记忆数据，生成本周复盘报告。

## 用户记忆数据
%s

## 输出格式
# 📋 周度投资复盘 (%s)

## 本周概况
（本周用户的投资活动概述）

## 决策回顾
（本周做了哪些投资决策，当时的判断依据是什么）

## 经验总结
（哪些判断是对的，哪些需要改进）

## 下周关注
（基于当前持仓和市场情况，下周需要关注什么）

## 一句话总结
（本周投资一句话概括）`, memoryText, time.Now().Format("2006-01-02"))

	resp, err := t.client.Chat(ctx, &llm.ChatRequest{
		Model:       llm.ModelGLM5,
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: reviewPrompt}},
		MaxTokens:   0,
		Temperature: 0.3,
	})
	if err != nil {
		return fmt.Errorf("LLM 生成复盘失败: %w", err)
	}

	logger.Info("周度复盘生成完成", zap.Int("length", len(resp.Content)))

	if t.outputFunc != nil {
		return t.outputFunc(ctx, resp.Content)
	}

	logger.Info("周度复盘:\n" + resp.Content)
	return nil
}
