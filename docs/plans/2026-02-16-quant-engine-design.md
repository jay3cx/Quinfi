# FundMind 量化分析引擎设计文档

> 日期: 2026-02-16
> 状态: 已批准，待实施

## 目标

为 FundMind 新增三大智能分析能力，以 Agent 工具 + 独立前端页面两种方式融入系统：

1. **组合回测与风控指标** — 回测任意基金组合的历史表现
2. **定投策略模拟** — 对比固定/价值/智能定投策略的收益差异
3. **跨基金对比分析** — 多只基金多维度横向 PK

## 技术方向

**统一量化引擎**（方向 A）— 在 `internal/quant/` 构建统一计算引擎，三个功能共享基础指标模块。Go 原生实现，无外部依赖。

---

## 1. 核心架构

### 包结构

```
internal/quant/
├── series.go        # 时间序列基础类型 + 收益率计算
├── metrics.go       # 风控指标（夏普、回撤、波动率、Sortino、Calmar）
├── backtest.go      # 组合回测引擎
├── dca.go           # 定投策略模拟器
├── compare.go       # 跨基金对比分析（含相关性矩阵）
└── result.go        # 统一结果结构体（供 API 和 Agent 工具使用）
```

### 数据管道

```
nav_history (DB)
  → []NavPoint{Date, UnitNAV, AccumNAV}
    → ReturnSeries (日收益率序列)
      → 各种指标计算
```

### 核心类型

```go
// 净值时间序列
type NavSeries struct {
    FundCode string
    Points   []NavPoint  // 按日期升序
}

type NavPoint struct {
    Date    time.Time
    NAV     float64      // 单位净值
    AccNAV  float64      // 累计净值
}

// 日收益率序列
type ReturnSeries struct {
    FundCode string
    Dates    []time.Time
    Returns  []float64   // 日收益率 = (NAV_t - NAV_{t-1}) / NAV_{t-1}
}
```

### 基础风控指标 (`metrics.go`)

| 指标 | 公式/说明 |
|------|-----------|
| 年化收益率 | `(终值/初值)^(365/天数) - 1` |
| 年化波动率 | `日收益率标准差 × √252` |
| 夏普比率 | `(年化收益 - 无风险利率) / 年化波动率`，无风险利率默认 2% |
| 最大回撤 | 历史最高点到最低点的最大百分比跌幅 |
| Sortino 比率 | 仅计算下行波动率（负收益率） |
| Calmar 比率 | `年化收益率 / 最大回撤` |

数据来源：从 `nav_history` 表查询，复用现有 `FundRepository`。DB 中净值数据不足时，自动通过 `DataSourceManager` 拉取并缓存入库。

---

## 2. 组合回测引擎 (`backtest.go`)

### 输入

```go
type BacktestRequest struct {
    Holdings    []HoldingWeight  // 基金代码 + 权重
    StartDate   time.Time
    EndDate     time.Time
    InitialCash float64          // 初始资金，默认 100000
    Rebalance   RebalanceType    // 再平衡策略
    Benchmark   string           // 对比基准（可选，如沪深300 "000300"）
}

type HoldingWeight struct {
    FundCode string
    Weight   float64   // 占比，总和 = 1.0
}

type RebalanceType string
const (
    RebalanceNone      RebalanceType = "none"      // 不调仓，买入后持有
    RebalanceMonthly   RebalanceType = "monthly"   // 每月再平衡
    RebalanceQuarterly RebalanceType = "quarterly"  // 每季再平衡
)
```

### 输出

```go
type BacktestResult struct {
    // 组合整体指标
    TotalReturn     float64
    AnnualReturn    float64
    MaxDrawdown     float64
    SharpeRatio     float64
    Volatility      float64
    SortinoRatio    float64
    CalmarRatio     float64

    // 时间序列（供前端画图）
    EquityCurve     []CurvePoint   // 组合净值曲线
    DrawdownCurve   []CurvePoint   // 回撤曲线
    BenchmarkCurve  []CurvePoint   // 基准对比（可选）

    // 每只基金的贡献
    FundMetrics     []FundMetric

    // 再平衡记录
    RebalanceEvents []RebalanceEvent
}
```

### 核心逻辑

1. 从 DB/DataSource 获取所有基金的日净值序列
2. 按权重计算组合日净值：`P_t = Σ(w_i × NAV_i_t / NAV_i_0)`
3. 遇到再平衡日期时，重新归一化权重
4. 计算组合的收益率序列，调用 `metrics.go` 算各项指标
5. 如有基准，同样计算基准的指标用于对比

---

## 3. 定投策略模拟器 (`dca.go`)

### 策略类型

| 策略 | 逻辑 | 适用场景 |
|------|------|---------|
| `fixed` 固定金额 | 每期投入固定金额 X | 最简单，"懒人定投" |
| `value` 目标价值 | 每期补足到目标累计价值，低位多买高位少买甚至卖出 | 更理性，需要资金弹性 |
| `smart` 智能定投 | 基于 MA(250) 均线偏离度调整投入。低于均线多投（最多 1.5x），高于均线少投（最少 0.5x） | 折中方案 |

### 输入

```go
type DCARequest struct {
    FundCode    string
    Strategy    DCAStrategy   // "fixed" / "value" / "smart"
    Amount      float64       // 每期基础金额
    Frequency   string        // "weekly" / "biweekly" / "monthly"
    StartDate   time.Time
    EndDate     time.Time
}
```

### 输出

```go
type DCAResult struct {
    Strategy       DCAStrategy
    TotalInvested  float64      // 累计投入
    FinalValue     float64      // 终值
    TotalReturn    float64      // 累计收益率
    AnnualReturn   float64      // 年化收益率
    AvgCost        float64      // 平均成本（加权）

    // 对比一次性投入
    LumpSumReturn  float64
    ExcessReturn   float64

    // 时间序列
    InvestCurve    []CurvePoint // 累计投入金额
    ValueCurve     []CurvePoint // 账户市值
    CostCurve      []CurvePoint // 平均成本变化

    // 每期明细
    Transactions   []DCATransaction
}
```

### 核心逻辑

1. 按频率生成定投日期序列（跳过非交易日取最近交易日）
2. 每个定投日根据策略计算本次投入金额
3. 计算本次买入份额 = 金额 / 当日净值
4. 累积份额，计算每日市值 = 持有份额 × 当日净值
5. 同时计算 Lump Sum 对比（首日一次性投入总金额）

---

## 4. 跨基金对比分析 (`compare.go`)

### 输入

```go
type CompareRequest struct {
    FundCodes  []string    // 2-5 只基金代码
    Period     string      // "3m" / "6m" / "1y" / "3y" / "5y" / "max"
}
```

### 四个分析维度

**1. 业绩对比** — 归一化净值曲线 + 各区间收益率

```go
type FundProfile struct {
    Code, Name, Type string
    Return1M, Return3M, Return6M, Return1Y, ReturnYTD float64
    AnnualReturn, Volatility, SharpeRatio, MaxDrawdown float64
    SortinoRatio float64
    NormalizedNAV []CurvePoint  // 起点归一化为 1.0
}
```

**2. 风险对比** — 波动率、回撤、风险收益比散点图数据

**3. 相关性矩阵** — N×N 矩阵，衡量基金间收益联动程度

```go
type CorrelationMatrix struct {
    Codes  []string
    Values [][]float64  // correlation[i][j]
}
```

- 相关性 < 0.3：低相关（适合配置组合）
- 相关性 > 0.7：高相关（分散化效果差）

**4. 重仓股重合度**（可选）— 从 holdings_snapshot 表计算前十大重仓股重合比例

```go
type HoldingsOverlap struct {
    Pairs []OverlapPair
}
type OverlapPair struct {
    FundA, FundB   string
    CommonStocks   []string
    OverlapRatio   float64
}
```

---

## 5. Agent 工具集成

新增 3 个工具注册到 `FundAgent`：

| 工具名 | 触发示例 | 返回 |
|--------|---------|------|
| `backtest_portfolio` | "帮我回测招商中证白酒60%+易方达蓝筹40%这个组合" | 核心指标文本 + LLM 解读 |
| `simulate_dca` | "模拟每月定投500块到易方达中小盘，近3年" | 定投结果 + 对比一次性投入 |
| `compare_funds` | "对比一下招商中证白酒和天弘中证食品饮料" | 多维度对比摘要 |

Agent 工具返回结构化数据 + 文字摘要，LLM 基于数据生成个性化洞察。聊天流中展示为 `ToolCallCard`。

---

## 6. API 端点

```
POST /api/v1/quant/backtest     → BacktestResult (JSON)
POST /api/v1/quant/dca          → DCAResult (JSON)
POST /api/v1/quant/compare      → CompareResult (JSON)
```

供前端独立页面调用（参数完整、结果详尽）。Agent 工具内部直接调用 `quant` 包函数。

---

## 7. 前端页面

### 7.1 回测实验室 (`/backtest`)
- 基金选择器（搜索+添加多只）+ 权重滑块
- 时间范围选择 + 再平衡策略选择
- 结果展示：净值曲线图、回撤图、指标卡片、各基金贡献表
- 支持与基准（如沪深300）对比

### 7.2 定投模拟 (`/dca`)
- 单只基金选择 + 金额/频率/策略配置
- 结果展示：投入金额 vs 账户市值双线图、平均成本线、交易明细表
- 一次性投入对比虚线

### 7.3 基金PK (`/compare`)
- 多只基金选择（2-5只）+ 时间区间
- 结果展示：归一化净值叠加图、雷达图（多指标）、相关性热力图、重仓股重合表

### 图表库

使用 **Recharts**（React 生态轻量图表库），契合 React 19 + TailwindCSS 技术栈。

---

## 8. 实施优先级建议

1. **Phase 1**: `internal/quant/` 基础模块（series + metrics）+ 单元测试
2. **Phase 2**: 跨基金对比（数据需求最少，可快速验证管道）
3. **Phase 3**: 组合回测引擎
4. **Phase 4**: 定投策略模拟
5. **Phase 5**: Agent 工具注册 + API 端点
6. **Phase 6**: 前端三个页面
