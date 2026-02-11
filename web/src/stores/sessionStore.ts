import { create } from "zustand"
import { getSessions, deleteSession as apiDeleteSession } from "@/lib/api"

export interface ChatSession {
  id: string
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
  activeSessionId: "new",
  sidebarCollapsed: localStorage.getItem("sidebar_collapsed") === "true",
  recentsVisible: localStorage.getItem("recents_visible") !== "false",

  fetchSessions: async () => {
    try {
      const res = await getSessions()
      if (res.data?.length) {
        set({
          sessions: res.data.map((s) => ({
            id: s.id,
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
    set({ activeSessionId: id })
  },

  newChat: () => {
    set({ activeSessionId: "new-" + Date.now() })
  },

  addSession: (session: ChatSession) => {
    set((state) => {
      if (state.sessions.some((s) => s.id === session.id)) return state
      return { sessions: [session, ...state.sessions] }
    })
  },

  deleteSession: (id: string) => {
    apiDeleteSession(id).catch(() => {})
    set((state) => {
      const next: Partial<SessionState> = {
        sessions: state.sessions.filter((s) => s.id !== id),
      }
      if (state.activeSessionId === id) {
        next.activeSessionId = "new-" + Date.now()
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
