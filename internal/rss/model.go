// Package rss 提供 RSS 数据模型
package rss

import "time"

// Feed RSS 订阅源
type Feed struct {
	ID          string        `json:"id"`           // 唯一标识
	URL         string        `json:"url"`          // 订阅地址
	Title       string        `json:"title"`        // 源标题
	Description string        `json:"description"`  // 描述
	LastFetched time.Time     `json:"last_fetched"` // 最后抓取时间
	Interval    time.Duration `json:"interval"`     // 抓取间隔
	Enabled     bool          `json:"enabled"`      // 是否启用
}

// Sentiment 情绪类型
type Sentiment string

const (
	SentimentPositive Sentiment = "positive" // 利好
	SentimentNegative Sentiment = "negative" // 利空
	SentimentNeutral  Sentiment = "neutral"  // 中性
)

// Article 新闻文章
type Article struct {
	GUID        string    `json:"guid"`         // 唯一标识
	FeedID      string    `json:"feed_id"`      // 所属订阅源
	Title       string    `json:"title"`        // 标题
	Link        string    `json:"link"`         // 原文链接
	Description string    `json:"description"`  // 内容摘要
	Content     string    `json:"content"`      // 完整内容
	Author      string    `json:"author"`       // 作者
	PubDate     time.Time `json:"pub_date"`     // 发布时间
	FetchedAt   time.Time `json:"fetched_at"`   // 抓取时间
	Source      string    `json:"source"`       // 来源名称

	// AI 摘要字段
	Summary        string    `json:"summary,omitempty"`         // AI 生成的摘要
	Sentiment      Sentiment `json:"sentiment,omitempty"`       // 情绪倾向
	SentimentReason string   `json:"sentiment_reason,omitempty"` // 情绪判断理由
	Keywords       []string  `json:"keywords,omitempty"`        // 关键词列表
	SummarizedAt   time.Time `json:"summarized_at,omitempty"`   // 摘要生成时间
}

// FeedConfig 订阅源配置
type FeedConfig struct {
	URL      string        `yaml:"url"`
	Name     string        `yaml:"name"`
	Interval time.Duration `yaml:"interval"`
	Enabled  bool          `yaml:"enabled"`
}

// DefaultInterval 默认抓取间隔
const DefaultInterval = 15 * time.Minute

// NewFeed 创建订阅源
func NewFeed(url, title string) *Feed {
	return &Feed{
		ID:       generateID(url),
		URL:      url,
		Title:    title,
		Interval: DefaultInterval,
		Enabled:  true,
	}
}

// generateID 生成订阅源 ID
func generateID(url string) string {
	// 简单使用 URL 哈希作为 ID
	h := 0
	for _, c := range url {
		h = 31*h + int(c)
	}
	if h < 0 {
		h = -h
	}
	return string(rune('a'+h%26)) + string(rune('0'+h%10)) + url[len(url)-6:]
}
