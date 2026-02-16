import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import rehypeRaw from "rehype-raw"
import { useNavigate } from "react-router-dom"
import { DebateCard, isDebateContent } from "@/components/DebateCard"

// 把文本中的6位基金代码变成可点击链接
function FundCodeLinker({ children, navigate }: { children: string; navigate: (path: string) => void }) {
    const parts = children.split(/(\b\d{6}\b)/g)
    return (
        <>
            {parts.map((part, i) =>
                /^\d{6}$/.test(part) ? (
                    <span
                        key={i}
                        className="text-[var(--color-primary)] cursor-pointer hover:underline font-medium"
                        onClick={() => navigate(`/fund/${part}`)}
                    >
                        {part}
                    </span>
                ) : (
                    <span key={i}>{part}</span>
                )
            )}
        </>
    )
}

interface Message {
    role: "user" | "assistant"
    content: string
    children?: React.ReactNode
}

export function MessageBubble({ role, content, children }: Message) {
    const isUser = role === "user"
    const navigate = useNavigate()

    return (
        <div className={`flex mb-8 ${isUser ? "justify-end" : ""}`}>
            <div className={`flex flex-col max-w-[650px] ${isUser ? "items-end" : "items-start"}`}>
                {isUser ? (
                    <div className="bg-[var(--color-sidebar-bg)] px-4 py-3 rounded-2xl rounded-tr-sm text-[var(--color-text)] leading-relaxed">
                        {content}
                    </div>
                ) : (
                    <div className="text-[var(--color-text)] leading-relaxed space-y-3 w-full">
                        <ReactMarkdown
                            remarkPlugins={[remarkGfm]}
                            rehypePlugins={[rehypeRaw]}
                            components={{
                                h1: ({ node, ...props }) => <h1 className="text-xl font-semibold mb-3" {...props} />,
                                h2: ({ node, ...props }) => <h2 className="text-lg font-semibold mb-2" {...props} />,
                                h3: ({ node, ...props }) => <h3 className="text-base font-semibold mb-1" {...props} />,
                                p: ({ node, children, ...props }) => (
                                    <p className="mb-3 last:mb-0" {...props}>
                                        {Array.isArray(children) ? children.map((child, i) =>
                                            typeof child === "string" ? <FundCodeLinker key={i} navigate={navigate}>{child}</FundCodeLinker> : child
                                        ) : typeof children === "string" ? <FundCodeLinker navigate={navigate}>{children}</FundCodeLinker> : children}
                                    </p>
                                ),
                                strong: ({ node, ...props }) => <strong className="font-semibold" {...props} />,
                                code: ({ node, ...props }) => <code className="bg-[var(--color-sidebar-bg)] px-1.5 py-0.5 rounded text-sm font-mono" {...props} />,
                                ul: ({ node, ...props }) => <ul className="list-disc pl-5 mb-3 space-y-1" {...props} />,
                                ol: ({ node, ...props }) => <ol className="list-decimal pl-5 mb-3 space-y-1" {...props} />,
                                table: ({ node, ...props }) => <div className="overflow-x-auto mb-3"><table className="w-full text-sm border-collapse" {...props} /></div>,
                                th: ({ node, ...props }) => <th className="border border-[var(--color-border)] px-3 py-2 bg-[var(--color-sidebar-bg)] text-left font-medium text-sm" {...props} />,
                                td: ({ node, ...props }) => <td className="border border-[var(--color-border)] px-3 py-2 text-sm" {...props} />,
                            }}
                        >
                            {content}
                        </ReactMarkdown>

                        {isDebateContent(content) && <DebateCard content={content} />}
                        {children && <div className="mt-4">{children}</div>}
                    </div>
                )}
            </div>
        </div>
    )
}
