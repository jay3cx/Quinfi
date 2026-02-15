package datasource

import (
	"context"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// DataSourceManager 多数据源管理器（Primary/Fallback 降级策略）
type DataSourceManager struct {
	primary  FundDataSource
	fallback FundDataSource
}

// NewDataSourceManager 创建多源管理器
func NewDataSourceManager(primary, fallback FundDataSource) *DataSourceManager {
	return &DataSourceManager{
		primary:  primary,
		fallback: fallback,
	}
}

func (m *DataSourceManager) GetFundInfo(ctx context.Context, code string) (*Fund, error) {
	fund, err := m.primary.GetFundInfo(ctx, code)
	if err == nil {
		return fund, nil
	}
	logger.Warn("主数据源失败，降级到备用", zap.String("code", code), zap.Error(err))
	if m.fallback != nil {
		return m.fallback.GetFundInfo(ctx, code)
	}
	return nil, err
}

func (m *DataSourceManager) GetFundNAV(ctx context.Context, code string, days int) ([]NAV, error) {
	nav, err := m.primary.GetFundNAV(ctx, code, days)
	if err == nil {
		return nav, nil
	}
	logger.Warn("主数据源净值获取失败，降级到备用", zap.String("code", code), zap.Error(err))
	if m.fallback != nil {
		return m.fallback.GetFundNAV(ctx, code, days)
	}
	return nil, err
}

func (m *DataSourceManager) GetFundManager(ctx context.Context, code string) (*Manager, error) {
	mgr, err := m.primary.GetFundManager(ctx, code)
	if err == nil {
		return mgr, nil
	}
	logger.Warn("主数据源经理获取失败，降级到备用", zap.String("code", code), zap.Error(err))
	if m.fallback != nil {
		return m.fallback.GetFundManager(ctx, code)
	}
	return nil, err
}

func (m *DataSourceManager) GetFundHoldings(ctx context.Context, code string) ([]Holding, error) {
	holdings, err := m.primary.GetFundHoldings(ctx, code)
	if err == nil {
		return holdings, nil
	}
	logger.Warn("主数据源持仓获取失败，降级到备用", zap.String("code", code), zap.Error(err))
	if m.fallback != nil {
		return m.fallback.GetFundHoldings(ctx, code)
	}
	return nil, err
}
