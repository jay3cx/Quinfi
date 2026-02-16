// Package rss 提供 RSS 摘要生成器
package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Summarizer RSS 文章摘要生成器
type Summarizer struct {
	client     llm.Client
	maxRetries int
	semaphore  chan struct{} // 并发控制
}

// SummarizerOption 摘要生成器配置选项
type SummarizerOption func(*Summarizer)

// WithMaxConcurrency 设置最大并发数
func WithMaxConcurrency(n int) SummarizerOption {
	return func(s *Summarizer) {
		s.semaphore = make(chan struct{}, n)
	}
}

// WithSummarizerRetries 设置最大重试次数
func WithSummarizerRetries(n int) SummarizerOption {
	return func(s *Summarizer) {
		s.maxRetries = n
	}
}

// NewSummarizer 创建摘要生成器
func NewSummarizer(client llm.Client, opts ...SummarizerOption) *Summarizer {
	s := &Summarizer{
		client:     client,
		maxRetries: 2,
		semaphore:  make(chan struct{}, 5), // 默认最大并发 5
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SummaryResult 摘要结果
type SummaryResult struct {
	Summary   string    `json:"summary"`
	Sentiment Sentiment `json:"sentiment"`
	Reason    string    `json:"reason"`
	Keywords  []string  `json:"keywords"`
}

// summaryPrompt 摘要生成 Prompt 模板
const summaryPrompt = `你是一个金融新闻分析专家。请分析以下新闻文章，并返回 JSON 格式的结果。

文章标题：%s
文章内容：%s

请返回以下 JSON 格式（不要添加其他文字）：
{
  "summary": "100字以内的中文摘要，保留核心观点和关键数据",
  "sentiment": "positive/negative/neutral 之一",
  "reason": "20字以内的情绪判断理由",
  "keywords": ["关键词1", "关键词2", "关键词3"]
}

注意：
- summary 必须是中文，即使原文是英文
- sentiment 只能是 positive（利好）、negative（利空）或 neutral（中性）
- keywords 提取 3-5 个关键词，优先提取公司名、行业、事件类型
- 每个 keyword 不超过 10 个字符`

// Summarize 生成单篇文章摘要
func (s *Summarizer) Summarize(ctx context.Context, article *Article) error {
	var lastErr error

	for i := 0; i <= s.maxRetries; i++ {
		if i > 0 {
			// 指数退避
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := s.doSummarize(ctx, article)
		if err == nil {
			return nil
		}

		lastErr = err
		logger.Warn("摘要生成失败，重试中",
			zap.String("title", article.Title),
			zap.Int("attempt", i+1),
			zap.Error(err),
		)
	}

	return fmt.Errorf("摘要生成失败（重试 %d 次）: %w", s.maxRetries, lastErr)
}

// doSummarize 执行摘要生成
func (s *Summarizer) doSummarize(ctx context.Context, article *Article) error {
	// 准备内容
	content := article.Content
	if content == "" {
		content = article.Description
	}
	if len(content) > 2000 {
		content = content[:2000] + "..."
	}

	prompt := fmt.Sprintf(summaryPrompt, article.Title, content)

	req := &llm.ChatRequest{
		Model: llm.GetDefaultModel(llm.TaskLight), // 使用 Gemini Flash
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   0,
		Temperature: 0.3,
	}

	resp, err := s.client.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("LLM 调用失败: %w", err)
	}

	// 解析 JSON 响应
	var result SummaryResult
	jsonStr := extractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fmt.Errorf("解析摘要结果失败: %w, 原始响应: %s", err, resp.Content)
	}

	// 更新文章
	article.Summary = result.Summary
	article.Sentiment = parseSentiment(result.Sentiment)
	article.SentimentReason = result.Reason
	article.Keywords = result.Keywords
	article.SummarizedAt = time.Now()

	logger.Info("摘要生成成功",
		zap.String("title", article.Title),
		zap.String("sentiment", string(article.Sentiment)),
		zap.Strings("keywords", article.Keywords),
	)

	return nil
}

// SummarizeBatch 批量生成摘要
func (s *Summarizer) SummarizeBatch(ctx context.Context, articles []*Article) (int, int) {
	var wg sync.WaitGroup
	var successCount, failCount int
	var mu sync.Mutex

	for _, article := range articles {
		// 跳过已有摘要的文章
		if article.Summary != "" {
			continue
		}

		wg.Add(1)
		go func(a *Article) {
			defer wg.Done()

			// 获取信号量
			select {
			case s.semaphore <- struct{}{}:
				defer func() { <-s.semaphore }()
			case <-ctx.Done():
				return
			}

			err := s.Summarize(ctx, a)
			mu.Lock()
			if err != nil {
				failCount++
			} else {
				successCount++
			}
			mu.Unlock()
		}(article)
	}

	wg.Wait()
	return successCount, failCount
}

// extractJSON 从响应中提取 JSON
func extractJSON(s string) string {
	// 移除 markdown 代码块标记
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	s = strings.TrimSpace(s)

	// 尝试找到 JSON 对象
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// parseSentiment 解析情绪字符串
func parseSentiment(s Sentiment) Sentiment {
	switch strings.ToLower(string(s)) {
	case "positive", "利好":
		return SentimentPositive
	case "negative", "利空":
		return SentimentNegative
	default:
		return SentimentNeutral
	}
}
