import { useState, useEffect } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { SketchChart } from "@/components/SketchChart"
import { Button } from "@/components/ui/button"
import { ArrowLeft, MessageSquare, Swords, Search } from "lucide-react"
import { getFund, getFundNAV, getFundHoldings } from "@/lib/api"
import type { Fund, NAV, Holding } from "@/types"

type TimeRange = "1w" | "1m" | "3m" | "1y"
const TIME_RANGES: { key: TimeRange; label: string }[] = [
    { key: "1w", label: "1周" },
    { key: "1m", label: "1月" },
    { key: "3m", label: "3月" },
    { key: "1y", label: "1年" },
]

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

    // 初始加载：基金信息 + 持仓 + 默认净值
    useEffect(() => {
        if (!code) return
        setLoading(true)
        setError("")

        Promise.all([
            getFund(code).catch(() => null),
            getFundNAV(code, { period: "1m" }).catch(() => ({ data: [] })),
            getFundHoldings(code).catch(() => ({ data: [] })),
        ]).then(([fundData, navData, holdingsData]) => {
            if (fundData) setFund(fundData)
            else setError("基金不存在")
            setNavList((navData as { data: NAV[] }).data || [])
            setHoldings((holdingsData as { data: Holding[] }).data || [])
            setLoading(false)
        })
    }, [code])

    // 切换时间范围：只刷新净值
    useEffect(() => {
        if (!code || loading) return
        setNavLoading(true)
        getFundNAV(code, { period: timeRange })
            .then((res) => setNavList(res.data || []))
            .catch(() => setNavList([]))
            .finally(() => setNavLoading(false))
        // loading 不加依赖，只在 timeRange 变化时触发
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [code, timeRange])

    const chartData = navList.map((n) => ({
        label: n.date.slice(5), // MM-DD
        value: n.unit_nav,
    })).reverse()

    const latestNav = navList[0]

    const goChat = (message: string) => {
        navigate(`/chat?q=${encodeURIComponent(message)}`)
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
                    <div className="flex gap-3">
                        <Button variant="default" onClick={() => goChat(`帮我分析 ${code}`)}>
                            <MessageSquare className="w-4 h-4 mr-2" /> 让小基分析
                        </Button>
                        <Button variant="outline" className="border-[var(--color-border)] text-[var(--color-text)]" onClick={() => goChat(`对 ${code} 做多空辩论`)}>
                            <Swords className="w-4 h-4 mr-2" /> 多空辩论
                        </Button>
                        <Button variant="outline" className="border-[var(--color-border)] text-[var(--color-text)]" onClick={() => goChat(`帮我找和 ${code} 同类型的基金`)}>
                            <Search className="w-4 h-4 mr-2" /> 同类基金
                        </Button>
                    </div>
                </div>
        </div>
    )
}
