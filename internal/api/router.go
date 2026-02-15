// Package api 提供 HTTP 路由和处理器
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/internal/config"
	"github.com/jay3cx/fundmind/internal/datasource"
	"github.com/jay3cx/fundmind/internal/datasource/eastmoney"
	"github.com/jay3cx/fundmind/internal/analyzer"
	funddb "github.com/jay3cx/fundmind/internal/db"
	"github.com/jay3cx/fundmind/internal/debate"
	"github.com/jay3cx/fundmind/internal/memory"
	"github.com/jay3cx/fundmind/internal/orchestrator"
	"github.com/jay3cx/fundmind/internal/portfolio"
	"github.com/jay3cx/fundmind/internal/quant"
	"github.com/jay3cx/fundmind/internal/task"
	"github.com/jay3cx/fundmind/internal/rss"
	"github.com/jay3cx/fundmind/internal/scheduler"
	"github.com/jay3cx/fundmind/internal/vision"
	"github.com/jay3cx/fundmind/pkg/llm"
	llmopenai "github.com/jay3cx/fundmind/pkg/llm/openai"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// SetupResult 路由初始化结果，包含生命周期管理所需的组件
type SetupResult struct {
	Engine       *gin.Engine
	Scheduler    *scheduler.Scheduler
	RSSScheduler *rss.Scheduler // RSS 抓取调度器（可为 nil）
}

// SetupRouter 配置并返回 Gin 路由和生命周期管理组件
// db 可为 nil，此时使用纯内存存储
func SetupRouter(cfg *config.Config, db *sql.DB) *SetupResult {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// 中间件
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	// 健康检查
	r.GET("/health", healthCheck)

	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		v1.GET("/ping", ping)
	}

	// ====== 数据源 ======
	emClient := eastmoney.NewClient()
	cachedDS := datasource.NewCachedDataSource(emClient, 5*time.Minute)

	// 注册基金数据 API
	fundHandler := NewFundHandler(cachedDS)
	fundHandler.RegisterRoutes(v1)

	// ====== LLM 客户端 ======
	agentClient := createLLMClient(cfg)

	// ====== RSS ======
	rssStore := createRSSStore(db)
	var rssScheduler *rss.Scheduler

	// ====== Agent 工具 ======
	toolRegistry := createToolRegistry(cachedDS, emClient, rssStore)

	// 创建多空辩论编排器 + 注册为 Agent 工具
	debateOrch := debate.NewOrchestrator(agentClient, toolRegistry)
	debateAdapter := debate.NewToolAdapter(debateOrch)
	toolRegistry.Register(agent.NewRunDebateTool(debateAdapter))

	// ====== Orchestrator（深度分析） ======
	var fundRepo *funddb.FundRepository
	if db != nil {
		fundRepo = funddb.NewFundRepository(db)
	}
	managerAnalyzer := analyzer.NewManagerAnalyzer(cachedDS, agentClient)
	macroAnalyzer := analyzer.NewMacroAnalyzer(agentClient)
	fundAnalyzer := analyzer.NewFundAnalyzer(cachedDS, agentClient, fundRepo)

	// 注册调仓检测工具
	toolRegistry.Register(agent.NewDetectRebalanceTool(&rebalanceAdapter{analyzer: fundAnalyzer}))

	orch := orchestrator.NewOrchestrator(
		fundAnalyzer, managerAnalyzer, macroAnalyzer,
		debateOrch, nil, // newsFunc 后续接入
	)

	// ====== 异步任务管理器 ======
	taskManager := task.NewManager(db, 3)
	taskHandler := NewTaskHandler(taskManager)
	taskHandler.RegisterRoutes(v1)

	// 注册深度分析端点（异步）
	v1.POST("/analysis/deep", func(c *gin.Context) {
		var req struct {
			Code string `json:"code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "缺少基金代码"})
			return
		}

		taskID, err := taskManager.Submit("deep_analysis", map[string]string{"code": req.Code})
		if err != nil {
			c.JSON(500, gin.H{"error": "创建任务失败: " + err.Error()})
			return
		}

		// 后台执行
		code := req.Code
		taskManager.Execute(taskID, func(ctx context.Context, reportProgress func(int, string, ...string)) (string, error) {
			report, err := orch.DeepAnalysis(ctx, code, orchestrator.ProgressFunc(reportProgress))
			if err != nil {
				return "", err
			}
			data, _ := json.Marshal(report)
			return string(data), nil
		})

		WriteTaskAccepted(c, taskID)
	})

	// ====== 量化分析 API ======
	quantHandler := NewQuantHandler(cachedDS, fundRepo)
	quantHandler.RegisterRoutes(v1)

	// 注册量化 Agent 工具
	quantLoader := &quantDataLoaderAdapter{ds: cachedDS}
	toolRegistry.Register(agent.NewBacktestPortfolioTool(quantLoader))
	toolRegistry.Register(agent.NewSimulateDCATool(quantLoader))
	toolRegistry.Register(agent.NewCompareFundsTool(quantLoader))

	// ====== Agent ======
	fundAgent := agent.NewFundAgent(agentClient, toolRegistry)
	sessionManager := NewSessionManager(cfg.Session.GetTTL(), db)

	// ====== Vision 扫描器 ======
	visionScanner := vision.NewScanner(agentClient)

	// ====== 记忆系统 ======
	memStore, memExtractor := createMemorySystem(db, agentClient, toolRegistry)
	if memStore != nil {
		registerPortfolioTool(toolRegistry, memStore)
		registerPortfolioAPI(v1, memStore, visionScanner)
	}

	// ====== HTTP 路由注册 ======
	chatHandler := NewChatHandler(fundAgent, sessionManager, memStore, memExtractor)
	chatHandler.RegisterRoutes(v1)

	// ====== 简报任务（API + 调度器共用） ======
	briefTask := scheduler.NewDailyBriefTask(
		agentClient, toolRegistry, memStore,
		func(ctx context.Context, brief string) error {
			logger.Info("每日投资简报已生成", zap.Int("length", len(brief)))
			if db != nil {
				_, err := db.Exec(`INSERT INTO briefs (content, type) VALUES (?, 'daily')`, brief)
				if err != nil {
					logger.Error("保存简报失败", zap.Error(err))
				}
			}
			return nil
		},
	)
	registerBriefsAPI(v1, db, briefTask)

	// ====== 定时调度器 ======
	sched := createScheduler(briefTask, agentClient, toolRegistry, memStore)
	sched.Start()

	// ====== RSS 抓取调度器 ======
	if cfg.RSS.Enabled && len(cfg.RSS.Feeds) > 0 {
		rssScheduler = createRSSScheduler(cfg, rssStore, agentClient)
	}

	newsHandler := NewNewsHandler(rssStore, rssScheduler)
	newsHandler.RegisterRoutes(v1)

	logger.Info("路由初始化完成",
		zap.String("llm_base_url", cfg.LLM.BaseURL),
		zap.Bool("llm_api_key_set", cfg.LLM.APIKey != ""),
		zap.Int("tools_registered", len(toolRegistry.List())),
		zap.Duration("session_ttl", cfg.Session.GetTTL()),
		zap.Bool("rss_enabled", cfg.RSS.Enabled),
		zap.Int("rss_feeds", len(cfg.RSS.Feeds)),
	)

	return &SetupResult{
		Engine:       r,
		Scheduler:    sched,
		RSSScheduler: rssScheduler,
	}
}

// ====== 初始化辅助函数 ======

// createLLMClient 创建 LLM 客户端（OpenAI 兼容 + 重试）
func createLLMClient(cfg *config.Config) llm.Client {
	openaiBaseURL := cfg.LLM.BaseURL
	if !strings.HasSuffix(openaiBaseURL, "/v1") {
		openaiBaseURL += "/v1"
	}
	base := llmopenai.NewClient(openaiBaseURL, cfg.LLM.APIKey)

	// 包装重试
	maxRetries := cfg.LLM.MaxRetries
	if maxRetries <= 0 {
		return base
	}
	return llm.NewRetryClient(base, maxRetries, 1*time.Second)
}

// createRSSStore 创建 RSS Store
func createRSSStore(db *sql.DB) *rss.Store {
	if db != nil {
		return rss.NewStoreWithDB(db)
	}
	return rss.NewStore()
}

// createToolRegistry 创建并注册所有 Agent 工具
func createToolRegistry(cachedDS *datasource.CachedDataSource, emClient *eastmoney.Client, rssStore *rss.Store) *agent.ToolRegistry {
	toolRegistry := agent.NewToolRegistry()

	// 基金数据工具
	toolRegistry.Register(agent.NewGetFundInfoTool(cachedDS))
	toolRegistry.Register(agent.NewGetNAVHistoryTool(cachedDS))
	toolRegistry.Register(agent.NewGetFundHoldingsTool(cachedDS))

	// 新闻搜索
	toolRegistry.Register(agent.NewSearchNewsTool(rssStore))

	// 基金搜索 + 排行榜（对接东方财富真实 API）
	toolRegistry.Register(agent.NewSearchFundsTool(&eastmoneySearchAdapter{client: emClient}))
	toolRegistry.Register(agent.NewGetFundRankingTool(&eastmoneyRankingAdapter{client: emClient}))

	return toolRegistry
}

// createMemorySystem 创建记忆系统（需要 DB）
func createMemorySystem(db *sql.DB, llmClient llm.Client, tools *agent.ToolRegistry) (*memory.Store, *memory.Extractor) {
	if db == nil {
		return nil, nil
	}

	memStore := memory.NewStore(db)
	memExtractor := memory.NewExtractor(llmClient)
	logger.Info("记忆系统已启用")
	return memStore, memExtractor
}

// registerPortfolioTool 注册持仓管理工具
func registerPortfolioTool(toolRegistry *agent.ToolRegistry, memStore *memory.Store) {
	portfolioMgr := portfolio.NewManager(memStore)
	toolRegistry.Register(agent.NewGetPortfolioTool(
		agent.NewPortfolioAdapter(func(ctx context.Context) string {
			p, err := portfolioMgr.GetPortfolio(ctx, "default")
			if err != nil {
				return "获取持仓失败: " + err.Error()
			}
			return p.FormatAsText()
		}),
	))
}

// registerPortfolioAPI 注册持仓 REST API
func registerPortfolioAPI(v1 *gin.RouterGroup, memStore *memory.Store, scanner *vision.Scanner) {
	portfolioMgrAPI := portfolio.NewManager(memStore)

	// 查询持仓
	v1.GET("/portfolio", func(c *gin.Context) {
		p, err := portfolioMgrAPI.GetPortfolio(c.Request.Context(), "default")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data":        p.Holdings,
			"total":       len(p.Holdings),
			"total_value": p.TotalValue,
		})
	})

	// 添加持仓 — 向记忆系统写入一条 fact
	v1.POST("/portfolio", func(c *gin.Context) {
		var req struct {
			Code   string  `json:"code" binding:"required"`
			Name   string  `json:"name"`
			Amount float64 `json:"amount"` // 持有金额（元），可选
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供基金代码 code"})
			return
		}
		if len(req.Code) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "基金代码必须为6位数字"})
			return
		}

		// 使用统一格式，与 portfolio.parseHoldingFromMemory 兼容
		content := portfolio.FormatMemoryContent(req.Code, req.Name, req.Amount)

		entry := memory.MemoryEntry{
			UserID:     "default",
			Type:       memory.TypeFact,
			Content:    content,
			Importance: 0.9,
		}
		if err := memStore.Save(c.Request.Context(), []memory.MemoryEntry{entry}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存持仓失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "code": req.Code})
	})

	// 删除持仓 — 使包含该基金代码的持仓记忆失效
	v1.DELETE("/portfolio/:code", func(c *gin.Context) {
		code := c.Param("code")
		if len(code) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "基金代码必须为6位数字"})
			return
		}

		// 找到包含该代码的持仓记忆并使其失效
		memories, err := memStore.Recall(c.Request.Context(), "default", "持有 "+code, 20)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		invalidated := 0
		for _, mem := range memories {
			if mem.Type == memory.TypeFact && strings.Contains(mem.Content, code) && strings.Contains(mem.Content, "持有") {
				if err := memStore.Invalidate(c.Request.Context(), mem.ID); err == nil {
					invalidated++
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "invalidated": invalidated})
	})

	// 截图识别持仓
	v1.POST("/portfolio/scan", func(c *gin.Context) {
		var req struct {
			Image   string `json:"image" binding:"required"` // base64 编码的图片（支持 data URI）
			AutoAdd bool   `json:"auto_add"`                 // 是否自动添加到持仓
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 image 字段（base64 编码的持仓截图）"})
			return
		}

		result, err := scanner.ScanPortfolio(c.Request.Context(), req.Image)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "识别失败: " + err.Error()})
			return
		}

		if result.Error != "" {
			c.JSON(http.StatusOK, gin.H{
				"holdings":    result.Holdings,
				"total_value": result.TotalValue,
				"error":       result.Error,
			})
			return
		}

		// 自动添加到持仓
		if req.AutoAdd && len(result.Holdings) > 0 {
			added := 0
			for _, h := range result.Holdings {
				content := portfolio.FormatMemoryContentFull(h.Code, h.Name, h.Amount, h.TotalProfit, h.TotalProfitRate)
				entry := memory.MemoryEntry{
					UserID:     "default",
					Type:       memory.TypeFact,
					Content:    content,
					Importance: 0.9,
				}
				if err := memStore.Save(c.Request.Context(), []memory.MemoryEntry{entry}); err == nil {
					added++
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"holdings":    result.Holdings,
				"total_value": result.TotalValue,
				"auto_added":  added,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"holdings":    result.Holdings,
			"total_value": result.TotalValue,
		})
	})
}

// registerBriefsAPI 注册简报 API
func registerBriefsAPI(v1 *gin.RouterGroup, db *sql.DB, briefTask *scheduler.DailyBriefTask) {
	v1.GET("/briefs", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusOK, gin.H{"data": []any{}, "total": 0})
			return
		}
		rows, err := db.Query(`SELECT id, content, type, created_at FROM briefs ORDER BY created_at DESC LIMIT 5`)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"data": []any{}, "total": 0})
			return
		}
		defer rows.Close()
		type Brief struct {
			ID        int    `json:"id"`
			Content   string `json:"content"`
			Type      string `json:"type"`
			CreatedAt string `json:"created_at"`
		}
		var briefs []Brief
		for rows.Next() {
			var b Brief
			if err := rows.Scan(&b.ID, &b.Content, &b.Type, &b.CreatedAt); err != nil {
				continue
			}
			briefs = append(briefs, b)
		}
		if briefs == nil {
			briefs = []Brief{}
		}
		c.JSON(http.StatusOK, gin.H{"data": briefs, "total": len(briefs)})
	})

	// 手动生成简报
	v1.POST("/briefs/generate", func(c *gin.Context) {
		if briefTask == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "简报任务未初始化"})
			return
		}
		go func() {
			if err := briefTask.Run(context.Background()); err != nil {
				logger.Error("手动生成简报失败", zap.Error(err))
			}
		}()
		c.JSON(http.StatusOK, gin.H{"status": "generating", "message": "简报生成中，请稍后刷新"})
	})
}

// createScheduler 创建定时任务调度器
func createScheduler(briefTask *scheduler.DailyBriefTask, llmClient llm.Client, tools *agent.ToolRegistry, memStore *memory.Store) *scheduler.Scheduler {
	sched := scheduler.New()

	// 每日投资简报（每 12 小时）
	sched.Register(briefTask, 12*time.Hour)

	// 净值异动监控（每 4 小时）
	sched.Register(scheduler.NewNAVMonitorTask(
		tools, memStore,
		func(ctx context.Context, alert string) error {
			logger.Warn("净值异动预警:\n" + alert)
			return nil
		},
	), 4*time.Hour)

	// 周度复盘（每 7 天）
	sched.Register(scheduler.NewWeeklyReviewTask(
		llmClient, memStore,
		func(ctx context.Context, review string) error {
			logger.Info("周度复盘已生成", zap.Int("length", len(review)))
			return nil
		},
	), 7*24*time.Hour)

	return sched
}

// createRSSScheduler 创建并启动 RSS 抓取调度器
func createRSSScheduler(cfg *config.Config, store *rss.Store, llmClient llm.Client) *rss.Scheduler {
	fetcher := rss.NewFetcher()
	rssSched := rss.NewScheduler(fetcher, store)

	// 设置 AI 摘要生成器（使用轻量模型）
	summarizer := rss.NewSummarizer(llmClient)
	rssSched.SetSummarizer(summarizer)

	// 从配置添加订阅源
	for _, feedCfg := range cfg.RSS.Feeds {
		feed := rss.NewFeed(feedCfg.URL, feedCfg.Name)
		feed.Interval = feedCfg.GetFeedInterval()
		feed.Enabled = true
		rssSched.AddFeed(feed) // AddFeed 会立即触发首次抓取
	}

	rssSched.Start()

	logger.Info("RSS 抓取调度器已启动",
		zap.Int("feeds", len(cfg.RSS.Feeds)),
	)

	return rssSched
}

// ====== 东方财富适配器 ======

// eastmoneySearchAdapter 适配 eastmoney.Client → agent.FundSearcher
type eastmoneySearchAdapter struct {
	client *eastmoney.Client
}

func (a *eastmoneySearchAdapter) SearchFunds(ctx context.Context, keyword string) ([]agent.FundSearchResult, error) {
	results, err := a.client.SearchFunds(ctx, keyword)
	if err != nil {
		return nil, err
	}
	out := make([]agent.FundSearchResult, len(results))
	for i, r := range results {
		out[i] = agent.FundSearchResult{Code: r.Code, Name: r.Name, Type: r.Type}
	}
	return out, nil
}

// eastmoneyRankingAdapter 适配 eastmoney.Client → agent.FundRankingProvider
type eastmoneyRankingAdapter struct {
	client *eastmoney.Client
}

func (a *eastmoneyRankingAdapter) GetFundRanking(ctx context.Context, sortType string, fundType int, count int) ([]agent.FundRankItem, error) {
	items, err := a.client.GetFundRanking(ctx, sortType, fundType, count)
	if err != nil {
		return nil, err
	}
	out := make([]agent.FundRankItem, len(items))
	for i, item := range items {
		out[i] = agent.FundRankItem{
			Code:             item.Code,
			Name:             item.Name,
			NAV:              item.NAV,
			DailyReturn:      item.DailyReturn,
			WeekReturn:       item.WeekReturn,
			MonthReturn:      item.MonthReturn,
			ThreeMonthReturn: item.ThreeMonthReturn,
			YearReturn:       item.YearReturn,
		}
	}
	return out, nil
}

// ====== 调仓检测适配器 ======

// rebalanceAdapter 适配 analyzer.FundAnalyzer → agent.rebalanceDetector
type rebalanceAdapter struct {
	analyzer analyzer.FundAnalyzer
}

func (a *rebalanceAdapter) DetectRebalance(ctx context.Context, code string) (string, error) {
	result, err := a.analyzer.DetectRebalance(ctx, code)
	if err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}

// ====== 量化数据加载适配器 ======

// quantDataLoaderAdapter 适配 datasource.FundDataSource → agent.QuantDataLoader
type quantDataLoaderAdapter struct {
	ds datasource.FundDataSource
}

func (a *quantDataLoaderAdapter) LoadNAVSeries(ctx context.Context, code string, days int) (*quant.NavSeries, error) {
	navList, err := a.ds.GetFundNAV(ctx, code, days)
	if err != nil {
		return nil, err
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

func (a *quantDataLoaderAdapter) GetFundName(ctx context.Context, code string) string {
	fund, err := a.ds.GetFundInfo(ctx, code)
	if err != nil {
		return code
	}
	return fund.Name
}

// ====== 通用处理器 ======

// requestLogger 请求日志中间件
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		status := c.Writer.Status()
		logger.Info("HTTP 请求",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
		)
	}
}

// healthCheck 健康检查端点
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

// ping 测试端点
func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
