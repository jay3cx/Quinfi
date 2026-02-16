// Package llm 提供 LLM 重试包装器
package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// RetryClient 带指数退避重试的 LLM 客户端包装器
type RetryClient struct {
	inner      Client
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewRetryClient 创建重试客户端
// maxRetries: 最大重试次数（0 = 不重试）
// baseDelay: 基础延迟（每次重试翻倍）
func NewRetryClient(inner Client, maxRetries int, baseDelay time.Duration) *RetryClient {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	return &RetryClient{
		inner:      inner,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   30 * time.Second,
	}
}

// Chat 同步对话调用（带重试）
func (c *RetryClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if attempt < c.maxRetries {
			delay := c.calcDelay(attempt)
			logger.Warn("LLM 调用失败，准备重试",
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", c.maxRetries),
				zap.Duration("delay", delay),
				zap.Error(err),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("LLM 调用失败（重试 %d 次后）: %w", c.maxRetries, lastErr)
}

// ChatStream 流式对话调用（带重试，仅重试连接阶段）
func (c *RetryClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		ch, err := c.inner.ChatStream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err

		if attempt < c.maxRetries {
			delay := c.calcDelay(attempt)
			logger.Warn("LLM 流式连接失败，准备重试",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", delay),
				zap.Error(err),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("LLM 流式调用失败（重试 %d 次后）: %w", c.maxRetries, lastErr)
}

// calcDelay 计算指数退避延迟
func (c *RetryClient) calcDelay(attempt int) time.Duration {
	delay := c.baseDelay * time.Duration(1<<uint(attempt))
	if delay > c.maxDelay {
		delay = c.maxDelay
	}
	return delay
}

// 确保 RetryClient 实现 Client 接口
var _ Client = (*RetryClient)(nil)
