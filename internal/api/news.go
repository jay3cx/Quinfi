// Package api 提供新闻 API 端点
package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/Quinfi/internal/rss"
)

// NewsHandler 新闻 API 处理器
type NewsHandler struct {
	store     *rss.Store
	scheduler *rss.Scheduler // 可为 nil（未启用 RSS 时）
}

// NewNewsHandler 创建新闻处理器
func NewNewsHandler(store *rss.Store, scheduler *rss.Scheduler) *NewsHandler {
	return &NewsHandler{store: store, scheduler: scheduler}
}

// RegisterRoutes 注册新闻相关路由
func (h *NewsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	news := rg.Group("/news")
	{
		news.GET("", h.GetNewsList)
		news.GET("/:guid", h.GetNewsDetail)
	}

	rg.GET("/feeds", h.GetFeedsList)

	rssGroup := rg.Group("/rss")
	{
		rssGroup.GET("/status", h.GetRSSStatus)
		rssGroup.POST("/toggle", h.ToggleRSS)
	}
}

// NewsListResponse 新闻列表响应
type NewsListResponse struct {
	Data   []*rss.Article `json:"data"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// GetNewsList 获取新闻列表
// GET /api/v1/news?limit=20&offset=0&sentiment=positive
func (h *NewsHandler) GetNewsList(c *gin.Context) {
	// 解析分页参数
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	sentiment := c.Query("sentiment")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// 获取所有文章
	allArticles := h.store.GetAllArticles(0)

	// 情绪筛选
	var filtered []*rss.Article
	if sentiment != "" {
		sentimentType := rss.Sentiment(sentiment)
		for _, a := range allArticles {
			if a.Sentiment == sentimentType {
				filtered = append(filtered, a)
			}
		}
	} else {
		filtered = allArticles
	}

	// 按发布时间倒序排列
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].PubDate.After(filtered[j].PubDate)
	})

	total := len(filtered)

	// 分页
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	paged := filtered[start:end]

	c.JSON(http.StatusOK, NewsListResponse{
		Data:   paged,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetNewsDetail 获取新闻详情
// GET /api/v1/news/:guid
func (h *NewsHandler) GetNewsDetail(c *gin.Context) {
	guid := c.Param("guid")

	article, found := h.store.GetArticle(guid)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "新闻不存在",
			"guid":  guid,
		})
		return
	}

	c.JSON(http.StatusOK, article)
}

// GetFeedsList 获取订阅源列表
// GET /api/v1/feeds
func (h *NewsHandler) GetFeedsList(c *gin.Context) {
	feeds := h.store.GetAllFeeds()
	c.JSON(http.StatusOK, gin.H{
		"data":  feeds,
		"total": len(feeds),
	})
}

// GetRSSStatus 获取 RSS 调度器状态
// GET /api/v1/rss/status
func (h *NewsHandler) GetRSSStatus(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusOK, gin.H{
			"running":    false,
			"feed_count": 0,
			"enabled":    false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"running":    !h.scheduler.IsPaused(),
		"feed_count": h.scheduler.FeedCount(),
		"enabled":    true,
	})
}

// ToggleRSS 切换 RSS 调度器暂停/恢复
// POST /api/v1/rss/toggle
func (h *NewsHandler) ToggleRSS(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "RSS 未启用",
		})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效请求: " + err.Error(),
		})
		return
	}

	if req.Enabled {
		h.scheduler.Resume()
	} else {
		h.scheduler.Pause()
	}

	c.JSON(http.StatusOK, gin.H{
		"running": !h.scheduler.IsPaused(),
		"message": map[bool]string{true: "RSS 已恢复", false: "RSS 已暂停"}[req.Enabled],
	})
}
