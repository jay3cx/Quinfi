import { TrendingUp, TrendingDown, Scale, Loader2, Database, AlertTriangle, ShieldCheck, ShieldAlert } from "lucide-react"
import type { DebateArgument, DebateVerdict, DebateResult, DebatePhaseKey, DebatePhaseUpdate } from "@/types"

// 阶段定义
const PHASES = [
    { key: "data_gather", label: "数据收集", icon: Database, color: "text-[var(--color-text-muted)]", border: "border-[var(--color-border)]", bg: "bg-[var(--color-sidebar-bg)]/30" },
    { key: "bull_case", label: "Bull 立论", icon: TrendingUp, color: "text-[var(--color-up)]", border: "border-[var(--color-up)]/20", bg: "bg-[var(--color-up)]/[0.06]" },
    { key: "bear_case", label: "Bear 立论", icon: TrendingDown, color: "text-[var(--color-down)]", border: "border-[var(--color-down)]/20", bg: "bg-[var(--color-down)]/[0.06]" },
    { key: "bull_rebuttal", label: "Bull 反驳", icon: TrendingUp, color: "text-[var(--color-up)]", border: "border-[var(--color-up)]/15", bg: "bg-[var(--color-up)]/[0.04]" },
    { key: "bear_rebuttal", label: "Bear 反驳", icon: TrendingDown, color: "text-[var(--color-down)]", border: "border-[var(--color-down)]/15", bg: "bg-[var(--color-down)]/[0.04]" },
    { key: "judge_verdict", label: "裁判裁决", icon: Scale, color: "text-[var(--color-primary)]", border: "border-[var(--color-primary)]/30", bg: "bg-[var(--color-primary-bg)]/50" },
] as const

interface DebateTimelineProps {
    // 实时模式：SSE 推送的阶段更新
    phases?: DebatePhaseUpdate[]
    activePhase?: DebatePhaseKey | DebatePhaseKey[] | null
    // 实时模式下的系统置信度（从 SSE 推送获得）
    systemConfidence?: number
    decisionGate?: string
    // 完成模式：完整结果
    result?: DebateResult
}

function ConfidenceBar({ value, color }: { value: number; color: string }) {
    const safeValue = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0
    return (
        <div className="flex items-center gap-2">
            <div className="flex-1 h-1.5 bg-[var(--color-border)] rounded-full overflow-hidden">
                <div
                    className={`h-full rounded-full transition-all duration-700 ${color === "up" ? "bg-[var(--color-up)]" : color === "down" ? "bg-[var(--color-down)]" : "bg-[var(--color-primary)]"}`}
                    style={{ width: `${safeValue}%` }}
                />
            </div>
            <span className="text-xs text-[var(--color-text-muted)] w-8 text-right">{safeValue}</span>
        </div>
    )
}

function ArgumentCard({ arg, phaseConfig }: { arg: DebateArgument; phaseConfig: typeof PHASES[number] }) {
    const Icon = phaseConfig.icon

    return (
        <div className={`rounded-lg border ${phaseConfig.border} ${phaseConfig.bg} p-4 transition-opacity duration-500`}>
            <div className="flex items-center gap-2 mb-2">
                <Icon className={`w-4 h-4 ${phaseConfig.color}`} />
                <span className={`text-sm font-semibold ${phaseConfig.color}`}>{phaseConfig.label}</span>
            </div>
            {arg.position && (
                <p className="text-sm text-[var(--color-text)] mt-1 font-medium">{arg.position}</p>
            )}
            {arg.points && arg.points.length > 0 && (
                <ol className="text-xs text-[var(--color-text-secondary)] mt-2 space-y-1 list-decimal pl-4">
                    {arg.points.map((p, i) => <li key={i}>{p}</li>)}
                </ol>
            )}
        </div>
    )
}

const GATE_LABELS: Record<string, { label: string; color: string; Icon: typeof ShieldCheck }> = {
    pass: { label: "可信", color: "text-[var(--color-up)]", Icon: ShieldCheck },
    degrade: { label: "证据不足", color: "text-[var(--color-warn)]", Icon: ShieldAlert },
    review: { label: "复核中", color: "text-[var(--color-text-muted)]", Icon: ShieldAlert },
}

function VerdictCard({ verdict, systemConfidence, decisionGate }: {
    verdict: DebateVerdict
    systemConfidence?: number
    decisionGate?: string
}) {
    const phaseConfig = PHASES.find((p) => p.key === "judge_verdict")!
    const Icon = phaseConfig.icon
    const gate = decisionGate && GATE_LABELS[decisionGate]
    const hasSystemConf = systemConfidence != null && systemConfidence > 0

    return (
        <div className={`rounded-lg border ${phaseConfig.border} ${phaseConfig.bg} p-4 transition-opacity duration-500`}>
            <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                    <Icon className={`w-4 h-4 ${phaseConfig.color}`} />
                    <span className={`text-sm font-semibold ${phaseConfig.color}`}>{phaseConfig.label}</span>
                </div>
                {gate && (
                    <div className="flex items-center gap-1">
                        <gate.Icon className={`w-3.5 h-3.5 ${gate.color}`} />
                        <span className={`text-xs font-medium ${gate.color}`}>{gate.label}</span>
                    </div>
                )}
            </div>
            {hasSystemConf && (
                <ConfidenceBar value={systemConfidence} color={decisionGate === "pass" ? "up" : "primary"} />
            )}
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
                <div className="mt-3 p-2.5 rounded bg-[var(--color-warn)]/[0.06] border border-[var(--color-warn)]/20">
                    <div className="flex items-center gap-1.5 mb-1.5">
                        <AlertTriangle className="w-3 h-3 text-[var(--color-warn)]" />
                        <span className="text-xs font-medium text-[var(--color-warn)]">风险提示</span>
                    </div>
                    <ul className="text-xs text-[var(--color-warn)]/80 space-y-0.5 ml-4 list-disc">
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

export function DebateTimeline({ phases, activePhase, systemConfidence, decisionGate, result }: DebateTimelineProps) {
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
                    if (item.verdict) return <VerdictCard key={item.key} verdict={item.verdict} systemConfidence={result.system_confidence} decisionGate={result.decision_gate} />
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
	const activeKeys = Array.isArray(activePhase) ? activePhase : activePhase ? [activePhase] : []
	const activeSet = new Set(activeKeys)
	const hasActivePhase = activeSet.size > 0
	const isTerminalState = activePhase === null

	return (
		<div className="space-y-3">
			{PHASES.map((config) => {
                // data_gather 阶段不需要可视化卡片
                if (config.key === "data_gather") return null

                const phaseUpdate = phases?.find(p => p.phase === config.key)

                if (phaseUpdate) {
                    // 已完成的阶段
                    if (phaseUpdate.verdict) {
                        return <VerdictCard key={config.key} verdict={phaseUpdate.verdict} systemConfidence={systemConfidence} decisionGate={decisionGate} />
                    }
                    if (phaseUpdate.argument) {
                        return <ArgumentCard key={config.key} arg={phaseUpdate.argument} phaseConfig={config} />
                    }
				}

				// 未完成的阶段：仅在实时执行中显示 active，结束态一律显示等待中
				if (completedPhases.size > 0 || hasActivePhase || isTerminalState) {
					const isActive = activeSet.has(config.key)
					return <PhaseSkeleton key={config.key} phaseConfig={config} isActive={isActive} />
				}

				return null
			})}
		</div>
	)
}
