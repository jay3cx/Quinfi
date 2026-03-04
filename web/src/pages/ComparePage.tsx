import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Plus, Trash2, Loader2, GitCompareArrows } from "lucide-react"
import { runCompare } from "@/lib/api"
import type { CompareResult, CurvePoint } from "@/types"
import {
    ResponsiveContainer, LineChart, Line,
    XAxis, YAxis, Tooltip, CartesianGrid, Legend,
} from "recharts"

const COLORS = ["#166534", "#1e6091", "#9a5c16", "#7e3794", "#b34525"]

export default function ComparePage() {
    const [codes, setCodes] = useState(["", ""])
    const [days, setDays] = useState("365")
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState("")
    const [result, setResult] = useState<CompareResult | null>(null)

    const validCodes = codes.filter(c => /^\d{6}$/.test(c.trim()))
    const isValid = validCodes.length >= 2

    const addCode = () => {
        if (codes.length >= 5) return
        setCodes(prev => [...prev, ""])
    }

    const removeCode = (idx: number) => {
        if (codes.length <= 2) return
        setCodes(prev => prev.filter((_, i) => i !== idx))
    }

    const updateCode = (idx: number, value: string) => {
        setCodes(prev => prev.map((c, i) => i === idx ? value : c))
    }

    const handleRun = async () => {
        if (!isValid) return
        setLoading(true)
        setError("")
        try {
            const res = await runCompare(
                validCodes.map(c => c.trim()),
                { days: parseInt(days) || 365 },
            )
            setResult(res)
        } catch (err) {
            setError(err instanceof Error ? err.message : "对比失败")
        } finally {
            setLoading(false)
        }
    }

    // 合并归一化净值曲线为一个数据数组，供 Recharts 使用
    const mergedCurve = result ? mergeNormalizedCurves(result.funds.map(f => ({
        code: f.code,
        points: f.normalized_nav,
    }))) : []

    return (
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <div className="max-w-5xl mx-auto">
                <h1 className="text-2xl font-serif font-semibold text-[var(--color-primary)] mb-1 flex items-center gap-2">
                    <GitCompareArrows className="w-6 h-6" />
                    基金PK
                </h1>
                <p className="text-sm text-[var(--color-text-muted)] mb-6">
                    横向对比 2-5 只基金的业绩、风险和相关性
                </p>

                {/* 输入区 */}
                <Card className="mb-6">
                    <CardHeader>
                        <CardTitle className="text-lg">选择基金</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        {codes.map((code, idx) => (
                            <div key={idx} className="flex items-center gap-3">
                                <div className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: COLORS[idx % COLORS.length] }} />
                                <Input
                                    variant="boxed"
                                    placeholder={`基金代码 ${idx + 1}`}
                                    value={code}
                                    onChange={e => updateCode(idx, e.target.value)}
                                    className="flex-1 font-sans text-sm"
                                    maxLength={6}
                                />
                                <button
                                    onClick={() => removeCode(idx)}
                                    className="p-1.5 text-[var(--color-text-muted)] hover:text-[var(--color-down)] transition-colors disabled:opacity-30"
                                    disabled={codes.length <= 2}
                                >
                                    <Trash2 className="w-4 h-4" />
                                </button>
                            </div>
                        ))}

                        <div className="flex items-center justify-between pt-2">
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={addCode}
                                disabled={codes.length >= 5}
                            >
                                <Plus className="w-4 h-4 mr-1" /> 添加基金
                            </Button>
                            <div className="flex items-center gap-3">
                                <select
                                    value={days}
                                    onChange={e => setDays(e.target.value)}
                                    className="h-8 px-2 text-sm border rounded-md bg-white text-[var(--color-text)]"
                                >
                                    <option value="180">近半年</option>
                                    <option value="365">近一年</option>
                                    <option value="730">近两年</option>
                                    <option value="1095">近三年</option>
                                </select>
                                <Button onClick={handleRun} disabled={!isValid || loading}>
                                    {loading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : null}
                                    开始对比
                                </Button>
                            </div>
                        </div>

                        {error ? <p className="text-sm text-[var(--color-down)] mt-2">{error}</p> : null}
                    </CardContent>
                </Card>

                {/* 结果区 */}
                {result ? (
                    <div className="space-y-6">
                        {/* 归一化净值叠加图 */}
                        {mergedCurve.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">归一化净值走势</CardTitle></CardHeader>
                                <CardContent>
                                    <ResponsiveContainer width="100%" height={360}>
                                        <LineChart data={mergedCurve}>
                                            <CartesianGrid strokeDasharray="3 3" stroke="#E8E5DE" />
                                            <XAxis dataKey="date" tick={{ fontSize: 11 }} tickFormatter={d => d.slice(5)} />
                                            <YAxis tick={{ fontSize: 11 }} domain={["auto", "auto"]} />
                                            <Tooltip />
                                            <Legend />
                                            {result.funds.map((f, idx) => (
                                                <Line
                                                    key={f.code}
                                                    type="monotone"
                                                    dataKey={f.code}
                                                    stroke={COLORS[idx % COLORS.length]}
                                                    strokeWidth={2}
                                                    dot={false}
                                                    name={`${f.name}(${f.code})`}
                                                />
                                            ))}
                                        </LineChart>
                                    </ResponsiveContainer>
                                </CardContent>
                            </Card>
                        ) : null}

                        {/* 指标对比表 */}
                        <Card>
                            <CardHeader><CardTitle className="text-lg">业绩与风险指标</CardTitle></CardHeader>
                            <CardContent>
                                <div className="overflow-x-auto">
                                    <table className="w-full text-sm">
                                        <thead>
                                            <tr className="border-b text-left text-[var(--color-text-secondary)]">
                                                <th className="pb-2 pr-4">基金</th>
                                                <th className="pb-2 pr-4">累计收益</th>
                                                <th className="pb-2 pr-4">年化收益</th>
                                                <th className="pb-2 pr-4">最大回撤</th>
                                                <th className="pb-2 pr-4">夏普</th>
                                                <th className="pb-2 pr-4">波动率</th>
                                                <th className="pb-2">Sortino</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {result.funds.map((f, idx) => (
                                                <tr key={f.code} className="border-b border-[var(--color-border)]">
                                                    <td className="py-2 pr-4 font-medium">
                                                        <span className="inline-block w-2.5 h-2.5 rounded-full mr-2" style={{ backgroundColor: COLORS[idx % COLORS.length] }} />
                                                        {f.name}({f.code})
                                                    </td>
                                                    <td className={`py-2 pr-4 ${f.total_return >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                        {(f.total_return * 100).toFixed(2)}%
                                                    </td>
                                                    <td className={`py-2 pr-4 ${f.annual_return >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                        {(f.annual_return * 100).toFixed(2)}%
                                                    </td>
                                                    <td className="py-2 pr-4 text-[var(--color-down)]">{(f.max_drawdown * 100).toFixed(2)}%</td>
                                                    <td className="py-2 pr-4">{f.sharpe_ratio.toFixed(3)}</td>
                                                    <td className="py-2 pr-4">{(f.volatility * 100).toFixed(2)}%</td>
                                                    <td className="py-2">{f.sortino_ratio.toFixed(3)}</td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </CardContent>
                        </Card>

                        {/* 相关性矩阵 */}
                        {result.correlation_matrix?.codes?.length > 1 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">相关性矩阵</CardTitle></CardHeader>
                                <CardContent>
                                    <div className="overflow-x-auto">
                                        <table className="text-sm">
                                            <thead>
                                                <tr>
                                                    <th className="pb-2 pr-4" />
                                                    {result.correlation_matrix.codes.map(c => (
                                                        <th key={c} className="pb-2 px-3 text-center text-[var(--color-text-secondary)] font-normal">{c}</th>
                                                    ))}
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {result.correlation_matrix.codes.map((rowCode, i) => (
                                                    <tr key={rowCode}>
                                                        <td className="py-1.5 pr-4 text-[var(--color-text-secondary)]">{rowCode}</td>
                                                        {result.correlation_matrix.values[i].map((val, j) => (
                                                            <td key={j} className="py-1.5 px-3 text-center">
                                                                <span
                                                                    className="inline-block px-2 py-0.5 rounded text-xs font-medium"
                                                                    style={{
                                                                        backgroundColor: corrColor(val),
                                                                        color: val > 0.5 ? "#fff" : "#2D2B28",
                                                                    }}
                                                                >
                                                                    {val.toFixed(3)}
                                                                </span>
                                                            </td>
                                                        ))}
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                    <div className="mt-3 text-xs text-[var(--color-text-muted)]">
                                        相关性越低，组合分散效果越好。低于 0.3 为低相关，0.3-0.7 为中等相关，高于 0.7 为高相关。
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

// 将多只基金的归一化净值合并为 [{date, code1: val, code2: val, ...}]
function mergeNormalizedCurves(funds: { code: string; points: CurvePoint[] }[]): Record<string, unknown>[] {
    const dateMap = new Map<string, Record<string, unknown>>()
    for (const fund of funds) {
        if (!fund.points) continue
        for (const p of fund.points) {
            const row = dateMap.get(p.date) ?? { date: p.date }
            row[fund.code] = p.value
            dateMap.set(p.date, row)
        }
    }
    return [...dateMap.values()].sort((a: Record<string, unknown>, b: Record<string, unknown>) =>
        (a.date as string).localeCompare(b.date as string)
    )
}

// 相关性热力色：低→绿色，高→红色
function corrColor(val: number): string {
    const abs = Math.abs(val)
    if (abs >= 0.7) return "#dc2626"
    if (abs >= 0.3) return "#f59e0b"
    return "#22c55e"
}
