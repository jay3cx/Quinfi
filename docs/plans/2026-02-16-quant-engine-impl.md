# 量化分析引擎 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 FundMind 新增组合回测、定投模拟、跨基金对比三大量化分析能力，以 Agent 工具 + 独立前端页面融入系统。

**Architecture:** 在 `internal/quant/` 构建统一量化引擎，三个功能共享基础指标计算模块。数据从 `nav_history` 表读取，不足时自动通过 DataSource 拉取并入库。后端新增 3 个 API 端点 + 3 个 Agent 工具，前端新增 3 个页面（回测实验室、定投模拟、基金PK）。

**Tech Stack:** Go 1.25, Gin, SQLite, React 19, TypeScript, Recharts, TailwindCSS 4

**Design Doc:** `docs/plans/2026-02-16-quant-engine-design.md`

---

## Task 1: FundRepository — 新增 GetNAVHistory 查询方法

量化引擎需要从 DB 读取净值历史。当前 `FundRepository` 只有 `SaveNAVHistory`，缺少读取方法。

**Files:**
- Modify: `internal/db/fund_repo.go`
- Modify: `internal/db/fund_repo_test.go`

**Step 1: 在 `fund_repo_test.go` 末尾新增测试**

```go
func TestFundRepository_GetNAVHistory(t *testing.T) {
	tmpDir := t.TempDir()
	sqlDB, err := Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer sqlDB.Close()

	repo := NewFundRepository(sqlDB)
	ctx := context.Background()

	// 保存测试数据
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

	// 查询全部
	got, err := repo.GetNAVHistory(ctx, "000001", "2026-01-10", "2026-01-14")
	if err != nil {
		t.Fatalf("GetNAVHistory failed: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 nav points, got %d", len(got))
	}
	// 应按日期升序
	if got[0].Date != "2026-01-10" {
		t.Errorf("expected first date 2026-01-10, got %s", got[0].Date)
	}
	if got[4].Date != "2026-01-14" {
		t.Errorf("expected last date 2026-01-14, got %s", got[4].Date)
	}

	// 查询子区间
	sub, err := repo.GetNAVHistory(ctx, "000001", "2026-01-11", "2026-01-13")
	if err != nil {
		t.Fatalf("GetNAVHistory sub-range failed: %v", err)
	}
	if len(sub) != 3 {
		t.Errorf("expected 3 nav points, got %d", len(sub))
	}

	// 查询不存在的基金
	empty, err := repo.GetNAVHistory(ctx, "999999", "2026-01-10", "2026-01-14")
	if err != nil {
		t.Fatalf("GetNAVHistory non-existent failed: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0, got %d", len(empty))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestFundRepository_GetNAVHistory ./internal/db/...`
Expected: FAIL — `repo.GetNAVHistory undefined`

**Step 3: Implement `GetNAVHistory` in `fund_repo.go`**

Add after `SaveNAVHistory`:

```go
// GetNAVHistory 查询指定日期范围的净值历史（按日期升序）
func (r *FundRepository) GetNAVHistory(ctx context.Context, code, startDate, endDate string) ([]datasource.NAV, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT date, unit_nav, accum_nav, daily_return
		 FROM nav_history
		 WHERE fund_code = ? AND date >= ? AND date <= ?
		 ORDER BY date ASC`, code, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("查询净值历史失败: %w", err)
	}
	defer rows.Close()

	var navList []datasource.NAV
	for rows.Next() {
		var nav datasource.NAV
		if err := rows.Scan(&nav.Date, &nav.UnitNAV, &nav.AccumNAV, &nav.DailyReturn); err != nil {
			return nil, fmt.Errorf("扫描净值数据失败: %w", err)
		}
		navList = append(navList, nav)
	}
	return navList, rows.Err()
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestFundRepository_GetNAVHistory ./internal/db/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/fund_repo.go internal/db/fund_repo_test.go
git commit -m "feat(db): add GetNAVHistory query method to FundRepository"
```

---

## Task 2: quant/series.go — 时间序列基础类型 + 收益率计算

**Files:**
- Create: `internal/quant/series.go`
- Create: `internal/quant/series_test.go`

**Step 1: Write the failing test `series_test.go`**

```go
package quant

import (
	"math"
	"testing"
)

func TestNavSeriesToReturns(t *testing.T) {
	series := &NavSeries{
		FundCode: "000001",
		Points: []NavPoint{
			{Date: "2026-01-10", NAV: 1.0000},
			{Date: "2026-01-11", NAV: 1.0100},
			{Date: "2026-01-12", NAV: 0.9900},
			{Date: "2026-01-13", NAV: 1.0200},
		},
	}

	returns := series.ToReturns()

	if returns.FundCode != "000001" {
		t.Errorf("expected fund code 000001, got %s", returns.FundCode)
	}
	// 4 个净值点产生 3 个收益率
	if len(returns.Returns) != 3 {
		t.Fatalf("expected 3 returns, got %d", len(returns.Returns))
	}
	// 第一个收益率: (1.01 - 1.00) / 1.00 = 0.01
	if math.Abs(returns.Returns[0]-0.01) > 1e-9 {
		t.Errorf("expected return[0] ≈ 0.01, got %f", returns.Returns[0])
	}
	// 第二个收益率: (0.99 - 1.01) / 1.01 ≈ -0.01980
	if math.Abs(returns.Returns[1]-(-0.0198019801980198)) > 1e-9 {
		t.Errorf("expected return[1] ≈ -0.0198, got %f", returns.Returns[1])
	}
}

func TestNavSeries_Empty(t *testing.T) {
	series := &NavSeries{FundCode: "000001", Points: []NavPoint{}}
	returns := series.ToReturns()
	if len(returns.Returns) != 0 {
		t.Errorf("expected 0 returns for empty series, got %d", len(returns.Returns))
	}
}

func TestNavSeries_SinglePoint(t *testing.T) {
	series := &NavSeries{FundCode: "000001", Points: []NavPoint{{Date: "2026-01-10", NAV: 1.0}}}
	returns := series.ToReturns()
	if len(returns.Returns) != 0 {
		t.Errorf("expected 0 returns for single point, got %d", len(returns.Returns))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestNavSeries ./internal/quant/...`
Expected: FAIL — package does not exist

**Step 3: Implement `series.go`**

```go
package quant

// NavPoint 单个净值数据点
type NavPoint struct {
	Date   string  // YYYY-MM-DD
	NAV    float64 // 单位净值
	AccNAV float64 // 累计净值（可选）
}

// NavSeries 基金净值时间序列（必须按日期升序排列）
type NavSeries struct {
	FundCode string
	Points   []NavPoint
}

// ReturnSeries 日收益率序列
type ReturnSeries struct {
	FundCode string
	Dates    []string  // 与 Returns 一一对应
	Returns  []float64 // 日收益率
}

// CurvePoint 时间序列数据点（供前端图表使用）
type CurvePoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// ToReturns 将净值序列转换为日收益率序列
func (s *NavSeries) ToReturns() *ReturnSeries {
	rs := &ReturnSeries{FundCode: s.FundCode}
	if len(s.Points) < 2 {
		return rs
	}
	rs.Dates = make([]string, 0, len(s.Points)-1)
	rs.Returns = make([]float64, 0, len(s.Points)-1)
	for i := 1; i < len(s.Points); i++ {
		prev := s.Points[i-1].NAV
		curr := s.Points[i].NAV
		if prev == 0 {
			continue
		}
		rs.Dates = append(rs.Dates, s.Points[i].Date)
		rs.Returns = append(rs.Returns, (curr-prev)/prev)
	}
	return rs
}
```

**Step 4: Run tests**

Run: `go test -v -run TestNavSeries ./internal/quant/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/quant/series.go internal/quant/series_test.go
git commit -m "feat(quant): add NavSeries, ReturnSeries types and ToReturns conversion"
```

---

## Task 3: quant/metrics.go — 风控指标计算

**Files:**
- Create: `internal/quant/metrics.go`
- Create: `internal/quant/metrics_test.go`

**Step 1: Write tests `metrics_test.go`**

```go
package quant

import (
	"math"
	"testing"
)

const tolerance = 1e-6

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

// 用已知数据验证指标计算
// 净值序列: 1.0 → 1.05 → 1.02 → 1.08 → 1.10
// 日收益率: 0.05, -0.02857, 0.05882, 0.01852
func makeTestReturns() *ReturnSeries {
	return &ReturnSeries{
		FundCode: "TEST",
		Dates:    []string{"d1", "d2", "d3", "d4"},
		Returns:  []float64{0.05, -0.02857142857, 0.05882352941, 0.01851851852},
	}
}

func TestAnnualizedReturn(t *testing.T) {
	// 1.0 → 1.10, 4 个交易日
	r := AnnualizedReturn(1.0, 1.10, 4)
	// (1.1/1.0)^(252/4) - 1 ≈ very large, just check > 0
	if r <= 0 {
		t.Errorf("expected positive annualized return, got %f", r)
	}

	// 相同起终值
	r2 := AnnualizedReturn(1.0, 1.0, 100)
	if !almostEqual(r2, 0, tolerance) {
		t.Errorf("expected 0 for no change, got %f", r2)
	}
}

func TestVolatility(t *testing.T) {
	rs := makeTestReturns()
	vol := Volatility(rs.Returns)
	// 年化波动率 = 日标准差 * sqrt(252), should be > 0
	if vol <= 0 {
		t.Errorf("expected positive volatility, got %f", vol)
	}
}

func TestMaxDrawdown(t *testing.T) {
	// NAV 序列有明确的回撤: 1.0, 1.1, 0.9, 1.05
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 0.9},
		{Date: "d4", NAV: 1.05},
	}
	mdd := MaxDrawdown(navs)
	// 最大回撤 = (1.1 - 0.9) / 1.1 ≈ 0.18182
	expected := (1.1 - 0.9) / 1.1
	if !almostEqual(mdd, expected, tolerance) {
		t.Errorf("expected max drawdown ≈ %.6f, got %.6f", expected, mdd)
	}
}

func TestMaxDrawdown_NoDrawdown(t *testing.T) {
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 1.2},
	}
	mdd := MaxDrawdown(navs)
	if !almostEqual(mdd, 0, tolerance) {
		t.Errorf("expected 0 drawdown for monotonic increase, got %f", mdd)
	}
}

func TestSharpeRatio(t *testing.T) {
	rs := makeTestReturns()
	sr := SharpeRatio(rs.Returns, 0.02) // 2% 无风险利率
	// 正收益 + 正波动率 → 夏普 > 0
	if sr <= 0 {
		t.Errorf("expected positive Sharpe ratio, got %f", sr)
	}
}

func TestSortinoRatio(t *testing.T) {
	rs := makeTestReturns()
	sortino := SortinoRatio(rs.Returns, 0.02)
	// 有正收益时 Sortino > 0
	if sortino <= 0 {
		t.Errorf("expected positive Sortino ratio, got %f", sortino)
	}
}

func TestCalmarRatio(t *testing.T) {
	navs := []NavPoint{
		{Date: "d1", NAV: 1.0},
		{Date: "d2", NAV: 1.1},
		{Date: "d3", NAV: 0.9},
		{Date: "d4", NAV: 1.2},
	}
	calmar := CalmarRatio(navs)
	// 有正收益和非零回撤 → Calmar > 0
	if calmar <= 0 {
		t.Errorf("expected positive Calmar ratio, got %f", calmar)
	}
}

func TestMetrics_EmptyInput(t *testing.T) {
	// 所有函数应安全处理空输入
	if vol := Volatility(nil); vol != 0 {
		t.Errorf("Volatility(nil) should be 0, got %f", vol)
	}
	if mdd := MaxDrawdown(nil); mdd != 0 {
		t.Errorf("MaxDrawdown(nil) should be 0, got %f", mdd)
	}
	if sr := SharpeRatio(nil, 0.02); sr != 0 {
		t.Errorf("SharpeRatio(nil) should be 0, got %f", sr)
	}
	if sor := SortinoRatio(nil, 0.02); sor != 0 {
		t.Errorf("SortinoRatio(nil) should be 0, got %f", sor)
	}
	if cal := CalmarRatio(nil); cal != 0 {
		t.Errorf("CalmarRatio(nil) should be 0, got %f", cal)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestAnnual\|TestVol\|TestMax\|TestSharpe\|TestSortino\|TestCalmar\|TestMetrics ./internal/quant/...`
Expected: FAIL — functions not defined

**Step 3: Implement `metrics.go`**

```go
package quant

import "math"

const tradingDaysPerYear = 252

// AnnualizedReturn 年化收益率
// startNAV: 起始净值, endNAV: 终止净值, tradingDays: 交易天数
func AnnualizedReturn(startNAV, endNAV float64, tradingDays int) float64 {
	if startNAV <= 0 || tradingDays <= 0 {
		return 0
	}
	totalReturn := endNAV / startNAV
	return math.Pow(totalReturn, float64(tradingDaysPerYear)/float64(tradingDays)) - 1
}

// Volatility 年化波动率 = 日收益率标准差 × √252
func Volatility(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	return stddev(returns) * math.Sqrt(float64(tradingDaysPerYear))
}

// MaxDrawdown 最大回撤（基于净值序列）
func MaxDrawdown(navs []NavPoint) float64 {
	if len(navs) < 2 {
		return 0
	}
	peak := navs[0].NAV
	maxDD := 0.0
	for _, p := range navs {
		if p.NAV > peak {
			peak = p.NAV
		}
		dd := (peak - p.NAV) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// SharpeRatio 夏普比率 = (年化收益 - 无风险利率) / 年化波动率
func SharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	avgDaily := mean(returns)
	annualReturn := avgDaily * float64(tradingDaysPerYear)
	vol := Volatility(returns)
	if vol == 0 {
		return 0
	}
	return (annualReturn - riskFreeRate) / vol
}

// SortinoRatio 仅计算下行波动率
func SortinoRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	avgDaily := mean(returns)
	annualReturn := avgDaily * float64(tradingDaysPerYear)
	downside := downsideDeviation(returns) * math.Sqrt(float64(tradingDaysPerYear))
	if downside == 0 {
		return 0
	}
	return (annualReturn - riskFreeRate) / downside
}

// CalmarRatio 年化收益 / 最大回撤
func CalmarRatio(navs []NavPoint) float64 {
	if len(navs) < 2 {
		return 0
	}
	annRet := AnnualizedReturn(navs[0].NAV, navs[len(navs)-1].NAV, len(navs)-1)
	mdd := MaxDrawdown(navs)
	if mdd == 0 {
		return 0
	}
	return annRet / mdd
}

// DrawdownSeries 回撤曲线
func DrawdownSeries(navs []NavPoint) []CurvePoint {
	if len(navs) == 0 {
		return nil
	}
	result := make([]CurvePoint, len(navs))
	peak := navs[0].NAV
	for i, p := range navs {
		if p.NAV > peak {
			peak = p.NAV
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - p.NAV) / peak
		}
		result[i] = CurvePoint{Date: p.Date, Value: -dd} // 负值表示回撤
	}
	return result
}

// Correlation 两个收益率序列的皮尔逊相关系数
func Correlation(a, b []float64) float64 {
	n := min(len(a), len(b))
	if n < 2 {
		return 0
	}
	meanA := mean(a[:n])
	meanB := mean(b[:n])
	var sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		sumAB += da * db
		sumA2 += da * da
		sumB2 += db * db
	}
	denom := math.Sqrt(sumA2 * sumB2)
	if denom == 0 {
		return 0
	}
	return sumAB / denom
}

// ===== 辅助函数 =====

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func stddev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := mean(data)
	sumSq := 0.0
	for _, v := range data {
		d := v - m
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(data)-1)) // 样本标准差
}

func downsideDeviation(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	sumSq := 0.0
	count := 0
	for _, r := range returns {
		if r < 0 {
			sumSq += r * r
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(sumSq / float64(count))
}
```

**Step 4: Run tests**

Run: `go test -v ./internal/quant/...`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/quant/metrics.go internal/quant/metrics_test.go
git commit -m "feat(quant): add risk metrics — Sharpe, Sortino, Calmar, MaxDrawdown, Volatility"
```

---

## Task 4: quant/backtest.go — 组合回测引擎

**Files:**
- Create: `internal/quant/backtest.go`
- Create: `internal/quant/backtest_test.go`

**Step 1: Write tests `backtest_test.go`**

```go
package quant

import (
	"math"
	"testing"
)

func TestBacktest_SingleFund_NoRebalance(t *testing.T) {
	// 单只基金，权重100%，应与原始净值走势一致
	navData := map[string]*NavSeries{
		"000001": {
			FundCode: "000001",
			Points: []NavPoint{
				{Date: "2026-01-10", NAV: 1.0},
				{Date: "2026-01-11", NAV: 1.05},
				{Date: "2026-01-12", NAV: 1.02},
				{Date: "2026-01-13", NAV: 1.10},
			},
		},
	}

	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "000001", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}

	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}

	// 总收益率 = (1.10 - 1.0) / 1.0 = 10%
	if !almostEqual(result.TotalReturn, 0.10, 0.001) {
		t.Errorf("expected total return ≈ 0.10, got %f", result.TotalReturn)
	}

	// 净值曲线应有 4 个点
	if len(result.EquityCurve) != 4 {
		t.Errorf("expected 4 equity curve points, got %d", len(result.EquityCurve))
	}
	// 起始值 = InitialCash
	if !almostEqual(result.EquityCurve[0].Value, 100000, 1) {
		t.Errorf("expected first equity ≈ 100000, got %f", result.EquityCurve[0].Value)
	}
	// 终止值 = 110000
	if !almostEqual(result.EquityCurve[3].Value, 110000, 1) {
		t.Errorf("expected last equity ≈ 110000, got %f", result.EquityCurve[3].Value)
	}
}

func TestBacktest_TwoFunds_EqualWeight(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.2},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 0.8},
		}},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "A", Weight: 0.5}, {FundCode: "B", Weight: 0.5}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	// 组合收益 = 0.5 * 20% + 0.5 * (-20%) = 0%
	if !almostEqual(result.TotalReturn, 0.0, 0.001) {
		t.Errorf("expected total return ≈ 0, got %f", result.TotalReturn)
	}
}

func TestBacktest_WithBenchmark(t *testing.T) {
	navData := map[string]*NavSeries{
		"000001": {FundCode: "000001", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.1}, {Date: "d3", NAV: 1.15},
		}},
		"000300": {FundCode: "000300", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.05}, {Date: "d3", NAV: 1.08},
		}},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "000001", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
		Benchmark:   "000300",
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	if len(result.BenchmarkCurve) != 3 {
		t.Errorf("expected 3 benchmark points, got %d", len(result.BenchmarkCurve))
	}
}

func TestBacktest_InvalidWeights(t *testing.T) {
	navData := map[string]*NavSeries{}
	req := &BacktestRequest{
		Holdings: []HoldingWeight{{FundCode: "A", Weight: 0.3}}, // 不等于 1.0
	}
	_, err := RunBacktest(req, navData)
	if err == nil {
		t.Error("expected error for invalid weights, got nil")
	}
}

func TestBacktest_MetricsPresent(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: func() []NavPoint {
			pts := make([]NavPoint, 30)
			nav := 1.0
			for i := range pts {
				pts[i] = NavPoint{Date: "d" + string(rune('0'+i)), NAV: nav}
				nav *= 1 + (float64(i%5)-2)*0.01
			}
			return pts
		}()},
	}
	req := &BacktestRequest{
		Holdings:    []HoldingWeight{{FundCode: "A", Weight: 1.0}},
		InitialCash: 100000,
		Rebalance:   RebalanceNone,
	}
	result, err := RunBacktest(req, navData)
	if err != nil {
		t.Fatalf("RunBacktest failed: %v", err)
	}
	// 验证指标不为 NaN
	if math.IsNaN(result.SharpeRatio) {
		t.Error("SharpeRatio is NaN")
	}
	if math.IsNaN(result.Volatility) {
		t.Error("Volatility is NaN")
	}
}
```

**Step 2: Run to verify failure**

Run: `go test -v -run TestBacktest ./internal/quant/...`
Expected: FAIL

**Step 3: Implement `backtest.go`**

```go
package quant

import (
	"fmt"
	"math"
)

// RebalanceType 再平衡策略
type RebalanceType string

const (
	RebalanceNone      RebalanceType = "none"
	RebalanceMonthly   RebalanceType = "monthly"
	RebalanceQuarterly RebalanceType = "quarterly"
)

// BacktestRequest 回测请求
type BacktestRequest struct {
	Holdings    []HoldingWeight
	InitialCash float64
	Rebalance   RebalanceType
	Benchmark   string // 基准基金代码（可选）
}

// HoldingWeight 持仓权重
type HoldingWeight struct {
	FundCode string  `json:"fund_code"`
	Weight   float64 `json:"weight"`
}

// BacktestResult 回测结果
type BacktestResult struct {
	TotalReturn  float64 `json:"total_return"`
	AnnualReturn float64 `json:"annual_return"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	Volatility   float64 `json:"volatility"`
	SortinoRatio float64 `json:"sortino_ratio"`
	CalmarRatio  float64 `json:"calmar_ratio"`

	EquityCurve    []CurvePoint    `json:"equity_curve"`
	DrawdownCurve  []CurvePoint    `json:"drawdown_curve"`
	BenchmarkCurve []CurvePoint    `json:"benchmark_curve,omitempty"`
	FundMetrics    []FundMetricRow `json:"fund_metrics"`
}

// FundMetricRow 单只基金在回测期内的表现
type FundMetricRow struct {
	FundCode    string  `json:"fund_code"`
	Weight      float64 `json:"weight"`
	TotalReturn float64 `json:"total_return"`
}

// RunBacktest 执行组合回测
// navData: 所有需要用到的基金净值（key = fund code）
func RunBacktest(req *BacktestRequest, navData map[string]*NavSeries) (*BacktestResult, error) {
	if req.InitialCash <= 0 {
		req.InitialCash = 100000
	}

	// 权重校验
	totalWeight := 0.0
	for _, h := range req.Holdings {
		totalWeight += h.Weight
	}
	if math.Abs(totalWeight-1.0) > 0.01 {
		return nil, fmt.Errorf("权重之和必须为 1.0，当前为 %.4f", totalWeight)
	}

	// 确定公共交易日（所有基金都有净值的日期）
	dates := commonDates(req.Holdings, navData)
	if len(dates) < 2 {
		return nil, fmt.Errorf("公共交易日不足（至少需要2天），当前 %d 天", len(dates))
	}

	// 构建日期 → 净值的索引
	navIndex := buildNavIndex(navData)

	// 计算组合每日净值
	equityCurve := make([]CurvePoint, len(dates))
	portfolioNAVs := make([]NavPoint, len(dates))

	for i, date := range dates {
		portfolioReturn := 0.0
		for _, h := range req.Holdings {
			navStart := navIndex[h.FundCode][dates[0]]
			navCurr := navIndex[h.FundCode][date]
			if navStart > 0 {
				portfolioReturn += h.Weight * (navCurr / navStart)
			}
		}
		value := req.InitialCash * portfolioReturn
		equityCurve[i] = CurvePoint{Date: date, Value: value}
		portfolioNAVs[i] = NavPoint{Date: date, NAV: portfolioReturn}
	}

	// 收益率序列
	pSeries := &NavSeries{FundCode: "portfolio", Points: portfolioNAVs}
	pReturns := pSeries.ToReturns()

	result := &BacktestResult{
		TotalReturn:   portfolioNAVs[len(portfolioNAVs)-1].NAV - 1.0,
		AnnualReturn:  AnnualizedReturn(1.0, portfolioNAVs[len(portfolioNAVs)-1].NAV, len(dates)-1),
		MaxDrawdown:   MaxDrawdown(portfolioNAVs),
		SharpeRatio:   SharpeRatio(pReturns.Returns, 0.02),
		Volatility:    Volatility(pReturns.Returns),
		SortinoRatio:  SortinoRatio(pReturns.Returns, 0.02),
		CalmarRatio:   CalmarRatio(portfolioNAVs),
		EquityCurve:   equityCurve,
		DrawdownCurve: DrawdownSeries(portfolioNAVs),
	}

	// 每只基金的单独表现
	for _, h := range req.Holdings {
		navStart := navIndex[h.FundCode][dates[0]]
		navEnd := navIndex[h.FundCode][dates[len(dates)-1]]
		ret := 0.0
		if navStart > 0 {
			ret = (navEnd - navStart) / navStart
		}
		result.FundMetrics = append(result.FundMetrics, FundMetricRow{
			FundCode: h.FundCode, Weight: h.Weight, TotalReturn: ret,
		})
	}

	// 基准曲线
	if req.Benchmark != "" {
		if benchSeries, ok := navData[req.Benchmark]; ok {
			benchCurve := make([]CurvePoint, 0, len(dates))
			bIndex := make(map[string]float64)
			for _, p := range benchSeries.Points {
				bIndex[p.Date] = p.NAV
			}
			startNAV := bIndex[dates[0]]
			if startNAV > 0 {
				for _, date := range dates {
					if nav, ok := bIndex[date]; ok {
						benchCurve = append(benchCurve, CurvePoint{
							Date: date, Value: req.InitialCash * nav / startNAV,
						})
					}
				}
			}
			result.BenchmarkCurve = benchCurve
		}
	}

	return result, nil
}

// commonDates 提取所有基金都有数据的公共交易日（升序）
func commonDates(holdings []HoldingWeight, navData map[string]*NavSeries) []string {
	if len(holdings) == 0 {
		return nil
	}
	// 以第一只基金的日期为基础
	first := navData[holdings[0].FundCode]
	if first == nil {
		return nil
	}
	dateSet := make(map[string]int)
	for _, p := range first.Points {
		dateSet[p.Date] = 1
	}
	// 与其他基金取交集
	for i := 1; i < len(holdings); i++ {
		series := navData[holdings[i].FundCode]
		if series == nil {
			return nil
		}
		otherDates := make(map[string]bool)
		for _, p := range series.Points {
			otherDates[p.Date] = true
		}
		for d := range dateSet {
			if !otherDates[d] {
				delete(dateSet, d)
			}
		}
	}
	// 排序
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sortStrings(dates)
	return dates
}

// buildNavIndex 构建 fundCode → date → NAV 索引
func buildNavIndex(navData map[string]*NavSeries) map[string]map[string]float64 {
	index := make(map[string]map[string]float64)
	for code, series := range navData {
		m := make(map[string]float64, len(series.Points))
		for _, p := range series.Points {
			m[p.Date] = p.NAV
		}
		index[code] = m
	}
	return index
}

// sortStrings 简单字符串排序（日期格式 YYYY-MM-DD 可直接字典序）
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
```

**Step 4: Run tests**

Run: `go test -v -run TestBacktest ./internal/quant/...`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/quant/backtest.go internal/quant/backtest_test.go
git commit -m "feat(quant): add portfolio backtest engine with rebalance support"
```

---

## Task 5: quant/dca.go — 定投策略模拟器

**Files:**
- Create: `internal/quant/dca.go`
- Create: `internal/quant/dca_test.go`

**Step 1: Write tests `dca_test.go`**

```go
package quant

import (
	"testing"
)

func makeMonthlyNAV() *NavSeries {
	// 12 个月数据，每月一个点
	return &NavSeries{
		FundCode: "000001",
		Points: []NavPoint{
			{Date: "2025-01-15", NAV: 1.00},
			{Date: "2025-02-15", NAV: 0.95},
			{Date: "2025-03-15", NAV: 1.05},
			{Date: "2025-04-15", NAV: 0.90},
			{Date: "2025-05-15", NAV: 1.00},
			{Date: "2025-06-15", NAV: 1.10},
			{Date: "2025-07-15", NAV: 1.05},
			{Date: "2025-08-15", NAV: 0.98},
			{Date: "2025-09-15", NAV: 1.02},
			{Date: "2025-10-15", NAV: 1.08},
			{Date: "2025-11-15", NAV: 1.12},
			{Date: "2025-12-15", NAV: 1.15},
		},
	}
}

func TestDCA_FixedStrategy(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCAFixed,
		Amount:    1000,
		Frequency: "monthly",
	}

	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}

	// 12 次定投，每次 1000
	if result.TotalInvested != 12000 {
		t.Errorf("expected total invested 12000, got %f", result.TotalInvested)
	}
	// 终值 > 0
	if result.FinalValue <= 0 {
		t.Errorf("expected positive final value, got %f", result.FinalValue)
	}
	// 应有 12 笔交易
	if len(result.Transactions) != 12 {
		t.Errorf("expected 12 transactions, got %d", len(result.Transactions))
	}
	// 第一笔：买入份额 = 1000 / 1.00 = 1000
	if !almostEqual(result.Transactions[0].Shares, 1000, 0.01) {
		t.Errorf("expected first transaction shares ≈ 1000, got %f", result.Transactions[0].Shares)
	}
	// 平均成本应有值
	if result.AvgCost <= 0 {
		t.Errorf("expected positive avg cost, got %f", result.AvgCost)
	}
}

func TestDCA_LumpSumComparison(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCAFixed,
		Amount:    1000,
		Frequency: "monthly",
	}
	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}
	// 一次性投入：12000 在第一天买入，终值 = 12000 * (1.15/1.00)
	expectedLumpSum := (1.15/1.00 - 1)
	if !almostEqual(result.LumpSumReturn, expectedLumpSum, 0.001) {
		t.Errorf("expected lump sum return ≈ %f, got %f", expectedLumpSum, result.LumpSumReturn)
	}
}

func TestDCA_SmartStrategy(t *testing.T) {
	nav := makeMonthlyNAV()
	req := &DCARequest{
		FundCode:  "000001",
		Strategy:  DCASmart,
		Amount:    1000,
		Frequency: "monthly",
	}
	result, err := RunDCA(req, nav)
	if err != nil {
		t.Fatalf("RunDCA failed: %v", err)
	}
	// 智能定投：总投入不等于固定金额 * 次数
	if result.TotalInvested == 12000 {
		t.Error("smart DCA should vary investment amounts")
	}
	if len(result.Transactions) != 12 {
		t.Errorf("expected 12 transactions, got %d", len(result.Transactions))
	}
}

func TestDCA_EmptyNAV(t *testing.T) {
	nav := &NavSeries{FundCode: "000001", Points: []NavPoint{}}
	req := &DCARequest{FundCode: "000001", Strategy: DCAFixed, Amount: 1000, Frequency: "monthly"}
	_, err := RunDCA(req, nav)
	if err == nil {
		t.Error("expected error for empty NAV")
	}
}
```

**Step 2: Run to verify failure**

Run: `go test -v -run TestDCA ./internal/quant/...`
Expected: FAIL

**Step 3: Implement `dca.go`**

```go
package quant

import "fmt"

// DCAStrategy 定投策略类型
type DCAStrategy string

const (
	DCAFixed DCAStrategy = "fixed" // 固定金额
	DCAValue DCAStrategy = "value" // 目标价值
	DCASmart DCAStrategy = "smart" // 智能定投（均线偏离）
)

// DCARequest 定投模拟请求
type DCARequest struct {
	FundCode  string      `json:"fund_code"`
	Strategy  DCAStrategy `json:"strategy"`
	Amount    float64     `json:"amount"`    // 每期基础金额
	Frequency string      `json:"frequency"` // "monthly"
}

// DCAResult 定投模拟结果
type DCAResult struct {
	Strategy      DCAStrategy      `json:"strategy"`
	TotalInvested float64          `json:"total_invested"`
	FinalValue    float64          `json:"final_value"`
	TotalReturn   float64          `json:"total_return"`
	AnnualReturn  float64          `json:"annual_return"`
	AvgCost       float64          `json:"avg_cost"`
	LumpSumReturn float64          `json:"lump_sum_return"`
	ExcessReturn  float64          `json:"excess_return"`
	InvestCurve   []CurvePoint     `json:"invest_curve"`
	ValueCurve    []CurvePoint     `json:"value_curve"`
	CostCurve     []CurvePoint     `json:"cost_curve"`
	Transactions  []DCATransaction `json:"transactions"`
}

// DCATransaction 单次定投交易
type DCATransaction struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"` // 本次投入金额
	NAV    float64 `json:"nav"`    // 买入时净值
	Shares float64 `json:"shares"` // 买入份额
}

// RunDCA 执行定投模拟
func RunDCA(req *DCARequest, nav *NavSeries) (*DCAResult, error) {
	if len(nav.Points) < 2 {
		return nil, fmt.Errorf("净值数据不足")
	}

	// 按 frequency 筛选定投日（目前只支持 monthly：每个月取第一个数据点）
	investDates := selectInvestDates(nav.Points, req.Frequency)
	if len(investDates) == 0 {
		return nil, fmt.Errorf("无有效定投日期")
	}

	var totalInvested, totalShares float64
	transactions := make([]DCATransaction, 0, len(investDates))
	investCurve := make([]CurvePoint, 0, len(investDates))
	valueCurve := make([]CurvePoint, 0, len(investDates))
	costCurve := make([]CurvePoint, 0, len(investDates))

	// 计算移动平均（用于 smart 策略）
	maMap := buildMAMap(nav.Points, 250)

	for i, idx := range investDates {
		p := nav.Points[idx]
		amount := req.Amount

		switch req.Strategy {
		case DCAValue:
			// 目标价值定投：目标累计价值 = (i+1) * Amount
			targetValue := float64(i+1) * req.Amount
			currentValue := totalShares * p.NAV
			amount = targetValue - currentValue
			if amount < 0 {
				amount = 0 // 不做卖出
			}
		case DCASmart:
			// 均线偏离法：低于均线多投，高于均线少投
			if ma, ok := maMap[p.Date]; ok && ma > 0 {
				deviation := (p.NAV - ma) / ma
				// 偏离度映射: -20% → 1.5x, +20% → 0.5x
				multiplier := 1.0 - deviation*2.5
				if multiplier < 0.5 {
					multiplier = 0.5
				}
				if multiplier > 1.5 {
					multiplier = 1.5
				}
				amount = req.Amount * multiplier
			}
		}

		shares := amount / p.NAV
		totalInvested += amount
		totalShares += shares

		transactions = append(transactions, DCATransaction{
			Date: p.Date, Amount: amount, NAV: p.NAV, Shares: shares,
		})

		currentValue := totalShares * p.NAV
		avgCost := 0.0
		if totalShares > 0 {
			avgCost = totalInvested / totalShares
		}
		investCurve = append(investCurve, CurvePoint{Date: p.Date, Value: totalInvested})
		valueCurve = append(valueCurve, CurvePoint{Date: p.Date, Value: currentValue})
		costCurve = append(costCurve, CurvePoint{Date: p.Date, Value: avgCost})
	}

	// 终值（用最后一个净值点算）
	lastNAV := nav.Points[len(nav.Points)-1].NAV
	finalValue := totalShares * lastNAV
	avgCost := 0.0
	if totalShares > 0 {
		avgCost = totalInvested / totalShares
	}
	totalReturn := 0.0
	if totalInvested > 0 {
		totalReturn = (finalValue - totalInvested) / totalInvested
	}

	// 一次性投入对比
	firstNAV := nav.Points[0].NAV
	lumpSumReturn := 0.0
	if firstNAV > 0 {
		lumpSumReturn = (lastNAV - firstNAV) / firstNAV
	}

	return &DCAResult{
		Strategy:      req.Strategy,
		TotalInvested: totalInvested,
		FinalValue:    finalValue,
		TotalReturn:   totalReturn,
		AnnualReturn:  AnnualizedReturn(totalInvested, finalValue, len(nav.Points)-1),
		AvgCost:       avgCost,
		LumpSumReturn: lumpSumReturn,
		ExcessReturn:  totalReturn - lumpSumReturn,
		InvestCurve:   investCurve,
		ValueCurve:    valueCurve,
		CostCurve:     costCurve,
		Transactions:  transactions,
	}, nil
}

// selectInvestDates 根据频率选取定投日在 Points 中的索引
func selectInvestDates(points []NavPoint, freq string) []int {
	if len(points) == 0 {
		return nil
	}
	var indices []int
	lastMonth := ""
	for i, p := range points {
		if len(p.Date) < 7 {
			continue
		}
		month := p.Date[:7] // "YYYY-MM"
		if month != lastMonth {
			indices = append(indices, i)
			lastMonth = month
		}
	}
	return indices
}

// buildMAMap 计算移动平均线（date → MA value）
func buildMAMap(points []NavPoint, window int) map[string]float64 {
	m := make(map[string]float64, len(points))
	for i, p := range points {
		start := 0
		if i-window+1 > 0 {
			start = i - window + 1
		}
		sum := 0.0
		count := 0
		for j := start; j <= i; j++ {
			sum += points[j].NAV
			count++
		}
		m[p.Date] = sum / float64(count)
	}
	return m
}
```

**Step 4: Run tests**

Run: `go test -v -run TestDCA ./internal/quant/...`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/quant/dca.go internal/quant/dca_test.go
git commit -m "feat(quant): add DCA simulator with fixed/value/smart strategies"
```

---

## Task 6: quant/compare.go — 跨基金对比分析

**Files:**
- Create: `internal/quant/compare.go`
- Create: `internal/quant/compare_test.go`

**Step 1: Write tests `compare_test.go`**

```go
package quant

import (
	"testing"
)

func TestCompare_TwoFunds(t *testing.T) {
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "2025-01-01", NAV: 1.0}, {Date: "2025-02-01", NAV: 1.1},
			{Date: "2025-03-01", NAV: 1.05}, {Date: "2025-04-01", NAV: 1.2},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "2025-01-01", NAV: 2.0}, {Date: "2025-02-01", NAV: 2.1},
			{Date: "2025-03-01", NAV: 1.9}, {Date: "2025-04-01", NAV: 2.3},
		}},
	}
	fundNames := map[string]string{"A": "基金A", "B": "基金B"}

	req := &CompareRequest{FundCodes: []string{"A", "B"}, Period: "max"}
	result, err := RunCompare(req, navData, fundNames)
	if err != nil {
		t.Fatalf("RunCompare failed: %v", err)
	}

	// 应有 2 只基金的 profile
	if len(result.Funds) != 2 {
		t.Fatalf("expected 2 fund profiles, got %d", len(result.Funds))
	}

	// 归一化净值曲线起点应为 1.0
	for _, f := range result.Funds {
		if len(f.NormalizedNAV) == 0 {
			t.Errorf("fund %s has no normalized NAV", f.Code)
			continue
		}
		if !almostEqual(f.NormalizedNAV[0].Value, 1.0, 0.001) {
			t.Errorf("fund %s first normalized NAV should be 1.0, got %f", f.Code, f.NormalizedNAV[0].Value)
		}
	}

	// 相关性矩阵应为 2x2
	if len(result.Matrix.Values) != 2 || len(result.Matrix.Values[0]) != 2 {
		t.Errorf("expected 2x2 correlation matrix, got %dx%d", len(result.Matrix.Values), len(result.Matrix.Values[0]))
	}
	// 对角线应为 1.0
	if !almostEqual(result.Matrix.Values[0][0], 1.0, 0.001) {
		t.Errorf("expected self-correlation = 1.0, got %f", result.Matrix.Values[0][0])
	}
}

func TestCompare_TooFewFunds(t *testing.T) {
	navData := map[string]*NavSeries{}
	req := &CompareRequest{FundCodes: []string{"A"}, Period: "1y"}
	_, err := RunCompare(req, navData, nil)
	if err == nil {
		t.Error("expected error for single fund")
	}
}

func TestCompare_Correlation(t *testing.T) {
	// 完全正相关的两只基金
	navData := map[string]*NavSeries{
		"A": {FundCode: "A", Points: []NavPoint{
			{Date: "d1", NAV: 1.0}, {Date: "d2", NAV: 1.1}, {Date: "d3", NAV: 1.2}, {Date: "d4", NAV: 1.3},
		}},
		"B": {FundCode: "B", Points: []NavPoint{
			{Date: "d1", NAV: 2.0}, {Date: "d2", NAV: 2.2}, {Date: "d3", NAV: 2.4}, {Date: "d4", NAV: 2.6},
		}},
	}
	req := &CompareRequest{FundCodes: []string{"A", "B"}, Period: "max"}
	result, err := RunCompare(req, navData, nil)
	if err != nil {
		t.Fatalf("RunCompare failed: %v", err)
	}
	// 完全正相关 → 相关系数 ≈ 1.0
	corr := result.Matrix.Values[0][1]
	if !almostEqual(corr, 1.0, 0.01) {
		t.Errorf("expected correlation ≈ 1.0 for perfectly correlated funds, got %f", corr)
	}
}
```

**Step 2: Run to verify failure**

Run: `go test -v -run TestCompare ./internal/quant/...`
Expected: FAIL

**Step 3: Implement `compare.go`**

```go
package quant

import "fmt"

// CompareRequest 跨基金对比请求
type CompareRequest struct {
	FundCodes []string `json:"fund_codes"`
	Period    string   `json:"period"` // "3m" / "6m" / "1y" / "3y" / "max"
}

// CompareResult 对比结果
type CompareResult struct {
	Period string              `json:"period"`
	Funds  []FundProfile       `json:"funds"`
	Matrix CorrelationMatrix   `json:"correlation_matrix"`
}

// FundProfile 单只基金画像
type FundProfile struct {
	Code          string       `json:"code"`
	Name          string       `json:"name"`
	TotalReturn   float64      `json:"total_return"`
	AnnualReturn  float64      `json:"annual_return"`
	Volatility    float64      `json:"volatility"`
	SharpeRatio   float64      `json:"sharpe_ratio"`
	MaxDrawdown   float64      `json:"max_drawdown"`
	SortinoRatio  float64      `json:"sortino_ratio"`
	NormalizedNAV []CurvePoint `json:"normalized_nav"`
}

// CorrelationMatrix 相关性矩阵
type CorrelationMatrix struct {
	Codes  []string    `json:"codes"`
	Values [][]float64 `json:"values"`
}

// RunCompare 执行跨基金对比分析
func RunCompare(req *CompareRequest, navData map[string]*NavSeries, fundNames map[string]string) (*CompareResult, error) {
	if len(req.FundCodes) < 2 {
		return nil, fmt.Errorf("至少需要 2 只基金进行对比")
	}

	// 取公共日期
	holdings := make([]HoldingWeight, len(req.FundCodes))
	for i, code := range req.FundCodes {
		holdings[i] = HoldingWeight{FundCode: code, Weight: 1}
	}
	dates := commonDates(holdings, navData)
	if len(dates) < 2 {
		return nil, fmt.Errorf("公共交易日不足")
	}

	navIndex := buildNavIndex(navData)

	// 计算每只基金的 profile
	profiles := make([]FundProfile, len(req.FundCodes))
	returnSeriesMap := make(map[string][]float64)

	for i, code := range req.FundCodes {
		startNAV := navIndex[code][dates[0]]
		endNAV := navIndex[code][dates[len(dates)-1]]

		// 归一化净值
		normalized := make([]CurvePoint, len(dates))
		navPoints := make([]NavPoint, len(dates))
		for j, d := range dates {
			nav := navIndex[code][d]
			normVal := 1.0
			if startNAV > 0 {
				normVal = nav / startNAV
			}
			normalized[j] = CurvePoint{Date: d, Value: normVal}
			navPoints[j] = NavPoint{Date: d, NAV: nav}
		}

		series := &NavSeries{FundCode: code, Points: navPoints}
		rs := series.ToReturns()
		returnSeriesMap[code] = rs.Returns

		totalReturn := 0.0
		if startNAV > 0 {
			totalReturn = (endNAV - startNAV) / startNAV
		}

		name := code
		if fundNames != nil {
			if n, ok := fundNames[code]; ok {
				name = n
			}
		}

		profiles[i] = FundProfile{
			Code:          code,
			Name:          name,
			TotalReturn:   totalReturn,
			AnnualReturn:  AnnualizedReturn(startNAV, endNAV, len(dates)-1),
			Volatility:    Volatility(rs.Returns),
			SharpeRatio:   SharpeRatio(rs.Returns, 0.02),
			MaxDrawdown:   MaxDrawdown(navPoints),
			SortinoRatio:  SortinoRatio(rs.Returns, 0.02),
			NormalizedNAV: normalized,
		}
	}

	// 相关性矩阵
	n := len(req.FundCodes)
	matrix := CorrelationMatrix{
		Codes:  req.FundCodes,
		Values: make([][]float64, n),
	}
	for i := 0; i < n; i++ {
		matrix.Values[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				matrix.Values[i][j] = 1.0
			} else {
				matrix.Values[i][j] = Correlation(
					returnSeriesMap[req.FundCodes[i]],
					returnSeriesMap[req.FundCodes[j]],
				)
			}
		}
	}

	return &CompareResult{
		Period: req.Period,
		Funds:  profiles,
		Matrix: matrix,
	}, nil
}
```

**Step 4: Run tests**

Run: `go test -v -run TestCompare ./internal/quant/...`
Expected: ALL PASS

**Step 5: Run all quant tests**

Run: `go test -v ./internal/quant/...`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add internal/quant/compare.go internal/quant/compare_test.go
git commit -m "feat(quant): add cross-fund comparison with correlation matrix"
```

---

## Task 7: quant API 端点 + 数据加载

**Files:**
- Create: `internal/api/quant_handler.go`
- Modify: `internal/api/router.go` (注册路由)

**Step 1: Create `quant_handler.go`**

```go
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/datasource"
	funddb "github.com/jay3cx/fundmind/internal/db"
	"github.com/jay3cx/fundmind/internal/quant"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// QuantHandler 量化分析 API
type QuantHandler struct {
	ds       datasource.FundDataSource
	fundRepo *funddb.FundRepository
}

// NewQuantHandler 创建量化分析处理器
func NewQuantHandler(ds datasource.FundDataSource, fundRepo *funddb.FundRepository) *QuantHandler {
	return &QuantHandler{ds: ds, fundRepo: fundRepo}
}

// RegisterRoutes 注册量化分析路由
func (h *QuantHandler) RegisterRoutes(v1 *gin.RouterGroup) {
	qg := v1.Group("/quant")
	{
		qg.POST("/backtest", h.handleBacktest)
		qg.POST("/dca", h.handleDCA)
		qg.POST("/compare", h.handleCompare)
	}
}

// loadNAVSeries 从 DB 加载净值序列，不足时自动从数据源拉取
func (h *QuantHandler) loadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error) {
	navList, err := h.ds.GetFundNAV(ctx, code, days)
	if err != nil {
		return nil, err
	}

	// 同时缓存到 DB
	if h.fundRepo != nil && len(navList) > 0 {
		if saveErr := h.fundRepo.SaveNAVHistory(ctx, code, navList); saveErr != nil {
			logger.Warn("缓存净值失败", zap.String("code", code), zap.Error(saveErr))
		}
	}

	// 转换为 quant.NavSeries（需要倒序 → 升序）
	points := make([]quant.NavPoint, len(navList))
	for i, nav := range navList {
		points[len(navList)-1-i] = quant.NavPoint{
			Date:   nav.Date,
			NAV:    nav.UnitNAV,
			AccNAV: nav.AccumNAV,
		}
	}

	return &quant.NavSeries{FundCode: code, Points: points}, nil
}

func (h *QuantHandler) handleBacktest(c *gin.Context) {
	var req struct {
		Holdings    []quant.HoldingWeight `json:"holdings" binding:"required"`
		Days        int                   `json:"days"`
		InitialCash float64              `json:"initial_cash"`
		Rebalance   string               `json:"rebalance"`
		Benchmark   string               `json:"benchmark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Days <= 0 {
		req.Days = 365
	}

	ctx := c.Request.Context()
	navData := make(map[string]*quant.NavSeries)

	// 加载所有基金净值
	for _, h := range req.Holdings {
		series, err := h.loadNAVSeries(ctx, h.FundCode, req.Days) // 注意：这里需要用 handler 方法
		_ = series
		_ = err
	}
	// 修正：通过 handler 加载
	for _, holding := range req.Holdings {
		series, err := h.loadNAVSeries(ctx, holding.FundCode, req.Days)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "加载基金 " + holding.FundCode + " 净值失败: " + err.Error()})
			return
		}
		navData[holding.FundCode] = series
	}

	// 加载基准
	if req.Benchmark != "" {
		series, err := h.loadNAVSeries(ctx, req.Benchmark, req.Days)
		if err == nil {
			navData[req.Benchmark] = series
		}
	}

	btReq := &quant.BacktestRequest{
		Holdings:    req.Holdings,
		InitialCash: req.InitialCash,
		Rebalance:   quant.RebalanceType(req.Rebalance),
		Benchmark:   req.Benchmark,
	}

	result, err := quant.RunBacktest(btReq, navData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *QuantHandler) handleDCA(c *gin.Context) {
	var req struct {
		FundCode  string  `json:"fund_code" binding:"required"`
		Strategy  string  `json:"strategy"`
		Amount    float64 `json:"amount" binding:"required"`
		Frequency string  `json:"frequency"`
		Days      int     `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Days <= 0 {
		req.Days = 1095 // 默认3年
	}
	if req.Frequency == "" {
		req.Frequency = "monthly"
	}
	if req.Strategy == "" {
		req.Strategy = "fixed"
	}

	series, err := h.loadNAVSeries(c.Request.Context(), req.FundCode, req.Days)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "加载净值失败: " + err.Error()})
		return
	}

	dcaReq := &quant.DCARequest{
		FundCode:  req.FundCode,
		Strategy:  quant.DCAStrategy(req.Strategy),
		Amount:    req.Amount,
		Frequency: req.Frequency,
	}

	result, err := quant.RunDCA(dcaReq, series)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *QuantHandler) handleCompare(c *gin.Context) {
	var req struct {
		FundCodes []string `json:"fund_codes" binding:"required"`
		Period    string   `json:"period"`
		Days      int      `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if len(req.FundCodes) < 2 || len(req.FundCodes) > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 2-5 只基金代码"})
		return
	}
	if req.Days <= 0 {
		req.Days = 365
	}
	if req.Period == "" {
		req.Period = "1y"
	}

	ctx := c.Request.Context()
	navData := make(map[string]*quant.NavSeries)
	fundNames := make(map[string]string)

	for _, code := range req.FundCodes {
		series, err := h.loadNAVSeries(ctx, code, req.Days)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "加载基金 " + code + " 净值失败: " + err.Error()})
			return
		}
		navData[code] = series

		// 获取基金名称
		fund, err := h.ds.GetFundInfo(ctx, code)
		if err == nil {
			fundNames[code] = fund.Name
		}
	}

	compareReq := &quant.CompareRequest{
		FundCodes: req.FundCodes,
		Period:    req.Period,
	}

	result, err := quant.RunCompare(compareReq, navData, fundNames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

**Step 2: Register routes in `router.go`**

在 `SetupRouter` 中，在 `taskHandler.RegisterRoutes(v1)` 之后添加:

```go
// ====== 量化分析 ======
quantHandler := NewQuantHandler(cachedDS, fundRepo)
quantHandler.RegisterRoutes(v1)
```

**Step 3: Fix compilation issue in handleBacktest** — 第一个循环有变量遮蔽，删除那段：

原始代码中有两个循环加载 holdings，删除第一个错误的循环（`for _, h := range req.Holdings`），只保留第二个正确的（`for _, holding := range req.Holdings`）。

**Step 4: Build**

Run: `go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add internal/api/quant_handler.go internal/api/router.go
git commit -m "feat(api): add /quant/backtest, /dca, /compare API endpoints"
```

---

## Task 8: Agent 工具 — backtest_portfolio, simulate_dca, compare_funds

**Files:**
- Create: `internal/agent/quant_tools.go`
- Modify: `internal/api/router.go` (注册工具)

**Step 1: Create `quant_tools.go`**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jay3cx/fundmind/internal/quant"
)

// ===== quantDataLoader 接口 =====

// QuantDataLoader 量化数据加载接口（解耦 API 层的数据获取）
type QuantDataLoader interface {
	LoadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error)
	GetFundName(ctx context.Context, code string) string
}

// ===== BacktestPortfolioTool =====

type BacktestPortfolioTool struct {
	loader QuantDataLoader
}

func NewBacktestPortfolioTool(loader QuantDataLoader) *BacktestPortfolioTool {
	return &BacktestPortfolioTool{loader: loader}
}

func (t *BacktestPortfolioTool) Name() string { return "backtest_portfolio" }

func (t *BacktestPortfolioTool) Description() string {
	return "回测基金组合的历史表现。输入一组基金代码和权重比例，返回年化收益、最大回撤、夏普比率等风控指标。适合用户想要评估组合配置效果时使用。"
}

func (t *BacktestPortfolioTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"holdings": map[string]any{
				"type":        "string",
				"description": "基金组合，格式: '代码1:权重1,代码2:权重2'，如 '005827:0.6,110011:0.4'。权重之和须为1。",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "回测天数，默认365天（1年）",
			},
		},
		"required": []string{"holdings"},
	}
}

func (t *BacktestPortfolioTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	holdingsStr := getStringArg(args, "holdings")
	days := getIntArg(args, "days", 365)

	holdings, err := parseHoldings(holdingsStr)
	if err != nil {
		return "", err
	}

	navData := make(map[string]*quant.NavSeries)
	for _, h := range holdings {
		series, err := t.loader.LoadNAVSeries(ctx, h.FundCode, days)
		if err != nil {
			return "", fmt.Errorf("加载 %s 净值失败: %w", h.FundCode, err)
		}
		navData[h.FundCode] = series
	}

	req := &quant.BacktestRequest{
		Holdings:    holdings,
		InitialCash: 100000,
		Rebalance:   quant.RebalanceNone,
	}

	result, err := quant.RunBacktest(req, navData)
	if err != nil {
		return "", err
	}

	return formatBacktestResult(result, holdings, t.loader, ctx), nil
}

func formatBacktestResult(r *quant.BacktestResult, holdings []quant.HoldingWeight, loader QuantDataLoader, ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("## 组合回测结果\n\n")
	sb.WriteString("### 组合配置\n")
	for _, h := range holdings {
		name := loader.GetFundName(ctx, h.FundCode)
		sb.WriteString(fmt.Sprintf("- %s（%s）: %.0f%%\n", name, h.FundCode, h.Weight*100))
	}
	sb.WriteString("\n### 风控指标\n")
	sb.WriteString(fmt.Sprintf("- 累计收益率: %.2f%%\n", r.TotalReturn*100))
	sb.WriteString(fmt.Sprintf("- 年化收益率: %.2f%%\n", r.AnnualReturn*100))
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", r.MaxDrawdown*100))
	sb.WriteString(fmt.Sprintf("- 夏普比率: %.3f\n", r.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- 年化波动率: %.2f%%\n", r.Volatility*100))
	sb.WriteString(fmt.Sprintf("- Sortino 比率: %.3f\n", r.SortinoRatio))
	sb.WriteString(fmt.Sprintf("- Calmar 比率: %.3f\n", r.CalmarRatio))
	return sb.String()
}

// ===== SimulateDCATool =====

type SimulateDCATool struct {
	loader QuantDataLoader
}

func NewSimulateDCATool(loader QuantDataLoader) *SimulateDCATool {
	return &SimulateDCATool{loader: loader}
}

func (t *SimulateDCATool) Name() string { return "simulate_dca" }

func (t *SimulateDCATool) Description() string {
	return "模拟定投策略的历史收益。输入基金代码、每期金额和策略类型，返回累计投入、终值、年化收益、对比一次性投入的超额收益。适合用户想要比较定投方案时使用。"
}

func (t *SimulateDCATool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码",
			},
			"amount": map[string]any{
				"type":        "number",
				"description": "每期定投金额（元），默认1000",
			},
			"strategy": map[string]any{
				"type":        "string",
				"description": "定投策略: fixed(固定金额) / value(目标价值) / smart(智能定投)，默认 fixed",
				"enum":        []string{"fixed", "value", "smart"},
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "模拟天数，默认1095天（3年）",
			},
		},
		"required": []string{"code"},
	}
}

func (t *SimulateDCATool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}
	amount := getFloatArg(args, "amount", 1000)
	strategy := getStringArg(args, "strategy")
	if strategy == "" {
		strategy = "fixed"
	}
	days := getIntArg(args, "days", 1095)

	series, err := t.loader.LoadNAVSeries(ctx, code, days)
	if err != nil {
		return "", fmt.Errorf("加载净值失败: %w", err)
	}

	req := &quant.DCARequest{
		FundCode:  code,
		Strategy:  quant.DCAStrategy(strategy),
		Amount:    amount,
		Frequency: "monthly",
	}

	result, err := quant.RunDCA(req, series)
	if err != nil {
		return "", err
	}

	name := t.loader.GetFundName(ctx, code)
	return formatDCAResult(result, name, code), nil
}

func formatDCAResult(r *quant.DCAResult, name, code string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（%s）定投模拟\n\n", name, code))
	sb.WriteString(fmt.Sprintf("- 策略: %s\n", r.Strategy))
	sb.WriteString(fmt.Sprintf("- 定投次数: %d 期\n", len(r.Transactions)))
	sb.WriteString(fmt.Sprintf("- 累计投入: %.0f 元\n", r.TotalInvested))
	sb.WriteString(fmt.Sprintf("- 当前市值: %.0f 元\n", r.FinalValue))
	sb.WriteString(fmt.Sprintf("- 累计收益率: %.2f%%\n", r.TotalReturn*100))
	sb.WriteString(fmt.Sprintf("- 平均成本: %.4f\n", r.AvgCost))
	sb.WriteString(fmt.Sprintf("\n### 对比一次性投入\n"))
	sb.WriteString(fmt.Sprintf("- 一次性投入收益率: %.2f%%\n", r.LumpSumReturn*100))
	sb.WriteString(fmt.Sprintf("- 定投超额收益: %.2f%%\n", r.ExcessReturn*100))
	return sb.String()
}

// ===== CompareFundsTool =====

type CompareFundsTool struct {
	loader QuantDataLoader
}

func NewCompareFundsTool(loader QuantDataLoader) *CompareFundsTool {
	return &CompareFundsTool{loader: loader}
}

func (t *CompareFundsTool) Name() string { return "compare_funds" }

func (t *CompareFundsTool) Description() string {
	return "对比多只基金的表现。输入2-5只基金代码，返回各基金收益率、风险指标、相关性矩阵的对比。适合用户想要横向对比筛选基金时使用。"
}

func (t *CompareFundsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"codes": map[string]any{
				"type":        "string",
				"description": "基金代码列表，逗号分隔，如 '005827,110011,161725'",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "对比天数，默认365天",
			},
		},
		"required": []string{"codes"},
	}
}

func (t *CompareFundsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	codesStr := getStringArg(args, "codes")
	days := getIntArg(args, "days", 365)

	codes := strings.Split(codesStr, ",")
	for i := range codes {
		codes[i] = strings.TrimSpace(codes[i])
	}
	if len(codes) < 2 || len(codes) > 5 {
		return "", fmt.Errorf("请提供 2-5 只基金代码")
	}

	navData := make(map[string]*quant.NavSeries)
	fundNames := make(map[string]string)

	for _, code := range codes {
		series, err := t.loader.LoadNAVSeries(ctx, code, days)
		if err != nil {
			return "", fmt.Errorf("加载 %s 净值失败: %w", code, err)
		}
		navData[code] = series
		fundNames[code] = t.loader.GetFundName(ctx, code)
	}

	req := &quant.CompareRequest{FundCodes: codes, Period: "1y"}
	result, err := quant.RunCompare(req, navData, fundNames)
	if err != nil {
		return "", err
	}

	return formatCompareResult(result), nil
}

func formatCompareResult(r *quant.CompareResult) string {
	var sb strings.Builder
	sb.WriteString("## 基金对比分析\n\n")

	// 业绩表
	sb.WriteString("### 业绩与风险指标\n")
	sb.WriteString("| 基金 | 累计收益 | 年化收益 | 最大回撤 | 夏普比率 | 波动率 |\n")
	sb.WriteString("|------|---------|---------|---------|---------|--------|\n")
	for _, f := range r.Funds {
		sb.WriteString(fmt.Sprintf("| %s（%s）| %.2f%% | %.2f%% | %.2f%% | %.3f | %.2f%% |\n",
			f.Name, f.Code, f.TotalReturn*100, f.AnnualReturn*100,
			f.MaxDrawdown*100, f.SharpeRatio, f.Volatility*100))
	}

	// 相关性
	sb.WriteString("\n### 相关性矩阵\n")
	n := len(r.Matrix.Codes)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			corr := r.Matrix.Values[i][j]
			label := "低相关"
			if corr > 0.7 {
				label = "高相关"
			} else if corr > 0.3 {
				label = "中等相关"
			}
			sb.WriteString(fmt.Sprintf("- %s ↔ %s: %.3f（%s）\n",
				r.Matrix.Codes[i], r.Matrix.Codes[j], corr, label))
		}
	}

	return sb.String()
}

// ===== 解析辅助 =====

func parseHoldings(s string) ([]quant.HoldingWeight, error) {
	parts := strings.Split(s, ",")
	var holdings []quant.HoldingWeight
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("格式错误: '%s'，应为 '代码:权重'", part)
		}
		code := strings.TrimSpace(kv[0])
		weight, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("权重解析失败: '%s'", kv[1])
		}
		holdings = append(holdings, quant.HoldingWeight{FundCode: code, Weight: weight})
	}
	return holdings, nil
}

func getFloatArg(args map[string]any, key string, defaultVal float64) float64 {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return defaultVal
}
```

**Step 2: 在 `router.go` 注册工具**

在 `createToolRegistry` 函数不动。在 `SetupRouter` 中添加量化工具注册。在 `quantHandler.RegisterRoutes(v1)` 之后，创建 QuantDataLoader 并注册工具:

```go
// ====== 量化 Agent 工具 ======
quantLoader := &quantDataLoaderImpl{ds: cachedDS}
toolRegistry.Register(agent.NewBacktestPortfolioTool(quantLoader))
toolRegistry.Register(agent.NewSimulateDCATool(quantLoader))
toolRegistry.Register(agent.NewCompareFundsTool(quantLoader))
```

并在 `router.go` 底部添加适配器:

```go
// quantDataLoaderImpl 适配 QuantDataLoader
type quantDataLoaderImpl struct {
	ds datasource.FundDataSource
}

func (l *quantDataLoaderImpl) LoadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error) {
	navList, err := l.ds.GetFundNAV(ctx, code, days)
	if err != nil {
		return nil, err
	}
	points := make([]quant.NavPoint, len(navList))
	for i, nav := range navList {
		points[len(navList)-1-i] = quant.NavPoint{Date: nav.Date, NAV: nav.UnitNAV, AccNAV: nav.AccumNAV}
	}
	return &quant.NavSeries{FundCode: code, Points: points}, nil
}

func (l *quantDataLoaderImpl) GetFundName(ctx context.Context, code string) string {
	fund, err := l.ds.GetFundInfo(ctx, code)
	if err != nil {
		return code
	}
	return fund.Name
}
```

**Step 3: Build**

Run: `go build ./...`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add internal/agent/quant_tools.go internal/api/quant_handler.go internal/api/router.go
git commit -m "feat: add quant Agent tools (backtest/dca/compare) and API endpoints"
```

---

## Task 9: 前端 — 安装 Recharts + 新增 TypeScript 类型

**Files:**
- Modify: `web/package.json` (install recharts)
- Modify: `web/src/types/index.ts` (add quant types)
- Modify: `web/src/lib/api.ts` (add quant API functions)

**Step 1: Install Recharts**

Run: `cd web && npm install recharts`

**Step 2: Add TypeScript types to `web/src/types/index.ts`**

在文件末尾添加:

```typescript
// ===== 量化分析类型 =====

export interface HoldingWeight {
  fund_code: string
  weight: number
}

export interface BacktestResult {
  total_return: number
  annual_return: number
  max_drawdown: number
  sharpe_ratio: number
  volatility: number
  sortino_ratio: number
  calmar_ratio: number
  equity_curve: CurvePoint[]
  drawdown_curve: CurvePoint[]
  benchmark_curve?: CurvePoint[]
  fund_metrics: FundMetricRow[]
}

export interface CurvePoint {
  date: string
  value: number
}

export interface FundMetricRow {
  fund_code: string
  weight: number
  total_return: number
}

export interface DCAResult {
  strategy: string
  total_invested: number
  final_value: number
  total_return: number
  annual_return: number
  avg_cost: number
  lump_sum_return: number
  excess_return: number
  invest_curve: CurvePoint[]
  value_curve: CurvePoint[]
  cost_curve: CurvePoint[]
  transactions: DCATransaction[]
}

export interface DCATransaction {
  date: string
  amount: number
  nav: number
  shares: number
}

export interface CompareResult {
  period: string
  funds: FundCompareProfile[]
  correlation_matrix: CorrelationMatrix
}

export interface FundCompareProfile {
  code: string
  name: string
  total_return: number
  annual_return: number
  volatility: number
  sharpe_ratio: number
  max_drawdown: number
  sortino_ratio: number
  normalized_nav: CurvePoint[]
}

export interface CorrelationMatrix {
  codes: string[]
  values: number[][]
}
```

**Step 3: Add API functions to `web/src/lib/api.ts`**

在文件末尾添加:

```typescript
// ===== 量化分析 API =====

export async function runBacktest(params: {
  holdings: { fund_code: string; weight: number }[]
  days?: number
  initial_cash?: number
  rebalance?: string
  benchmark?: string
}): Promise<BacktestResult> {
  const res = await fetch('/api/v1/quant/backtest', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runDCA(params: {
  fund_code: string
  amount: number
  strategy?: string
  frequency?: string
  days?: number
}): Promise<DCAResult> {
  const res = await fetch('/api/v1/quant/dca', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function runCompare(params: {
  fund_codes: string[]
  period?: string
  days?: number
}): Promise<CompareResult> {
  const res = await fetch('/api/v1/quant/compare', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
```

**Step 4: Type check**

Run: `cd web && npx tsc --noEmit`
Expected: SUCCESS（或只有不相关的已有错误）

**Step 5: Commit**

```bash
git add web/package.json web/package-lock.json web/src/types/index.ts web/src/lib/api.ts
git commit -m "feat(web): add Recharts dependency and quant API types/functions"
```

---

## Task 10: 前端 — 回测实验室页面

**Files:**
- Create: `web/src/pages/BacktestPage.tsx`
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create `BacktestPage.tsx`**

实现：基金搜索添加组件 + 权重滑块 + 天数选择 + 运行回测按钮 + 结果图表展示（净值曲线、回撤曲线、指标卡片）。使用 Recharts `LineChart` + `AreaChart`。

页面结构:
- 顶部：基金选择区域（输入代码+回车添加，每只基金有权重 input + 删除按钮）
- 中间：参数配置（天数下拉、再平衡策略、基准代码）
- 底部：结果区域（条件渲染）
  - 指标卡片网格（4列：年化收益、最大回撤、夏普、波动率）
  - 净值曲线图 (`LineChart` from `equity_curve`)
  - 回撤图 (`AreaChart` from `drawdown_curve`)

**Step 2: Register route in `App.tsx`**

```tsx
import BacktestPage from './pages/BacktestPage'
// 在 Route 列表中添加:
<Route path="/backtest" element={<BacktestPage />} />
```

**Step 3: Build**

Run: `cd web && npm run build`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add web/src/pages/BacktestPage.tsx web/src/App.tsx
git commit -m "feat(web): add Backtest Lab page with equity/drawdown charts"
```

---

## Task 11: 前端 — 定投模拟页面

**Files:**
- Create: `web/src/pages/DCAPage.tsx`
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create `DCAPage.tsx`**

页面结构:
- 基金代码输入
- 参数配置：每期金额、频率（月）、策略类型（固定/价值/智能）、天数
- 结果展示：
  - 指标摘要（累计投入、终值、收益率、平均成本、对比一次性投入）
  - 双线图（`LineChart`：投入金额 vs 账户市值 + 一次性投入虚线）
  - 交易明细表（可折叠）

**Step 2: Register route**

```tsx
import DCAPage from './pages/DCAPage'
<Route path="/dca" element={<DCAPage />} />
```

**Step 3: Build**

Run: `cd web && npm run build`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add web/src/pages/DCAPage.tsx web/src/App.tsx
git commit -m "feat(web): add DCA simulation page with invest vs value chart"
```

---

## Task 12: 前端 — 基金PK页面

**Files:**
- Create: `web/src/pages/ComparePage.tsx`
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create `ComparePage.tsx`**

页面结构:
- 基金代码输入（最多5只，回车添加）
- 时间区间选择
- 结果展示：
  - 归一化净值叠加图（每只基金一条线，`LineChart`）
  - 指标对比表格（sortable）
  - 相关性热力图（用 Recharts 的自定义 cell 或简单表格 + 背景色）

**Step 2: Register route**

```tsx
import ComparePage from './pages/ComparePage'
<Route path="/compare" element={<ComparePage />} />
```

**Step 3: Build**

Run: `cd web && npm run build`
Expected: SUCCESS

**Step 4: Commit**

```bash
git add web/src/pages/ComparePage.tsx web/src/App.tsx
git commit -m "feat(web): add Fund PK page with normalized NAV overlay and correlation matrix"
```

---

## Task 13: 侧边栏导航 + 最终集成测试

**Files:**
- Modify: `web/src/components/Sidebar.tsx` 或 `Layout.tsx` (add navigation links)

**Step 1: Add navigation links**

在侧边栏或导航栏添加三个新入口:
- 回测实验室 → `/backtest`
- 定投模拟 → `/dca`
- 基金PK → `/compare`

**Step 2: Run full backend tests**

Run: `go test -v ./internal/quant/... ./internal/db/...`
Expected: ALL PASS

**Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: SUCCESS

**Step 4: Build backend**

Run: `go build -o bin/fundmind ./cmd/...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add web/src/components/
git commit -m "feat(web): add navigation links for quant pages (backtest/dca/compare)"
```

---

## Summary

| Task | Component | Estimated Steps |
|------|-----------|----------------|
| 1 | DB: GetNAVHistory | 5 |
| 2 | quant/series.go | 5 |
| 3 | quant/metrics.go | 5 |
| 4 | quant/backtest.go | 5 |
| 5 | quant/dca.go | 5 |
| 6 | quant/compare.go | 6 |
| 7 | API endpoints | 5 |
| 8 | Agent tools | 4 |
| 9 | Frontend types + Recharts | 5 |
| 10 | BacktestPage | 4 |
| 11 | DCAPage | 4 |
| 12 | ComparePage | 4 |
| 13 | Navigation + final tests | 5 |
| **Total** | | **62 steps** |

Dependencies: Task 1 → Task 7. Tasks 2-6 are independent. Task 7 depends on 4-6. Task 8 depends on 4-6. Tasks 9-12 depend on 7. Task 13 depends on 10-12.
