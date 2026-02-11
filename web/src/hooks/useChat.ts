import { useRef, useState, useCallback } from "react"
import type { SSEEvent } from "@/types"

export type ConnectionStatus = "idle" | "connecting" | "streaming" | "error"

export interface ChatCallbacks {
  onChunk: (content: string) => void
  onToolStart?: (toolName: string, text: string) => void
  onToolResult?: (toolName: string, text: string) => void
  onSessionId?: (sessionId: string) => void
  onDone: () => void
  onError: (err: string) => void
}

/** SSE 连接超时（等待首个响应） */
const CONNECT_TIMEOUT_MS = 60_000
/** 流式传输中，两次数据之间最大静默时间
 * 辩论工具需要连续调用 6 轮 LLM（约 2-3 分钟），期间无数据推送
 * 因此空闲超时设为 5 分钟 */
const STREAM_IDLE_TIMEOUT_MS = 5 * 60_000

export function useChat() {
  const abortControllerRef = useRef<AbortController | null>(null)
  const [status, setStatus] = useState<ConnectionStatus>("idle")

  const cancel = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
    }
    setStatus("idle")
  }, [])

  const sendMessage = useCallback(
    async (
      message: string,
      sessionId: string | undefined,
      callbacks: ChatCallbacks,
      images?: string[]
    ) => {
      // 取消之前进行中的请求
      if (abortControllerRef.current) {
        abortControllerRef.current.abort()
      }
      const controller = new AbortController()
      abortControllerRef.current = controller

      setStatus("connecting")

      // 连接超时定时器
      let connectTimer: ReturnType<typeof setTimeout> | null = setTimeout(() => {
        controller.abort()
        setStatus("error")
        callbacks.onError("连接超时，请稍后重试")
      }, CONNECT_TIMEOUT_MS)

      // 流式空闲超时定时器
      let idleTimer: ReturnType<typeof setTimeout> | null = null

      const resetIdleTimer = () => {
        if (idleTimer) clearTimeout(idleTimer)
        idleTimer = setTimeout(() => {
          controller.abort()
          setStatus("error")
          callbacks.onError("响应超时，服务器长时间未返回数据")
        }, STREAM_IDLE_TIMEOUT_MS)
      }

      const clearTimers = () => {
        if (connectTimer) {
          clearTimeout(connectTimer)
          connectTimer = null
        }
        if (idleTimer) {
          clearTimeout(idleTimer)
          idleTimer = null
        }
      }

      try {
        const body: Record<string, unknown> = {
          message,
          session_id: sessionId || "",
          stream: true,
        }
        if (images?.length) {
          body.images = images
        }

        const response = await fetch("/api/v1/chat", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
          signal: controller.signal,
        })

        // 连接已建立，清除连接超时
        if (connectTimer) {
          clearTimeout(connectTimer)
          connectTimer = null
        }

        if (!response.ok) {
          const errBody = await response.json().catch(() => null)
          const errMsg = errBody?.error || `请求失败 (${response.status})`
          setStatus("error")
          callbacks.onError(errMsg)
          return
        }

        const reader = response.body?.getReader()
        if (!reader) {
          setStatus("error")
          callbacks.onError("服务器未返回响应体")
          return
        }

        setStatus("streaming")
        resetIdleTimer()

        const decoder = new TextDecoder()
        let buffer = ""

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          // 收到数据，重置空闲超时
          resetIdleTimer()

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split("\n\n")
          buffer = lines.pop() || ""

          for (const line of lines) {
            if (!line.startsWith("data: ")) continue
            const dataStr = line.slice(6).trim()

            if (dataStr === "[DONE]") {
              callbacks.onDone()
              continue
            }

            try {
              const event = JSON.parse(dataStr) as SSEEvent

              if (event.error) {
                callbacks.onError(event.error)
                continue
              }

              if (event.session_id) {
                callbacks.onSessionId?.(event.session_id)
              }

              switch (event.type) {
                case "tool_start":
                  callbacks.onToolStart?.(
                    event.tool_name || "",
                    event.content || ""
                  )
                  break
                case "tool_result":
                  callbacks.onToolResult?.(
                    event.tool_name || "",
                    event.content || ""
                  )
                  break
                case "text":
                default:
                  if (event.content) callbacks.onChunk(event.content)
              }
            } catch {
              // 非 JSON 数据，跳过
            }
          }
        }

        // 流正常结束
        callbacks.onDone()
      } catch (err: unknown) {
        if (err instanceof Error && err.name === "AbortError") {
          // 如果是超时触发的 abort，status 已被设为 error，不覆盖
          if (abortControllerRef.current === controller) {
            setStatus("idle")
          }
          return
        }

        setStatus("error")
        const msg =
          err instanceof Error ? err.message : "发送消息失败，请检查网络连接"
        callbacks.onError(msg)
      } finally {
        clearTimers()
        if (abortControllerRef.current === controller) {
          abortControllerRef.current = null
          setStatus((prev) => (prev === "error" ? prev : "idle"))
        }
      }
    },
    []
  )

  return { sendMessage, cancel, status }
}
