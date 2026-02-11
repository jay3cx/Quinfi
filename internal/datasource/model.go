// Package datasource 提供基金数据模型定义
package datasource

import "time"

// FundType 基金类型
type FundType string

const (
	FundTypeStock   FundType = "股票型"
	FundTypeMixed   FundType = "混合型"
	FundTypeBond    FundType = "债券型"
	FundTypeMoney   FundType = "货币型"
	FundTypeQDII    FundType = "QDII"
	FundTypeFOF     FundType = "FOF"
	FundTypeIndex   FundType = "指数型"
	FundTypeUnknown FundType = "其他"
)

// Fund 基金基本信息
type Fund struct {
	Code        string    `json:"code"`         // 基金代码（6位数字）
	Name        string    `json:"name"`         // 基金名称
	Type        FundType  `json:"type"`         // 基金类型
	Company     string    `json:"company"`      // 基金公司
	Scale       float64   `json:"scale"`        // 基金规模（亿元）
	EstablishAt time.Time `json:"establish_at"` // 成立日期
	Manager     *Manager  `json:"manager"`      // 基金经理
}

// Manager 基金经理信息
type Manager struct {
	ID          string   `json:"id"`           // 经理ID
	Name        string   `json:"name"`         // 姓名
	Years       float64  `json:"years"`        // 任职年限
	TotalScale  float64  `json:"total_scale"`  // 管理总规模（亿元）
	FundCount   int      `json:"fund_count"`   // 管理基金数量
	BestFunds   []string `json:"best_funds"`   // 代表作品（基金代码列表）
	Background  string   `json:"background"`   // 背景简介
	StartDate   string   `json:"start_date"`   // 任职开始日期
}

// NAV 基金净值
type NAV struct {
	Date        string  `json:"date"`          // 日期 YYYY-MM-DD
	UnitNAV     float64 `json:"unit_nav"`      // 单位净值
	AccumNAV    float64 `json:"accum_nav"`     // 累计净值
	DailyReturn float64 `json:"daily_return"`  // 日涨幅（%）
}

// Holding 基金持仓（股票）
type Holding struct {
	StockCode   string  `json:"stock_code"`   // 股票代码
	StockName   string  `json:"stock_name"`   // 股票名称
	Ratio       float64 `json:"ratio"`        // 持仓占比（%）
	ShareCount  float64 `json:"share_count"`  // 持股数量（万股）
	MarketValue float64 `json:"market_value"` // 持仓市值（万元）
}

// FundDetail 基金详细信息（聚合）
type FundDetail struct {
	Fund     *Fund     `json:"fund"`
	NAVList  []NAV     `json:"nav_list"`
	Holdings []Holding `json:"holdings"`
}
