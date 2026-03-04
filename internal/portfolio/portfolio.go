// Package portfolio 提供持仓管理
package portfolio

import (
	"context"
	"fmt"
	"strings"

	"github.com/jay3cx/Quinfi/internal/db"
)

// Holding 单只持仓
type Holding struct {
	Code   string  `json:"code"`   // 基金代码
	Name   string  `json:"name"`   // 基金名称
	Shares float64 `json:"shares"` // 持有份额
	Cost   float64 `json:"cost"`   // 持仓成本（元）
	Amount float64 `json:"amount"` // 当前市值（元），= shares × 最新 unit_nav
	Weight float64 `json:"weight"` // 仓位比例（%），自动计算
}

// Portfolio 用户持仓组合
type Portfolio struct {
	Holdings   []Holding `json:"holdings"`
	TotalValue float64   `json:"total_value"` // 总市值（元）
}

// navQuerier 查询最新净值的接口
type navQuerier interface {
	GetLatestUnitNAV(ctx context.Context, code string) (float64, error)
}

// Manager 持仓管理器
// 从 user_holdings 表读取持仓，动态查询最新净值计算市值
type Manager struct {
	holdingsRepo *db.UserHoldingsRepo
	navQuery     navQuerier
}

// NewManager 创建持仓管理器
func NewManager(repo *db.UserHoldingsRepo, navQuery navQuerier) *Manager {
	return &Manager{holdingsRepo: repo, navQuery: navQuery}
}

// GetPortfolio 获取当前持仓组合
func (m *Manager) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	if m.holdingsRepo == nil {
		return &Portfolio{}, nil
	}

	rows, err := m.holdingsRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询持仓失败: %w", err)
	}

	holdings := make([]Holding, 0, len(rows))
	totalValue := 0.0

	for _, row := range rows {
		h := Holding{
			Code:   row.FundCode,
			Name:   row.FundName,
			Shares: row.Shares,
			Cost:   row.Cost,
		}

		// 动态计算市值: shares × 最新 unit_nav
		if m.navQuery != nil && row.Shares > 0 {
			nav, err := m.navQuery.GetLatestUnitNAV(ctx, row.FundCode)
			if err == nil && nav > 0 {
				h.Amount = row.Shares * nav
			}
		}
		// 如果没有净值数据，回退到成本作为市值
		if h.Amount == 0 && h.Cost > 0 {
			h.Amount = h.Cost
		}

		totalValue += h.Amount
		holdings = append(holdings, h)
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
		if h.Shares > 0 {
			sb.WriteString(fmt.Sprintf(" 份额%.2f", h.Shares))
		}
		if h.Cost > 0 {
			profit := h.Amount - h.Cost
			if profit != 0 {
				sign := "+"
				if profit < 0 {
					sign = ""
				}
				sb.WriteString(fmt.Sprintf(" 持有收益%s%.2f元", sign, profit))
			}
			if h.Cost > 0 {
				rate := (h.Amount - h.Cost) / h.Cost * 100
				sb.WriteString(fmt.Sprintf("(%.2f%%)", rate))
			}
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
