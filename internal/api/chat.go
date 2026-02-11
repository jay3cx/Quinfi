// Package api 提供对话 API 处理器
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/internal/memory"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// ChatRequest 对话请求
type ChatRequest struct {
	Message   string   `json:"message" binding:"required"`
	SessionID string   `json:"session_id"`
	Stream    bool     `json:"stream"`
	Images    []string `json:"images,omitempty"` // base64 编码的图片列表（支持 data URI）
}

// ChatResponse 对话响应
type ChatResponse struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id"`
	CreatedAt string `json:"created_at"`
}

// ChatHandler 对话 API 处理器
type ChatHandler struct {
	agent          agent.Agent
	sessionManager *SessionManager
	memoryStore    *memory.Store     // 可为 nil（无记忆模式）
	memoryExtractor *memory.Extractor // 可为 nil
}

// NewChatHandler 创建对话处理器
func NewChatHandler(a agent.Agent, sm *SessionManager, memStore *memory.Store, memExtractor *memory.Extractor) *ChatHandler {
	return &ChatHandler{
		agent:           a,
		sessionManager:  sm,
		memoryStore:     memStore,
		memoryExtractor: memExtractor,
	}
}

// RegisterRoutes 注册路由
func (h *ChatHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/chat", h.Chat)
	rg.GET("/sessions", h.ListSessions)
	rg.GET("/sessions/:id/messages", h.GetSessionMessages)
	rg.DELETE("/sessions/:id", h.DeleteSession)
}

// SessionInfo 会话摘要（列表用）
type SessionInfo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	LastActiveAt string `json:"last_active_at"`
	MessageCount int    `json:"message_count"`
}

// ListSessions 获取会话列表
// GET /api/v1/sessions
func (h *ChatHandler) ListSessions(c *gin.Context) {
	sessions := h.sessionManager.ListSessions()

	list := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		title := ""
		for _, msg := range s.History {
			if msg.Role == "user" {
				title = msg.Content
				break
			}
		}
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30]) + "..."
		}
		if title == "" {
			title = "New chat"
		}

		list = append(list, SessionInfo{
			ID:           s.ID,
			Title:        title,
			LastActiveAt: s.LastActiveAt.Format(time.RFC3339),
			MessageCount: len(s.History),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  list,
		"total": len(list),
	})
}

// GetSessionMessages 获取会话消息历史
// GET /api/v1/sessions/:id/messages
func (h *ChatHandler) GetSessionMessages(c *gin.Context) {
	id := c.Param("id")

	session, ok := h.sessionManager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": session.ID,
		"messages":   session.History,
	})
}

// Chat 对话处理
func (h *ChatHandler) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求: " + err.Error(),
		})
		return
	}

	// 获取或创建会话
	session := h.sessionManager.GetOrCreate(req.SessionID)

	// 注意：FundAgent.buildMessages 会把 Context.History 全量拼进 prompt，
	// 之后还会再追加 input.Query 作为“本轮用户消息”。
	// 因此这里传给 Agent 的 History 必须是“本轮之前”的快照，避免用户消息重复注入两次。
	historySnapshot := append([]agent.Message(nil), session.History...)

	logger.Info("收到对话请求",
		zap.String("session_id", session.ID),
		zap.String("message", req.Message),
		zap.Bool("stream", req.Stream),
	)

	// 记录用户消息
	h.sessionManager.Update(session.ID, agent.Message{
		Role:    "user",
		Content: req.Message,
	})

	// 召回相关记忆（对话前注入）
	var memoryContext string
	if h.memoryStore != nil {
		memories, err := h.memoryStore.Recall(c.Request.Context(), "default", req.Message, 8)
		if err != nil {
			logger.Warn("记忆召回失败", zap.Error(err))
		} else if len(memories) > 0 {
			memoryContext = memory.FormatMemoriesForPrompt(memories)
			logger.Info("注入记忆上下文", zap.Int("memories", len(memories)))
		}
	}

	// 构建 Agent 输入（记忆通过 Variables 传递）
	agentCtx := &agent.AgentContext{
		SessionID: session.ID,
		History:   historySnapshot,
		Variables: make(map[string]string),
	}
	if memoryContext != "" {
		agentCtx.Variables["memory_context"] = memoryContext
	}

	input := &agent.AgentInput{
		Query:   req.Message,
		Context: agentCtx,
		Images:  req.Images,
	}

	if req.Stream {
		h.handleStream(c, input, session.ID, req.Message)
	} else {
		h.handleSync(c, input, session.ID, req.Message)
	}
}

// handleSync 同步处理
func (h *ChatHandler) handleSync(c *gin.Context, input *agent.AgentInput, sessionID, userMessage string) {
	response, err := h.agent.Run(c.Request.Context(), input)
	if err != nil {
		logger.Error("Agent 执行失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "处理失败: " + err.Error(),
		})
		return
	}

	// 记录 AI 响应
	h.sessionManager.Update(sessionID, agent.Message{
		Role:    "assistant",
		Content: response.Content,
	})

	// 异步提取记忆（不阻塞响应）
	h.asyncExtractMemory(userMessage, response.Content, sessionID)

	c.JSON(http.StatusOK, ChatResponse{
		Content:   response.Content,
		SessionID: sessionID,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

// handleStream 流式处理
func (h *ChatHandler) handleStream(c *gin.Context, input *agent.AgentInput, sessionID, userMessage string) {
	sse := NewSSEWriter(c)

	// 发送会话信息
	sse.WriteSSEResponse(&SSEResponse{
		SessionID: sessionID,
	})

	chunks, err := h.agent.RunStream(c.Request.Context(), input)
	if err != nil {
		logger.Error("Agent 流式执行失败", zap.Error(err))
		sse.WriteError("处理失败: " + err.Error())
		return
	}

	var fullContent string
	var toolCalls []map[string]string // 收集工具调用记录

	for chunk := range chunks {
		if sse.IsClientDisconnected() {
			logger.Info("客户端断开连接", zap.String("session_id", sessionID))
			return
		}

		if chunk.Error != nil {
			sse.WriteError(chunk.Error.Error())
			return
		}

		if chunk.Done {
			break
		}

		// 只有文本内容计入最终响应
		if chunk.Type == "" || chunk.Type == agent.ChunkText {
			fullContent += chunk.Content
		}

		// 收集工具调用记录
		if chunk.Type == agent.ChunkToolStart || chunk.Type == agent.ChunkToolResult {
			toolCalls = append(toolCalls, map[string]string{
				"tool": chunk.ToolName,
				"type": string(chunk.Type),
			})
		}

		sse.WriteSSEResponse(&SSEResponse{
			Content:  chunk.Content,
			Type:     string(chunk.Type),
			ToolName: chunk.ToolName,
		})
	}

	// 记录完整响应（含工具调用元数据）
	metadata := ""
	if len(toolCalls) > 0 {
		if b, err := json.Marshal(toolCalls); err == nil {
			metadata = string(b)
		}
	}
	h.sessionManager.Update(sessionID, agent.Message{
		Role:     "assistant",
		Content:  fullContent,
		Metadata: metadata,
	})

	// 异步提取记忆
	h.asyncExtractMemory(userMessage, fullContent, sessionID)

	// 发送完成信号
	sse.WriteDone()
}

// asyncExtractMemory 异步提取并存储记忆
func (h *ChatHandler) asyncExtractMemory(userMessage, assistantResponse, sessionID string) {
	if h.memoryExtractor == nil || h.memoryStore == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries := h.memoryExtractor.Extract(ctx, userMessage, assistantResponse, sessionID)
		if len(entries) > 0 {
			if err := h.memoryStore.Save(ctx, entries); err != nil {
				logger.Warn("异步存储记忆失败", zap.Error(err))
			}
		}
	}()
}

// DeleteSession 删除会话
// DELETE /api/v1/sessions/:id
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	h.sessionManager.Delete(id)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
