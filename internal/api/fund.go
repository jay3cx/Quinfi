// Package api 提供基金数据 API 端点
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/datasource"
)

// FundHandler 基金 API 处理器
type FundHandler struct {
	ds datasource.FundDataSource
}

// NewFundHandler 创建基金处理器
func NewFundHandler(ds datasource.FundDataSource) *FundHandler {
	return &FundHandler{ds: ds}
}

// RegisterRoutes 注册基金相关路由
func (h *FundHandler) RegisterRoutes(rg *gin.RouterGroup) {
	fund := rg.Group("/fund")
	{
		fund.GET("/:code", h.GetFundInfo)
		fund.GET("/:code/nav", h.GetFundNAV)
		fund.GET("/:code/holdings", h.GetFundHoldings)
	}
}

// GetFundInfo 获取基金详情
// GET /api/v1/fund/:code
func (h *FundHandler) GetFundInfo(c *gin.Context) {
	code := c.Param("code")

	fund, err := h.ds.GetFundInfo(c.Request.Context(), code)
	if err != nil {
		var notFound *datasource.ErrFundNotFound
		if errors.As(err, &notFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "基金不存在",
				"code":  code,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, fund)
}

// GetFundNAV 获取净值历史
// GET /api/v1/fund/:code/nav?days=30
// GET /api/v1/fund/:code/nav?period=1w  (支持 1w/1m/3m/6m/1y/3y)
func (h *FundHandler) GetFundNAV(c *gin.Context) {
	code := c.Param("code")

	// 优先用 period 参数，其次用 days
	days := 30
	period := c.Query("period")
	if period != "" {
		days = periodToDays(period)
	} else if daysStr := c.Query("days"); daysStr != "" {
		days, _ = strconv.Atoi(daysStr)
	}

	if days <= 0 {
		days = 30
	}

	navList, err := h.ds.GetFundNAV(c.Request.Context(), code, days)
	if err != nil {
		var notFound *datasource.ErrFundNotFound
		if errors.As(err, &notFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "基金不存在",
				"code":  code,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":   code,
		"days":   days,
		"period": period,
		"data":   navList,
	})
}

// periodToDays 将时间范围标识转为天数
func periodToDays(period string) int {
	switch period {
	case "1w":
		return 7
	case "1m":
		return 30
	case "3m":
		return 90
	case "6m":
		return 180
	case "1y":
		return 365
	case "3y":
		return 1095
	default:
		return 30
	}
}

// GetFundHoldings 获取持仓明细
// GET /api/v1/fund/:code/holdings
func (h *FundHandler) GetFundHoldings(c *gin.Context) {
	code := c.Param("code")

	holdings, err := h.ds.GetFundHoldings(c.Request.Context(), code)
	if err != nil {
		var notFound *datasource.ErrFundNotFound
		if errors.As(err, &notFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "基金不存在",
				"code":  code,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"data": holdings,
	})
}
