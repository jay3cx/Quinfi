import { create } from "zustand"
import { getSessions, deleteSession as apiDeleteSession } from "@/lib/api"

export interface ChatSession {
  id: string
  backendId?: string // 后端真实 session UUID（删除/API 调用时使用）
  title: string
  createdAt: string
}

interface SessionState {
  // --- 数据 ---
  sessions: ChatSession[]
  activeSessionId: string
  sidebarCollapsed: boolean
  recentsVisible: boolean

  // --- Actions ---
  fetchSessions: () => Promise<void>
  setActiveSessionId: (id: string) => void
  newChat: () => void
  addSession: (session: ChatSession) => void
  deleteSession: (id: string) => void
  toggleSidebar: () => void
  toggleRecents: () => void
}

export const useSessionStore = create<SessionState>((set) => ({
  sessions: [],
  activeSessionId: localStorage.getItem("active_session_id") || "new",
  sidebarCollapsed: localStorage.getItem("sidebar_collapsed") === "true",
  recentsVisible: localStorage.getItem("recents_visible") !== "false",

  fetchSessions: async () => {
    try {
      const res = await getSessions()
      if (res.data?.length) {
        set({
          sessions: res.data.map((s) => ({
            id: s.id,
            backendId: s.id, // 从后端加载的会话，id 就是后端 UUID
            title: s.title,
            createdAt: s.last_active_at,
          })),
        })
      }
    } catch {
      // 首次加载失败不阻塞用户使用
    }
  },

  setActiveSessionId: (id: string) => {
    localStorage.setItem("active_session_id", id)
    set({ activeSessionId: id })
  },

  newChat: () => {
    const id = "new-" + Date.now()
    localStorage.setItem("active_session_id", id)
    set({ activeSessionId: id })
  },

  addSession: (session: ChatSession) => {
    set((state) => {
      if (state.sessions.some((s) => s.id === session.id)) return state
      return { sessions: [session, ...state.sessions] }
    })
  },

  deleteSession: (id: string) => {
    set((state) => {
      // 用 backendId（后端真实 UUID）发 DELETE 请求
      const session = state.sessions.find((s) => s.id === id)
      const apiId = session?.backendId || id
      apiDeleteSession(apiId).catch(() => {})

      const next: Partial<SessionState> = {
        sessions: state.sessions.filter((s) => s.id !== id),
      }
      if (state.activeSessionId === id) {
        const newId = "new-" + Date.now()
        localStorage.setItem("active_session_id", newId)
        next.activeSessionId = newId
      }
      return next
    })
  },

  toggleSidebar: () => {
    set((state) => {
      const next = !state.sidebarCollapsed
      localStorage.setItem("sidebar_collapsed", String(next))
      return { sidebarCollapsed: next }
    })
  },

  toggleRecents: () => {
    set((state) => {
      const next = !state.recentsVisible
      localStorage.setItem("recents_visible", String(next))
      return { recentsVisible: next }
    })
  },
}))
