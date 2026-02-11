// Package analyzer 提供基金分析器实现
package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/jay3cx/fundmind/internal/datasource"
	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// DefaultAnalyzer 默认基金分析器实现
type DefaultAnalyzer struct {
	dataSource datasource.FundDataSource
	llmClient  llm.Client
	cache      *AnalysisCache
}

// AnalysisCache 分析结果缓存
type AnalysisCache struct {
	mu      sync.RWMutex
	reports map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	report    *AnalysisReport
	expiresAt time.Time
}

// NewAnalysisCache 创建分析缓存
func NewAnalysisCache(ttl time.Duration) *AnalysisCache {
	return &AnalysisCache{
		reports: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get 获取缓存
func (c *AnalysisCache) Get(code string) (*AnalysisReport, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.reports[code]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.report, true
}

// Set 设置缓存
func (c *AnalysisCache) Set(code string, report *AnalysisReport) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reports[code] = &cacheEntry{
		report:    report,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// NewFundAnalyzer 创建基金分析器
func NewFundAnalyzer(ds datasource.FundDataSource, client llm.Client) *DefaultAnalyzer {
	return &DefaultAnalyzer{
		dataSource: ds,
		llmClient:  client,
		cache:      NewAnalysisCache(1 * time.Hour),
	}
}

// Analyze 综合分析
func (a *DefaultAnalyzer) Analyze(ctx context.Context, code string, forceRefresh bool) (*AnalysisReport, error) {
	// 检查缓存
	if !forceRefresh {
		if cached, ok := a.cache.Get(code); ok {
			logger.Info("使用缓存的分析结果", zap.String("code", code))
			return cached, nil
		}
	}

	logger.Info("开始分析基金", zap.String("code", code))

	// 获取基金数据
	fund, err := a.dataSource.GetFundInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	holdings, err := a.dataSource.GetFundHoldings(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取持仓数据失败: %w", err)
	}

	navList, err := a.dataSource.GetFundNAV(ctx, code, 30)
	if err != nil {
		return nil, fmt.Errorf("获取净值数据失败: %w", err)
	}

	// 渲染 Prompt
	data := AnalyzeData{
		Fund:     fund,
		Holdings: holdings,
		NAVList:  navList,
	}

	prompt, err := RenderPrompt(PromptAnalyze, data)
	if err != nil {
		return nil, fmt.Errorf("渲染 Prompt 失败: %w", err)
	}

	// 调用 LLM
	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model: llm.GetDefaultModel(llm.TaskDaily),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	// 解析响应
	report, err := parseAnalysisReport(resp.Content, fund)
	if err != nil {
		return nil, fmt.Errorf("解析分析报告失败: %w", err)
	}

	// 缓存结果
	a.cache.Set(code, report)

	logger.Info("分析完成", zap.String("code", code))
	return report, nil
}

// AnalyzeHoldings 持仓分析
func (a *DefaultAnalyzer) AnalyzeHoldings(ctx context.Context, code string) (*HoldingAnalysis, error) {
	logger.Info("开始分析持仓", zap.String("code", code))

	fund, err := a.dataSource.GetFundInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	holdings, err := a.dataSource.GetFundHoldings(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取持仓数据失败: %w", err)
	}

	data := HoldingsData{
		Fund:     fund,
		Holdings: holdings,
	}

	prompt, err := RenderPrompt(PromptHoldings, data)
	if err != nil {
		return nil, fmt.Errorf("渲染 Prompt 失败: %w", err)
	}

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model: llm.GetDefaultModel(llm.TaskDaily),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	analysis, err := parseHoldingAnalysis(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("解析持仓分析失败: %w", err)
	}

	return analysis, nil
}

// DetectRebalance 调仓检测
func (a *DefaultAnalyzer) DetectRebalance(ctx context.Context, code string) (*RebalanceResult, error) {
	logger.Info("开始检测调仓", zap.String("code", code))

	fund, err := a.dataSource.GetFundInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	// 获取当前持仓
	currHoldings, err := a.dataSource.GetFundHoldings(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取持仓数据失败: %w", err)
	}

	// 注意：实际场景需要获取历史持仓数据
	// 这里使用当前持仓模拟，实际应从数据库或缓存获取上期数据
	prevHoldings := currHoldings // 临时使用当前持仓作为上期数据

	data := RebalanceData{
		FundCode:     code,
		FundName:     fund.Name,
		PrevDate:     "上期",
		CurrDate:     time.Now().Format("2006-01-02"),
		PrevHoldings: prevHoldings,
		CurrHoldings: currHoldings,
	}

	prompt, err := RenderPrompt(PromptRebalance, data)
	if err != nil {
		return nil, fmt.Errorf("渲染 Prompt 失败: %w", err)
	}

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model: llm.GetDefaultModel(llm.TaskDaily),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	result, err := parseRebalanceResult(resp.Content, code)
	if err != nil {
		return nil, fmt.Errorf("解析调仓结果失败: %w", err)
	}

	return result, nil
}

// extractJSON 从响应中提取 JSON
func extractJSON(content string) string {
	// 尝试匹配 JSON 块
	re := regexp.MustCompile(`(?s)\{.*\}`)
	match := re.FindString(content)
	if match != "" {
		return match
	}
	return content
}

// parseAnalysisReport 解析分析报告
func parseAnalysisReport(content string, fund *datasource.Fund) (*AnalysisReport, error) {
	jsonStr := extractJSON(content)

	var result struct {
		Summary         string           `json:"summary"`
		HoldingAnalysis *HoldingAnalysis `json:"holding_analysis"`
		RiskAssessment  *RiskAssessment  `json:"risk_assessment"`
		Recommendation  *Recommendation  `json:"recommendation"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, content: %s", err, jsonStr[:min(200, len(jsonStr))])
	}

	return &AnalysisReport{
		FundCode:        fund.Code,
		FundName:        fund.Name,
		Summary:         result.Summary,
		HoldingAnalysis: result.HoldingAnalysis,
		RiskAssessment:  result.RiskAssessment,
		Recommendation:  result.Recommendation,
		GeneratedAt:     time.Now().Format(time.RFC3339),
	}, nil
}

// parseHoldingAnalysis 解析持仓分析
func parseHoldingAnalysis(content string) (*HoldingAnalysis, error) {
	jsonStr := extractJSON(content)

	var analysis HoldingAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	return &analysis, nil
}

// parseRebalanceResult 解析调仓结果
func parseRebalanceResult(content string, code string) (*RebalanceResult, error) {
	jsonStr := extractJSON(content)

	var result struct {
		Changes []RebalanceInfo `json:"changes"`
		Summary string          `json:"summary"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	return &RebalanceResult{
		FundCode:    code,
		ReportDate:  time.Now().Format("2006-01-02"),
		Changes:     result.Changes,
		Summary:     result.Summary,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}, nil
}
