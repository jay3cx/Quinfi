import { useState, useEffect, useRef } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { SketchChart } from "@/components/SketchChart"
import { DeepAnalysisPanel } from "@/components/DeepAnalysisPanel"
import { DebateTimeline } from "@/components/DebateTimeline"
import { Button } from "@/components/ui/button"
import { ArrowLeft, MessageSquare, Swords, Search, Microscope, Loader2 } from "lucide-react"
import { getFund, getFundNAV, getFundHoldings, submitDeepAnalysis, streamTaskProgress } from "@/lib/api"
import type { Fund, NAV, Holding, DeepReport, DebatePhaseKey, DebatePhaseUpdate } from "@/types"

type TimeRange = "1w" | "1m" | "3m" | "1y"
const TIME_RANGES: { key: TimeRange; label: string }[] = [
    { key: "1w", label: "1周" },
    { key: "1m", label: "1月" },
    { key: "3m", label: "3月" },
    { key: "1y", label: "1年" },
]

function computeDebateActive(completed: Set<DebatePhaseKey>): DebatePhaseKey[] {
    if (!completed.has("bull_case") || !completed.has("bear_case")) {
        const active: DebatePhaseKey[] = []
        if (!completed.has("bull_case")) active.push("bull_case")
        if (!completed.has("bear_case")) active.push("bear_case")
        return active
    }
    if (!completed.has("bull_rebuttal") || !completed.has("bear_rebuttal")) {
        const active: DebatePhaseKey[] = []
        if (!completed.has("bull_rebuttal")) active.push("bull_rebuttal")
        if (!completed.has("bear_rebuttal")) active.push("bear_rebuttal")
        return active
    }
    if (!completed.has("judge_verdict")) return ["judge_verdict"]
    return []
}

export default function FundDetailPage() {
    const { code } = useParams<{ code: string }>()
    const navigate = useNavigate()

    const [fund, setFund] = useState<Fund | null>(null)
    const [navList, setNavList] = useState<NAV[]>([])
    const [holdings, setHoldings] = useState<Holding[]>([])
    const [timeRange, setTimeRange] = useState<TimeRange>("1m")
    const [loading, setLoading] = useState(true)
    const [, setNavLoading] = useState(false)
    const [error, setError] = useState("")
    const [deepReport, setDeepReport] = useState<DeepReport | null>(null)
    const [deepLoading, setDeepLoading] = useState(false)
    const [deepError, setDeepError] = useState("")
    const [deepProgress, setDeepProgress] = useState(0)
    const [deepProgressMsg, setDeepProgressMsg] = useState("")
    const [debatePhases, setDebatePhases] = useState<DebatePhaseUpdate[]>([])
    const [debateActive, setDebateActive] = useState<DebatePhaseKey[] | null>(null)
    const [debateSystemConfidence, setDebateSystemConfidence] = useState<number | undefined>()
    const [debateDecisionGate, setDebateDecisionGate] = useState<string | undefined>()
    const deepStreamCleanupRef = useRef<(() => void) | null>(null)
    const initReqSeqRef = useRef(0)
    const navReqSeqRef = useRef(0)

    // 初始加载：基金信息 + 持仓 + 默认净值
    useEffect(() => {
        if (!code) return
        const reqSeq = ++initReqSeqRef.current
        navReqSeqRef.current++ // 切换基金时立即失效旧净值请求
        setLoading(true)
        setError("")
        setNavList([])

        Promise.all([
            getFund(code).catch(() => null),
            getFundHoldings(code).catch(() => ({ data: [] })),
        ]).then(([fundData, holdingsData]) => {
            if (reqSeq !== initReqSeqRef.current) return
            if (fundData) setFund(fundData)
            else setError("基金不存在")
            setHoldings((holdingsData as { data: Holding[] }).data || [])
            setLoading(false)
        })
    }, [code])

    // 切换时间范围：只刷新净值
    useEffect(() => {
        if (!code || loading) return
        const reqSeq = ++navReqSeqRef.current
        setNavLoading(true)
        getFundNAV(code, { period: timeRange })
            .then((res) => {
                if (reqSeq !== navReqSeqRef.current) return
                setNavList(res.data || [])
            })
            .catch(() => {
                if (reqSeq !== navReqSeqRef.current) return
                setNavList([])
            })
            .finally(() => {
                if (reqSeq !== navReqSeqRef.current) return
                setNavLoading(false)
            })
        // loading 不加依赖，只在 timeRange 变化时触发
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [code, timeRange, loading])

    useEffect(() => {
        return () => {
            if (deepStreamCleanupRef.current) {
                deepStreamCleanupRef.current()
                deepStreamCleanupRef.current = null
            }
        }
    }, [])

    const chartData = navList.map((n) => ({
        label: n.date.slice(5), // MM-DD
        value: n.unit_nav,
    })).reverse()

    const latestNav = navList[0]

    const goChat = (message: string) => {
        navigate(`/chat?q=${encodeURIComponent(message)}`)
    }

    const handleDeepAnalysis = async () => {
        if (!code || deepLoading) return
        setDeepLoading(true)
        setDeepError("")
        setDeepReport(null)
        setDeepProgress(0)
        setDeepProgressMsg("提交分析任务...")
        setDebatePhases([])
        setDebateActive(null)
        if (deepStreamCleanupRef.current) {
            deepStreamCleanupRef.current()
            deepStreamCleanupRef.current = null
        }

        try {
            const { task_id } = await submitDeepAnalysis(code)

            const stopStream = streamTaskProgress(task_id, {
                onProgress: (progress, msg, metadata) => {
                    setDeepProgress(progress)
                    setDeepProgressMsg(msg)
                    if (metadata) {
                        try {
                            const update = JSON.parse(metadata) as DebatePhaseUpdate
                            if (update.type === "debate_phase") {
                                if (update.phase === "confidence_gate") {
                                    setDebateSystemConfidence(update.system_confidence)
                                    setDebateDecisionGate(update.decision_gate)
                                    return
                                }
                                setDebatePhases((prev) => {
                                    const next = [...prev, update]
                                    const completed = new Set(next.map((p) => p.phase))
                                    const nextActive = computeDebateActive(completed)
                                    setDebateActive(nextActive.length > 0 ? nextActive : null)
                                    return next
                                })
                            }
                        } catch { /* ignore parse errors */ }
                    }
                },
                onComplete: (result) => {
                    try {
                        const report = JSON.parse(result) as DeepReport
                        setDeepReport(report)
                    } catch {
                        setDeepError("解析分析结果失败")
                    }
                    setDeepLoading(false)
                    if (deepStreamCleanupRef.current) {
                        deepStreamCleanupRef.current()
                        deepStreamCleanupRef.current = null
                    }
                },
                onError: (err) => {
                    setDeepError(err)
                    setDeepLoading(false)
                    if (deepStreamCleanupRef.current) {
                        deepStreamCleanupRef.current()
                        deepStreamCleanupRef.current = null
                    }
                },
            })
            deepStreamCleanupRef.current = stopStream
        } catch (e: unknown) {
            setDeepError((e as Error).message || "提交分析失败")
            setDeepLoading(false)
        }
    }

    if (loading) {
        return (
            <div className="flex-1 flex items-center justify-center text-[var(--color-text-muted)]">
                加载中...
            </div>
        )
    }

    if (error || !fund) {
        return (
            <div className="flex-1 flex flex-col items-center justify-center gap-4">
                <div className="text-[var(--color-text-muted)]">{error || "未找到基金"}</div>
                <Button variant="ghost" onClick={() => navigate(-1)}>
                    <ArrowLeft className="w-4 h-4 mr-2" /> 返回
                </Button>
            </div>
        )
    }

    return (
        <div className="flex-1 overflow-y-auto">
                <div className="max-w-4xl mx-auto px-6 py-8">
                    {/* Header */}
                    <div className="flex items-center gap-4 mb-8">
                        <button onClick={() => navigate(-1)} className="text-[var(--color-text-secondary)] hover:text-[var(--color-text)] transition-colors">
                            <ArrowLeft className="w-5 h-5" />
                        </button>
                        <div>
                            <h1 className="text-xl font-semibold text-[var(--color-text)]">{fund.name}</h1>
                            <span className="text-sm text-[var(--color-text-muted)]">{fund.code}</span>
                        </div>
                    </div>

                    {/* Overview */}
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
                        {[
                            { label: "类型", value: fund.type },
                            { label: "规模", value: fund.scale ? `${fund.scale.toFixed(1)}亿` : "-" },
                            { label: "公司", value: fund.company || "-" },
                            { label: "经理", value: fund.manager?.name || "-" },
                        ].map((item) => (
                            <div key={item.label} className="bg-white rounded-lg border border-[var(--color-border)] p-4">
                                <div className="text-xs text-[var(--color-text-muted)] mb-1">{item.label}</div>
                                <div className="text-sm font-medium text-[var(--color-text)]">{item.value}</div>
                            </div>
                        ))}
                    </div>

                    {/* NAV Chart */}
                    <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-8">
                        <div className="flex items-center justify-between mb-4">
                            <h2 className="text-base font-semibold text-[var(--color-text)]">净值走势</h2>
                            <div className="flex gap-1">
                                {TIME_RANGES.map((t) => (
                                    <button
                                        key={t.key}
                                        onClick={() => setTimeRange(t.key)}
                                        className={`px-3 py-1 text-xs rounded-md transition-colors ${timeRange === t.key
                                            ? "bg-[var(--color-primary-bg)] text-[var(--color-primary)] font-medium"
                                            : "text-[var(--color-text-muted)] hover:bg-[var(--color-sidebar-bg)]"
                                            }`}
                                    >
                                        {t.label}
                                    </button>
                                ))}
                            </div>
                        </div>

                        {chartData.length > 1 ? (
                            <SketchChart data={chartData} height={240} />
                        ) : (
                            <div className="h-[240px] flex items-center justify-center text-[var(--color-text-muted)] text-sm">
                                暂无净值数据
                            </div>
                        )}

                        {latestNav && (
                            <div className="flex gap-6 mt-4 pt-4 border-t border-[var(--color-border)]">
                                <div>
                                    <span className="text-xs text-[var(--color-text-muted)]">最新净值</span>
                                    <div className="text-lg font-semibold text-[var(--color-text)]">
                                        {latestNav.unit_nav.toFixed(4)}
                                    </div>
                                </div>
                                <div>
                                    <span className="text-xs text-[var(--color-text-muted)]">日涨幅</span>
                                    <div className={`text-lg font-semibold ${latestNav.daily_return >= 0
                                        ? "text-[var(--color-up)]"
                                        : "text-[var(--color-down)]"
                                        }`}>
                                        {latestNav.daily_return >= 0 ? "+" : ""}
                                        {latestNav.daily_return.toFixed(2)}%
                                    </div>
                                </div>
                                <div>
                                    <span className="text-xs text-[var(--color-text-muted)]">日期</span>
                                    <div className="text-sm text-[var(--color-text)]">{latestNav.date}</div>
                                </div>
                            </div>
                        )}
                    </div>

                    {/* Holdings */}
                    {holdings.length > 0 && (
                        <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-8">
                            <h2 className="text-base font-semibold text-[var(--color-text)] mb-4">
                                前十大持仓
                            </h2>
                            <div className="space-y-2">
                                {holdings.map((h, i) => (
                                    <div key={h.stock_code} className="flex items-center justify-between py-2 border-b border-[var(--color-border)] last:border-0">
                                        <div className="flex items-center gap-3">
                                            <span className="text-xs text-[var(--color-text-muted)] w-5">{i + 1}</span>
                                            <div>
                                                <div className="text-sm font-medium text-[var(--color-text)]">{h.stock_name}</div>
                                                <div className="text-xs text-[var(--color-text-muted)]">{h.stock_code}</div>
                                            </div>
                                        </div>
                                        <div className="text-sm font-medium text-[var(--color-text)]">
                                            {h.ratio.toFixed(2)}%
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {/* AI Actions */}
                    <div className="flex flex-wrap gap-3">
                        <Button variant="default" onClick={() => goChat(`帮我分析 ${code}`)}>
                            <MessageSquare className="w-4 h-4 mr-2" /> 让 Quinfi 分析
                        </Button>
                        <Button
                            variant="outline"
                            className="border-[var(--color-primary)] text-[var(--color-primary)] hover:bg-[var(--color-primary-bg)]"
                            onClick={handleDeepAnalysis}
                            disabled={deepLoading}
                        >
                            {deepLoading
                                ? <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                : <Microscope className="w-4 h-4 mr-2" />
                            }
                            {deepLoading ? "分析中..." : "深度分析"}
                        </Button>
                        <Button variant="outline" className="border-[var(--color-border)] text-[var(--color-text)]" onClick={() => goChat(`对 ${code} 做多空辩论`)}>
                            <Swords className="w-4 h-4 mr-2" /> 多空辩论
                        </Button>
                        <Button variant="outline" className="border-[var(--color-border)] text-[var(--color-text)]" onClick={() => goChat(`帮我找和 ${code} 同类型的基金`)}>
                            <Search className="w-4 h-4 mr-2" /> 同类基金
                        </Button>
                    </div>

                    {/* Deep Analysis Loading + Progress */}
                    {deepLoading && (
                        <div className="mt-6 p-4 rounded-lg bg-[var(--color-sidebar-bg)] border border-[var(--color-border)]">
                            <div className="flex items-center gap-3 mb-3">
                                <Loader2 className="w-5 h-5 animate-spin text-[var(--color-primary)]" />
                                <div>
                                    <div className="text-sm font-medium text-[var(--color-text)]">
                                        {deepProgressMsg || "正在进行深度分析..."}
                                    </div>
                                    <div className="text-xs text-[var(--color-text-muted)]">
                                        包含基金分析、经理评估、调仓检测、宏观研判、多空辩论
                                    </div>
                                </div>
                            </div>
                            <div className="w-full bg-[var(--color-border)] rounded-full h-2">
                                <div
                                    className="bg-[var(--color-primary)] h-2 rounded-full transition-all duration-500"
                                    style={{ width: `${deepProgress}%` }}
                                />
                            </div>
                            <div className="text-xs text-[var(--color-text-muted)] mt-1 text-right">
                                {deepProgress}%
                            </div>
                            {/* 辩论实时可视化 */}
                            {debatePhases.length > 0 && (
                                <div className="mt-4">
                                    <DebateTimeline phases={debatePhases} activePhase={debateActive} systemConfidence={debateSystemConfidence} decisionGate={debateDecisionGate} />
                                </div>
                            )}
                        </div>
                    )}

                    {/* Deep Analysis Error */}
                    {deepError && (
                        <div className="mt-6 p-4 rounded-lg bg-[var(--color-down)]/[0.06] border border-[var(--color-down)]/20 text-sm text-[var(--color-down)]">
                            深度分析失败: {deepError}
                        </div>
                    )}

                    {/* Deep Analysis Report */}
                    {deepReport && <DeepAnalysisPanel report={deepReport} />}
                </div>
        </div>
    )
}
