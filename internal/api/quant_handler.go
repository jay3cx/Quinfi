package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/datasource"
	funddb "github.com/jay3cx/fundmind/internal/db"
	"github.com/jay3cx/fundmind/internal/quant"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// QuantHandler 量化分析 API
type QuantHandler struct {
	ds       datasource.FundDataSource
	fundRepo *funddb.FundRepository
}

// NewQuantHandler 创建量化分析处理器
func NewQuantHandler(ds datasource.FundDataSource, fundRepo *funddb.FundRepository) *QuantHandler {
	return &QuantHandler{ds: ds, fundRepo: fundRepo}
}

// RegisterRoutes 注册量化分析路由
func (h *QuantHandler) RegisterRoutes(v1 *gin.RouterGroup) {
	qg := v1.Group("/quant")
	{
		qg.POST("/backtest", h.handleBacktest)
		qg.POST("/dca", h.handleDCA)
		qg.POST("/compare", h.handleCompare)
	}
}

// loadNAVSeries 从数据源加载净值序列，同时缓存到 DB
func (h *QuantHandler) loadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error) {
	navList, err := h.ds.GetFundNAV(ctx, code, days)
	if err != nil {
		return nil, err
	}

	if h.fundRepo != nil && len(navList) > 0 {
		if saveErr := h.fundRepo.SaveNAVHistory(ctx, code, navList); saveErr != nil {
			logger.Warn("缓存净值失败", zap.String("code", code), zap.Error(saveErr))
		}
	}

	// 数据源返回按日期降序，转换为升序
	points := make([]quant.NavPoint, len(navList))
	for i, nav := range navList {
		points[len(navList)-1-i] = quant.NavPoint{
			Date:   nav.Date,
			NAV:    nav.UnitNAV,
			AccNAV: nav.AccumNAV,
		}
	}
	return &quant.NavSeries{FundCode: code, Points: points}, nil
}

func (h *QuantHandler) handleBacktest(c *gin.Context) {
	var req struct {
		Holdings    []quant.HoldingWeight `json:"holdings" binding:"required"`
		Days        int                   `json:"days"`
		InitialCash float64              `json:"initial_cash"`
		Rebalance   string               `json:"rebalance"`
		Benchmark   string               `json:"benchmark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Days <= 0 {
		req.Days = 365
	}

	ctx := c.Request.Context()
	navData := make(map[string]*quant.NavSeries)

	for _, holding := range req.Holdings {
		series, err := h.loadNAVSeries(ctx, holding.FundCode, req.Days)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "加载基金 " + holding.FundCode + " 净值失败: " + err.Error()})
			return
		}
		navData[holding.FundCode] = series
	}

	if req.Benchmark != "" {
		series, err := h.loadNAVSeries(ctx, req.Benchmark, req.Days)
		if err == nil {
			navData[req.Benchmark] = series
		}
	}

	btReq := &quant.BacktestRequest{
		Holdings:    req.Holdings,
		InitialCash: req.InitialCash,
		Rebalance:   quant.RebalanceType(req.Rebalance),
		Benchmark:   req.Benchmark,
	}
	result, err := quant.RunBacktest(btReq, navData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *QuantHandler) handleDCA(c *gin.Context) {
	var req struct {
		FundCode  string  `json:"fund_code" binding:"required"`
		Strategy  string  `json:"strategy"`
		Amount    float64 `json:"amount" binding:"required"`
		Frequency string  `json:"frequency"`
		Days      int     `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Days <= 0 {
		req.Days = 1095
	}
	if req.Frequency == "" {
		req.Frequency = "monthly"
	}
	if req.Strategy == "" {
		req.Strategy = "fixed"
	}

	series, err := h.loadNAVSeries(c.Request.Context(), req.FundCode, req.Days)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "加载净值失败: " + err.Error()})
		return
	}

	dcaReq := &quant.DCARequest{
		FundCode:  req.FundCode,
		Strategy:  quant.DCAStrategy(req.Strategy),
		Amount:    req.Amount,
		Frequency: req.Frequency,
	}
	result, err := quant.RunDCA(dcaReq, series)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *QuantHandler) handleCompare(c *gin.Context) {
	var req struct {
		FundCodes []string `json:"fund_codes" binding:"required"`
		Period    string   `json:"period"`
		Days      int      `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if len(req.FundCodes) < 2 || len(req.FundCodes) > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 2-5 只基金代码"})
		return
	}
	if req.Days <= 0 {
		req.Days = 365
	}
	if req.Period == "" {
		req.Period = "1y"
	}

	ctx := c.Request.Context()
	navData := make(map[string]*quant.NavSeries)
	fundNames := make(map[string]string)

	for _, code := range req.FundCodes {
		series, err := h.loadNAVSeries(ctx, code, req.Days)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "加载基金 " + code + " 净值失败: " + err.Error()})
			return
		}
		navData[code] = series

		fund, err := h.ds.GetFundInfo(ctx, code)
		if err == nil {
			fundNames[code] = fund.Name
		}
	}

	compareReq := &quant.CompareRequest{FundCodes: req.FundCodes, Period: req.Period}
	result, err := quant.RunCompare(compareReq, navData, fundNames)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
