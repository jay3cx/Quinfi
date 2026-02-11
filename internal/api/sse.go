// Package api 提供 SSE 流式响应工具
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SSEWriter SSE 写入器
type SSEWriter struct {
	ctx *gin.Context
}

// NewSSEWriter 创建 SSE 写入器
func NewSSEWriter(c *gin.Context) *SSEWriter {
	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

	return &SSEWriter{ctx: c}
}

// WriteEvent 写入 SSE 事件
func (w *SSEWriter) WriteEvent(data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w.ctx.Writer, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}

	w.ctx.Writer.Flush()
	return nil
}

// WriteString 写入字符串事件
func (w *SSEWriter) WriteString(data string) error {
	_, err := fmt.Fprintf(w.ctx.Writer, "data: %s\n\n", data)
	if err != nil {
		return err
	}

	w.ctx.Writer.Flush()
	return nil
}

// WriteError 写入错误事件
func (w *SSEWriter) WriteError(errMsg string) error {
	return w.WriteEvent(gin.H{"error": errMsg})
}

// WriteDone 写入完成信号
func (w *SSEWriter) WriteDone() error {
	_, err := fmt.Fprint(w.ctx.Writer, "data: [DONE]\n\n")
	if err != nil {
		return err
	}

	w.ctx.Writer.Flush()
	return nil
}

// SSEResponse SSE 响应数据
type SSEResponse struct {
	Content   string `json:"content,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Type      string `json:"type,omitempty"`      // text, tool_start, tool_result, thinking
	ToolName  string `json:"tool_name,omitempty"` // 工具名称
	Done      bool   `json:"done,omitempty"`
	Error     string `json:"error,omitempty"`
}

// WriteSSEResponse 写入 SSE 响应
func (w *SSEWriter) WriteSSEResponse(resp *SSEResponse) error {
	return w.WriteEvent(resp)
}

// IsClientDisconnected 检查客户端是否断开连接
func (w *SSEWriter) IsClientDisconnected() bool {
	select {
	case <-w.ctx.Request.Context().Done():
		return true
	default:
		return false
	}
}

// SetStatus 设置 HTTP 状态码（必须在写入前调用）
func (w *SSEWriter) SetStatus(code int) {
	w.ctx.Status(code)
}

// Writer 返回底层 http.ResponseWriter
func (w *SSEWriter) Writer() http.ResponseWriter {
	return w.ctx.Writer
}
