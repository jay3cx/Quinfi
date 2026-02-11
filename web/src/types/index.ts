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
  type?: "text" | "tool_start" | "tool_result" | "thinking"
  tool_name?: string
  done?: boolean
  error?: string
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
