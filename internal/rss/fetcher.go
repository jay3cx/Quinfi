// Package rss 提供 RSS 抓取器
package rss

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Fetcher RSS 抓取器
type Fetcher struct {
	parser     *Parser
	httpClient *http.Client
	maxRetries int
}

// FetcherOption 抓取器配置选项
type FetcherOption func(*Fetcher)

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) FetcherOption {
	return func(f *Fetcher) {
		f.httpClient.Timeout = timeout
	}
}

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(retries int) FetcherOption {
	return func(f *Fetcher) {
		f.maxRetries = retries
	}
}

// NewFetcher 创建抓取器
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		parser: NewParser(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// Fetch 抓取 RSS 源
func (f *Fetcher) Fetch(ctx context.Context, url string) (*Feed, []*Article, error) {
	var lastErr error

	for i := 0; i <= f.maxRetries; i++ {
		if i > 0 {
			// 指数退避
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			logger.Info("RSS 抓取重试",
				zap.String("url", url),
				zap.Int("attempt", i+1),
				zap.Duration("backoff", backoff),
			)
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		feed, articles, err := f.parser.ParseURL(ctx, url)
		if err == nil {
			feed.LastFetched = time.Now()
			logger.Info("RSS 抓取成功",
				zap.String("url", url),
				zap.String("title", feed.Title),
				zap.Int("articles", len(articles)),
			)
			return feed, articles, nil
		}

		lastErr = err
		logger.Warn("RSS 抓取失败",
			zap.String("url", url),
			zap.Int("attempt", i+1),
			zap.Error(err),
		)
	}

	return nil, nil, fmt.Errorf("抓取失败（重试 %d 次）: %w", f.maxRetries, lastErr)
}

// FetchMultiple 并发抓取多个 RSS 源
func (f *Fetcher) FetchMultiple(ctx context.Context, urls []string) map[string]*FetchResult {
	results := make(map[string]*FetchResult)
	resultChan := make(chan *FetchResult, len(urls))

	for _, url := range urls {
		go func(u string) {
			feed, articles, err := f.Fetch(ctx, u)
			resultChan <- &FetchResult{
				URL:      u,
				Feed:     feed,
				Articles: articles,
				Error:    err,
			}
		}(url)
	}

	for range urls {
		result := <-resultChan
		results[result.URL] = result
	}

	return results
}

// FetchResult 抓取结果
type FetchResult struct {
	URL      string
	Feed     *Feed
	Articles []*Article
	Error    error
}
