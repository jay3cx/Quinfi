import { useState, useEffect } from "react"
import { SentimentBadge } from "@/components/SentimentBadge"
import { getNews, getBriefs, type Brief } from "@/lib/api"
import type { Article } from "@/types"
import { ExternalLink, FileText } from "lucide-react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeRaw from "rehype-raw"

type SentimentFilter = "all" | "positive" | "negative" | "neutral"

const FILTERS: { key: SentimentFilter; label: string }[] = [
    { key: "all", label: "全部" },
    { key: "positive", label: "利好" },
    { key: "negative", label: "利空" },
    { key: "neutral", label: "中性" },
]

export default function MarketPage() {
    const [articles, setArticles] = useState<Article[]>([])
    const [total, setTotal] = useState(0)
    const [filter, setFilter] = useState<SentimentFilter>("all")
    const [loading, setLoading] = useState(true)
    const [offset, setOffset] = useState(0)
    const [briefs, setBriefs] = useState<Brief[]>([])
    const limit = 20

    // 加载简报
    useEffect(() => {
        getBriefs().then((res) => setBriefs(res.data || [])).catch((err) => {
            console.error("[MarketPage] Failed to load briefs:", err)
        })
    }, [])

    useEffect(() => {
        setLoading(true)
        setOffset(0)
        getNews({
            limit,
            offset: 0,
            sentiment: filter === "all" ? undefined : filter,
        })
            .then((res) => {
                setArticles(res.data || [])
                setTotal(res.total)
            })
            .catch((err) => {
                console.error("[MarketPage] Failed to load news:", err)
                setArticles([])
            })
            .finally(() => setLoading(false))
    }, [filter])

    const loadMore = () => {
        const newOffset = offset + limit
        getNews({
            limit,
            offset: newOffset,
            sentiment: filter === "all" ? undefined : filter,
        }).then((res) => {
            setArticles((prev) => [...prev, ...(res.data || [])])
            setOffset(newOffset)
        }).catch((err) => {
            console.error("[MarketPage] Failed to load more news:", err)
        })
    }

    const formatDate = (dateStr: string) => {
        try {
            const d = new Date(dateStr)
            return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}`
        } catch {
            return dateStr
        }
    }

    return (
        <div className="flex-1 overflow-y-auto">
                <div className="max-w-4xl mx-auto px-6 py-8">
                    <h1 className="text-xl font-semibold text-[var(--color-text)] mb-6">Market Overview</h1>

                    {/* 每日简报 */}
                    {briefs.length > 0 && (
                        <div className="bg-white rounded-xl border border-[var(--color-border)] p-6 mb-6">
                            <div className="flex items-center gap-2 mb-3">
                                <FileText className="w-4 h-4 text-[var(--color-primary)]" />
                                <span className="text-sm font-medium text-[var(--color-text)]">每日投资简报</span>
                                <span className="text-xs text-[var(--color-text-muted)]">{formatDate(briefs[0].created_at)}</span>
                            </div>
                            <div className="text-sm text-[var(--color-text)] leading-relaxed space-y-3">
                                <ReactMarkdown
                                    remarkPlugins={[remarkGfm]}
                                    rehypePlugins={[rehypeRaw]}
                                    components={{
                                        h1: ({ node, ...props }) => <h1 className="text-lg font-semibold mb-3" {...props} />,
                                        h2: ({ node, ...props }) => <h2 className="text-base font-semibold mb-2 mt-4" {...props} />,
                                        h3: ({ node, ...props }) => <h3 className="text-sm font-semibold mb-1 mt-3" {...props} />,
                                        p: ({ node, ...props }) => <p className="mb-2 last:mb-0" {...props} />,
                                        strong: ({ node, ...props }) => <strong className="font-semibold" {...props} />,
                                        ul: ({ node, ...props }) => <ul className="list-disc pl-5 mb-2 space-y-1" {...props} />,
                                        ol: ({ node, ...props }) => <ol className="list-decimal pl-5 mb-2 space-y-1" {...props} />,
                                        table: ({ node, ...props }) => <div className="overflow-x-auto mb-3"><table className="w-full text-sm border-collapse" {...props} /></div>,
                                        th: ({ node, ...props }) => <th className="border border-[var(--color-border)] px-3 py-2 bg-[var(--color-sidebar-bg)] text-left font-medium text-xs" {...props} />,
                                        td: ({ node, ...props }) => <td className="border border-[var(--color-border)] px-3 py-2 text-xs" {...props} />,
                                    }}
                                >
                                    {briefs[0].content}
                                </ReactMarkdown>
                            </div>
                        </div>
                    )}

                    {/* Filter */}
                    <div className="flex gap-2 mb-6">
                        {FILTERS.map((f) => (
                            <button
                                key={f.key}
                                onClick={() => setFilter(f.key)}
                                className={`px-4 py-1.5 text-sm rounded-full transition-colors ${filter === f.key
                                    ? "bg-[var(--color-primary-bg)] text-[var(--color-primary)] font-medium"
                                    : "text-[var(--color-text-secondary)] hover:bg-[var(--color-sidebar-bg)]"
                                    }`}
                            >
                                {f.label}
                            </button>
                        ))}
                        <span className="text-xs text-[var(--color-text-muted)] self-center ml-2">
                            共 {total} 条
                        </span>
                    </div>

                    {/* News List */}
                    {loading ? (
                        <div className="text-center py-20 text-[var(--color-text-muted)]">加载中...</div>
                    ) : articles.length === 0 ? (
                        <div className="text-center py-20 text-[var(--color-text-muted)]">
                            暂无新闻资讯
                            <div className="text-xs mt-2">需要先启动 RSS 抓取任务</div>
                        </div>
                    ) : (
                        <div className="space-y-3">
                            {articles.map((article) => (
                                <div
                                    key={article.guid}
                                    className="bg-white rounded-lg border border-[var(--color-border)] p-5 hover:shadow-sm transition-shadow"
                                >
                                    <div className="flex items-start justify-between gap-4">
                                        <div className="flex-1 min-w-0">
                                            <div className="flex items-center gap-2 mb-1.5">
                                                {article.sentiment && (
                                                    <SentimentBadge sentiment={article.sentiment} />
                                                )}
                                                <span className="text-xs text-[var(--color-text-muted)]">
                                                    {article.source || "RSS"}
                                                </span>
                                                <span className="text-xs text-[var(--color-text-muted)]">
                                                    {formatDate(article.pub_date)}
                                                </span>
                                            </div>

                                            <h3 className="text-sm font-medium text-[var(--color-text)] mb-1.5 leading-snug">
                                                {article.title}
                                            </h3>

                                            {article.summary && (
                                                <p className="text-sm text-[var(--color-text-secondary)] line-clamp-2">
                                                    {article.summary}
                                                </p>
                                            )}

                                            {article.keywords && article.keywords.length > 0 && (
                                                <div className="flex gap-1.5 mt-2">
                                                    {article.keywords.map((kw) => (
                                                        <span key={kw} className="text-xs px-2 py-0.5 rounded bg-[var(--color-sidebar-bg)] text-[var(--color-text-muted)]">
                                                            {kw}
                                                        </span>
                                                    ))}
                                                </div>
                                            )}
                                        </div>

                                        {article.link && (
                                            <a
                                                href={article.link}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="text-[var(--color-text-muted)] hover:text-[var(--color-primary)] transition-colors shrink-0"
                                            >
                                                <ExternalLink className="w-4 h-4" />
                                            </a>
                                        )}
                                    </div>
                                </div>
                            ))}

                            {articles.length < total && (
                                <button
                                    onClick={loadMore}
                                    className="w-full py-3 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-primary)] transition-colors"
                                >
                                    加载更多
                                </button>
                            )}
                        </div>
                    )}
                </div>
        </div>
    )
}
