// API 封装层
import type { Fund, NAV, Holding, Article, Feed, ScanResponse, DeepReport, TaskInfo, HoldingWeight, BacktestResult, DCAResult, CompareResult } from "@/types"

const API_BASE = "/api/v1"

async function apiFetch<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || "Request failed")
  }
  return res.json()
}

// 基金
export const getFund = (code: string) =>
  apiFetch<Fund>(`/fund/${code}`)

export const getFundNAV = (code: string, opts?: { period?: string; days?: number }) => {
  const q = new URLSearchParams()
  if (opts?.period) q.set("period", opts.period)
  else if (opts?.days) q.set("days", String(opts.days))
  else q.set("period", "1m")
  return apiFetch<{ code: string; data: NAV[] }>(`/fund/${code}/nav?${q}`)
}

export const getFundHoldings = (code: string) =>
  apiFetch<{ code: string; data: Holding[] }>(`/fund/${code}/holdings`)

// 新闻
export const getNews = (params: { limit?: number; offset?: number; sentiment?: string } = {}) => {
  const q = new URLSearchParams()
  if (params.limit) q.set("limit", String(params.limit))
  if (params.offset) q.set("offset", String(params.offset))
  if (params.sentiment) q.set("sentiment", params.sentiment)
  return apiFetch<{ data: Article[]; total: number; limit: number; offset: number }>(`/news?${q}`)
}

export const getNewsDetail = (guid: string) =>
  apiFetch<Article>(`/news/${guid}`)

export const getFeeds = () =>
  apiFetch<{ data: Feed[]; total: number }>("/feeds")

// RSS 调度器控制
export const getRSSStatus = () =>
  apiFetch<{ running: boolean; feed_count: number; enabled: boolean }>("/rss/status")

export const toggleRSS = (enabled: boolean) =>
  fetch(`${API_BASE}/rss/toggle`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || "操作失败")
    return data as { running: boolean; message: string }
  })

// 会话
export interface SessionInfo {
  id: string
  title: string
  last_active_at: string
  message_count: number
}

export interface SessionMessage {
  role: string
  content: string
  metadata?: string  // JSON: 工具调用记录
}

export const getSessions = () =>
  apiFetch<{ data: SessionInfo[]; total: number }>("/sessions")

export const getSessionMessages = (id: string) =>
  apiFetch<{ session_id: string; messages: SessionMessage[] }>(`/sessions/${id}/messages`)

export const deleteSession = (id: string) =>
  fetch(`${API_BASE}/sessions/${id}`, { method: "DELETE" })

// 持仓
export interface PortfolioHolding {
  code: string
  name: string
  amount: number
  weight: number
  total_profit?: number
  total_profit_rate?: number
}

export const getPortfolio = () =>
  apiFetch<{ data: PortfolioHolding[]; total: number; total_value?: number }>("/portfolio")

export const addPortfolioHolding = (code: string, name?: string, amount?: number) =>
  fetch(`${API_BASE}/portfolio`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, name, amount }),
  }).then((res) => {
    if (!res.ok) throw new Error("添加持仓失败")
    return res.json() as Promise<{ status: string; code: string }>
  })

export const scanPortfolio = (imageBase64: string, autoAdd = false) =>
  fetch(`${API_BASE}/portfolio/scan`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ image: imageBase64, auto_add: autoAdd }),
  }).then(async (res) => {
    const data = await res.json() as ScanResponse
    if (!res.ok) throw new Error(data.error || "识别失败")
    return data
  })

export const removePortfolioHolding = (code: string) =>
  fetch(`${API_BASE}/portfolio/${code}`, { method: "DELETE" }).then((res) => {
    if (!res.ok) throw new Error("删除持仓失败")
    return res.json() as Promise<{ status: string; invalidated: number }>
  })

// 简报
export interface Brief {
  id: number
  content: string
  type: string
  created_at: string
}

export const getBriefs = () =>
  apiFetch<{ data: Brief[]; total: number }>("/briefs")

export const generateBrief = () =>
  fetch(`${API_BASE}/briefs/generate`, { method: "POST" }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || "生成失败")
    return data as { status: string; message: string }
  })

// 深度分析（异步任务）
export const submitDeepAnalysis = (code: string) =>
  fetch(`${API_BASE}/analysis/deep`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || "提交分析失败")
    return data as { task_id: string; status: string; stream: string }
  })

// 任务查询
export const getTask = (id: string) =>
  apiFetch<TaskInfo>(`/tasks/${id}`)

// 任务进度 SSE 流
export const streamTaskProgress = (
  taskId: string,
  callbacks: {
    onProgress?: (progress: number, msg: string, metadata?: string) => void
    onComplete?: (result: string) => void
    onError?: (error: string) => void
  }
) => {
  const es = new EventSource(`${API_BASE}/tasks/${taskId}/stream`)

  es.onmessage = (event) => {
    if (event.data === "[DONE]") {
      es.close()
      return
    }
    try {
      const update = JSON.parse(event.data) as {
        status: string
        progress: number
        progress_msg?: string
        metadata?: string
        result?: string
        error?: string
        done?: boolean
      }

      if (update.status === "failed" && update.error) {
        callbacks.onError?.(update.error)
        es.close()
        return
      }

      callbacks.onProgress?.(update.progress, update.progress_msg || "", update.metadata)

      if (update.done && update.result) {
        callbacks.onComplete?.(update.result)
        es.close()
      }
    } catch { /* ignore parse errors */ }
  }

  es.onerror = () => {
    es.close()
    // SSE 断线降级为轮询
    const poll = setInterval(async () => {
      try {
        const task = await getTask(taskId)
        callbacks.onProgress?.(task.progress, task.progress_msg || "")
        if (task.status === "completed" && task.result) {
          callbacks.onComplete?.(task.result)
          clearInterval(poll)
        } else if (task.status === "failed") {
          callbacks.onError?.(task.error || "任务失败")
          clearInterval(poll)
        }
      } catch {
        clearInterval(poll)
        callbacks.onError?.("查询任务状态失败")
      }
    }, 3000)
  }

  return () => es.close()
}

// ====== 量化分析 ======

async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || "Request failed")
  }
  return res.json()
}

export const runBacktest = (holdings: HoldingWeight[], days = 365, opts?: { initial_cash?: number; rebalance?: string; benchmark?: string }) =>
  apiPost<BacktestResult>("/quant/backtest", { holdings, days, ...opts })

export const runDCA = (fund_code: string, amount: number, opts?: { strategy?: string; frequency?: string; days?: number }) =>
  apiPost<DCAResult>("/quant/dca", { fund_code, amount, ...opts })

export const runCompare = (fund_codes: string[], opts?: { period?: string; days?: number }) =>
  apiPost<CompareResult>("/quant/compare", { fund_codes, ...opts })

/** @deprecated 使用 submitDeepAnalysis + streamTaskProgress */
export const deepAnalysis = (code: string) =>
  fetch(`${API_BASE}/analysis/deep`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  }).then(async (res) => {
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || "深度分析失败")
    return data as DeepReport
  })
