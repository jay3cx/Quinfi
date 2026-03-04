import { useState, useRef, useEffect } from "react"
import { useSearchParams } from "react-router-dom"
import { MessageBubble } from "@/components/MessageBubble"
import { ToolCallCard } from "@/components/ToolCallCard"
import { DebateTimeline } from "@/components/DebateTimeline"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Send, ImagePlus, X } from "lucide-react"
import { useChat } from "@/hooks/useChat"
import { getSessionMessages } from "@/lib/api"
import { useSessionStore } from "@/stores/sessionStore"
import type { DebatePhaseKey, DebatePhaseUpdate } from "@/types"

interface ToolCall {
    toolName: string
    status: "loading" | "done"
}

interface Message {
    id: string
    role: "user" | "assistant"
    content: string
    toolCalls?: ToolCall[]
    debatePhases?: DebatePhaseUpdate[]
    debateActive?: DebatePhaseKey[] | null
    systemConfidence?: number
    decisionGate?: string
    images?: string[] // 用户发送的图片
}

interface SessionData {
    messages: Message[]
    sessionId?: string
}

const WELCOME_MSG: Message = {
    id: "welcome",
    role: "assistant",
    content: "你好，我是**Quinfi** — 你的首席投研官。\n\n我可以帮你分析基金、扫描市场、管理组合、发起多空辩论。告诉我你想了解什么？",
}

const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB
const MAX_IMAGES = 3

function computeDebateActive(completed: Set<DebatePhaseKey>): DebatePhaseKey[] {
    if (!completed.has("bull_case") || !completed.has("bear_case")) {
        const active: DebatePhaseKey[] = []
        if (!completed.has("bull_case")) active.push("bull_case")
        if (!completed.has("bear_case")) active.push("bear_case")
        return active
    }
    if (!completed.has("bull_rebuttal") || !completed.has("bear_rebuttal")) {
        const active: DebatePhaseKey[] = []
        if (!completed.has("bull_rebuttal")) active.push("bull_rebuttal")
        if (!completed.has("bear_rebuttal")) active.push("bear_rebuttal")
        return active
    }
    if (!completed.has("judge_verdict")) return ["judge_verdict"]
    return []
}

export default function ChatPage() {
    const activeSessionId = useSessionStore((s) => s.activeSessionId)
    const setActiveSessionId = useSessionStore((s) => s.setActiveSessionId)
    const newChat = useSessionStore((s) => s.newChat)
    const addSession = useSessionStore((s) => s.addSession)
    const [searchParams, setSearchParams] = useSearchParams()

    const [sessionDataMap, setSessionDataMap] = useState<Record<string, SessionData>>({})
    const [input, setInput] = useState("")
    const [isLoading, setIsLoading] = useState(false)
    const [thinkingText, setThinkingText] = useState("")
    const [attachedImages, setAttachedImages] = useState<string[]>([])
    const scrollRef = useRef<HTMLDivElement>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const pendingQueryRef = useRef<string | null>(null)
    // sessionKeyRef 跟踪当前流式回调应写入 sessionDataMap 的 key
    // 当 onSessionId 收到后端 UUID 后会迁移 key，后续回调通过 ref 获取新值
    const sessionKeyRef = useRef(activeSessionId)
    const pollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const recoveryTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
    const recoveryTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
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
    const hasConversation = messages.length > 1 || (messages.length === 1 && messages[0].id !== "welcome")

    // 只在用户已经在底部附近时才自动滚动（避免轮询更新打断阅读）
    // programmaticScroll 标记：区分程序化滚动和用户手动滚动，防止竞态
    const isNearBottomRef = useRef(true)
    const programmaticScrollRef = useRef(false)
    // 滚动容器按 hasConversation 条件渲染；依赖它可确保容器出现后绑定监听
    useEffect(() => {
        const el = scrollRef.current
        if (!el) return
        const onScroll = () => {
            // 程序化滚动（含 scroll-smooth 动画期间）不更新标记
            if (programmaticScrollRef.current) return
            isNearBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100
        }
        el.addEventListener("scroll", onScroll, { passive: true })
        return () => el.removeEventListener("scroll", onScroll)
    }, [hasConversation])

    useEffect(() => {
        if (isNearBottomRef.current && scrollRef.current) {
            programmaticScrollRef.current = true
            scrollRef.current.scrollTop = scrollRef.current.scrollHeight
            // scroll-smooth 动画结束后解除标记（动画通常 < 300ms）
            setTimeout(() => { programmaticScrollRef.current = false }, 300)
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

    // 将 API 消息转为前端 Message 的辅助函数
    const parseSessionMessages = (sessionId: string, rawMessages: { role: string; content: string; metadata?: string }[]): Message[] => {
        return rawMessages.map((m, i) => {
            const msg: Message = {
                id: `${sessionId}-${i}`,
                role: m.role as "user" | "assistant",
                content: m.content,
            }
            if (m.metadata) {
                try {
                    const parsed = JSON.parse(m.metadata)

                    // 兼容旧格式（扁平数组 [{tool, type}]）和新格式（{tools: [...], debate_phases: [...]})
                    const toolList: { tool: string; type: string }[] = Array.isArray(parsed)
                        ? parsed
                        : Array.isArray(parsed.tools) ? parsed.tools : []

                    if (toolList.length > 0) {
                        const seen = new Set<string>()
                        msg.toolCalls = toolList
                            .filter((t) => t.type === "tool_result" && !seen.has(t.tool) && (seen.add(t.tool), true))
                            .map((t) => ({ toolName: t.tool, status: "done" as const }))

                        // 历史恢复时，若 run_debate 已返回结果，标记为终态，避免错误显示“进行中”
                        const hasDebateResult = toolList.some((t) => t.tool === "run_debate" && t.type === "tool_result")
                        if (hasDebateResult) {
                            msg.debateActive = null
                        }
                    }

                    // 恢复辩论阶段数据
                    if (!Array.isArray(parsed) && Array.isArray(parsed.debate_phases) && parsed.debate_phases.length > 0) {
                        msg.debatePhases = parsed.debate_phases as DebatePhaseUpdate[]
                        // 从 confidence_gate 条目中提取系统置信度
                        const gate = (parsed.debate_phases as DebatePhaseUpdate[]).find(
                            (p) => p.phase === "confidence_gate"
                        )
                        if (gate) {
                            msg.systemConfidence = gate.system_confidence
                            msg.decisionGate = gate.decision_gate
                        }
                    }
                } catch { /* ignore malformed metadata */ }
            }
            return msg
        })
    }

    const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

    useEffect(() => {
        // 清理上一次轮询
        if (pollTimerRef.current) {
            clearInterval(pollTimerRef.current)
            pollTimerRef.current = null
        }

        if (!activeSessionId || activeSessionId.startsWith("new")) return
        if (sessionDataMap[activeSessionId]?.messages?.length) return

        const sid = activeSessionId

        getSessionMessages(sid).then((res) => {
            if (!res.messages?.length) return

            const msgs = parseSessionMessages(sid, res.messages)
            setSessionDataMap((p) => ({ ...p, [sid]: { messages: msgs, sessionId: sid } }))

            // 检测是否需要轮询（后端可能还在流式处理中）
            const lastMsg = res.messages[res.messages.length - 1]
            const isStreaming = (() => {
                // 最后一条是 user：后端还没保存任何 assistant 中间状态
                if (lastMsg?.role === "user") return true
                // assistant content 为空：辩论/工具进行中，只有 metadata
                if (lastMsg?.role === "assistant" && !lastMsg.content) return true
                // assistant 有 content 但工具仍在执行（有 tool_start 但没有 tool_result）
                if (lastMsg?.role === "assistant" && lastMsg.metadata) {
                    try {
                        const meta = JSON.parse(lastMsg.metadata)
                        const tools: { type: string }[] = Array.isArray(meta) ? meta : (meta.tools || [])
                        const startCount = tools.filter((t) => t.type === "tool_start").length
                        const resultCount = tools.filter((t) => t.type === "tool_result").length
                        if (startCount > resultCount) return true
                    } catch { /* ignore */ }
                }
                if (lastMsg?.role === "assistant") return true
                return false
            })()

            if (isStreaming) {
                if (lastMsg?.role === "user") {
                    // 插入空占位消息，触发 loading 指示器
                    const pendingAiMsg: Message = { id: `${sid}-pending`, role: "assistant", content: "" }
                    setSessionDataMap((p) => ({ ...p, [sid]: { messages: [...msgs, pendingAiMsg], sessionId: sid } }))
                }
                // else: assistant 消息已存在（有 metadata），msgs 已包含解析后的 toolCalls/debatePhases
                setIsLoading(true)

                // 轮询检测中间状态更新
                let lastSnapshot = ""
                let stableCount = 0

                pollTimerRef.current = setInterval(async () => {
                    try {
                        const updated = await getSessionMessages(sid)
                        if (!updated.messages?.length) return

                        const lastUpdated = updated.messages[updated.messages.length - 1]

                        if (lastUpdated?.role === "assistant") {
                            const newMsgs = parseSessionMessages(sid, updated.messages)
                            setSessionDataMap((p) => ({ ...p, [sid]: { messages: newMsgs, sessionId: sid } }))

                            // 用 content + metadata 联合判断稳定性，兼容 metadata-only 完成态。
                            const snapshot = (lastUpdated.content || "") + (lastUpdated.metadata || "")
                            if (snapshot === "") {
                                stableCount = 0
                                return
                            }
                            if (snapshot === lastSnapshot) {
                                stableCount++
                                if (stableCount >= 3) {
                                    clearInterval(pollTimerRef.current!)
                                    pollTimerRef.current = null
                                    setIsLoading(false)
                                }
                            } else {
                                lastSnapshot = snapshot
                                stableCount = 0
                            }
                        }
                    } catch {
                        clearInterval(pollTimerRef.current!)
                        pollTimerRef.current = null
                        setIsLoading(false)
                    }
                }, 3000)

                // 最多轮询 5 分钟
                pollTimeoutRef.current = setTimeout(() => {
                    if (pollTimerRef.current) {
                        clearInterval(pollTimerRef.current)
                        pollTimerRef.current = null
                        setIsLoading(false)
                    }
                }, 5 * 60_000)
            }
        }).catch((err) => {
            console.error("[ChatPage] Failed to load session messages:", err)
        })

        return () => {
            if (recoveryTimerRef.current) {
                clearInterval(recoveryTimerRef.current)
                recoveryTimerRef.current = null
            }
            if (recoveryTimeoutRef.current) {
                clearTimeout(recoveryTimeoutRef.current)
                recoveryTimeoutRef.current = null
            }
            if (pollTimeoutRef.current) {
                clearTimeout(pollTimeoutRef.current)
                pollTimeoutRef.current = null
            }
            if (pollTimerRef.current) {
                clearInterval(pollTimerRef.current)
                pollTimerRef.current = null
            }
        }
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

        // 锁定当前会话 key，后续所有回调通过 ref 获取（onSessionId 可能迁移 key）
        sessionKeyRef.current = activeSessionId

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
        if (recoveryTimerRef.current) {
            clearInterval(recoveryTimerRef.current)
            recoveryTimerRef.current = null
        }
        if (recoveryTimeoutRef.current) {
            clearTimeout(recoveryTimeoutRef.current)
            recoveryTimeoutRef.current = null
        }

        const updateAssistantForKey = (key: string, updater: (msg: Message) => Message) => {
            setSessionDataMap((prev) => {
                const session = prev[key]
                if (!session) return prev
                return {
                    ...prev,
                    [key]: {
                        ...session,
                        messages: session.messages.map((msg) => (msg.id === aiMsgId ? updater(msg) : msg)),
                    },
                }
            })
        }

        sendMessage(originalInput, currentData.sessionId, {
            onChunk: (chunk) => {
                setThinkingText("") // 文本开始流式输出，清除 thinking 状态
                const key = sessionKeyRef.current
                updateAssistantForKey(key, (msg) => ({ ...msg, content: msg.content + chunk }))
            },
            onToolStart: (toolName) => {
                const key = sessionKeyRef.current
                updateAssistantForKey(key, (msg) => ({
                    ...msg,
                    toolCalls: [...(msg.toolCalls || []), { toolName, status: "loading" as const }],
                }))
            },
            onToolResult: (toolName) => {
                const key = sessionKeyRef.current
                updateAssistantForKey(key, (msg) => ({
                    ...msg,
                    toolCalls: (msg.toolCalls || []).map((tc) =>
                        tc.toolName === toolName ? { ...tc, status: "done" as const } : tc
                    ),
                }))
            },
            onSessionId: (id) => {
                const oldKey = sessionKeyRef.current
                // 迁移 sessionDataMap：将 "new-xxx" key 下的数据迁移到后端 UUID key
                setSessionDataMap((prev) => {
                    const oldData = prev[oldKey]
                    if (!oldData) return prev
                    const newMap = { ...prev }
                    delete newMap[oldKey]
                    newMap[id] = { ...oldData, sessionId: id }
                    return newMap
                })
                // 更新 ref 和 store，后续回调使用新 key
                sessionKeyRef.current = id
                setActiveSessionId(id)
                const title = (msgText || "图片对话").slice(0, 20) + (msgText.length > 20 ? "..." : "")
                addSession({ id, backendId: id, title, createdAt: new Date().toISOString() })
            },
            onDebatePhase: (content) => {
                try {
                    const update = JSON.parse(content) as DebatePhaseUpdate
                    if (update.type === "debate_phase") {
                        const key = sessionKeyRef.current
                        // 置信度门控结果：更新到消息上，不加入 phases 数组
                        if (update.phase === "confidence_gate") {
                            updateAssistantForKey(key, (msg) => ({
                                ...msg,
                                systemConfidence: update.system_confidence,
                                decisionGate: update.decision_gate,
                            }))
                            return
                        }
                        updateAssistantForKey(key, (msg) => {
                            const nextPhases = [...(msg.debatePhases || []), update]
                            const completed = new Set(nextPhases.map((p) => p.phase))
                            const nextActive = computeDebateActive(completed)
                            return {
                                ...msg,
                                debatePhases: nextPhases,
                                debateActive: nextActive.length > 0 ? nextActive : null,
                            }
                        })
                    }
                } catch { /* ignore parse errors */ }
            },
            onThinking: (text) => setThinkingText(text),
            onDone: () => {
                const key = sessionKeyRef.current
                updateAssistantForKey(key, (msg) => ({ ...msg, debateActive: null }))
                if (recoveryTimerRef.current) {
                    clearInterval(recoveryTimerRef.current)
                    recoveryTimerRef.current = null
                }
                if (recoveryTimeoutRef.current) {
                    clearTimeout(recoveryTimeoutRef.current)
                    recoveryTimeoutRef.current = null
                }
                setIsLoading(false)
                setThinkingText("")
            },
            onError: (err) => {
                const key = sessionKeyRef.current
                updateAssistantForKey(key, (msg) => ({
                    ...msg,
                    content: msg.content + `\n\n*Error: ${err}*`,
                    debateActive: null,
                }))
                setIsLoading(false)

                // 断线恢复：轮询服务端消息直到后端完成
                const sid = currentData.sessionId || sessionKeyRef.current
                if (sid && !sid.startsWith("new")) {
                    let lastSnapshot = ""
                    let stableCount = 0
                    if (recoveryTimerRef.current) {
                        clearInterval(recoveryTimerRef.current)
                        recoveryTimerRef.current = null
                    }
                    if (recoveryTimeoutRef.current) {
                        clearTimeout(recoveryTimeoutRef.current)
                        recoveryTimeoutRef.current = null
                    }
                    recoveryTimerRef.current = setInterval(async () => {
                        try {
                            const res = await getSessionMessages(sid)
                            if (!res.messages?.length) return

                            const lastMsg = res.messages[res.messages.length - 1]
                            const msgs = parseSessionMessages(sid, res.messages)
                            setSessionDataMap((p) => ({
                                ...(p[key]
                                    ? { ...p, [key]: { ...p[key], messages: msgs } }
                                    : p),
                            }))

                            if (lastMsg?.role === "assistant") {
                                const snapshot = (lastMsg.content || "") + (lastMsg.metadata || "")
                                if (snapshot === "") {
                                    stableCount = 0
                                    return
                                }
                                if (snapshot === lastSnapshot) {
                                    stableCount++
                                    if (stableCount >= 3) {
                                        if (recoveryTimerRef.current) {
                                            clearInterval(recoveryTimerRef.current)
                                            recoveryTimerRef.current = null
                                        }
                                    }
                                } else {
                                    lastSnapshot = snapshot
                                    stableCount = 0
                                }
                            }
                        } catch {
                            if (recoveryTimerRef.current) {
                                clearInterval(recoveryTimerRef.current)
                                recoveryTimerRef.current = null
                            }
                        }
                    }, 3000)

                    // 最长轮询 5 分钟
                    recoveryTimeoutRef.current = setTimeout(() => {
                        if (recoveryTimerRef.current) {
                            clearInterval(recoveryTimerRef.current)
                            recoveryTimerRef.current = null
                        }
                        recoveryTimeoutRef.current = null
                    }, 5 * 60_000)
                }
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
                        placeholder={isWelcome ? "输入基金代码、投资问题，或让 Quinfi 帮你扫描市场..." : "继续对话..."}
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
                        <h1 className="text-4xl font-hand font-semibold text-[var(--color-text)] mb-1">
                            Hi, how can I help you?
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
                                        <div className="flex mb-2 justify-end">
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
                                        <div className="mb-3">
                                            <div className="space-y-1.5">
                                                {msg.toolCalls.map((tc, i) => (
                                                    <ToolCallCard key={`${tc.toolName}-${i}`} toolName={tc.toolName} status={tc.status} />
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                    {msg.role === "assistant" && msg.debatePhases && msg.debatePhases.length > 0 && (
                                        <div className="mb-3">
                                            <div className="max-w-xl">
                                                <DebateTimeline phases={msg.debatePhases} activePhase={msg.debateActive} systemConfidence={msg.systemConfidence} decisionGate={msg.decisionGate} />
                                            </div>
                                        </div>
                                    )}
                                    <MessageBubble role={msg.role} content={msg.content} />
                                </div>
                            ))}

                            {isLoading && messages[messages.length - 1]?.content === "" && (
                                <div className="mb-8">
                                    <span className="text-[var(--color-text-muted)] text-sm animate-pulse">
                                        {status === "connecting" ? "连接中..." : thinkingText || "思考中..."}
                                    </span>
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
