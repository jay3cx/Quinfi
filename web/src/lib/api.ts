// API 封装层
import type { Fund, NAV, Holding, Article, Feed, ScanResponse } from "@/types"

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
