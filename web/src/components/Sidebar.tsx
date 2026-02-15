import { useState } from "react"
import { LayoutDashboard, Briefcase, Plus, X, MessageSquare, PanelLeftClose, PanelLeft, Search, FlaskConical, PiggyBank, GitCompareArrows } from "lucide-react"
import { useLocation, useNavigate } from "react-router-dom"
import { useSessionStore } from "@/stores/sessionStore"

const navItems = [
    { icon: LayoutDashboard, label: "Market", path: "/market" },
    { icon: Briefcase, label: "Portfolio", path: "/portfolio" },
    { icon: FlaskConical, label: "回测实验室", path: "/backtest" },
    { icon: PiggyBank, label: "定投模拟", path: "/dca" },
    { icon: GitCompareArrows, label: "基金PK", path: "/compare" },
]

export function Sidebar() {
    const location = useLocation()
    const navigate = useNavigate()
    const [searchQuery, setSearchQuery] = useState("")
    const [showSearch, setShowSearch] = useState(false)

    const sessions = useSessionStore((s) => s.sessions)
    const activeSessionId = useSessionStore((s) => s.activeSessionId)
    const collapsed = useSessionStore((s) => s.sidebarCollapsed)
    const showRecents = useSessionStore((s) => s.recentsVisible)
    const newChat = useSessionStore((s) => s.newChat)
    const setActiveSessionId = useSessionStore((s) => s.setActiveSessionId)
    const deleteSession = useSessionStore((s) => s.deleteSession)
    const toggleSidebar = useSessionStore((s) => s.toggleSidebar)
    const toggleRecents = useSessionStore((s) => s.toggleRecents)

    const handleSearch = () => {
        const q = searchQuery.trim()
        if (!q) return
        if (/^\d{6}$/.test(q)) {
            navigate(`/fund/${q}`)
        } else {
            navigate(`/chat`)
            newChat()
        }
        setSearchQuery("")
        setShowSearch(false)
    }

    // 折叠态
    if (collapsed) {
        return (
            <aside className="w-[48px] shrink-0 bg-[var(--color-sidebar-bg)] h-screen flex flex-col items-center py-3 border-r border-[var(--color-border)]">
                <button
                    onClick={toggleSidebar}
                    className="p-2 rounded-md text-[var(--color-text-muted)] hover:bg-white/40 transition-colors mb-3"
                    title="展开侧边栏"
                >
                    <PanelLeft className="w-4 h-4" />
                </button>
                <button
                    onClick={() => { navigate("/chat"); newChat() }}
                    className="p-2 rounded-md text-[var(--color-text-muted)] hover:bg-white/40 transition-colors mb-2"
                    title="New chat"
                >
                    <Plus className="w-4 h-4" />
                </button>
                {navItems.map((item) => (
                    <button
                        key={item.path}
                        onClick={() => navigate(item.path)}
                        className={`p-2 rounded-md transition-colors mb-1 ${
                            location.pathname.startsWith(item.path)
                                ? "bg-white/50 text-[var(--color-text)]"
                                : "text-[var(--color-text-muted)] hover:bg-white/40"
                        }`}
                        title={item.label}
                    >
                        <item.icon className="w-4 h-4" />
                    </button>
                ))}
                <button
                    onClick={() => { toggleSidebar(); setTimeout(() => setShowSearch(true), 100) }}
                    className="p-2 rounded-md text-[var(--color-text-muted)] hover:bg-white/40 transition-colors mb-1"
                    title="搜索基金"
                >
                    <Search className="w-4 h-4" />
                </button>
            </aside>
        )
    }

    // 展开态
    return (
        <aside className="w-[260px] shrink-0 bg-[var(--color-sidebar-bg)] h-screen flex flex-col border-r border-[var(--color-border)]">
            {/* 顶部：折叠 + 新建 */}
            <div className="flex items-center justify-between px-3 py-3">
                <button
                    onClick={toggleSidebar}
                    className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:bg-white/40 transition-colors"
                    title="收起侧边栏"
                >
                    <PanelLeftClose className="w-4 h-4" />
                </button>
                <button
                    onClick={() => { navigate("/chat"); newChat() }}
                    className="p-1.5 rounded-md text-[var(--color-text-muted)] hover:bg-white/40 transition-colors"
                    title="New chat"
                >
                    <Plus className="w-4 h-4" />
                </button>
            </div>

            {/* 导航 */}
            <nav className="px-3 space-y-0.5 mb-2">
                {navItems.map((item) => {
                    const isActive = location.pathname.startsWith(item.path)
                    return (
                        <button
                            key={item.path}
                            onClick={() => navigate(item.path)}
                            className={`w-full flex items-center gap-3 px-3 py-2 rounded-md transition-colors text-sm ${
                                isActive
                                    ? "bg-white/50 text-[var(--color-text)] font-medium"
                                    : "text-[var(--color-text-secondary)] hover:bg-white/30"
                            }`}
                        >
                            <item.icon className="w-4 h-4" />
                            {item.label}
                        </button>
                    )
                })}
            </nav>

            {/* 搜索 */}
            <div className="px-3 mb-2">
                {showSearch ? (
                    <div className="flex items-center gap-1 bg-white/60 rounded-md border border-[var(--color-border)] px-2">
                        <Search className="w-3.5 h-3.5 text-[var(--color-text-muted)] shrink-0" />
                        <input
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            onKeyDown={(e) => { if (e.key === "Enter") handleSearch(); if (e.key === "Escape") setShowSearch(false) }}
                            placeholder="基金代码..."
                            className="w-full bg-transparent text-sm py-1.5 outline-none text-[var(--color-text)] placeholder:text-[var(--color-text-muted)]"
                            autoFocus
                        />
                    </div>
                ) : (
                    <button
                        onClick={() => setShowSearch(true)}
                        className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-[var(--color-text-secondary)] hover:bg-white/30 transition-colors"
                    >
                        <Search className="w-4 h-4" />
                        Search
                    </button>
                )}
            </div>

            {/* Recents */}
            <div className="px-3 mt-1">
                <button
                    onClick={toggleRecents}
                    className="w-full flex items-center justify-between px-3 py-1.5 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] transition-colors"
                >
                    <span>Recents</span>
                    <span className="text-[10px]">{showRecents ? "Hide" : "Show"}</span>
                </button>
            </div>

            {showRecents && (
                <div className="flex-1 overflow-y-auto px-3 min-h-0">
                    <div className="space-y-0.5">
                        {sessions.length === 0 ? (
                            <div className="px-3 py-4 text-xs text-[var(--color-text-muted)]">
                                No recent chats
                            </div>
                        ) : (
                            sessions.map((session) => (
                                <div
                                    key={session.id}
                                    className={`group flex items-center gap-2 px-3 py-2 rounded-md cursor-pointer transition-colors text-sm truncate ${
                                        session.id === activeSessionId
                                            ? "bg-white/50 text-[var(--color-text)] font-medium"
                                            : "text-[var(--color-text-secondary)] hover:bg-white/30"
                                    }`}
                                    onClick={() => {
                                        navigate("/chat")
                                        setActiveSessionId(session.id)
                                    }}
                                >
                                    <MessageSquare className="w-3.5 h-3.5 shrink-0" />
                                    <span className="truncate flex-1">{session.title}</span>
                                    <button
                                        className="opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                                        onClick={(e) => {
                                            e.stopPropagation()
                                            deleteSession(session.id)
                                        }}
                                    >
                                        <X className="w-3 h-3 text-[var(--color-text-muted)] hover:text-[var(--color-down)]" />
                                    </button>
                                </div>
                            ))
                        )}
                    </div>
                </div>
            )}

            {!showRecents && <div className="flex-1" />}

            {/* 底部用户 */}
            <div className="px-3 py-4 border-t border-[var(--color-border)]">
                <div className="flex items-center gap-3 px-2">
                    <div className="w-7 h-7 rounded-full bg-[var(--color-primary)] flex items-center justify-center text-white text-xs font-medium">
                        J
                    </div>
                    <div className="text-sm text-[var(--color-text-secondary)]">Jay</div>
                </div>
            </div>
        </aside>
    )
}
