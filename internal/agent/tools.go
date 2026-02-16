// Package agent 提供内置工具实现
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jay3cx/Quinfi/internal/datasource"
	"github.com/jay3cx/Quinfi/internal/rss"
)

// ===== GetFundInfo 工具 =====

// GetFundInfoTool 获取基金基本信息
type GetFundInfoTool struct {
	ds datasource.FundDataSource
}

func NewGetFundInfoTool(ds datasource.FundDataSource) *GetFundInfoTool {
	return &GetFundInfoTool{ds: ds}
}

func (t *GetFundInfoTool) Name() string { return "get_fund_info" }

func (t *GetFundInfoTool) Description() string {
	return "获取基金基本信息，包括名称、类型、规模、基金公司、基金经理等。输入6位基金代码。"
}

func (t *GetFundInfoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码，如 005827",
			},
		},
		"required": []string{"code"},
	}
}

func (t *GetFundInfoTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}

	fund, err := t.ds.GetFundInfo(ctx, code)
	if err != nil {
		return "", fmt.Errorf("获取基金信息失败: %w", err)
	}

	result, _ := json.MarshalIndent(fund, "", "  ")
	return string(result), nil
}

// ===== GetNAVHistory 工具 =====

// GetNAVHistoryTool 获取基金净值走势
type GetNAVHistoryTool struct {
	ds datasource.FundDataSource
}

func NewGetNAVHistoryTool(ds datasource.FundDataSource) *GetNAVHistoryTool {
	return &GetNAVHistoryTool{ds: ds}
}

func (t *GetNAVHistoryTool) Name() string { return "get_nav_history" }

func (t *GetNAVHistoryTool) Description() string {
	return "获取基金净值历史走势，包括单位净值、累计净值和日涨幅。可指定天数。"
}

func (t *GetNAVHistoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码",
			},
			"days": map[string]any{
				"type":        "integer",
				"description": "获取最近多少天的数据，默认30天",
			},
		},
		"required": []string{"code"},
	}
}

func (t *GetNAVHistoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}
	days := getIntArg(args, "days", 30)

	navList, err := t.ds.GetFundNAV(ctx, code, days)
	if err != nil {
		return "", fmt.Errorf("获取净值数据失败: %w", err)
	}

	// 生成可读的文本摘要，避免返回过多原始数据
	if len(navList) == 0 {
		return "暂无净值数据", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("基金 %s 近 %d 天净值数据（共 %d 条）：\n", code, days, len(navList)))

	// 最新几条
	showCount := min(len(navList), 10)
	for i := 0; i < showCount; i++ {
		nav := navList[i]
		sb.WriteString(fmt.Sprintf("- %s: 单位净值 %.4f, 日涨幅 %.2f%%\n", nav.Date, nav.UnitNAV, nav.DailyReturn))
	}

	if len(navList) > 10 {
		sb.WriteString(fmt.Sprintf("...（省略 %d 条）\n", len(navList)-10))
		// 最早的一条
		last := navList[len(navList)-1]
		sb.WriteString(fmt.Sprintf("- %s: 单位净值 %.4f, 日涨幅 %.2f%%\n", last.Date, last.UnitNAV, last.DailyReturn))
	}

	// 统计信息
	latest := navList[0]
	oldest := navList[len(navList)-1]
	returnRate := (latest.UnitNAV - oldest.UnitNAV) / oldest.UnitNAV * 100
	sb.WriteString(fmt.Sprintf("\n期间收益率: %.2f%% (%.4f → %.4f)", returnRate, oldest.UnitNAV, latest.UnitNAV))

	return sb.String(), nil
}

// ===== GetFundHoldings 工具 =====

// GetFundHoldingsTool 获取基金持仓
type GetFundHoldingsTool struct {
	ds datasource.FundDataSource
}

func NewGetFundHoldingsTool(ds datasource.FundDataSource) *GetFundHoldingsTool {
	return &GetFundHoldingsTool{ds: ds}
}

func (t *GetFundHoldingsTool) Name() string { return "get_fund_holdings" }

func (t *GetFundHoldingsTool) Description() string {
	return "获取基金前十大重仓股持仓明细，包括股票代码、名称、持仓占比和市值。"
}

func (t *GetFundHoldingsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码",
			},
		},
		"required": []string{"code"},
	}
}

func (t *GetFundHoldingsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}

	holdings, err := t.ds.GetFundHoldings(ctx, code)
	if err != nil {
		return "", fmt.Errorf("获取持仓数据失败: %w", err)
	}

	if len(holdings) == 0 {
		return "暂无持仓数据（持仓信息通常每季度披露一次）", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("基金 %s 前十大重仓股：\n", code))

	totalRatio := 0.0
	for i, h := range holdings {
		sb.WriteString(fmt.Sprintf("%d. %s（%s）: 占比 %.2f%%", i+1, h.StockName, h.StockCode, h.Ratio))
		if h.MarketValue > 0 {
			sb.WriteString(fmt.Sprintf(", 市值 %.0f万元", h.MarketValue))
		}
		sb.WriteString("\n")
		totalRatio += h.Ratio
	}
	sb.WriteString(fmt.Sprintf("\n前十大持仓合计: %.2f%%", totalRatio))

	return sb.String(), nil
}

// ===== SearchNews 工具 =====

// SearchNewsTool 搜索相关新闻资讯
type SearchNewsTool struct {
	store *rss.Store
}

func NewSearchNewsTool(store *rss.Store) *SearchNewsTool {
	return &SearchNewsTool{store: store}
}

func (t *SearchNewsTool) Name() string { return "search_news" }

func (t *SearchNewsTool) Description() string {
	return "搜索金融新闻资讯，可按关键词过滤。返回最新的相关新闻标题、摘要和情绪判断。"
}

func (t *SearchNewsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "搜索关键词（可选，为空则返回最新资讯）",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "返回条数，默认5条",
			},
		},
	}
}

func (t *SearchNewsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	keyword := getStringArg(args, "keyword")
	limit := getIntArg(args, "limit", 5)

	var articles []*rss.Article

	if keyword != "" {
		// 有关键词 → FTS5 全文搜索（自动降级为内存匹配）
		articles = t.store.SearchArticles(keyword, limit)
	} else {
		// 无关键词 → 返回最新资讯
		articles = t.store.GetAllArticles(limit)
	}

	if len(articles) == 0 {
		if keyword != "" {
			return fmt.Sprintf("未找到与 \"%s\" 相关的新闻", keyword), nil
		}
		return "暂无新闻资讯", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条相关资讯：\n\n", len(articles)))

	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, a.Title))
		if a.Summary != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", a.Summary))
		}
		if a.Sentiment != "" {
			sb.WriteString(fmt.Sprintf("   情绪: %s", a.Sentiment))
			if a.SentimentReason != "" {
				sb.WriteString(fmt.Sprintf("（%s）", a.SentimentReason))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("   时间: %s\n\n", a.PubDate.Format("2006-01-02 15:04")))
	}

	return sb.String(), nil
}

// ===== RunDebate 工具 =====

// RunDebateTool 多空辩论工具
// 通过 debate.Orchestrator 编排 Bull/Bear/Judge 辩论
type RunDebateTool struct {
	orchestrator debateRunner
}

// debateRunner 辩论编排器接口（解耦 debate 包的直接依赖）
type debateRunner interface {
	RunDebate(ctx context.Context, fundCode string, onPhase ...func(phaseJSON string)) (formattedResult string, err error)
}

func NewRunDebateTool(runner debateRunner) *RunDebateTool {
	return &RunDebateTool{orchestrator: runner}
}

func (t *RunDebateTool) Name() string { return "run_debate" }

func (t *RunDebateTool) Description() string {
	return "对指定基金进行多空辩论分析。Bull（看多）和 Bear（看空）双方各自立论并反驳，最后由 Judge 给出综合评判。适用于用户想要多角度分析、决策挑战、或要求辩论时使用。"
}

func (t *RunDebateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码，如 005827",
			},
		},
		"required": []string{"code"},
	}
}

func (t *RunDebateTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}

	// 如果有流式进度回调（RunStream 模式），传递给辩论编排器
	if progressFn := ToolProgressFromContext(ctx); progressFn != nil {
		return t.orchestrator.RunDebate(ctx, code, func(phaseJSON string) {
			progressFn(StreamChunk{
				Type:     ChunkDebatePhase,
				ToolName: "run_debate",
				Content:  phaseJSON,
			})
		})
	}

	return t.orchestrator.RunDebate(ctx, code)
}

// ===== SearchFunds 工具 =====

// FundSearchResult 基金搜索结果（解耦数据源实现）
type FundSearchResult struct {
	Code string
	Name string
	Type string
}

// FundSearcher 基金搜索接口
type FundSearcher interface {
	SearchFunds(ctx context.Context, keyword string) ([]FundSearchResult, error)
}

// SearchFundsTool 全市场基金搜索
type SearchFundsTool struct {
	searcher FundSearcher
}

func NewSearchFundsTool(searcher FundSearcher) *SearchFundsTool {
	return &SearchFundsTool{searcher: searcher}
}

func (t *SearchFundsTool) Name() string { return "search_funds" }

func (t *SearchFundsTool) Description() string {
	return "按关键词搜索全市场基金。可以搜基金名称、基金公司、主题（如'消费'、'新能源'、'医药'）。返回匹配的基金列表。"
}

func (t *SearchFundsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword": map[string]any{
				"type":        "string",
				"description": "搜索关键词，如'消费'、'新能源'、'易方达'",
			},
		},
		"required": []string{"keyword"},
	}
}

func (t *SearchFundsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	keyword := getStringArg(args, "keyword")
	if keyword == "" {
		return "", fmt.Errorf("关键词不能为空")
	}

	results, err := t.searcher.SearchFunds(ctx, keyword)
	if err != nil {
		return "", fmt.Errorf("搜索基金失败: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("未找到与 \"%s\" 相关的基金", keyword), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索 \"%s\" 找到 %d 只基金：\n\n", keyword, len(results)))

	showCount := min(len(results), 20)
	for i := 0; i < showCount; i++ {
		r := results[i]
		sb.WriteString(fmt.Sprintf("%d. %s（%s）", i+1, r.Name, r.Code))
		if r.Type != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", r.Type))
		}
		sb.WriteString("\n")
	}
	if len(results) > 20 {
		sb.WriteString(fmt.Sprintf("...（共 %d 只，仅显示前 20 只）\n", len(results)))
	}

	sb.WriteString("\n可以告诉我感兴趣的基金代码，我来查询详细信息。")
	return sb.String(), nil
}

// ===== GetFundRanking 工具 =====

// FundRankItem 基金排行条目（解耦数据源实现）
type FundRankItem struct {
	Code             string
	Name             string
	NAV              float64
	DailyReturn      float64
	WeekReturn       float64
	MonthReturn      float64
	ThreeMonthReturn float64
	YearReturn       float64
}

// FundRankingProvider 基金排行榜数据提供者接口
type FundRankingProvider interface {
	GetFundRanking(ctx context.Context, sortType string, fundType int, count int) ([]FundRankItem, error)
}

// GetFundRankingTool 获取基金排行榜
type GetFundRankingTool struct {
	provider FundRankingProvider
}

func NewGetFundRankingTool(provider FundRankingProvider) *GetFundRankingTool {
	return &GetFundRankingTool{provider: provider}
}

func (t *GetFundRankingTool) Name() string { return "get_fund_ranking" }

func (t *GetFundRankingTool) Description() string {
	return "获取基金排行榜。可按近1周/1月/3月/1年/3年排序，可筛选基金类型（股票型/混合型/债券型/指数型）。用于发现表现最好的基金。"
}

func (t *GetFundRankingTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sort_by": map[string]any{
				"type":        "string",
				"description": "排序维度: 1w(近1周) / 1m(近1月) / 3m(近3月) / 1y(近1年) / 3y(近3年)",
				"enum":        []string{"1w", "1m", "3m", "1y", "3y"},
			},
			"fund_type": map[string]any{
				"type":        "string",
				"description": "基金类型: all(全部) / stock(股票型) / mixed(混合型) / bond(债券型) / index(指数型)",
				"enum":        []string{"all", "stock", "mixed", "bond", "index"},
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "返回条数，默认10",
			},
		},
	}
}

func (t *GetFundRankingTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	sortBy := getStringArg(args, "sort_by")
	if sortBy == "" {
		sortBy = "1y"
	}
	fundTypeStr := getStringArg(args, "fund_type")
	count := getIntArg(args, "count", 10)

	fundType := fundTypeToInt(fundTypeStr)

	items, err := t.provider.GetFundRanking(ctx, sortBy, fundType, count)
	if err != nil {
		return "", fmt.Errorf("获取排行榜失败: %w", err)
	}

	if len(items) == 0 {
		return "暂无排行榜数据", nil
	}

	sortLabel := sortByLabel(sortBy)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("基金排行榜（排序: %s, 前 %d 名）：\n\n", sortLabel, len(items)))

	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s（%s）\n", i+1, item.Name, item.Code))
		sb.WriteString(fmt.Sprintf("   最新净值: %.4f | 日涨幅: %.2f%%\n", item.NAV, item.DailyReturn))
		sb.WriteString(fmt.Sprintf("   近1周: %.2f%% | 近1月: %.2f%% | 近3月: %.2f%% | 近1年: %.2f%%\n",
			item.WeekReturn, item.MonthReturn, item.ThreeMonthReturn, item.YearReturn))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// fundTypeToInt 将基金类型字符串映射为东方财富 API 参数
func fundTypeToInt(t string) int {
	switch t {
	case "stock":
		return 1
	case "bond":
		return 2
	case "mixed":
		return 3
	case "index":
		return 5
	default:
		return 0 // 全部
	}
}

// sortByLabel 排序维度的中文标签
func sortByLabel(s string) string {
	switch s {
	case "1w":
		return "近1周"
	case "1m":
		return "近1月"
	case "3m":
		return "近3月"
	case "1y":
		return "近1年"
	case "3y":
		return "近3年"
	default:
		return s
	}
}

// ===== DetectRebalance 工具 =====

// rebalanceDetector 调仓检测接口（解耦 analyzer 包）
type rebalanceDetector interface {
	DetectRebalance(ctx context.Context, code string) (string, error)
}

// DetectRebalanceTool 调仓检测工具
type DetectRebalanceTool struct {
	detector rebalanceDetector
}

func NewDetectRebalanceTool(detector rebalanceDetector) *DetectRebalanceTool {
	return &DetectRebalanceTool{detector: detector}
}

func (t *DetectRebalanceTool) Name() string { return "detect_rebalance" }

func (t *DetectRebalanceTool) Description() string {
	return "检测基金的调仓变动，对比上期和本期持仓差异，分析基金经理的调仓方向和意图。输入6位基金代码。"
}

func (t *DetectRebalanceTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "6位基金代码，如 005827",
			},
		},
		"required": []string{"code"},
	}
}

func (t *DetectRebalanceTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	code := getStringArg(args, "code")
	if code == "" {
		return "", fmt.Errorf("基金代码不能为空")
	}
	return t.detector.DetectRebalance(ctx, code)
}

// ===== GetPortfolio 工具 =====

// portfolioProvider 持仓数据提供者接口
type portfolioProvider interface {
	GetPortfolioText(ctx context.Context) string
}

// GetPortfolioTool 获取用户持仓组合
type GetPortfolioTool struct {
	provider portfolioProvider
}

func NewGetPortfolioTool(provider portfolioProvider) *GetPortfolioTool {
	return &GetPortfolioTool{provider: provider}
}

func (t *GetPortfolioTool) Name() string { return "get_portfolio" }

func (t *GetPortfolioTool) Description() string {
	return "获取用户当前的基金持仓组合。包括持有的基金代码、名称和仓位比例。用于分析持仓配置、计算行业暴露、评估组合风险。"
}

func (t *GetPortfolioTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *GetPortfolioTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.provider.GetPortfolioText(ctx), nil
}

// ===== PortfolioAdapter =====

// PortfolioAdapter 适配 portfolio.Manager 到 portfolioProvider 接口
type PortfolioAdapter struct {
	getFunc func(ctx context.Context) string
}

func NewPortfolioAdapter(getFunc func(ctx context.Context) string) *PortfolioAdapter {
	return &PortfolioAdapter{getFunc: getFunc}
}

func (a *PortfolioAdapter) GetPortfolioText(ctx context.Context) string {
	return a.getFunc(ctx)
}
