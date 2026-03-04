import { ChevronDown, ChevronRight, BarChart3, User, Globe, RefreshCw, Scale, AlertTriangle, ThumbsUp } from "lucide-react"
import { useState } from "react"
import { DebateTimeline } from "@/components/DebateTimeline"
import type { DeepReport } from "@/types"

interface DeepAnalysisPanelProps {
    report: DeepReport
}

function Section({ title, icon, children, defaultOpen = false }: {
    title: string
    icon: React.ReactNode
    children: React.ReactNode
    defaultOpen?: boolean
}) {
    const [open, setOpen] = useState(defaultOpen)
    return (
        <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
            <button
                onClick={() => setOpen(!open)}
                className="w-full flex items-center gap-2 px-4 py-3 bg-white hover:bg-[var(--color-sidebar-bg)] transition-colors text-left"
            >
                {icon}
                <span className="text-sm font-semibold text-[var(--color-text)] flex-1">{title}</span>
                {open ? <ChevronDown className="w-4 h-4 text-[var(--color-text-muted)]" /> : <ChevronRight className="w-4 h-4 text-[var(--color-text-muted)]" />}
            </button>
            {open && <div className="px-4 py-4 border-t border-[var(--color-border)] bg-white">{children}</div>}
        </div>
    )
}

function Tag({ children, variant = "default" }: { children: React.ReactNode; variant?: "default" | "up" | "down" | "warn" }) {
    const colors = {
        default: "bg-[var(--color-sidebar-bg)] text-[var(--color-text-secondary)]",
        up: "bg-[var(--color-up)]/[0.06] text-[var(--color-up)]",
        down: "bg-[var(--color-down)]/[0.06] text-[var(--color-down)]",
        warn: "bg-[var(--color-warn)]/[0.06] text-[var(--color-warn)]",
    }
    return <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${colors[variant]}`}>{children}</span>
}

export function DeepAnalysisPanel({ report }: DeepAnalysisPanelProps) {
    return (
        <div className="space-y-3 mt-8">
            <div className="flex items-center justify-between mb-2">
                <h2 className="text-base font-semibold text-[var(--color-text)]">
                    深度分析报告
                </h2>
                <span className="text-xs text-[var(--color-text-muted)]">
                    {new Date(report.generated_at).toLocaleString("zh-CN")}
                </span>
            </div>

            {/* 基金综合分析 */}
            {report.fund_analysis && (
                <Section
                    title="基金综合分析"
                    icon={<BarChart3 className="w-4 h-4 text-[var(--color-primary)]" />}
                    defaultOpen
                >
                    {report.fund_analysis.summary && (
                        <p className="text-sm text-[var(--color-text)] mb-4 leading-relaxed">{report.fund_analysis.summary}</p>
                    )}

                    {/* 投资建议 */}
                    {report.fund_analysis.recommendation && (
                        <div className="mb-4 p-3 rounded-lg bg-[var(--color-primary-bg)]/30 border border-[var(--color-primary)]/20">
                            <div className="flex items-center gap-2 mb-2">
                                <ThumbsUp className="w-3.5 h-3.5 text-[var(--color-primary)]" />
                                <span className="text-sm font-medium text-[var(--color-primary)]">投资建议</span>
                                <Tag variant={report.fund_analysis.recommendation.action === "买入" ? "up" : report.fund_analysis.recommendation.action === "减持" ? "down" : "default"}>
                                    {report.fund_analysis.recommendation.action}
                                </Tag>
                                <Tag>{`置信度: ${report.fund_analysis.recommendation.confidence}`}</Tag>
                            </div>
                            {report.fund_analysis.recommendation.reasons.length > 0 && (
                                <ul className="text-xs text-[var(--color-text-secondary)] space-y-1 ml-5 list-disc">
                                    {report.fund_analysis.recommendation.reasons.map((r, i) => <li key={i}>{r}</li>)}
                                </ul>
                            )}
                        </div>
                    )}

                    {/* 风险评估 */}
                    {report.fund_analysis.risk_assessment && (
                        <div className="mb-4 p-3 rounded-lg bg-[var(--color-warn)]/[0.06] border border-[var(--color-warn)]/20">
                            <div className="flex items-center gap-2 mb-2">
                                <AlertTriangle className="w-3.5 h-3.5 text-[var(--color-warn)]" />
                                <span className="text-sm font-medium text-[var(--color-warn)]">风险评估</span>
                                <Tag variant="warn">{`风险等级: ${report.fund_analysis.risk_assessment.risk_level}`}</Tag>
                            </div>
                            <div className="text-xs text-[var(--color-text-secondary)] space-y-1">
                                <div>波动率: {report.fund_analysis.risk_assessment.volatility} | 最大回撤: {report.fund_analysis.risk_assessment.max_drawdown}</div>
                                {report.fund_analysis.risk_assessment.risk_warnings.length > 0 && (
                                    <ul className="ml-5 list-disc mt-1">
                                        {report.fund_analysis.risk_assessment.risk_warnings.map((w, i) => <li key={i}>{w}</li>)}
                                    </ul>
                                )}
                            </div>
                        </div>
                    )}

                    {/* 持仓分析 */}
                    {report.fund_analysis.holding_analysis && (
                        <div>
                            <div className="text-xs font-medium text-[var(--color-text-muted)] mb-2">持仓集中度</div>
                            <div className="flex gap-4 text-xs text-[var(--color-text-secondary)] mb-3">
                                <span>前3: {report.fund_analysis.holding_analysis.concentration.top3_ratio.toFixed(1)}%</span>
                                <span>前5: {report.fund_analysis.holding_analysis.concentration.top5_ratio.toFixed(1)}%</span>
                                <span>前10: {report.fund_analysis.holding_analysis.concentration.top10_ratio.toFixed(1)}%</span>
                                <Tag>{report.fund_analysis.holding_analysis.concentration.level}</Tag>
                            </div>
                            {report.fund_analysis.holding_analysis.analysis_text && (
                                <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed">{report.fund_analysis.holding_analysis.analysis_text}</p>
                            )}
                        </div>
                    )}
                </Section>
            )}

            {/* 调仓检测 */}
            {report.rebalance_result && report.rebalance_result.changes && report.rebalance_result.changes.length > 0 && (
                <Section
                    title="调仓检测"
                    icon={<RefreshCw className="w-4 h-4 text-blue-500" />}
                    defaultOpen
                >
                    <p className="text-sm text-[var(--color-text)] mb-3 leading-relaxed">{report.rebalance_result.summary}</p>
                    <div className="overflow-x-auto">
                        <table className="w-full text-xs">
                            <thead>
                                <tr className="border-b border-[var(--color-border)]">
                                    <th className="text-left py-2 text-[var(--color-text-muted)] font-medium">股票</th>
                                    <th className="text-center py-2 text-[var(--color-text-muted)] font-medium">操作</th>
                                    <th className="text-right py-2 text-[var(--color-text-muted)] font-medium">上期</th>
                                    <th className="text-right py-2 text-[var(--color-text-muted)] font-medium">本期</th>
                                    <th className="text-right py-2 text-[var(--color-text-muted)] font-medium">变动</th>
                                </tr>
                            </thead>
                            <tbody>
                                {report.rebalance_result.changes.map((c, i) => (
                                    <tr key={i} className="border-b border-[var(--color-border)] last:border-0">
                                        <td className="py-2">
                                            <div className="font-medium text-[var(--color-text)]">{c.stock_name}</div>
                                            <div className="text-[var(--color-text-muted)]">{c.stock_code}</div>
                                        </td>
                                        <td className="text-center py-2">
                                            <Tag variant={
                                                c.action === "新增" || c.action === "增持" ? "up"
                                                    : c.action === "减持" || c.action === "清仓" ? "down" : "default"
                                            }>
                                                {c.action}
                                            </Tag>
                                        </td>
                                        <td className="text-right py-2 text-[var(--color-text-secondary)]">{c.prev_ratio.toFixed(2)}%</td>
                                        <td className="text-right py-2 text-[var(--color-text-secondary)]">{c.curr_ratio.toFixed(2)}%</td>
                                        <td className={`text-right py-2 font-medium ${c.change_ratio > 0 ? "text-[var(--color-up)]" : c.change_ratio < 0 ? "text-[var(--color-down)]" : "text-[var(--color-text-secondary)]"}`}>
                                            {c.change_ratio > 0 ? "+" : ""}{c.change_ratio.toFixed(2)}%
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </Section>
            )}

            {/* 基金经理分析 */}
            {report.manager_report && (
                <Section
                    title={`基金经理: ${report.manager_report.manager_name}`}
                    icon={<User className="w-4 h-4 text-indigo-500" />}
                >
                    <div className="flex flex-wrap gap-2 mb-3">
                        <Tag>{`从业 ${report.manager_report.years} 年`}</Tag>
                        {report.manager_report.style && <Tag>{report.manager_report.style}</Tag>}
                    </div>
                    {report.manager_report.strengths.length > 0 && (
                        <div className="mb-2">
                            <span className="text-xs font-medium text-[var(--color-up)]">优势: </span>
                            <span className="text-xs text-[var(--color-text-secondary)]">{report.manager_report.strengths.join("、")}</span>
                        </div>
                    )}
                    {report.manager_report.weaknesses.length > 0 && (
                        <div className="mb-3">
                            <span className="text-xs font-medium text-[var(--color-down)]">劣势: </span>
                            <span className="text-xs text-[var(--color-text-secondary)]">{report.manager_report.weaknesses.join("、")}</span>
                        </div>
                    )}
                    {report.manager_report.analysis_text && (
                        <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed">{report.manager_report.analysis_text}</p>
                    )}
                </Section>
            )}

            {/* 宏观研判 */}
            {report.macro_report && (
                <Section
                    title="宏观研判"
                    icon={<Globe className="w-4 h-4 text-teal-500" />}
                >
                    <div className="flex items-center gap-2 mb-3">
                        <span className="text-xs text-[var(--color-text-muted)]">市场情绪:</span>
                        <Tag variant={
                            report.macro_report.market_sentiment === "乐观" ? "up"
                                : report.macro_report.market_sentiment === "悲观" ? "down" : "default"
                        }>
                            {report.macro_report.market_sentiment}
                        </Tag>
                    </div>
                    {report.macro_report.key_events.length > 0 && (
                        <div className="mb-3">
                            <div className="text-xs font-medium text-[var(--color-text-muted)] mb-1">关键事件</div>
                            <ul className="text-xs text-[var(--color-text-secondary)] space-y-1 ml-4 list-disc">
                                {report.macro_report.key_events.map((e, i) => <li key={i}>{e}</li>)}
                            </ul>
                        </div>
                    )}
                    {report.macro_report.impact && (
                        <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed mb-2">{report.macro_report.impact}</p>
                    )}
                    {report.macro_report.risk_factors.length > 0 && (
                        <div>
                            <div className="text-xs font-medium text-[var(--color-warn)] mb-1">风险因素</div>
                            <ul className="text-xs text-[var(--color-text-secondary)] space-y-1 ml-4 list-disc">
                                {report.macro_report.risk_factors.map((r, i) => <li key={i}>{r}</li>)}
                            </ul>
                        </div>
                    )}
                </Section>
            )}

            {/* 多空辩论 */}
            {report.debate_result && (
                <Section
                    title="多空辩论"
                    icon={<Scale className="w-4 h-4 text-[var(--color-primary)]" />}
                    defaultOpen
                >
                    <DebateTimeline result={report.debate_result} />
                </Section>
            )}
        </div>
    )
}
