interface SentimentBadgeProps {
    sentiment: "positive" | "negative" | "neutral" | string
}

const config: Record<string, { label: string; className: string }> = {
    positive: { label: "利好", className: "bg-green-50 text-[var(--color-up)]" },
    negative: { label: "利空", className: "bg-red-50 text-[var(--color-down)]" },
    neutral: { label: "中性", className: "bg-gray-50 text-[var(--color-text-muted)]" },
}

export function SentimentBadge({ sentiment }: SentimentBadgeProps) {
    const c = config[sentiment] || config.neutral
    return (
        <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${c.className}`}>
            {c.label}
        </span>
    )
}
