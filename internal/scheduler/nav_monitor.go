// Package scheduler 提供净值异动监控任务
package scheduler

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/internal/memory"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

const (
	// 日涨跌幅超过此阈值触发预警
	navAlertThreshold = 2.0 // 2%
)

// NAVMonitorTask 净值异动监控任务
// 检查持仓基金的最新净值，异常波动时触发预警
type NAVMonitorTask struct {
	tools       *agent.ToolRegistry
	memoryStore *memory.Store
	alertFunc   func(ctx context.Context, alert string) error
}

// NewNAVMonitorTask 创建净值监控任务
func NewNAVMonitorTask(
	tools *agent.ToolRegistry,
	memStore *memory.Store,
	alertFunc func(ctx context.Context, alert string) error,
) *NAVMonitorTask {
	return &NAVMonitorTask{
		tools:       tools,
		memoryStore: memStore,
		alertFunc:   alertFunc,
	}
}

func (t *NAVMonitorTask) Name() string { return "nav_monitor" }

func (t *NAVMonitorTask) Run(ctx context.Context) error {
	// 从记忆中获取持仓基金代码
	codes := t.getHoldingCodes(ctx)
	if len(codes) == 0 {
		logger.Info("净值监控：无持仓基金，跳过")
		return nil
	}

	logger.Info("净值监控开始", zap.Int("funds", len(codes)))

	var alerts []string

	for _, code := range codes {
		navData, err := t.tools.Execute(ctx, "get_nav_history", map[string]any{
			"code": code,
			"days": float64(3),
		})
		if err != nil {
			logger.Warn("获取净值失败", zap.String("code", code), zap.Error(err))
			continue
		}

		// 简单检测：从返回文本中查找日涨幅
		alert := t.checkAlert(code, navData)
		if alert != "" {
			alerts = append(alerts, alert)
		}
	}

	if len(alerts) > 0 {
		fullAlert := "# 净值异动预警\n\n" + strings.Join(alerts, "\n\n")
		logger.Warn("检测到净值异动", zap.Int("alerts", len(alerts)))

		if t.alertFunc != nil {
			return t.alertFunc(ctx, fullAlert)
		}
		logger.Info(fullAlert)
	} else {
		logger.Info("净值监控：所有持仓基金正常")
	}

	return nil
}

// getHoldingCodes 从记忆中提取持仓基金代码
func (t *NAVMonitorTask) getHoldingCodes(ctx context.Context) []string {
	if t.memoryStore == nil {
		return nil
	}

	memories, err := t.memoryStore.Recall(ctx, "default", "持有 基金", 20)
	if err != nil {
		return nil
	}

	var codes []string
	seen := make(map[string]bool)

	for _, m := range memories {
		if m.Type != memory.TypeFact {
			continue
		}
		// 从文本中提取 6 位数字基金代码
		for _, word := range strings.Fields(m.Content) {
			word = strings.TrimSpace(word)
			if len(word) == 6 && isDigits(word) && !seen[word] {
				codes = append(codes, word)
				seen[word] = true
			}
		}
	}

	return codes
}

// checkAlert 检查净值数据中是否有异常波动
func (t *NAVMonitorTask) checkAlert(code, navData string) string {
	// 在 nav 工具返回的文本中查找"日涨幅"数据
	// 格式示例: "- 2026-02-06: 单位净值 1.9384, 日涨幅 -3.50%"
	lines := strings.Split(navData, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "日涨幅") {
			continue
		}
		// 提取涨跌幅数值
		pctIdx := strings.Index(line, "日涨幅")
		if pctIdx < 0 {
			continue
		}
		rest := line[pctIdx:]
		pct := extractPercent(rest)
		if math.Abs(pct) >= navAlertThreshold {
			direction := "大涨"
			level := "[关注]"
			if pct < 0 {
				direction = "大跌"
				level = "[预警]"
			}
			return fmt.Sprintf("%s 基金 %s %s %.2f%%\n数据: %s",
				level, code, direction, pct, strings.TrimSpace(line))
		}
		// 只检查最新一条
		break
	}
	return ""
}

// extractPercent 从文本中提取百分比数值
func extractPercent(s string) float64 {
	var num float64
	var negative bool
	started := false

	for _, c := range s {
		if c == '-' && !started {
			negative = true
			continue
		}
		if c >= '0' && c <= '9' || c == '.' {
			started = true
			if c == '.' {
				// 简单处理小数
				continue
			}
			num = num*10 + float64(c-'0')
		} else if started {
			break
		}
	}

	// 粗略处理：假设是个位数百分比
	if num > 100 {
		num = num / 100
	}

	if negative {
		num = -num
	}
	return num
}
