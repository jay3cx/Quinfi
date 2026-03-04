package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserHolding 用户持仓记录
type UserHolding struct {
	ID        int64     `json:"id"`
	FundCode  string    `json:"fund_code"`
	FundName  string    `json:"fund_name"`
	Shares    float64   `json:"shares"`
	Cost      float64   `json:"cost"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserHoldingsRepo 用户持仓数据仓库
type UserHoldingsRepo struct {
	db *sql.DB
}

// NewUserHoldingsRepo 创建用户持仓仓库
func NewUserHoldingsRepo(db *sql.DB) *UserHoldingsRepo {
	return &UserHoldingsRepo{db: db}
}

// Upsert 添加或更新持仓（fund_code 唯一）
func (r *UserHoldingsRepo) Upsert(ctx context.Context, code, name string, shares, cost float64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_holdings (fund_code, fund_name, shares, cost, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(fund_code) DO UPDATE SET
		   fund_name = excluded.fund_name,
		   shares = excluded.shares,
		   cost = excluded.cost,
		   updated_at = CURRENT_TIMESTAMP`,
		code, name, shares, cost)
	if err != nil {
		return fmt.Errorf("upsert 持仓失败: %w", err)
	}
	return nil
}

// Delete 删除持仓
func (r *UserHoldingsRepo) Delete(ctx context.Context, code string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_holdings WHERE fund_code = ?`, code)
	if err != nil {
		return fmt.Errorf("删除持仓失败: %w", err)
	}
	return nil
}

// List 查询所有持仓
func (r *UserHoldingsRepo) List(ctx context.Context) ([]UserHolding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, fund_code, fund_name, shares, cost, created_at, updated_at
		 FROM user_holdings ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询持仓失败: %w", err)
	}
	defer rows.Close()

	var holdings []UserHolding
	for rows.Next() {
		var h UserHolding
		if err := rows.Scan(&h.ID, &h.FundCode, &h.FundName, &h.Shares, &h.Cost, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描持仓数据失败: %w", err)
		}
		holdings = append(holdings, h)
	}
	return holdings, nil
}

// UpsertBatch 批量添加/更新持仓
func (r *UserHoldingsRepo) UpsertBatch(ctx context.Context, holdings []UserHolding) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO user_holdings (fund_code, fund_name, shares, cost, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(fund_code) DO UPDATE SET
		   fund_name = excluded.fund_name,
		   shares = excluded.shares,
		   cost = excluded.cost,
		   updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	for _, h := range holdings {
		if _, err := stmt.ExecContext(ctx, h.FundCode, h.FundName, h.Shares, h.Cost); err != nil {
			return fmt.Errorf("批量插入持仓失败: %w", err)
		}
	}

	return tx.Commit()
}
