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
    shares: number
    cost: number
    amount: number
    weight: number
}

const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB

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

    const totalAmount = holdings.reduce((sum, h) => sum + h.amount, 0)
    const totalCost = holdings.reduce((sum, h) => sum + h.cost, 0)
    const totalProfit = totalAmount - totalCost

    useEffect(() => {
        getPortfolio().then((res) => {
            if (res.data?.length) {
                setHoldings(res.data.map((h) => ({
                    code: h.code,
                    name: h.name || `基金 ${h.code}`,
                    shares: h.shares || 0,
                    cost: h.cost || 0,
                    amount: h.amount || 0,
                    weight: h.weight || 0,
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
        setHoldings((prev) => [...prev, { code, name: "加载中...", shares: 0, cost: 0, amount: 0, weight: 0 }])
        setNewCode("")
        setShowAddDialog(false)

        fetch(`/api/v1/fund/${code}`)
            .then((r) => r.json())
            .then((fund) => {
                const name = fund.name || `基金 ${code}`
                setHoldings((prev) =>
                    prev.map((h) => (h.code === code ? { ...h, name } : h))
                )
                addPortfolioHolding(code, name, 0, 0).catch((err) => {
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
                setHoldings((prev) => [...prev, { code: h.code, name: h.name, shares: 0, cost: h.amount, amount: h.amount, weight: 0 }])
                await addPortfolioHolding(h.code, h.name, 0, h.amount).catch(() => {})
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
            <div className="max-w-3xl mx-auto px-6 py-8">
                {/* 顶栏：标题 + 操作 */}
                <div className="flex items-center justify-between mb-8">
                    <h1 className="text-lg font-semibold text-[var(--color-text)]">持仓</h1>
                    <div className="flex gap-2">
                        <Button
                            variant="outline"
                            className="gap-2 border-[var(--color-border)] text-[var(--color-text)] text-xs h-8 px-3"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={scanning}
                        >
                            {scanning ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Camera className="w-3.5 h-3.5" />}
                            {scanning ? "识别中..." : "截图导入"}
                        </Button>
                        <Button
                            variant="default"
                            className="gap-2 text-xs h-8 px-3"
                            onClick={() => setShowAddDialog(true)}
                        >
                            <Plus className="w-3.5 h-3.5" /> 添加
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
                    <div className="bg-[var(--color-down)]/[0.06] border border-[var(--color-down)]/20 rounded-lg p-3 mb-6 flex items-center justify-between">
                        <span className="text-sm text-[var(--color-down)]">{scanError}</span>
                        <button onClick={dismissScan} className="text-[var(--color-down)]/60 hover:text-[var(--color-down)]">
                            <X className="w-4 h-4" />
                        </button>
                    </div>
                )}

                {/* 截图识别 loading */}
                {scanning && (
                    <div className="rounded-lg border border-[var(--color-border)] p-8 mb-6 text-center bg-white">
                        <Loader2 className="w-6 h-6 animate-spin text-[var(--color-primary)] mx-auto mb-3" />
                        <div className="text-sm text-[var(--color-text)]">正在识别持仓截图...</div>
                        <div className="text-xs text-[var(--color-text-muted)] mt-1">通常需要 10-15 秒</div>
                    </div>
                )}

                {/* 截图识别结果确认 */}
                {scanResults && (
                    <div className="bg-white rounded-lg border border-[var(--color-border)] p-5 mb-6">
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
                                        <span className="text-sm tabular-nums text-[var(--color-text)]">
                                            {h.amount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                        </span>
                                    )}
                                </div>
                            ))}
                        </div>
                        <div className="flex gap-2">
                            <Button onClick={confirmScanResults} className="gap-2 h-8 text-xs">
                                <Check className="w-3.5 h-3.5" /> 确认添加
                            </Button>
                            <Button variant="ghost" onClick={dismissScan} className="h-8 text-xs">
                                取消
                            </Button>
                        </div>
                    </div>
                )}

                {/* 添加对话框 */}
                {showAddDialog && (
                    <div className="bg-white rounded-lg border border-[var(--color-border)] p-5 mb-6">
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
                            <Button onClick={addHolding} disabled={newCode.length !== 6} className="h-9">
                                添加
                            </Button>
                            <Button variant="ghost" onClick={() => setShowAddDialog(false)} className="h-9">
                                取消
                            </Button>
                        </div>
                        <p className="text-xs text-[var(--color-text-muted)] mt-2">
                            也可以在对话中告诉 Quinfi"我持有 005827"，系统会自动记录。
                        </p>
                    </div>
                )}

                {/* 主内容 */}
                {holdings.length === 0 && !scanning && !scanResults ? (
                    <div className="text-center py-20">
                        <div className="text-[var(--color-text-muted)] mb-4">暂无持仓记录</div>
                        <div className="text-sm text-[var(--color-text-muted)] mb-6">
                            添加基金代码、截图导入，或在对话中告诉 Quinfi 你的持仓
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
                                <MessageSquare className="w-4 h-4 mr-2" /> 告诉 Quinfi
                            </Button>
                        </div>
                    </div>
                ) : holdings.length > 0 ? (
                    <>
                        {/* 总资产 — 大数字直出 */}
                        {totalAmount > 0 && (
                            <div className="mb-8">
                                <div className="text-xs text-[var(--color-text-muted)] mb-1">总持仓资产</div>
                                <div className="text-3xl font-semibold text-[var(--color-text)] tracking-tight tabular-nums">
                                    {totalAmount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                    <span className="text-base font-normal text-[var(--color-text-muted)] ml-1.5">元</span>
                                </div>
                                <div className="flex items-center gap-3 mt-1.5">
                                    {totalCost > 0 && totalProfit !== 0 && (
                                        <span className={`text-sm tabular-nums ${totalProfit >= 0 ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                            {totalProfit >= 0 ? "+" : ""}{totalProfit.toFixed(2)}
                                        </span>
                                    )}
                                    <span className="text-xs text-[var(--color-text-muted)]">
                                        {holdings.length} 只基金
                                    </span>
                                </div>
                            </div>
                        )}

                        {/* 基金列表 */}
                        <div className="space-y-1">
                            {holdings.map((h) => (
                                <div
                                    key={h.code}
                                    className="group flex items-center gap-4 px-4 py-3.5 rounded-lg cursor-pointer hover:bg-white hover:shadow-sm transition-all"
                                    onClick={() => navigate(`/fund/${h.code}`)}
                                >
                                    {/* 基金信息 */}
                                    <div className="flex-1 min-w-0">
                                        <div className="text-sm font-medium text-[var(--color-text)] truncate">
                                            {h.name}
                                        </div>
                                        <div className="text-xs text-[var(--color-text-muted)] font-mono mt-0.5">
                                            {h.code}
                                        </div>
                                    </div>

                                    {/* 右侧：市值 + 盈亏 */}
                                    <div className="text-right shrink-0 flex items-center gap-3">
                                        {h.amount > 0 ? (
                                            <div>
                                                <div className="text-sm tabular-nums font-medium text-[var(--color-text)]">
                                                    {h.amount.toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                                                </div>
                                                {h.cost > 0 ? (
                                                    <div className={`text-xs tabular-nums mt-0.5 ${h.amount >= h.cost ? "text-[var(--color-up)]" : "text-[var(--color-down)]"}`}>
                                                        {h.amount >= h.cost ? "+" : ""}{((h.amount - h.cost) / h.cost * 100).toFixed(2)}%
                                                    </div>
                                                ) : h.weight > 0 ? (
                                                    <div className="text-xs text-[var(--color-text-muted)] mt-0.5">
                                                        {h.weight.toFixed(1)}%
                                                    </div>
                                                ) : null}
                                            </div>
                                        ) : null}

                                        {/* 移除 */}
                                        <button
                                            className="shrink-0 p-1 rounded text-[var(--color-text-muted)] opacity-0 group-hover:opacity-100 hover:text-[var(--color-down)] hover:bg-[var(--color-down)]/[0.06] transition-all"
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                removeHolding(h.code)
                                            }}
                                        >
                                            <X className="w-3.5 h-3.5" />
                                        </button>
                                    </div>
                                </div>
                            ))}
                        </div>

                        <div className="mt-8">
                            <Button
                                variant="outline"
                                className="border-[var(--color-border)] text-[var(--color-text)] gap-2 text-xs h-8 px-3"
                                onClick={() => navigate("/chat")}
                            >
                                <MessageSquare className="w-3.5 h-3.5" /> 让 Quinfi 分析组合
                            </Button>
                        </div>
                    </>
                ) : null}
            </div>
        </div>
    )
}
