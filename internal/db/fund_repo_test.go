package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jay3cx/fundmind/internal/datasource"
)

func TestFundRepository_HoldingsSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	sqlDB, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewFundRepository(sqlDB)
	ctx := context.Background()

	// 1. 空表查询应返回空
	holdings, err := repo.GetHoldingsSnapshot(ctx, "022364", "2025Q4")
	if err != nil {
		t.Fatalf("GetHoldingsSnapshot on empty table failed: %v", err)
	}
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings, got %d", len(holdings))
	}

	// 2. 保存 2025Q4 持仓快照
	prevHoldings := []datasource.Holding{
		{StockCode: "600183", StockName: "生益科技", Ratio: 9.11, ShareCount: 1972.29, MarketValue: 140841.24},
		{StockCode: "300308", StockName: "中际旭创", Ratio: 8.85, ShareCount: 224.49, MarketValue: 136937.74},
		{StockCode: "002463", StockName: "沪电股份", Ratio: 8.84, ShareCount: 1872.17, MarketValue: 136799.17},
	}
	if err := repo.SaveHoldingsSnapshot(ctx, "022364", "2025Q4", prevHoldings); err != nil {
		t.Fatalf("SaveHoldingsSnapshot failed: %v", err)
	}

	// 3. 读取 2025Q4 快照
	got, err := repo.GetHoldingsSnapshot(ctx, "022364", "2025Q4")
	if err != nil {
		t.Fatalf("GetHoldingsSnapshot failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 holdings, got %d", len(got))
	}
	// 验证按 ratio DESC 排序
	if got[0].StockCode != "600183" {
		t.Errorf("expected first stock 600183, got %s", got[0].StockCode)
	}
	if got[0].Ratio != 9.11 {
		t.Errorf("expected ratio 9.11, got %.2f", got[0].Ratio)
	}

	// 4. 保存 2026Q1 快照（模拟调仓：新增一只，减持一只）
	currHoldings := []datasource.Holding{
		{StockCode: "600183", StockName: "生益科技", Ratio: 10.5, ShareCount: 2200, MarketValue: 155000},
		{StockCode: "300308", StockName: "中际旭创", Ratio: 5.0, ShareCount: 120, MarketValue: 73000},
		{StockCode: "688981", StockName: "中芯国际", Ratio: 7.2, ShareCount: 500, MarketValue: 100000},
	}
	if err := repo.SaveHoldingsSnapshot(ctx, "022364", "2026Q1", currHoldings); err != nil {
		t.Fatalf("SaveHoldingsSnapshot Q1 failed: %v", err)
	}

	// 5. 验证两期快照独立存在
	q4, _ := repo.GetHoldingsSnapshot(ctx, "022364", "2025Q4")
	q1, _ := repo.GetHoldingsSnapshot(ctx, "022364", "2026Q1")
	if len(q4) != 3 {
		t.Errorf("Q4 should have 3 holdings, got %d", len(q4))
	}
	if len(q1) != 3 {
		t.Errorf("Q1 should have 3 holdings, got %d", len(q1))
	}

	// 6. 验证 GetLatestQuarter
	latest, err := repo.GetLatestQuarter(ctx, "022364")
	if err != nil {
		t.Fatalf("GetLatestQuarter failed: %v", err)
	}
	if latest != "2026Q1" {
		t.Errorf("expected latest quarter 2026Q1, got %s", latest)
	}

	// 7. 验证不存在的基金返回空
	empty, _ := repo.GetHoldingsSnapshot(ctx, "999999", "2025Q4")
	if len(empty) != 0 {
		t.Errorf("expected 0 for non-existent fund, got %d", len(empty))
	}

	// 8. 验证覆盖写入（同季度重新保存）
	updatedHoldings := []datasource.Holding{
		{StockCode: "600183", StockName: "生益科技", Ratio: 12.0, ShareCount: 2500, MarketValue: 180000},
	}
	if err := repo.SaveHoldingsSnapshot(ctx, "022364", "2026Q1", updatedHoldings); err != nil {
		t.Fatalf("SaveHoldingsSnapshot overwrite failed: %v", err)
	}
	q1Updated, _ := repo.GetHoldingsSnapshot(ctx, "022364", "2026Q1")
	if len(q1Updated) != 1 {
		t.Errorf("expected 1 holding after overwrite, got %d", len(q1Updated))
	}
	if q1Updated[0].Ratio != 12.0 {
		t.Errorf("expected updated ratio 12.0, got %.2f", q1Updated[0].Ratio)
	}
}

func TestGetPreviousQuarter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"2026Q1", "2025Q4"},
		{"2026Q2", "2026Q1"},
		{"2026Q3", "2026Q2"},
		{"2026Q4", "2026Q3"},
		{"2025Q1", "2024Q4"},
		{"", ""},
		{"bad", ""},
	}

	for _, tt := range tests {
		got := GetPreviousQuarter(tt.input)
		if got != tt.expected {
			t.Errorf("GetPreviousQuarter(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCurrentQuarter(t *testing.T) {
	q := CurrentQuarter()
	// 2026年2月 → 2026Q1
	if q != "2026Q1" {
		t.Errorf("expected 2026Q1, got %s", q)
	}
}

func TestFundRepository_NAVHistory(t *testing.T) {
	tmpDir := t.TempDir()
	sqlDB, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewFundRepository(sqlDB)
	ctx := context.Background()

	// 保存净值历史
	navList := []datasource.NAV{
		{Date: "2026-02-13", UnitNAV: 1.8805, AccumNAV: 1.8805, DailyReturn: -3.48},
		{Date: "2026-02-12", UnitNAV: 1.9482, AccumNAV: 1.9482, DailyReturn: 0.87},
	}
	if err := repo.SaveNAVHistory(ctx, "018490", navList); err != nil {
		t.Fatalf("SaveNAVHistory failed: %v", err)
	}

	// UPSERT：再次保存应覆盖
	navListUpdated := []datasource.NAV{
		{Date: "2026-02-13", UnitNAV: 1.8900, AccumNAV: 1.8900, DailyReturn: -2.98},
	}
	if err := repo.SaveNAVHistory(ctx, "018490", navListUpdated); err != nil {
		t.Fatalf("SaveNAVHistory upsert failed: %v", err)
	}

	// 验证数据正确覆盖
	var nav float64
	err = sqlDB.QueryRowContext(ctx,
		"SELECT unit_nav FROM nav_history WHERE fund_code = ? AND date = ?",
		"018490", "2026-02-13").Scan(&nav)
	if err != nil {
		t.Fatalf("query nav failed: %v", err)
	}
	if nav != 1.89 {
		t.Errorf("expected upserted nav 1.89, got %.4f", nav)
	}
}

func TestFundRepository_GetNAVHistory(t *testing.T) {
	tmpDir := t.TempDir()
	sqlDB, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewFundRepository(sqlDB)
	ctx := context.Background()

	navList := []datasource.NAV{
		{Date: "2026-01-10", UnitNAV: 1.5000, AccumNAV: 1.5000, DailyReturn: 0.50},
		{Date: "2026-01-11", UnitNAV: 1.5100, AccumNAV: 1.5100, DailyReturn: 0.67},
		{Date: "2026-01-12", UnitNAV: 1.4900, AccumNAV: 1.4900, DailyReturn: -1.32},
		{Date: "2026-01-13", UnitNAV: 1.5200, AccumNAV: 1.5200, DailyReturn: 2.01},
		{Date: "2026-01-14", UnitNAV: 1.5300, AccumNAV: 1.5300, DailyReturn: 0.66},
	}
	if err := repo.SaveNAVHistory(ctx, "000001", navList); err != nil {
		t.Fatalf("SaveNAVHistory failed: %v", err)
	}

	got, err := repo.GetNAVHistory(ctx, "000001", "2026-01-10", "2026-01-14")
	if err != nil {
		t.Fatalf("GetNAVHistory failed: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 nav points, got %d", len(got))
	}
	if got[0].Date != "2026-01-10" {
		t.Errorf("expected first date 2026-01-10, got %s", got[0].Date)
	}
	if got[4].Date != "2026-01-14" {
		t.Errorf("expected last date 2026-01-14, got %s", got[4].Date)
	}

	sub, err := repo.GetNAVHistory(ctx, "000001", "2026-01-11", "2026-01-13")
	if err != nil {
		t.Fatalf("GetNAVHistory sub-range failed: %v", err)
	}
	if len(sub) != 3 {
		t.Errorf("expected 3 nav points, got %d", len(sub))
	}

	empty, err := repo.GetNAVHistory(ctx, "999999", "2026-01-10", "2026-01-14")
	if err != nil {
		t.Fatalf("GetNAVHistory non-existent failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0, got %d", len(empty))
	}
}
