import { useState, useEffect, useRef } from "react"
import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Plus, MessageSquare, Camera, X, Check, Loader2 } from "lucide-react"
import { getPortfolio, addPortfolioHolding, removePortfolioHolding, scanPortfolio } from "@/lib/api"
import type { ScanHolding } from "@/types"

interface HoldingItem {
    code: string
    name: string
    amount?: number
    weight?: number
    totalProfit?: number
    totalProfitRate?: number
}

const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB

// 仓位色板 — 柔和的自然色系，与 Organic 风格匹配
const PALETTE = [
    "#166534", // forest green
    "#1e6091", // ocean blue
    "#9a5c16", // warm amber
    "#7e3794", // soft purple
    "#b34525", // terracotta
    "#0e7490", // teal
]

export default function PortfolioPage() {
    const navigate = useNavigate()
    const [holdings, setHoldings] = useState<HoldingItem[]>([])
    const [showAddDialog, setShowAddDialog] = useState(false)
    const [newCode, setNewCode] = useState("")

    // 截图识别状态
    const [scanning, setScanning] = useState(false)
    const [scanResults, setScanResults] = useState<ScanHolding[] | null>(null)
    const [scanTotalValue, setScanTotalValue] = useState(0)
    const [scanError, setScanError] = useState("")
    const fileInputRef = useRef<HTMLInputElement>(null)

    const totalAmount = holdings.reduce((sum, h) => sum + (h.amount || 0), 0)
    const totalProfit = holdings.reduce((sum, h) => sum + (h.totalProfit || 0), 0)

    useEffect(() => {
        getPortfolio().then((res) => {
            if (res.data?.length) {
                setHoldings(res.data.map((h) => ({
                    code: h.code,
                    name: h.name || `基金 ${h.code}`,
                    amount: h.amount || 0,
                    weight: h.weight || 0,
                    totalProfit: h.total_profit,
                    totalProfitRate: h.total_profit_rate,
                })))
            }
        }).catch((err) => {
            console.error("[PortfolioPage] Failed to load portfolio:", err)
        })
    }, [])

    const addHolding = () => {
        if (!newCode.trim() || newCode.length !== 6) return
        if (holdings.some((h) => h.code === newCode)) return

        const code = newCode
        setHoldings((prev) => [...prev, { code, name: "加载中..." }])
        setNewCode("")
        setShowAddDialog(false)

        fetch(`/api/v1/fund/${code}`)
            .then((r) => r.json())
            .then((fund) => {
                const name = fund.name || `基金 ${code}`
                setHoldings((prev) =>
                    prev.map((h) => (h.code === code ? { ...h, name } : h))
                )
                addPortfolioHolding(code, name).catch((err) => {
                    console.error("[PortfolioPage] Failed to persist holding:", err)
                })
            })
            .catch(() => {
                const fallbackName = `基金 ${code}`
                setHoldings((prev) =>
                    prev.map((h) => (h.code === code ? { ...h, name: fallbackName } : h))
                )
                addPortfolioHolding(code).catch((err) => {
                    console.error("[PortfolioPage] Failed to persist holding:", err)
                })
            })
    }

    const removeHolding = (code: string) => {
        setHoldings((prev) => prev.filter((h) => h.code !== code))
        removePortfolioHolding(code).catch((err) => {
            console.error("[PortfolioPage] Failed to remove holding:", err)
        })
    }

    // === 截图识别 ===
    const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return
        // 重置 input 以允许再次选择相同文件
        e.target.value = ""

        if (file.size > MAX_IMAGE_SIZE) {
            setScanError("图片过大，请选择 5MB 以内的图片")
            return
        }

        const reader = new FileReader()
        reader.onload = () => {
            const base64 = reader.result as string
            doScan(base64)
        }
        reader.readAsDataURL(file)
    }

    const doScan = async (imageBase64: string) => {
        setScanning(true)
        setScanError("")
        setScanResults(null)

        try {
            const data = await scanPortfolio(imageBase64, false)
            if (data.error) {
                setScanError(data.error)
                return
            }
            if (!data.holdings?.length) {
                setScanError("未识别到持仓信息，请确认截图内容")
                return
            }
            setScanResults(data.holdings)
            setScanTotalValue(data.total_value)
        } catch (err) {
            setScanError(err instanceof Error ? err.message : "识别失败，请重试")
        } finally {
            setScanning(false)
        }
    }

    const confirmScanResults = async () => {
        if (!scanResults) return

        for (const h of scanResults) {
            if (!holdings.some((ex) => ex.code === h.code)) {
                setHoldings((prev) => [...prev, { code: h.code, name: h.name, amount: h.amount }])
                await addPortfolioHolding(h.code, h.name, h.amount).catch(() => {})
            }
        }
        setScanResults(null)
    }

    const dismissScan = () => {
        setScanResults(null)
        setScanError("")
    }

    return (
        <div className="flex-1 overflow-y-auto">
            <div className="max-w-4xl mx-auto px-6 py-8">
                <div className="flex items-center justify-between mb-8">
                    <h1 className="text-xl font-semibold text-[var(--color-text)]">Portfolio</h1>
                    <div className="flex gap-2">
                        <Button
                            variant="outline"
                            className="gap-2 border-[var(--color-border)] text-[var(--color-text)]"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={scanning}
                        >
                            {scanning ? (
                                <Loader2 className="w-4 h-4 animate-spin" />
                            ) : (
                                <Camera className="w-4 h-4" />
                            )}
                            {scanning ? "识别中..." : "截图导入"}
                        </Button>
                        <Button
                            variant="default"
                            className="gap-2"
                            onClick={() => setShowAddDialog(true)}
                        >
                            <Plus className="w-4 h-4" /> 添加持仓
                        </Button>
                    </div>
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept="image/*"
                        className="hidden"
                        onChange={handleFileSelect}
                    />
                </div>

                {/* 截图识别错误 */}
                {scanError && (
                    <div className="bg-red-50 border border-red-200 rounded-xl p-4 mb-6 flex items-center justify-between">
                        <span className="text-sm text-red-700">{scanError}</span>
                        <button onClick={dismissScan} className="text-red-400 hover:text-red-600">
                            <X className="w-4 h-4" />
                        </button>
                    </div>
                )}

                {/* 截图识别 loading */}
                {scanning && (
                    <div className="bg-white rounded-xl border border-[var(--color-border)] p-8 mb-6 text-center">
                        <Loader2 className="w-8 h-8 animate-spin text-[var(--color-primary)] mx-auto mb-3" />
                        <div className="text-sm text-[var(--color-text)]">正在识别持仓截图...</div>
                        <div className="text-xs text-[var(--color-text-muted)] mt-1">通常需要 10-15 秒</div>
                    </div>
                )}

                {/* 截图识别结果确认 */}
                {scanResults && (
                    <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-6">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="text-sm font-medium text-[var(--color-text)]">
                                识别结果 — 共 {scanResults.length} 只基金
                            </h3>
                            {scanTotalValue > 0 && (
                                <span className="text-xs text-[var(--color-text-muted)]">
                                    总资产 {scanTotalValue.toLocaleString("zh-CN", { style: "currency", currency: "CNY" })}
                                </span>
                            )}
                        </div>
                        <div className="space-y-2 mb-4">
                            {scanResults.map((h) => (
                                <div key={h.code} className="flex items-center justify-between py-2 border-b border-[var(--color-border)] last:border-0">
                                    <div className="flex items-center gap-3">
                                        <span className="text-xs text-[var(--color-text-muted)] font-mono">{h.code}</span>
                                        <span className="text-sm text-[var(--color-text)]">{h.name}</span>
                                        {holdings.some((ex) => ex.code === h.code) && (
                                            <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-sidebar-bg)] text-[var(--color-text-muted)]">已持有</span>
                                        )}
                                    </div>
                                    {h.amount > 0 && (
                                        <span className="text-sm text-[var(--color-text)]">
                                            {h.amount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} 元
                                        </span>
                                    )}
                                </div>
                            ))}
                        </div>
                        <div className="flex gap-2">
                            <Button onClick={confirmScanResults} className="gap-2">
                                <Check className="w-4 h-4" /> 确认添加
                            </Button>
                            <Button variant="ghost" onClick={dismissScan}>
                                取消
                            </Button>
                        </div>
                    </div>
                )}

                {/* Add Dialog */}
                {showAddDialog && (
                    <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-6">
                        <h3 className="text-sm font-medium text-[var(--color-text)] mb-3">
                            添加基金到持仓
                        </h3>
                        <div className="flex gap-2">
                            <Input
                                value={newCode}
                                onChange={(e) => setNewCode(e.target.value)}
                                placeholder="输入6位基金代码，如 005827"
                                variant="boxed"
                                className="font-sans text-sm"
                                onKeyDown={(e) => e.key === "Enter" && addHolding()}
                                autoFocus
                            />
                            <Button onClick={addHolding} disabled={newCode.length !== 6}>
                                添加
                            </Button>
                            <Button variant="ghost" onClick={() => setShowAddDialog(false)}>
                                取消
                            </Button>
                        </div>
                        <p className="text-xs text-[var(--color-text-muted)] mt-2">
                            也可以在对话中告诉小基"我持有 005827"，记忆系统会自动记录。
                        </p>
                    </div>
                )}

                {/* Holdings */}
                {holdings.length === 0 && !scanning && !scanResults ? (
                    <div className="text-center py-20">
                        <div className="text-[var(--color-text-muted)] mb-4">
                            暂无持仓记录
                        </div>
                        <div className="text-sm text-[var(--color-text-muted)] mb-6">
                            添加基金代码、截图导入，或在对话中告诉小基你的持仓
                        </div>
                        <div className="flex gap-3 justify-center">
                            <Button
                                variant="outline"
                                className="border-[var(--color-border)] text-[var(--color-text)] gap-2"
                                onClick={() => fileInputRef.current?.click()}
                            >
                                <Camera className="w-4 h-4" /> 截图导入
                            </Button>
                            <Button variant="default" onClick={() => setShowAddDialog(true)}>
                                <Plus className="w-4 h-4 mr-2" /> 手动添加
                            </Button>
                            <Button
                                variant="outline"
                                className="border-[var(--color-border)] text-[var(--color-text)]"
                                onClick={() => navigate("/chat")}
                            >
                                <MessageSquare className="w-4 h-4 mr-2" /> 告诉小基
                            </Button>
                        </div>
                    </div>
                ) : holdings.length > 0 ? (
                    <>
                        {/* 总资产概览 */}
                        {totalAmount > 0 && (
                            <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-6">
                                <div className="flex items-end justify-between mb-1">
                                    <div>
                                        <div className="text-xs text-[var(--color-text-muted)] mb-1">总持仓资产</div>
                                        <div className="text-2xl font-semibold text-[var(--color-text)] tracking-tight">
                                            {totalAmount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                            <span className="text-sm font-normal text-[var(--color-text-muted)] ml-1">元</span>
                                        </div>
                                    </div>
                                    {totalProfit !== 0 && (
                                        <div className="text-right">
                                            <div className="text-xs text-[var(--color-text-muted)]">持有收益</div>
                                            <div className={`text-lg font-semibold ${totalProfit >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                {totalProfit >= 0 ? "+" : ""}{totalProfit.toFixed(2)}
                                            </div>
                                        </div>
                                    )}
                                </div>
                                <div className="text-xs text-[var(--color-text-muted)] mt-1">
                                    共 {holdings.length} 只基金
                                </div>
                                {/* 仓位分布条 */}
                                <div className="flex h-2 rounded-full overflow-hidden mt-4 bg-[var(--color-sidebar-bg)]">
                                    {holdings.map((h, i) => (
                                        <div
                                            key={h.code}
                                            className="h-full transition-all"
                                            style={{
                                                width: `${h.weight ?? 0}%`,
                                                backgroundColor: PALETTE[i % PALETTE.length],
                                                opacity: 0.75,
                                            }}
                                        />
                                    ))}
                                </div>
                            </div>
                        )}

                        {/* 基金列表 */}
                        <div className="bg-white rounded-xl border border-[var(--color-border)] overflow-hidden">
                            {holdings.map((h, i) => (
                                <div
                                    key={h.code}
                                    className={`group flex items-center gap-4 px-5 py-4 cursor-pointer hover:bg-[var(--color-sidebar-bg)]/50 transition-colors ${
                                        i < holdings.length - 1 ? "border-b border-[var(--color-border)]" : ""
                                    }`}
                                    onClick={() => navigate(`/fund/${h.code}`)}
                                >
                                    {/* 色块标识 */}
                                    <div
                                        className="w-1 h-10 rounded-full shrink-0"
                                        style={{ backgroundColor: PALETTE[i % PALETTE.length], opacity: 0.7 }}
                                    />

                                    {/* 基金信息 */}
                                    <div className="flex-1 min-w-0">
                                        <div className="text-sm font-medium text-[var(--color-text)] truncate">
                                            {h.name}
                                        </div>
                                        <div className="flex items-center gap-2 mt-0.5">
                                            <span className="text-xs text-[var(--color-text-muted)] font-mono">{h.code}</span>
                                            {h.weight && h.weight > 0 ? (
                                                <span className="text-xs text-[var(--color-text-muted)]">{h.weight.toFixed(1)}%</span>
                                            ) : null}
                                        </div>
                                    </div>

                                    {/* 金额 + 盈亏 */}
                                    {h.amount && h.amount > 0 ? (
                                        <div className="text-right shrink-0">
                                            <div className="text-sm tabular-nums font-medium text-[var(--color-text)]">
                                                {h.amount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                            </div>
                                            {h.totalProfitRate != null && (
                                                <div className={`text-xs tabular-nums ${(h.totalProfit ?? 0) >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                    {(h.totalProfit ?? 0) >= 0 ? "+" : ""}{(h.totalProfit ?? 0).toFixed(2)} ({(h.totalProfitRate ?? 0) >= 0 ? "+" : ""}{h.totalProfitRate.toFixed(2)}%)
                                                </div>
                                            )}
                                        </div>
                                    ) : null}

                                    {/* 移除按钮 */}
                                    <button
                                        className="shrink-0 text-xs text-[var(--color-text-muted)] opacity-0 group-hover:opacity-100 hover:text-[var(--color-down)] transition-all"
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            removeHolding(h.code)
                                        }}
                                    >
                                        <X className="w-4 h-4" />
                                    </button>
                                </div>
                            ))}
                        </div>

                        <div className="flex gap-3 mt-6">
                            <Button
                                variant="outline"
                                className="border-[var(--color-border)] text-[var(--color-text)] gap-2"
                                onClick={() => navigate("/chat")}
                            >
                                <MessageSquare className="w-4 h-4" /> 让小基分析组合
                            </Button>
                        </div>
                    </>
                ) : null}
            </div>
        </div>
    )
}
