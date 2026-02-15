import { TrendingUp, TrendingDown, Scale, Loader2, Database, AlertTriangle } from "lucide-react"
import type { DebateArgument, DebateVerdict, DebateResult, DebatePhaseUpdate } from "@/types"

// 阶段定义
const PHASES = [
    { key: "data_gather", label: "数据收集", icon: Database, color: "text-[var(--color-text-muted)]", border: "border-[var(--color-border)]", bg: "bg-[var(--color-sidebar-bg)]/30" },
    { key: "bull_case", label: "Bull 立论", icon: TrendingUp, color: "text-[var(--color-up)]", border: "border-green-200", bg: "bg-green-50/50" },
    { key: "bear_case", label: "Bear 立论", icon: TrendingDown, color: "text-[var(--color-down)]", border: "border-red-200", bg: "bg-red-50/50" },
    { key: "bull_rebuttal", label: "Bull 反驳", icon: TrendingUp, color: "text-[var(--color-up)]", border: "border-green-200/60", bg: "bg-green-50/30" },
    { key: "bear_rebuttal", label: "Bear 反驳", icon: TrendingDown, color: "text-[var(--color-down)]", border: "border-red-200/60", bg: "bg-red-50/30" },
    { key: "judge_verdict", label: "裁判裁决", icon: Scale, color: "text-[var(--color-primary)]", border: "border-[var(--color-primary)]/30", bg: "bg-[var(--color-primary-bg)]/50" },
] as const

interface DebateTimelineProps {
    // 实时模式：SSE 推送的阶段更新
    phases?: DebatePhaseUpdate[]
    activePhase?: string | null
    // 完成模式：完整结果
    result?: DebateResult
}

function ConfidenceBar({ value, color }: { value: number; color: string }) {
    return (
        <div className="flex items-center gap-2">
            <div className="flex-1 h-1.5 bg-[var(--color-border)] rounded-full overflow-hidden">
                <div
                    className={`h-full rounded-full transition-all duration-700 ${color === "up" ? "bg-[var(--color-up)]" : color === "down" ? "bg-[var(--color-down)]" : "bg-[var(--color-primary)]"}`}
                    style={{ width: `${value}%` }}
                />
            </div>
            <span className="text-xs text-[var(--color-text-muted)] w-8 text-right">{value}</span>
        </div>
    )
}

function ArgumentCard({ arg, phaseConfig }: { arg: DebateArgument; phaseConfig: typeof PHASES[number] }) {
    const Icon = phaseConfig.icon
    const colorType = phaseConfig.key.startsWith("bull") ? "up" : "down"

    return (
        <div className={`rounded-lg border ${phaseConfig.border} ${phaseConfig.bg} p-4 transition-opacity duration-500`}>
            <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                    <Icon className={`w-4 h-4 ${phaseConfig.color}`} />
                    <span className={`text-sm font-semibold ${phaseConfig.color}`}>{phaseConfig.label}</span>
                </div>
                <span className="text-xs text-[var(--color-text-muted)]">置信度</span>
            </div>
            <ConfidenceBar value={arg.confidence} color={colorType} />
            {arg.position && (
                <p className="text-sm text-[var(--color-text)] mt-3 font-medium">{arg.position}</p>
            )}
            {arg.points && arg.points.length > 0 && (
                <ol className="text-xs text-[var(--color-text-secondary)] mt-2 space-y-1 list-decimal pl-4">
                    {arg.points.map((p, i) => <li key={i}>{p}</li>)}
                </ol>
            )}
        </div>
    )
}

function VerdictCard({ verdict }: { verdict: DebateVerdict }) {
    const phaseConfig = PHASES[5] // judge_verdict
    const Icon = phaseConfig.icon

    return (
        <div className={`rounded-lg border ${phaseConfig.border} ${phaseConfig.bg} p-4 transition-opacity duration-500`}>
            <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                    <Icon className={`w-4 h-4 ${phaseConfig.color}`} />
                    <span className={`text-sm font-semibold ${phaseConfig.color}`}>{phaseConfig.label}</span>
                </div>
                <span className="text-xs text-[var(--color-text-muted)]">置信度</span>
            </div>
            <ConfidenceBar value={verdict.confidence} color="primary" />
            {verdict.summary && (
                <p className="text-sm text-[var(--color-text)] mt-3 leading-relaxed">{verdict.summary}</p>
            )}
            <div className="mt-3 space-y-2 text-xs">
                {verdict.bull_strength && (
                    <div className="flex gap-2">
                        <span className="text-[var(--color-up)] font-medium shrink-0">看多最强:</span>
                        <span className="text-[var(--color-text-secondary)]">{verdict.bull_strength}</span>
                    </div>
                )}
                {verdict.bear_strength && (
                    <div className="flex gap-2">
                        <span className="text-[var(--color-down)] font-medium shrink-0">看空最强:</span>
                        <span className="text-[var(--color-text-secondary)]">{verdict.bear_strength}</span>
                    </div>
                )}
                {verdict.suggestion && (
                    <div className="flex gap-2">
                        <span className="text-[var(--color-primary)] font-medium shrink-0">参考建议:</span>
                        <span className="text-[var(--color-text-secondary)]">{verdict.suggestion}</span>
                    </div>
                )}
            </div>
            {verdict.risk_warnings && verdict.risk_warnings.length > 0 && (
                <div className="mt-3 p-2.5 rounded bg-amber-50/60 border border-amber-200/40">
                    <div className="flex items-center gap-1.5 mb-1.5">
                        <AlertTriangle className="w-3 h-3 text-amber-600" />
                        <span className="text-xs font-medium text-amber-700">风险提示</span>
                    </div>
                    <ul className="text-xs text-amber-800/70 space-y-0.5 ml-4 list-disc">
                        {verdict.risk_warnings.map((w, i) => <li key={i}>{w}</li>)}
                    </ul>
                </div>
            )}
        </div>
    )
}

function PhaseSkeleton({ phaseConfig, isActive }: { phaseConfig: typeof PHASES[number]; isActive: boolean }) {
    const Icon = phaseConfig.icon

    return (
        <div className={`rounded-lg border border-dashed ${phaseConfig.border} p-4 ${isActive ? "animate-pulse" : "opacity-40"}`}>
            <div className="flex items-center gap-2">
                {isActive ? (
                    <Loader2 className={`w-4 h-4 ${phaseConfig.color} animate-spin`} />
                ) : (
                    <Icon className="w-4 h-4 text-[var(--color-text-muted)]" />
                )}
                <span className={`text-sm font-medium ${isActive ? phaseConfig.color : "text-[var(--color-text-muted)]"}`}>
                    {phaseConfig.label}
                </span>
                <span className="text-xs text-[var(--color-text-muted)] ml-auto">
                    {isActive ? "进行中..." : "等待中"}
                </span>
            </div>
        </div>
    )
}

export function DebateTimeline({ phases, activePhase, result }: DebateTimelineProps) {
    // 完成模式：从 DebateResult 构建展示
    if (result) {
        const items: { key: string; arg?: DebateArgument; verdict?: DebateVerdict }[] = []
        if (result.bull_case) items.push({ key: "bull_case", arg: result.bull_case })
        if (result.bear_case) items.push({ key: "bear_case", arg: result.bear_case })
        if (result.bull_rebuttal) items.push({ key: "bull_rebuttal", arg: result.bull_rebuttal })
        if (result.bear_rebuttal) items.push({ key: "bear_rebuttal", arg: result.bear_rebuttal })
        if (result.verdict) items.push({ key: "judge_verdict", verdict: result.verdict })

        if (items.length === 0) return null

        return (
            <div className="space-y-3">
                {items.map((item) => {
                    const config = PHASES.find(p => p.key === item.key)!
                    if (item.verdict) return <VerdictCard key={item.key} verdict={item.verdict} />
                    if (item.arg) return <ArgumentCard key={item.key} arg={item.arg} phaseConfig={config} />
                    return null
                })}
                <p className="text-xs text-[var(--color-text-muted)] text-center mt-2">
                    以上仅为多角度分析参考，不构成投资建议。
                </p>
            </div>
        )
    }

    // 实时模式：展示已完成的阶段 + 进行中骨架
    const completedPhases = new Set(phases?.map(p => p.phase) || [])

    // 找到辩论阶段顺序中的下一个待执行阶段
    const debatePhaseKeys: readonly string[] = PHASES.map(p => p.key)
    const currentActiveIdx = activePhase
        ? debatePhaseKeys.indexOf(activePhase)
        : -1

    return (
        <div className="space-y-3">
            {PHASES.map((config, idx) => {
                // data_gather 阶段不需要可视化卡片
                if (config.key === "data_gather") return null

                const phaseUpdate = phases?.find(p => p.phase === config.key)

                if (phaseUpdate) {
                    // 已完成的阶段
                    if (phaseUpdate.verdict) {
                        return <VerdictCard key={config.key} verdict={phaseUpdate.verdict} />
                    }
                    if (phaseUpdate.argument) {
                        return <ArgumentCard key={config.key} arg={phaseUpdate.argument} phaseConfig={config} />
                    }
                }

                // 未完成的阶段：如果有活跃阶段，判断当前是否是活跃或待处理
                if (completedPhases.size > 0 || activePhase) {
                    const isActive = config.key === activePhase || (currentActiveIdx === -1 && idx === getNextPhaseIdx(completedPhases))
                    return <PhaseSkeleton key={config.key} phaseConfig={config} isActive={isActive} />
                }

                return null
            })}
        </div>
    )
}

function getNextPhaseIdx(completed: Set<string>): number {
    for (let i = 1; i < PHASES.length; i++) { // skip data_gather
        if (!completed.has(PHASES[i].key)) return i
    }
    return -1
}
