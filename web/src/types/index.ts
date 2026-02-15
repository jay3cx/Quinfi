// 共享类型定义 — 对应后端 API 响应

export interface Fund {
  code: string
  name: string
  type: string
  company: string
  scale: number
  establish_at: string
  manager: Manager | null
}

export interface Manager {
  id: string
  name: string
  years: number
  total_scale: number
  fund_count: number
  background: string
  start_date: string
}

export interface NAV {
  date: string
  unit_nav: number
  accum_nav: number
  daily_return: number
}

export interface Holding {
  stock_code: string
  stock_name: string
  ratio: number
  share_count: number
  market_value: number
}

export interface Article {
  guid: string
  feed_id: string
  title: string
  link: string
  description: string
  content: string
  author: string
  pub_date: string
  fetched_at: string
  source: string
  summary?: string
  sentiment?: "positive" | "negative" | "neutral"
  sentiment_reason?: string
  keywords?: string[]
  summarized_at?: string
}

export interface Feed {
  id: string
  url: string
  title: string
  description: string
  last_fetched: string
  interval: number
  enabled: boolean
}

export interface SSEEvent {
  content?: string
  session_id?: string
  type?: "text" | "tool_start" | "tool_result" | "thinking" | "debate_phase"
  tool_name?: string
  done?: boolean
  error?: string
}

// 深度分析
export interface DeepReport {
  fund_code: string
  fund_name: string
  fund_analysis: AnalysisReport | null
  manager_report: ManagerReport | null
  macro_report: MacroReport | null
  rebalance_result: RebalanceResult | null
  debate_result: DebateResult | null
  generated_at: string
}

export interface AnalysisReport {
  fund_code: string
  fund_name: string
  summary: string
  holding_analysis: {
    industry_distribution: { industry: string; weight: number }[]
    concentration: { top3_ratio: number; top5_ratio: number; top10_ratio: number; level: string }
    top_holdings: { stock_code: string; stock_name: string; ratio: number; comment: string }[]
    analysis_text: string
  } | null
  risk_assessment: {
    risk_level: string
    volatility: string
    max_drawdown: string
    risk_warnings: string[]
    assessment_text: string
  } | null
  recommendation: {
    action: string
    confidence: string
    reasons: string[]
    caveats: string[]
    text: string
  } | null
}

export interface ManagerReport {
  manager_name: string
  years: number
  style: string
  strengths: string[]
  weaknesses: string[]
  best_performance: string
  analysis_text: string
}

export interface MacroReport {
  market_sentiment: string
  key_events: string[]
  impact: string
  risk_factors: string[]
  analysis_text: string
}

export interface RebalanceResult {
  fund_code: string
  report_date: string
  changes: RebalanceInfo[]
  summary: string
}

export interface RebalanceInfo {
  stock_code: string
  stock_name: string
  action: string
  prev_ratio: number
  curr_ratio: number
  change_ratio: number
  comment: string
}

// 辩论论点
export interface DebateArgument {
  role: string          // "bull" | "bear"
  position: string      // 核心立场
  points: string[]      // 论据列表
  confidence: number    // 置信度 0-100
}

// 裁判结论
export interface DebateVerdict {
  summary: string
  bull_strength: string
  bear_strength: string
  suggestion: string
  risk_warnings: string[]
  confidence: number
}

export interface DebateResult {
  fund_code: string
  fund_name: string
  bull_case: DebateArgument | null
  bear_case: DebateArgument | null
  bull_rebuttal: DebateArgument | null
  bear_rebuttal: DebateArgument | null
  verdict: DebateVerdict | null
  formatted: string  // Markdown 兜底
}

// SSE 辩论阶段更新
export interface DebatePhaseUpdate {
  type: "debate_phase"
  phase: string
  argument?: DebateArgument
  verdict?: DebateVerdict
}

// 异步任务
export interface TaskInfo {
  id: string
  type: string
  status: "pending" | "running" | "completed" | "failed"
  payload: string
  result?: string
  error?: string
  progress: number
  progress_msg?: string
  created_at: string
  completed_at?: string
}

export interface TaskUpdate {
  task_id: string
  status: string
  progress: number
  progress_msg?: string
  metadata?: string     // JSON 格式的结构化附加数据（如辩论阶段详情）
  result?: string
  error?: string
  done?: boolean
}

// ====== 量化分析 ======

export interface CurvePoint {
  date: string
  value: number
}

// 回测
export interface HoldingWeight {
  fund_code: string
  weight: number
}

export interface FundMetricRow {
  code: string
  name: string
  total_return: number
  annual_return: number
  max_drawdown: number
  sharpe_ratio: number
}

export interface BacktestResult {
  total_return: number
  annual_return: number
  max_drawdown: number
  sharpe_ratio: number
  volatility: number
  sortino_ratio: number
  calmar_ratio: number
  nav_curve: CurvePoint[]
  drawdown_curve: CurvePoint[]
  fund_metrics: FundMetricRow[]
  benchmark_return?: number
}

// 定投
export interface DCATransaction {
  date: string
  nav: number
  amount: number
  shares: number
  total_shares: number
  total_cost: number
  market_value: number
}

export interface DCAResult {
  strategy: string
  total_invested: number
  final_value: number
  total_return: number
  avg_cost: number
  lump_sum_return: number
  excess_return: number
  transactions: DCATransaction[]
  value_curve: CurvePoint[]
}

// 基金对比
export interface FundProfile {
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

export interface CompareResult {
  period: string
  funds: FundProfile[]
  correlation_matrix: CorrelationMatrix
}

// 截图扫描
export interface ScanHolding {
  code: string
  name: string
  amount: number
  daily_return?: number
  total_profit?: number
  total_profit_rate?: number
}

export interface ScanResponse {
  holdings: ScanHolding[]
  total_value: number
  auto_added?: number
  error?: string
}
