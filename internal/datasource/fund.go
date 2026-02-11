// Package datasource 提供基金数据源接口定义
package datasource

import "context"

// FundDataSource 基金数据源接口
type FundDataSource interface {
	// GetFundInfo 获取基金基本信息
	GetFundInfo(ctx context.Context, code string) (*Fund, error)

	// GetFundNAV 获取基金净值历史
	// days: 获取最近多少天的数据，0 表示全部
	GetFundNAV(ctx context.Context, code string, days int) ([]NAV, error)

	// GetFundManager 获取基金经理信息
	GetFundManager(ctx context.Context, code string) (*Manager, error)

	// GetFundHoldings 获取基金持仓明细
	GetFundHoldings(ctx context.Context, code string) ([]Holding, error)
}

// ErrFundNotFound 基金不存在错误
type ErrFundNotFound struct {
	Code string
}

func (e *ErrFundNotFound) Error() string {
	return "基金不存在: " + e.Code
}

// ErrDataSourceUnavailable 数据源不可用错误
type ErrDataSourceUnavailable struct {
	Source string
	Reason string
}

func (e *ErrDataSourceUnavailable) Error() string {
	return "数据源不可用 [" + e.Source + "]: " + e.Reason
}
