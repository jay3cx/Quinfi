// Package api 提供任务查询与进度推送 Handler
package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/Quinfi/internal/task"
)

// TaskHandler 异步任务 HTTP 处理器
type TaskHandler struct {
	manager *task.Manager
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(m *task.Manager) *TaskHandler {
	return &TaskHandler{manager: m}
}

// RegisterRoutes 注册任务相关路由
func (h *TaskHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/tasks", h.ListTasks)
	rg.GET("/tasks/:id", h.GetTask)
	rg.GET("/tasks/:id/stream", h.StreamTask)
}

// GetTask 查询单个任务状态
// GET /api/v1/tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")

	t, err := h.manager.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, t)
}

// ListTasks 列出最近任务
// GET /api/v1/tasks?type=deep_analysis&limit=10
func (h *TaskHandler) ListTasks(c *gin.Context) {
	taskType := c.Query("type")
	limit := 20
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	tasks, err := h.manager.List(taskType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if tasks == nil {
		tasks = []task.Task{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  tasks,
		"total": len(tasks),
	})
}

// StreamTask SSE 推送任务进度
// GET /api/v1/tasks/:id/stream
func (h *TaskHandler) StreamTask(c *gin.Context) {
	id := c.Param("id")

	// 先检查任务是否存在
	t, err := h.manager.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 如果任务已经完成/失败，直接返回最终状态
	if t.Status == task.StatusCompleted || t.Status == task.StatusFailed {
		sse := NewSSEWriter(c)
		sse.WriteEvent(task.Update{
			TaskID:      t.ID,
			Status:      t.Status,
			Progress:    t.Progress,
			ProgressMsg: t.ProgressMsg,
			Result:      t.Result,
			Error:       t.Error,
			Done:        true,
		})
		sse.WriteDone()
		return
	}

	// 订阅进度更新
	updates, unsubscribe := h.manager.Subscribe(id)
	defer unsubscribe()

	sse := NewSSEWriter(c)

	// 先发送当前状态
	sse.WriteEvent(task.Update{
		TaskID:      t.ID,
		Status:      t.Status,
		Progress:    t.Progress,
		ProgressMsg: t.ProgressMsg,
	})

	// 监听更新
	for {
		select {
		case update, ok := <-updates:
			if !ok {
				return
			}
			if err := sse.WriteEvent(update); err != nil {
				return
			}
			if update.Done {
				sse.WriteDone()
				return
			}
		case <-c.Request.Context().Done():
			// 客户端断开，只退出 SSE，不影响任务执行
			return
		}
	}
}

// WriteTaskAccepted 返回 202 Accepted 和任务 ID（通用辅助）
func WriteTaskAccepted(c *gin.Context, taskID string) {
	c.JSON(http.StatusAccepted, gin.H{
		"task_id": taskID,
		"status":  "pending",
		"stream":  fmt.Sprintf("/api/v1/tasks/%s/stream", taskID),
	})
}
