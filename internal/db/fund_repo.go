package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jay3cx/fundmind/internal/datasource"
)

// FundRepository 基金数据持久化
type FundRepository struct {
	db *sql.DB
}

// NewFundRepository 创建基金数据仓库
func NewFundRepository(db *sql.DB) *FundRepository {
	return &FundRepository{db: db}
}

// SaveNAVHistory 批量保存净值历史（UPSERT）
func (r *FundRepository) SaveNAVHistory(ctx context.Context, code string, navList []datasource.NAV) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO nav_history (fund_code, date, unit_nav, accum_nav, daily_return)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(fund_code, date) DO UPDATE SET
		   unit_nav = excluded.unit_nav,
		   accum_nav = excluded.accum_nav,
		   daily_return = excluded.daily_return`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, nav := range navList {
		if _, err := stmt.ExecContext(ctx, code, nav.Date, nav.UnitNAV, nav.AccumNAV, nav.DailyReturn); err != nil {
			return fmt.Errorf("插入净值失败: %w", err)
		}
	}

	return tx.Commit()
}

// SaveHoldingsSnapshot 保存持仓快照
func (r *FundRepository) SaveHoldingsSnapshot(ctx context.Context, code, quarter string, holdings []datasource.Holding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 删除旧数据
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM holdings_snapshot WHERE fund_code = ? AND quarter = ?`, code, quarter); err != nil {
		return fmt.Errorf("删除旧快照失败: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO holdings_snapshot (fund_code, quarter, stock_code, stock_name, ratio, share_count, market_value)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, h := range holdings {
		if _, err := stmt.ExecContext(ctx, code, quarter, h.StockCode, h.StockName, h.Ratio, h.ShareCount, h.MarketValue); err != nil {
			return fmt.Errorf("插入持仓快照失败: %w", err)
		}
	}

	return tx.Commit()
}

// GetHoldingsSnapshot 获取指定季度的持仓快照
func (r *FundRepository) GetHoldingsSnapshot(ctx context.Context, code, quarter string) ([]datasource.Holding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT stock_code, stock_name, ratio, share_count, market_value
		 FROM holdings_snapshot WHERE fund_code = ? AND quarter = ?
		 ORDER BY ratio DESC`, code, quarter)
	if err != nil {
		return nil, fmt.Errorf("查询持仓快照失败: %w", err)
	}
	defer rows.Close()

	var holdings []datasource.Holding
	for rows.Next() {
		var h datasource.Holding
		if err := rows.Scan(&h.StockCode, &h.StockName, &h.Ratio, &h.ShareCount, &h.MarketValue); err != nil {
			return nil, fmt.Errorf("扫描持仓数据失败: %w", err)
		}
		holdings = append(holdings, h)
	}

	return holdings, rows.Err()
}

// GetLatestQuarter 获取最新的持仓快照季度
func (r *FundRepository) GetLatestQuarter(ctx context.Context, code string) (string, error) {
	var quarter string
	err := r.db.QueryRowContext(ctx,
		`SELECT quarter FROM holdings_snapshot WHERE fund_code = ? ORDER BY quarter DESC LIMIT 1`, code).Scan(&quarter)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return quarter, err
}

// GetPreviousQuarter 获取上一个季度标识（如 2025Q4 → 2025Q3）
func GetPreviousQuarter(current string) string {
	if len(current) < 6 {
		return ""
	}
	year := current[:4]
	q := current[5:]
	switch q {
	case "Q1":
		y := atoi(year) - 1
		return fmt.Sprintf("%dQ4", y)
	case "Q2":
		return year + "Q1"
	case "Q3":
		return year + "Q2"
	case "Q4":
		return year + "Q3"
	}
	return ""
}

// CurrentQuarter 返回当前季度标识 (如 "2026Q1")
func CurrentQuarter() string {
	now := time.Now()
	q := (int(now.Month())-1)/3 + 1
	return fmt.Sprintf("%dQ%d", now.Year(), q)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
