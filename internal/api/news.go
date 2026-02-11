// Package api 提供新闻 API 端点
package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/rss"
)

// NewsHandler 新闻 API 处理器
type NewsHandler struct {
	store *rss.Store
}

// NewNewsHandler 创建新闻处理器
func NewNewsHandler(store *rss.Store) *NewsHandler {
	return &NewsHandler{store: store}
}

// RegisterRoutes 注册新闻相关路由
func (h *NewsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	news := rg.Group("/news")
	{
		news.GET("", h.GetNewsList)
		news.GET("/:guid", h.GetNewsDetail)
	}

	rg.GET("/feeds", h.GetFeedsList)
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
