// Package rss 提供 RSS 解析器
package rss

import (
	"context"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

// Parser RSS 解析器
type Parser struct {
	fp *gofeed.Parser
}

// NewParser 创建解析器
func NewParser() *Parser {
	return &Parser{
		fp: gofeed.NewParser(),
	}
}

// ParseURL 从 URL 解析 RSS
func (p *Parser) ParseURL(ctx context.Context, url string) (*Feed, []*Article, error) {
	feed, err := p.fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, nil, err
	}

	return p.convertFeed(feed, url)
}

// ParseString 从字符串解析 RSS
func (p *Parser) ParseString(content string) (*Feed, []*Article, error) {
	feed, err := p.fp.ParseString(content)
	if err != nil {
		return nil, nil, err
	}

	return p.convertFeed(feed, "")
}

// convertFeed 转换 gofeed.Feed 到本地模型
func (p *Parser) convertFeed(feed *gofeed.Feed, url string) (*Feed, []*Article, error) {
	f := &Feed{
		URL:         url,
		Title:       feed.Title,
		Description: feed.Description,
		Interval:    DefaultInterval,
		Enabled:     true,
	}

	if url != "" {
		f.ID = generateID(url)
	}

	articles := make([]*Article, 0, len(feed.Items))
	for _, item := range feed.Items {
		article := p.convertItem(item, f.ID, feed.Title)
		articles = append(articles, article)
	}

	return f, articles, nil
}

// convertItem 转换 gofeed.Item 到 Article
func (p *Parser) convertItem(item *gofeed.Item, feedID, source string) *Article {
	guid := item.GUID
	if guid == "" {
		guid = item.Link
	}

	pubDate := time.Now()
	if item.PublishedParsed != nil {
		pubDate = *item.PublishedParsed
	} else if item.UpdatedParsed != nil {
		pubDate = *item.UpdatedParsed
	}

	author := ""
	if item.Author != nil {
		author = item.Author.Name
	}

	content := item.Content
	if content == "" {
		content = item.Description
	}

	description := item.Description
	if description == "" && len(content) > 200 {
		description = content[:200] + "..."
	} else if description == "" {
		description = content
	}

	// 清理 HTML 标签（简单处理）
	description = stripHTMLTags(description)

	return &Article{
		GUID:        guid,
		FeedID:      feedID,
		Title:       item.Title,
		Link:        item.Link,
		Description: description,
		Content:     content,
		Author:      author,
		PubDate:     pubDate,
		FetchedAt:   time.Now(),
		Source:      source,
	}
}

// stripHTMLTags 简单移除 HTML 标签
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
