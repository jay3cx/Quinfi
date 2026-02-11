// Package rss 提供定时调度器
package rss

import (
	"context"
	"sync"
	"time"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Scheduler RSS 抓取调度器
type Scheduler struct {
	fetcher    *Fetcher
	store      *Store
	summarizer *Summarizer
	feeds      map[string]*scheduledFeed
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type scheduledFeed struct {
	feed   *Feed
	ticker *time.Ticker
	stopCh chan struct{}
}

// NewScheduler 创建调度器
func NewScheduler(fetcher *Fetcher, store *Store) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		fetcher:    fetcher,
		store:      store,
		summarizer: nil, // 可选，通过 SetSummarizer 设置
		feeds:      make(map[string]*scheduledFeed),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetSummarizer 设置摘要生成器
func (s *Scheduler) SetSummarizer(summarizer *Summarizer) {
	s.summarizer = summarizer
}

// AddFeed 添加订阅源
func (s *Scheduler) AddFeed(feed *Feed) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果已存在，先停止
	if sf, exists := s.feeds[feed.ID]; exists {
		close(sf.stopCh)
	}

	// 保存到存储
	s.store.SaveFeed(feed)

	if !feed.Enabled {
		return
	}

	// 创建调度
	interval := feed.Interval
	if interval == 0 {
		interval = DefaultInterval
	}

	sf := &scheduledFeed{
		feed:   feed,
		ticker: time.NewTicker(interval),
		stopCh: make(chan struct{}),
	}
	s.feeds[feed.ID] = sf

	// 启动抓取协程
	s.wg.Add(1)
	go s.runFeed(sf)

	// 立即抓取一次
	go s.fetchFeed(feed)

	logger.Info("添加 RSS 订阅",
		zap.String("id", feed.ID),
		zap.String("url", feed.URL),
		zap.Duration("interval", interval),
	)
}

// RemoveFeed 移除订阅源
func (s *Scheduler) RemoveFeed(feedID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sf, exists := s.feeds[feedID]; exists {
		close(sf.stopCh)
		delete(s.feeds, feedID)
		s.store.DeleteFeed(feedID)
		logger.Info("移除 RSS 订阅", zap.String("id", feedID))
	}
}

// runFeed 运行单个订阅源的抓取循环
func (s *Scheduler) runFeed(sf *scheduledFeed) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			sf.ticker.Stop()
			return
		case <-sf.stopCh:
			sf.ticker.Stop()
			return
		case <-sf.ticker.C:
			s.fetchFeed(sf.feed)
		}
	}
}

// fetchFeed 抓取单个订阅源
func (s *Scheduler) fetchFeed(feed *Feed) {
	ctx, cancel := context.WithTimeout(s.ctx, 60*time.Second)
	defer cancel()

	_, articles, err := s.fetcher.Fetch(ctx, feed.URL)
	if err != nil {
		logger.Error("RSS 抓取失败",
			zap.String("url", feed.URL),
			zap.Error(err),
		)
		return
	}

	saved := s.store.SaveArticles(articles)
	if saved > 0 {
		logger.Info("保存新文章",
			zap.String("feed", feed.Title),
			zap.Int("saved", saved),
			zap.Int("total", len(articles)),
		)

		// 异步生成摘要
		if s.summarizer != nil {
			go s.summarizeNewArticles(articles)
		}
	}
}

// summarizeNewArticles 为新文章生成摘要并持久化
func (s *Scheduler) summarizeNewArticles(articles []*Article) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	success, fail := s.summarizer.SummarizeBatch(ctx, articles)

	// 将摘要结果回写到 Store（内存 + DB）
	persisted := 0
	for _, a := range articles {
		if a.Summary != "" {
			s.store.UpdateArticle(a)
			persisted++
		}
	}

	if success > 0 || fail > 0 {
		logger.Info("批量摘要完成",
			zap.Int("success", success),
			zap.Int("fail", fail),
			zap.Int("persisted", persisted),
		)
	}
}

// Start 启动调度器
func (s *Scheduler) Start() {
	logger.Info("RSS 调度器启动")

	// 启动时为缺少摘要的已有文章补跑摘要（异步，不阻塞启动）
	if s.summarizer != nil {
		go s.backfillSummaries()
	}
}

// backfillSummaries 为已有但缺少摘要的文章补跑 AI 摘要
func (s *Scheduler) backfillSummaries() {
	// 等待 RSS 首次抓取完成
	select {
	case <-time.After(30 * time.Second):
	case <-s.ctx.Done():
		return
	}

	articles := s.store.GetAllArticles(0)
	var needSummary []*Article
	for _, a := range articles {
		if a.Summary == "" && a.Title != "" {
			needSummary = append(needSummary, a)
		}
	}

	if len(needSummary) == 0 {
		return
	}

	logger.Info("开始补跑文章摘要",
		zap.Int("need_summary", len(needSummary)),
		zap.Int("total", len(articles)),
	)

	// 分批处理，每批 20 篇，避免一次性发太多 LLM 请求
	batchSize := 20
	for i := 0; i < len(needSummary); i += batchSize {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		end := i + batchSize
		if end > len(needSummary) {
			end = len(needSummary)
		}
		batch := needSummary[i:end]

		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Minute)
		success, fail := s.summarizer.SummarizeBatch(ctx, batch)
		cancel()

		// 回写到 Store
		for _, a := range batch {
			if a.Summary != "" {
				s.store.UpdateArticle(a)
			}
		}

		logger.Info("摘要补跑进度",
			zap.Int("batch_success", success),
			zap.Int("batch_fail", fail),
			zap.Int("progress", min(end, len(needSummary))),
			zap.Int("total", len(needSummary)),
		)

		// 批间间隔，避免 LLM API 限流
		select {
		case <-time.After(5 * time.Second):
		case <-s.ctx.Done():
			return
		}
	}

	logger.Info("文章摘要补跑完成")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	logger.Info("RSS 调度器停止")
}

// GetStore 获取存储
func (s *Scheduler) GetStore() *Store {
	return s.store
}

// FeedCount 返回订阅源数量
func (s *Scheduler) FeedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.feeds)
}
