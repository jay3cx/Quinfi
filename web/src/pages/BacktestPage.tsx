import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Plus, Trash2, Loader2, FlaskConical } from "lucide-react"
import { runBacktest } from "@/lib/api"
import type { HoldingWeight, BacktestResult } from "@/types"
import {
    ResponsiveContainer, LineChart, Line, AreaChart, Area,
    XAxis, YAxis, Tooltip, CartesianGrid,
} from "recharts"

const COLORS = ["#166534", "#1e6091", "#9a5c16", "#7e3794", "#b34525"]

interface HoldingInput {
    code: string
    weight: string
}

export default function BacktestPage() {
    const [holdings, setHoldings] = useState<HoldingInput[]>([
        { code: "", weight: "" },
        { code: "", weight: "" },
    ])
    const [days, setDays] = useState("365")
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState("")
    const [result, setResult] = useState<BacktestResult | null>(null)

    // rerender-derived-state-no-effect: 渲染时派生
    const totalWeight = holdings.reduce((sum, h) => sum + (parseFloat(h.weight) || 0), 0)
    const isValid = holdings.filter(h => h.code.trim() && parseFloat(h.weight) > 0).length >= 1
        && Math.abs(totalWeight - 1) < 0.01

    const addHolding = () => {
        if (holdings.length >= 5) return
        setHoldings(prev => [...prev, { code: "", weight: "" }])
    }

    const removeHolding = (idx: number) => {
        if (holdings.length <= 2) return
        setHoldings(prev => prev.filter((_, i) => i !== idx))
    }

    const updateHolding = (idx: number, field: keyof HoldingInput, value: string) => {
        setHoldings(prev => prev.map((h, i) => i === idx ? { ...h, [field]: value } : h))
    }

    const handleRun = async () => {
        if (!isValid) return
        setLoading(true)
        setError("")

        const weights: HoldingWeight[] = holdings
            .filter(h => h.code.trim() && parseFloat(h.weight) > 0)
            .map(h => ({ fund_code: h.code.trim(), weight: parseFloat(h.weight) }))

        try {
            const res = await runBacktest(weights, parseInt(days) || 365)
            setResult(res)
        } catch (err) {
            setError(err instanceof Error ? err.message : "回测失败")
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <div className="max-w-5xl mx-auto">
                <h1 className="text-2xl font-serif font-semibold text-[var(--color-primary)] mb-1 flex items-center gap-2">
                    <FlaskConical className="w-6 h-6" />
                    回测实验室
                </h1>
                <p className="text-sm text-[var(--color-text-muted)] mb-6">
                    输入基金组合和权重，回测历史表现
                </p>

                {/* 输入区 */}
                <Card className="mb-6">
                    <CardHeader>
                        <CardTitle className="text-lg">组合配置</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        {holdings.map((h, idx) => (
                            <div key={idx} className="flex items-center gap-3">
                                <Input
                                    variant="boxed"
                                    placeholder="基金代码 (如 005827)"
                                    value={h.code}
                                    onChange={e => updateHolding(idx, "code", e.target.value)}
                                    className="flex-1 font-sans text-sm"
                                    maxLength={6}
                                />
                                <Input
                                    variant="boxed"
                                    placeholder="权重 (如 0.6)"
                                    value={h.weight}
                                    onChange={e => updateHolding(idx, "weight", e.target.value)}
                                    className="w-28 font-sans text-sm"
                                    type="number"
                                    step="0.1"
                                    min="0"
                                    max="1"
                                />
                                <button
                                    onClick={() => removeHolding(idx)}
                                    className="p-1.5 text-slate-400 hover:text-red-500 transition-colors disabled:opacity-30"
                                    disabled={holdings.length <= 2}
                                >
                                    <Trash2 className="w-4 h-4" />
                                </button>
                            </div>
                        ))}

                        <div className="flex items-center justify-between pt-2">
                            <div className="flex items-center gap-3">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={addHolding}
                                    disabled={holdings.length >= 5}
                                >
                                    <Plus className="w-4 h-4 mr-1" /> 添加基金
                                </Button>
                                <span className={`text-xs ${Math.abs(totalWeight - 1) < 0.01 ? "text-green-600" : "text-amber-600"}`}>
                                    权重合计: {totalWeight.toFixed(2)}
                                </span>
                            </div>

                            <div className="flex items-center gap-3">
                                <select
                                    value={days}
                                    onChange={e => setDays(e.target.value)}
                                    className="h-8 px-2 text-sm border rounded-md bg-white text-slate-700"
                                >
                                    <option value="180">近半年</option>
                                    <option value="365">近一年</option>
                                    <option value="730">近两年</option>
                                    <option value="1095">近三年</option>
                                </select>
                                <Button onClick={handleRun} disabled={!isValid || loading}>
                                    {loading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : null}
                                    开始回测
                                </Button>
                            </div>
                        </div>

                        {error ? <p className="text-sm text-red-500 mt-2">{error}</p> : null}
                    </CardContent>
                </Card>

                {/* 结果区 */}
                {result ? (
                    <div className="space-y-6">
                        {/* 核心指标 */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <MetricCard label="年化收益" value={`${(result.annual_return * 100).toFixed(2)}%`} positive={result.annual_return >= 0} />
                            <MetricCard label="最大回撤" value={`${(result.max_drawdown * 100).toFixed(2)}%`} positive={false} />
                            <MetricCard label="夏普比率" value={result.sharpe_ratio.toFixed(3)} positive={result.sharpe_ratio > 0} />
                            <MetricCard label="年化波动率" value={`${(result.volatility * 100).toFixed(2)}%`} />
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <MetricCard label="累计收益" value={`${(result.total_return * 100).toFixed(2)}%`} positive={result.total_return >= 0} />
                            <MetricCard label="Sortino" value={result.sortino_ratio.toFixed(3)} positive={result.sortino_ratio > 0} />
                            <MetricCard label="Calmar" value={result.calmar_ratio.toFixed(3)} positive={result.calmar_ratio > 0} />
                            {result.benchmark_return != null ? (
                                <MetricCard label="超额收益" value={`${((result.total_return - result.benchmark_return) * 100).toFixed(2)}%`} positive={result.total_return > result.benchmark_return} />
                            ) : (
                                <div />
                            )}
                        </div>

                        {/* 净值曲线 */}
                        {result.equity_curve?.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">净值曲线</CardTitle></CardHeader>
                                <CardContent>
                                    <ResponsiveContainer width="100%" height={320}>
                                        <LineChart data={result.equity_curve}>
                                            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                                            <XAxis dataKey="date" tick={{ fontSize: 11 }} tickFormatter={d => d.slice(5)} />
                                            <YAxis tick={{ fontSize: 11 }} />
                                            <Tooltip />
                                            <Line type="monotone" dataKey="value" stroke={COLORS[0]} strokeWidth={2} dot={false} name="组合净值" />
                                        </LineChart>
                                    </ResponsiveContainer>
                                </CardContent>
                            </Card>
                        ) : null}

                        {/* 回撤曲线 */}
                        {result.drawdown_curve?.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">回撤曲线</CardTitle></CardHeader>
                                <CardContent>
                                    <ResponsiveContainer width="100%" height={200}>
                                        <AreaChart data={result.drawdown_curve}>
                                            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                                            <XAxis dataKey="date" tick={{ fontSize: 11 }} tickFormatter={d => d.slice(5)} />
                                            <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `${(v * 100).toFixed(0)}%`} />
                                            <Tooltip formatter={(v) => `${(Number(v) * 100).toFixed(2)}%`} />
                                            <Area type="monotone" dataKey="value" stroke="#b34525" fill="#b3452520" strokeWidth={1.5} name="回撤" />
                                        </AreaChart>
                                    </ResponsiveContainer>
                                </CardContent>
                            </Card>
                        ) : null}

                        {/* 单基金指标 */}
                        {result.fund_metrics?.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">各基金表现</CardTitle></CardHeader>
                                <CardContent>
                                    <div className="overflow-x-auto">
                                        <table className="w-full text-sm">
                                            <thead>
                                                <tr className="border-b text-left text-slate-500">
                                                    <th className="pb-2 pr-4">基金代码</th>
                                                    <th className="pb-2 pr-4">权重</th>
                                                    <th className="pb-2">累计收益</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {result.fund_metrics.map(f => (
                                                    <tr key={f.fund_code} className="border-b border-slate-100">
                                                        <td className="py-2 pr-4 font-medium font-mono">{f.fund_code}</td>
                                                        <td className="py-2 pr-4">{(f.weight * 100).toFixed(0)}%</td>
                                                        <td className={`py-2 ${f.total_return >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>{(f.total_return * 100).toFixed(2)}%</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                </CardContent>
                            </Card>
                        ) : null}
                    </div>
                ) : null}
            </div>
        </div>
    )
}

function MetricCard({ label, value, positive }: { label: string; value: string; positive?: boolean }) {
    return (
        <Card>
            <CardContent className="p-4">
                <div className="text-xs text-slate-500 mb-1">{label}</div>
                <div className={`text-xl font-semibold ${
                    positive === true ? "text-[var(--color-up)]"
                        : positive === false ? "text-[var(--color-down)]"
                            : "text-slate-800"
                }`}>
                    {value}
                </div>
            </CardContent>
        </Card>
    )
}
