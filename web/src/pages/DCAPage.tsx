import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Loader2, PiggyBank } from "lucide-react"
import { runDCA } from "@/lib/api"
import type { DCAResult } from "@/types"
import {
    ResponsiveContainer, LineChart, Line,
    XAxis, YAxis, Tooltip, CartesianGrid, Legend,
} from "recharts"

export default function DCAPage() {
    const [code, setCode] = useState("")
    const [amount, setAmount] = useState("1000")
    const [strategy, setStrategy] = useState("fixed")
    const [days, setDays] = useState("1095")
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState("")
    const [result, setResult] = useState<DCAResult | null>(null)

    const isValid = /^\d{6}$/.test(code.trim()) && parseFloat(amount) > 0

    const handleRun = async () => {
        if (!isValid) return
        setLoading(true)
        setError("")
        try {
            const res = await runDCA(code.trim(), parseFloat(amount), {
                strategy,
                days: parseInt(days) || 1095,
            })
            setResult(res)
        } catch (err) {
            setError(err instanceof Error ? err.message : "模拟失败")
        } finally {
            setLoading(false)
        }
    }

    // 构造投入 vs 市值对比数据
    const valueCurveData = result?.transactions?.map(t => ({
        date: t.date,
        invested: t.total_cost,
        value: t.market_value,
    })) ?? []

    return (
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <div className="max-w-5xl mx-auto">
                <h1 className="text-2xl font-serif font-semibold text-[var(--color-primary)] mb-1 flex items-center gap-2">
                    <PiggyBank className="w-6 h-6" />
                    定投模拟
                </h1>
                <p className="text-sm text-[var(--color-text-muted)] mb-6">
                    模拟定投策略的历史收益，对比一次性投入
                </p>

                {/* 输入区 */}
                <Card className="mb-6">
                    <CardHeader>
                        <CardTitle className="text-lg">定投参数</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                            <div>
                                <label className="text-xs text-slate-500 mb-1 block">基金代码</label>
                                <Input
                                    variant="boxed"
                                    placeholder="如 005827"
                                    value={code}
                                    onChange={e => setCode(e.target.value)}
                                    className="font-sans text-sm"
                                    maxLength={6}
                                />
                            </div>
                            <div>
                                <label className="text-xs text-slate-500 mb-1 block">每期金额（元）</label>
                                <Input
                                    variant="boxed"
                                    placeholder="1000"
                                    value={amount}
                                    onChange={e => setAmount(e.target.value)}
                                    className="font-sans text-sm"
                                    type="number"
                                    min="100"
                                />
                            </div>
                            <div>
                                <label className="text-xs text-slate-500 mb-1 block">策略</label>
                                <select
                                    value={strategy}
                                    onChange={e => setStrategy(e.target.value)}
                                    className="w-full h-10 px-3 text-sm border rounded-md bg-white text-slate-700"
                                >
                                    <option value="fixed">固定金额</option>
                                    <option value="value">目标价值</option>
                                    <option value="smart">智能定投（MA偏离）</option>
                                </select>
                            </div>
                            <div>
                                <label className="text-xs text-slate-500 mb-1 block">模拟周期</label>
                                <select
                                    value={days}
                                    onChange={e => setDays(e.target.value)}
                                    className="w-full h-10 px-3 text-sm border rounded-md bg-white text-slate-700"
                                >
                                    <option value="365">1 年</option>
                                    <option value="730">2 年</option>
                                    <option value="1095">3 年</option>
                                    <option value="1825">5 年</option>
                                </select>
                            </div>
                        </div>

                        <div className="flex items-center justify-between">
                            <span />
                            <Button onClick={handleRun} disabled={!isValid || loading}>
                                {loading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : null}
                                开始模拟
                            </Button>
                        </div>

                        {error ? <p className="text-sm text-red-500 mt-2">{error}</p> : null}
                    </CardContent>
                </Card>

                {/* 结果区 */}
                {result ? (
                    <div className="space-y-6">
                        {/* 核心指标 */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <MetricCard label="累计投入" value={`¥${result.total_invested.toLocaleString()}`} />
                            <MetricCard label="当前市值" value={`¥${result.final_value.toLocaleString()}`} positive={result.final_value >= result.total_invested} />
                            <MetricCard label="定投收益率" value={`${(result.total_return * 100).toFixed(2)}%`} positive={result.total_return >= 0} />
                            <MetricCard label="平均成本" value={result.avg_cost.toFixed(4)} />
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                            <MetricCard label="定投次数" value={`${result.transactions?.length ?? 0} 期`} />
                            <MetricCard label="一次性投入收益" value={`${(result.lump_sum_return * 100).toFixed(2)}%`} positive={result.lump_sum_return >= 0} />
                            <MetricCard label="超额收益" value={`${(result.excess_return * 100).toFixed(2)}%`} positive={result.excess_return >= 0} />
                        </div>

                        {/* 投入 vs 市值曲线 */}
                        {valueCurveData.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">投入 vs 市值</CardTitle></CardHeader>
                                <CardContent>
                                    <ResponsiveContainer width="100%" height={320}>
                                        <LineChart data={valueCurveData}>
                                            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                                            <XAxis dataKey="date" tick={{ fontSize: 11 }} tickFormatter={d => d.slice(5)} />
                                            <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `${(v / 1000).toFixed(0)}k`} />
                                            <Tooltip formatter={(v) => `¥${Number(v).toLocaleString()}`} />
                                            <Legend />
                                            <Line type="monotone" dataKey="invested" stroke="#64748b" strokeWidth={1.5} dot={false} name="累计投入" />
                                            <Line type="monotone" dataKey="value" stroke="#166534" strokeWidth={2} dot={false} name="市值" />
                                        </LineChart>
                                    </ResponsiveContainer>
                                </CardContent>
                            </Card>
                        ) : null}

                        {/* 交易明细 */}
                        {result.transactions?.length > 0 ? (
                            <Card>
                                <CardHeader><CardTitle className="text-lg">交易明细</CardTitle></CardHeader>
                                <CardContent>
                                    <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
                                        <table className="w-full text-sm">
                                            <thead className="sticky top-0 bg-white">
                                                <tr className="border-b text-left text-slate-500">
                                                    <th className="pb-2 pr-3">日期</th>
                                                    <th className="pb-2 pr-3">净值</th>
                                                    <th className="pb-2 pr-3">投入</th>
                                                    <th className="pb-2 pr-3">买入份额</th>
                                                    <th className="pb-2 pr-3">累计份额</th>
                                                    <th className="pb-2">市值</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {result.transactions.map(t => (
                                                    <tr key={t.date} className="border-b border-slate-100">
                                                        <td className="py-1.5 pr-3">{t.date}</td>
                                                        <td className="py-1.5 pr-3">{t.nav.toFixed(4)}</td>
                                                        <td className="py-1.5 pr-3">¥{t.amount.toFixed(0)}</td>
                                                        <td className="py-1.5 pr-3">{t.shares.toFixed(2)}</td>
                                                        <td className="py-1.5 pr-3">{t.total_shares.toFixed(2)}</td>
                                                        <td className={`py-1.5 ${t.market_value >= t.total_cost ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                            ¥{t.market_value.toFixed(0)}
                                                        </td>
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
