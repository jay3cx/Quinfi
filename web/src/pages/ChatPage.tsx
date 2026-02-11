import { useState, useRef, useEffect } from "react"
import { useSearchParams } from "react-router-dom"
import { MessageBubble } from "@/components/MessageBubble"
import { ToolCallCard } from "@/components/ToolCallCard"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Send, Sparkles, ImagePlus, X } from "lucide-react"
import { useChat } from "@/hooks/useChat"
import { getSessionMessages } from "@/lib/api"
import { useSessionStore } from "@/stores/sessionStore"

interface ToolCall {
    toolName: string
    status: "loading" | "done"
}

interface Message {
    id: string
    role: "user" | "assistant"
    content: string
    toolCalls?: ToolCall[]
    images?: string[] // 用户发送的图片
}

interface SessionData {
    messages: Message[]
    sessionId?: string
}

const WELCOME_MSG: Message = {
    id: "welcome",
    role: "assistant",
    content: "你好，我是**小基** — 你的首席投研官。\n\n我可以帮你分析基金、扫描市场、管理组合、发起多空辩论。告诉我你想了解什么？",
}

const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB
const MAX_IMAGES = 3

export default function ChatPage() {
    const activeSessionId = useSessionStore((s) => s.activeSessionId)
    const newChat = useSessionStore((s) => s.newChat)
    const addSession = useSessionStore((s) => s.addSession)
    const [searchParams, setSearchParams] = useSearchParams()

    const [sessionDataMap, setSessionDataMap] = useState<Record<string, SessionData>>({})
    const [input, setInput] = useState("")
    const [isLoading, setIsLoading] = useState(false)
    const [attachedImages, setAttachedImages] = useState<string[]>([])
    const scrollRef = useRef<HTMLDivElement>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const pendingQueryRef = useRef<string | null>(null)
    const { sendMessage, status } = useChat()

    // 处理 URL ?q= 参数：先新建对话，写入输入框，标记待发送
    useEffect(() => {
        const q = searchParams.get("q")
        if (q) {
            setSearchParams({}, { replace: true }) // 清掉 URL 参数
            newChat()
            setInput(q)
            pendingQueryRef.current = q
        }
    }, []) // eslint-disable-line react-hooks/exhaustive-deps

    const currentData = sessionDataMap[activeSessionId] || { messages: [WELCOME_MSG] }
    const messages = currentData.messages

    useEffect(() => {
        if (scrollRef.current) {
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight
        }
    }, [messages])

    // ?q= 参数：等新对话 session 就绪后自动发送
    useEffect(() => {
        if (pendingQueryRef.current && activeSessionId.startsWith("new") && !isLoading) {
            const q = pendingQueryRef.current
            pendingQueryRef.current = null
            // 延迟一帧确保 state 稳定
            requestAnimationFrame(() => {
                setInput("")
                handleSendWithText(q)
            })
        }
    }, [activeSessionId]) // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        if (!activeSessionId || activeSessionId.startsWith("new")) return
        if (sessionDataMap[activeSessionId]?.messages?.length) return

        getSessionMessages(activeSessionId).then((res) => {
            if (res.messages) {
                const msgs: Message[] = res.messages.map((m, i) => {
                    const msg: Message = {
                        id: `${activeSessionId}-${i}`,
                        role: m.role as "user" | "assistant",
                        content: m.content,
                    }
                    if (m.metadata) {
                        try {
                            const tools = JSON.parse(m.metadata) as { tool: string; type: string }[]
                            if (Array.isArray(tools) && tools.length > 0) {
                                const seen = new Set<string>()
                                msg.toolCalls = tools
                                    .filter((t) => t.type === "tool_result" && !seen.has(t.tool) && (seen.add(t.tool), true))
                                    .map((t) => ({ toolName: t.tool, status: "done" as const }))
                            }
                        } catch { /* ignore malformed metadata */ }
                    }
                    return msg
                })
                setSessionDataMap((p) => ({ ...p, [activeSessionId]: { messages: msgs, sessionId: activeSessionId } }))
            }
        }).catch((err) => {
            console.error("[ChatPage] Failed to load session messages:", err)
        })
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [activeSessionId])

    const addImageFile = (file: File) => {
        if (file.size > MAX_IMAGE_SIZE) {
            alert("图片过大，请选择 5MB 以内的图片")
            return
        }
        if (attachedImages.length >= MAX_IMAGES) {
            alert(`最多附加 ${MAX_IMAGES} 张图片`)
            return
        }
        const reader = new FileReader()
        reader.onload = () => {
            setAttachedImages((prev) => [...prev, reader.result as string])
        }
        reader.readAsDataURL(file)
    }

    const handleImageAttach = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0]
        if (!file) return
        e.target.value = ""
        addImageFile(file)
    }

    const handlePaste = (e: React.ClipboardEvent) => {
        const items = e.clipboardData?.items
        if (!items) return
        for (const item of items) {
            if (item.type.startsWith("image/")) {
                e.preventDefault()
                const file = item.getAsFile()
                if (file) addImageFile(file)
                return
            }
        }
    }

    const removeImage = (index: number) => {
        setAttachedImages((prev) => prev.filter((_, i) => i !== index))
    }

    const handleSendWithText = (text: string, images?: string[]) => {
        if ((!text.trim() && !(images?.length) && !attachedImages.length) || isLoading) return

        const msgText = text || ""
        const msgImages = images || (attachedImages.length > 0 ? [...attachedImages] : undefined)

        const userMsg: Message = {
            id: Date.now().toString(),
            role: "user",
            content: msgText,
            images: msgImages,
        }
        const aiMsgId = (Date.now() + 1).toString()
        const aiMsg: Message = { id: aiMsgId, role: "assistant", content: "", toolCalls: [] }
        const originalInput = msgText || "（发送了图片）"
        const imagesToSend = msgImages

        setSessionDataMap((prev) => ({
            ...prev,
            [activeSessionId]: {
                ...prev[activeSessionId],
                messages: [...(prev[activeSessionId]?.messages || [WELCOME_MSG]), userMsg, aiMsg],
            },
        }))
        setInput("")
        setAttachedImages([])
        setIsLoading(true)

        sendMessage(originalInput, currentData.sessionId, {
            onChunk: (chunk) => {
                setSessionDataMap((prev) => ({
                    ...prev,
                    [activeSessionId]: {
                        ...prev[activeSessionId],
                        messages: prev[activeSessionId].messages.map((msg) =>
                            msg.id === aiMsgId ? { ...msg, content: msg.content + chunk } : msg
                        ),
                    },
                }))
            },
            onToolStart: (toolName) => {
                setSessionDataMap((prev) => ({
                    ...prev,
                    [activeSessionId]: {
                        ...prev[activeSessionId],
                        messages: prev[activeSessionId].messages.map((msg) =>
                            msg.id === aiMsgId
                                ? { ...msg, toolCalls: [...(msg.toolCalls || []), { toolName, status: "loading" as const }] }
                                : msg
                        ),
                    },
                }))
            },
            onToolResult: (toolName) => {
                setSessionDataMap((prev) => ({
                    ...prev,
                    [activeSessionId]: {
                        ...prev[activeSessionId],
                        messages: prev[activeSessionId].messages.map((msg) =>
                            msg.id === aiMsgId
                                ? {
                                    ...msg,
                                    toolCalls: (msg.toolCalls || []).map((tc) =>
                                        tc.toolName === toolName ? { ...tc, status: "done" as const } : tc
                                    ),
                                }
                                : msg
                        ),
                    },
                }))
            },
            onSessionId: (id) => {
                setSessionDataMap((prev) => ({
                    ...prev,
                    [activeSessionId]: { ...prev[activeSessionId], sessionId: id },
                }))
                const title = (input || "图片对话").slice(0, 20) + (input.length > 20 ? "..." : "")
                addSession({ id: activeSessionId, title, createdAt: new Date().toISOString() })
            },
            onDone: () => setIsLoading(false),
            onError: (err) => {
                setSessionDataMap((prev) => ({
                    ...prev,
                    [activeSessionId]: {
                        ...prev[activeSessionId],
                        messages: prev[activeSessionId].messages.map((msg) =>
                            msg.id === aiMsgId ? { ...msg, content: msg.content + `\n\n*Error: ${err}*` } : msg
                        ),
                    },
                }))
                setIsLoading(false)
            },
        }, imagesToSend)
    }

    const handleSend = () => {
        handleSendWithText(input)
    }

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault()
            handleSend()
        }
    }

    const hasConversation = messages.length > 1 || (messages.length === 1 && messages[0].id !== "welcome")

    // 图片预览条组件
    const ImagePreviewBar = () => {
        if (attachedImages.length === 0) return null
        return (
            <div className="flex gap-2 px-1 pb-2">
                {attachedImages.map((img, i) => (
                    <div key={i} className="relative group">
                        <img
                            src={img}
                            alt={`附件 ${i + 1}`}
                            className="w-14 h-14 rounded-lg object-cover border border-[var(--color-border)]"
                        />
                        <button
                            onClick={() => removeImage(i)}
                            className="absolute -top-1.5 -right-1.5 w-5 h-5 rounded-full bg-[var(--color-text)] text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
                        >
                            <X className="w-3 h-3" />
                        </button>
                    </div>
                ))}
            </div>
        )
    }

    // 渲染输入区域（用函数调用，不用 JSX 组件，避免每次渲染创建新组件导致 Input 失焦）
    const renderInputArea = (variant: "welcome" | "chat") => {
        const isWelcome = variant === "welcome"
        return (
            <div className={`bg-white ${isWelcome ? "rounded-2xl shadow-sm" : "rounded-xl shadow-sm"} border border-[var(--color-border)] p-2 pl-4 focus-within:ring-2 focus-within:ring-[var(--color-primary)]/15 transition-all`}>
                <ImagePreviewBar />
                <div className="flex items-center gap-2">
                    <button
                        onClick={() => fileInputRef.current?.click()}
                        className="shrink-0 p-1.5 rounded-md text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-sidebar-bg)] transition-colors"
                        title="添加图片"
                    >
                        <ImagePlus className="w-4.5 h-4.5" />
                    </button>
                    <Input
                        value={input}
                        onChange={(e) => setInput(e.target.value)}
                        onKeyDown={handleKeyDown}
                        onPaste={handlePaste}
                        placeholder={isWelcome ? "输入基金代码、投资问题，或让小基帮你扫描市场..." : "继续对话..."}
                        className="border-none shadow-none text-base py-3 h-auto font-sans text-[var(--color-text)] placeholder:text-[var(--color-text-muted)]"
                        variant="ghost"
                        autoFocus
                    />
                    <Button
                        size="icon"
                        onClick={handleSend}
                        disabled={isLoading || (!input.trim() && attachedImages.length === 0)}
                        className={`${isWelcome ? "rounded-xl h-10 w-10" : "rounded-lg h-9 w-9"} shrink-0 bg-[var(--color-primary)] hover:bg-[var(--color-primary-light)] text-white`}
                    >
                        <Send className="w-4 h-4" />
                    </Button>
                </div>
            </div>
        )
    }

    return (
        <>
            <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleImageAttach}
            />

            {!hasConversation ? (
                <div className="flex-1 flex flex-col items-center justify-center px-4">
                    <div className="mb-8 text-center">
                        <h1 className="text-2xl font-serif font-medium text-[var(--color-text)] mb-1">
                            Hi, 有什么可以帮你的？
                        </h1>
                    </div>
                    <div className="w-full max-w-2xl">
                        {renderInputArea("welcome")}
                    </div>
                </div>
            ) : (
                <>
                    <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 pt-8 pb-32 scroll-smooth">
                        <div className="max-w-3xl mx-auto">
                            {messages.filter(m => m.id !== "welcome").map((msg) => (
                                <div key={msg.id}>
                                    {/* 用户发送的图片 */}
                                    {msg.role === "user" && msg.images && msg.images.length > 0 && (
                                        <div className="flex gap-4 mb-2 flex-row-reverse">
                                            <div className="w-7 shrink-0" />
                                            <div className="flex gap-2 flex-wrap justify-end">
                                                {msg.images.map((img, i) => (
                                                    <img
                                                        key={i}
                                                        src={img}
                                                        alt={`附件 ${i + 1}`}
                                                        className="max-w-[200px] max-h-[200px] rounded-lg border border-[var(--color-border)] object-cover"
                                                    />
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                    {msg.role === "assistant" && msg.toolCalls && msg.toolCalls.length > 0 && (
                                        <div className="flex gap-4 mb-3">
                                            <div className="w-7 shrink-0" />
                                            <div className="space-y-1.5">
                                                {msg.toolCalls.map((tc, i) => (
                                                    <ToolCallCard key={`${tc.toolName}-${i}`} toolName={tc.toolName} status={tc.status} />
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                    <MessageBubble role={msg.role} content={msg.content} />
                                </div>
                            ))}

                            {isLoading && messages[messages.length - 1]?.content === "" && (
                                <div className="flex gap-4 mb-8">
                                    <div className="w-7 h-7 rounded-full bg-[var(--color-primary)] flex items-center justify-center shrink-0 text-white">
                                        <Sparkles className="w-3.5 h-3.5 animate-pulse" />
                                    </div>
                                    <div className="flex items-center">
                                        <span className="text-[var(--color-text-muted)] text-sm animate-pulse">
                                            {status === "connecting" ? "连接中..." : "思考中..."}
                                        </span>
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>

                    <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-[var(--color-bg)] via-[var(--color-bg)] to-transparent pt-10 pb-8 px-4">
                        <div className="max-w-3xl mx-auto">
                            {renderInputArea("chat")}
                        </div>
                    </div>
                </>
            )}
        </>
    )
}
