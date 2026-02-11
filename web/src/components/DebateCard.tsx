import { TrendingUp, TrendingDown, Scale } from "lucide-react"
import ReactMarkdown from "react-markdown"

interface DebateCardProps {
    content: string // 包含辩论结果的 Markdown 文本
}

export function DebateCard({ content }: DebateCardProps) {
    // 解析各部分
    const bullMatch = content.match(/###\s*.*看多方观点.*\n([\s\S]*?)(?=###|$)/)
    const bearMatch = content.match(/###\s*.*看空方观点.*\n([\s\S]*?)(?=###|$)/)
    const judgeMatch = content.match(/###\s*.*裁判结论.*\n([\s\S]*?)(?=###|$)/)

    if (!bullMatch && !bearMatch && !judgeMatch) return null

    return (
        <div className="space-y-3 my-4">
            {bullMatch && (
                <div className="rounded-lg border border-green-200 bg-green-50/50 p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <TrendingUp className="w-4 h-4 text-[var(--color-up)]" />
                        <span className="text-sm font-semibold text-[var(--color-up)]">看多方</span>
                    </div>
                    <div className="text-sm text-[var(--color-text)] leading-relaxed">
                        <ReactMarkdown>{bullMatch[1].trim()}</ReactMarkdown>
                    </div>
                </div>
            )}

            {bearMatch && (
                <div className="rounded-lg border border-red-200 bg-red-50/50 p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <TrendingDown className="w-4 h-4 text-[var(--color-down)]" />
                        <span className="text-sm font-semibold text-[var(--color-down)]">看空方</span>
                    </div>
                    <div className="text-sm text-[var(--color-text)] leading-relaxed">
                        <ReactMarkdown>{bearMatch[1].trim()}</ReactMarkdown>
                    </div>
                </div>
            )}

            {judgeMatch && (
                <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-sidebar-bg)]/50 p-4">
                    <div className="flex items-center gap-2 mb-2">
                        <Scale className="w-4 h-4 text-[var(--color-primary)]" />
                        <span className="text-sm font-semibold text-[var(--color-primary)]">裁判结论</span>
                    </div>
                    <div className="text-sm text-[var(--color-text)] leading-relaxed">
                        <ReactMarkdown>{judgeMatch[1].trim()}</ReactMarkdown>
                    </div>
                </div>
            )}
        </div>
    )
}

// 检测内容是否包含辩论结果
export function isDebateContent(content: string): boolean {
    return content.includes("多空辩论结果") || (content.includes("看多方观点") && content.includes("看空方观点"))
}
