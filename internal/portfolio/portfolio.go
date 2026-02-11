// Package portfolio 提供持仓管理
package portfolio

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jay3cx/fundmind/internal/memory"
)

// Holding 单只持仓
type Holding struct {
	Code            string  `json:"code"`                        // 基金代码
	Name            string  `json:"name"`                        // 基金名称
	Amount          float64 `json:"amount"`                      // 持有金额（元）
	Weight          float64 `json:"weight"`                      // 仓位比例（%），自动计算
	DailyReturn     float64 `json:"daily_return,omitempty"`      // 当日收益（元）
	TotalProfit     float64 `json:"total_profit,omitempty"`      // 持有收益（元）
	TotalProfitRate float64 `json:"total_profit_rate,omitempty"` // 持有收益率（%）
}

// Portfolio 用户持仓组合
type Portfolio struct {
	Holdings   []Holding `json:"holdings"`
	TotalValue float64   `json:"total_value"` // 总市值（元）
}

// Manager 持仓管理器
// 从记忆系统中读取用户持仓信息
type Manager struct {
	memoryStore *memory.Store
}

// NewManager 创建持仓管理器
func NewManager(memStore *memory.Store) *Manager {
	return &Manager{memoryStore: memStore}
}

// GetPortfolio 获取当前持仓组合
func (m *Manager) GetPortfolio(ctx context.Context, userID string) (*Portfolio, error) {
	if m.memoryStore == nil {
		return &Portfolio{}, nil
	}

	if userID == "" {
		userID = "default"
	}

	memories, err := m.memoryStore.Recall(ctx, userID, "持有 基金 持仓", 20)
	if err != nil {
		return nil, fmt.Errorf("获取持仓记忆失败: %w", err)
	}

	var holdings []Holding
	seen := make(map[string]bool)
	totalValue := 0.0

	for _, mem := range memories {
		if mem.Type != memory.TypeFact {
			continue
		}
		if !strings.Contains(mem.Content, "持有") {
			continue
		}

		// 从记忆文本中提取基金代码、名称和金额
		h := parseHoldingFromMemory(mem.Content)
		if h != nil && !seen[h.Code] {
			holdings = append(holdings, *h)
			seen[h.Code] = true
			totalValue += h.Amount
		}
	}

	// 计算仓位比例
	if totalValue > 0 {
		for i := range holdings {
			holdings[i].Weight = holdings[i].Amount / totalValue * 100
		}
	}

	return &Portfolio{Holdings: holdings, TotalValue: totalValue}, nil
}

// FormatAsText 格式化持仓为文本
func (p *Portfolio) FormatAsText() string {
	if len(p.Holdings) == 0 {
		return "暂无持仓记录。可以告诉我你持有哪些基金，如：\"我持有 005827\"，或者发送持仓截图。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前持仓（共 %d 只基金", len(p.Holdings)))
	if p.TotalValue > 0 {
		sb.WriteString(fmt.Sprintf("，总市值 %.2f 元", p.TotalValue))
	}
	sb.WriteString("）：\n")

	for i, h := range p.Holdings {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, h.Code))
		if h.Name != "" {
			sb.WriteString(" " + h.Name)
		}
		if h.Amount > 0 {
			sb.WriteString(fmt.Sprintf(" ¥%.2f", h.Amount))
		}
		if h.Weight > 0 {
			sb.WriteString(fmt.Sprintf(" (%.1f%%)", h.Weight))
		}
		// 盈亏信息
		if h.TotalProfit != 0 {
			sign := "+"
			if h.TotalProfit < 0 {
				sign = ""
			}
			sb.WriteString(fmt.Sprintf(" 持有收益%s%.2f元", sign, h.TotalProfit))
		}
		if h.TotalProfitRate != 0 {
			sb.WriteString(fmt.Sprintf("(%.2f%%)", h.TotalProfitRate))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FundCodes 返回所有持仓基金代码
func (p *Portfolio) FundCodes() []string {
	codes := make([]string, len(p.Holdings))
	for i, h := range p.Holdings {
		codes[i] = h.Code
	}
	return codes
}

// FormatMemoryContent 格式化为记忆内容（供写入记忆系统）
// 格式: "用户持有 018490 万家中证工业有色金属主题ETF 金额1011.29元 持有收益11.29元 收益率1.13%"
func FormatMemoryContent(code, name string, amount float64) string {
	content := "用户持有 " + code
	if name != "" {
		content += " " + name
	}
	if amount > 0 {
		content += fmt.Sprintf(" 金额%.2f元", amount)
	}
	return content
}

// FormatMemoryContentFull 格式化为记忆内容（含盈亏信息）
func FormatMemoryContentFull(code, name string, amount, totalProfit, totalProfitRate float64) string {
	content := FormatMemoryContent(code, name, amount)
	if totalProfit != 0 {
		content += fmt.Sprintf(" 持有收益%.2f元", totalProfit)
	}
	if totalProfitRate != 0 {
		content += fmt.Sprintf(" 收益率%.2f%%", totalProfitRate)
	}
	return content
}

// parseHoldingFromMemory 从记忆文本中提取持仓信息
// 支持格式:
//   - "用户持有 018490 万家中证工业有色金属主题ETF 金额1011.29元"
//   - "用户持有 018490 万家中证工业有色金属主题ETF"（无金额）
//   - "用户持有 018490"（仅代码）
func parseHoldingFromMemory(content string) *Holding {
	words := strings.Fields(content)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) == 6 && isAllDigits(w) {
			h := &Holding{Code: w}
			idx := strings.Index(content, w)
			if idx >= 0 {
				rest := strings.TrimSpace(content[idx+6:])
				// 提取基金名称（中文部分）
				name := extractChineseName(rest)
				if name != "" {
					h.Name = name
				}
				// 提取金额: "金额1011.29元"
				h.Amount = extractTaggedFloat(rest, "金额")
				// 提取盈亏: "持有收益11.29元"
				h.TotalProfit = extractTaggedFloat(rest, "持有收益")
				// 提取收益率: "收益率1.13%"
				h.TotalProfitRate = extractTaggedFloat(rest, "收益率")
			}
			return h
		}
	}
	return nil
}

// extractTaggedFloat 从文本中提取"标签XXX"格式的数值
// 如 extractTaggedFloat("金额1011.29元 持有收益-159.13元", "持有收益") → -159.13
func extractTaggedFloat(s, tag string) float64 {
	idx := strings.Index(s, tag)
	if idx < 0 {
		return 0
	}
	rest := s[idx+len(tag):]
	// 提取数字部分（支持负数和小数）
	numStr := ""
	for _, r := range rest {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			numStr += string(r)
		} else if len(numStr) > 0 {
			break
		}
	}
	if numStr == "" || numStr == "-" {
		return 0
	}
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	return f
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func extractChineseName(s string) string {
	// 提取开头连续的中文/英文字母文本作为名称（到"金额"或行尾为止）
	if amtIdx := strings.Index(s, "金额"); amtIdx > 0 {
		s = strings.TrimSpace(s[:amtIdx])
	}

	var name []rune
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff || // CJK 统一汉字
			r >= 0x3000 && r <= 0x303f || // CJK 标点
			(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || // 英文
			r == '(' || r == ')' || r == '（' || r == '）' {
			name = append(name, r)
		} else if len(name) > 0 {
			break
		}
	}
	return string(name)
}
