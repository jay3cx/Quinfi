// Package rss 提供内容存储（支持 SQLite 持久化 + FTS5 全文检索）
package rss

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Store 文章存储（Write-Through: 内存缓存 + DB 持久化）
type Store struct {
	mu       sync.RWMutex
	articles map[string]*Article // key: GUID
	byURL    map[string]string   // URL -> GUID 映射
	feeds    map[string]*Feed    // key: Feed ID
	db       *sql.DB             // 可选，为 nil 时纯内存模式
}

// NewStore 创建存储（纯内存模式）
func NewStore() *Store {
	return &Store{
		articles: make(map[string]*Article),
		byURL:    make(map[string]string),
		feeds:    make(map[string]*Feed),
	}
}

// NewStoreWithDB 创建带 DB 持久化的存储
func NewStoreWithDB(db *sql.DB) *Store {
	s := &Store{
		articles: make(map[string]*Article),
		byURL:    make(map[string]string),
		feeds:    make(map[string]*Feed),
		db:       db,
	}
	s.loadFromDB()
	return s
}

// SaveFeed 保存订阅源
func (s *Store) SaveFeed(feed *Feed) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feeds[feed.ID] = feed

	if s.db != nil {
		s.dbSaveFeed(feed)
	}
}

// GetFeed 获取订阅源
func (s *Store) GetFeed(id string) (*Feed, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	feed, ok := s.feeds[id]
	return feed, ok
}

// GetAllFeeds 获取所有订阅源
func (s *Store) GetAllFeeds() []*Feed {
	s.mu.RLock()
	defer s.mu.RUnlock()

	feeds := make([]*Feed, 0, len(s.feeds))
	for _, f := range s.feeds {
		feeds = append(feeds, f)
	}
	return feeds
}

// DeleteFeed 删除订阅源
func (s *Store) DeleteFeed(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.feeds, id)

	if s.db != nil {
		s.db.Exec(`DELETE FROM feeds WHERE id = ?`, id)
	}
}

// SaveArticles 保存文章（自动去重）
func (s *Store) SaveArticles(articles []*Article) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	saved := 0
	for _, article := range articles {
		if _, exists := s.articles[article.GUID]; exists {
			continue
		}
		if _, exists := s.byURL[article.Link]; exists {
			continue
		}

		s.articles[article.GUID] = article
		s.byURL[article.Link] = article.GUID
		saved++

		if s.db != nil {
			s.dbSaveArticle(article)
		}
	}

	return saved
}

// UpdateArticle 更新文章（如摘要生成后的回写）
func (s *Store) UpdateArticle(article *Article) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.articles[article.GUID] = article

	if s.db != nil {
		s.dbUpdateArticle(article)
	}
}

// GetArticle 获取文章
func (s *Store) GetArticle(guid string) (*Article, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	article, ok := s.articles[guid]
	return article, ok
}

// GetArticlesByFeed 获取订阅源的文章
func (s *Store) GetArticlesByFeed(feedID string, limit int) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()

	articles := make([]*Article, 0)
	for _, a := range s.articles {
		if a.FeedID == feedID {
			articles = append(articles, a)
		}
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].PubDate.After(articles[j].PubDate)
	})

	if limit > 0 && len(articles) > limit {
		articles = articles[:limit]
	}
	return articles
}

// GetRecentArticles 获取最近的文章
func (s *Store) GetRecentArticles(since time.Time, limit int) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()

	articles := make([]*Article, 0)
	for _, a := range s.articles {
		if a.PubDate.After(since) {
			articles = append(articles, a)
		}
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].PubDate.After(articles[j].PubDate)
	})

	if limit > 0 && len(articles) > limit {
		articles = articles[:limit]
	}
	return articles
}

// GetAllArticles 获取所有文章
func (s *Store) GetAllArticles(limit int) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()

	articles := make([]*Article, 0, len(s.articles))
	for _, a := range s.articles {
		articles = append(articles, a)
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].PubDate.After(articles[j].PubDate)
	})

	if limit > 0 && len(articles) > limit {
		articles = articles[:limit]
	}
	return articles
}

// Count 返回文章数量
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.articles)
}

// Exists 检查文章是否存在
func (s *Store) Exists(guid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.articles[guid]
	return exists
}

// === DB 操作 ===

func (s *Store) loadFromDB() {
	// 加载订阅源
	feedRows, err := s.db.Query(`SELECT id, url, title, description, last_fetched_at, interval_seconds, enabled FROM feeds`)
	if err != nil {
		logger.Error("加载订阅源失败", zap.Error(err))
	} else {
		defer feedRows.Close()
		for feedRows.Next() {
			var f Feed
			var lastFetched sql.NullTime
			var intervalSec int
			if err := feedRows.Scan(&f.ID, &f.URL, &f.Title, &f.Description, &lastFetched, &intervalSec, &f.Enabled); err != nil {
				continue
			}
			if lastFetched.Valid {
				f.LastFetched = lastFetched.Time
			}
			f.Interval = time.Duration(intervalSec) * time.Second
			s.feeds[f.ID] = &f
		}
	}

	// 加载文章
	rows, err := s.db.Query(`SELECT guid, feed_id, title, link, description, content, author, pub_date, fetched_at, source, summary, sentiment, sentiment_reason, keywords, summarized_at FROM articles ORDER BY pub_date DESC`)
	if err != nil {
		logger.Error("加载文章失败", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var a Article
		var pubDate, fetchedAt, summarizedAt sql.NullTime
		var keywordsJSON string

		if err := rows.Scan(
			&a.GUID, &a.FeedID, &a.Title, &a.Link, &a.Description, &a.Content,
			&a.Author, &pubDate, &fetchedAt, &a.Source,
			&a.Summary, &a.Sentiment, &a.SentimentReason, &keywordsJSON, &summarizedAt,
		); err != nil {
			logger.Error("解析文章失败", zap.Error(err))
			continue
		}

		if pubDate.Valid {
			a.PubDate = pubDate.Time
		}
		if fetchedAt.Valid {
			a.FetchedAt = fetchedAt.Time
		}
		if summarizedAt.Valid {
			a.SummarizedAt = summarizedAt.Time
		}
		json.Unmarshal([]byte(keywordsJSON), &a.Keywords)

		s.articles[a.GUID] = &a
		s.byURL[a.Link] = a.GUID
	}

	logger.Info("从数据库恢复数据",
		zap.Int("feeds", len(s.feeds)),
		zap.Int("articles", len(s.articles)),
	)

	// 重建 FTS5 索引（确保已有文章被索引）
	s.ftsRebuild()
}

func (s *Store) dbSaveFeed(f *Feed) {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO feeds (id, url, title, description, last_fetched_at, interval_seconds, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.URL, f.Title, f.Description, f.LastFetched, int(f.Interval.Seconds()), f.Enabled,
	)
	if err != nil {
		logger.Error("持久化订阅源失败", zap.String("id", f.ID), zap.Error(err))
	}
}

func (s *Store) dbSaveArticle(a *Article) {
	keywordsJSON, _ := json.Marshal(a.Keywords)

	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO articles (guid, feed_id, title, link, description, content, author, pub_date, fetched_at, source, summary, sentiment, sentiment_reason, keywords, summarized_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.GUID, a.FeedID, a.Title, a.Link, a.Description, a.Content,
		a.Author, a.PubDate, a.FetchedAt, a.Source,
		a.Summary, a.Sentiment, a.SentimentReason, string(keywordsJSON), a.SummarizedAt,
	)
	if err != nil {
		logger.Error("持久化文章失败", zap.String("guid", a.GUID), zap.Error(err))
		return
	}

	// 同步写入 FTS5 索引
	s.ftsIndex(a)
}

func (s *Store) dbUpdateArticle(a *Article) {
	keywordsJSON, _ := json.Marshal(a.Keywords)

	_, err := s.db.Exec(`
		UPDATE articles SET summary = ?, sentiment = ?, sentiment_reason = ?, keywords = ?, summarized_at = ?
		WHERE guid = ?`,
		a.Summary, a.Sentiment, a.SentimentReason, string(keywordsJSON), a.SummarizedAt, a.GUID,
	)
	if err != nil {
		logger.Error("更新文章失败", zap.String("guid", a.GUID), zap.Error(err))
		return
	}

	// 更新 FTS5 索引（先删后插）
	s.ftsDelete(a.GUID)
	s.ftsIndex(a)
}

// === FTS5 全文搜索 ===

// ftsIndex 将文章写入 FTS5 索引
func (s *Store) ftsIndex(a *Article) {
	kwText := strings.Join(a.Keywords, " ")
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO articles_fts (guid, title, summary, keywords) VALUES (?, ?, ?, ?)`,
		a.GUID, tokenizeCJK(a.Title), tokenizeCJK(a.Summary), tokenizeCJK(kwText),
	)
	if err != nil {
		// FTS5 表可能不存在（旧数据库），静默忽略
		logger.Debug("FTS5 索引写入失败", zap.Error(err))
	}
}

// ftsDelete 从 FTS5 索引删除文章
func (s *Store) ftsDelete(guid string) {
	s.db.Exec(`DELETE FROM articles_fts WHERE guid = ?`, guid)
}

// ftsRebuild 重建全部 FTS5 索引（启动时同步已有数据）
func (s *Store) ftsRebuild() {
	if s.db == nil {
		return
	}

	// 清空 FTS 表
	s.db.Exec(`DELETE FROM articles_fts`)

	count := 0
	for _, a := range s.articles {
		kwText := strings.Join(a.Keywords, " ")
		_, err := s.db.Exec(
			`INSERT INTO articles_fts (guid, title, summary, keywords) VALUES (?, ?, ?, ?)`,
			a.GUID, tokenizeCJK(a.Title), tokenizeCJK(a.Summary), tokenizeCJK(kwText),
		)
		if err == nil {
			count++
		}
	}

	if count > 0 {
		logger.Info("FTS5 索引重建完成", zap.Int("articles", count))
	}
}

// SearchArticles 使用 FTS5 全文搜索文章
// 返回按相关度排序的结果，如果 FTS5 不可用则降级为内存关键词匹配
func (s *Store) SearchArticles(query string, limit int) []*Article {
	if limit <= 0 {
		limit = 10
	}

	// 优先走 FTS5
	if s.db != nil {
		results := s.ftsSearch(query, limit)
		if results != nil {
			return results
		}
	}

	// 降级：内存关键词匹配（兼容无 DB 模式）
	return s.fallbackSearch(query, limit)
}

// ftsSearch 用 FTS5 执行搜索
func (s *Store) ftsSearch(query string, limit int) []*Article {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT guid FROM articles_fts
		WHERE articles_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		logger.Debug("FTS5 查询失败，降级为内存搜索", zap.Error(err))
		return nil
	}
	defer rows.Close()

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Article
	for rows.Next() {
		var guid string
		if rows.Scan(&guid) == nil {
			if a, ok := s.articles[guid]; ok {
				results = append(results, a)
			}
		}
	}

	return results
}

// fallbackSearch 内存关键词匹配（FTS5 不可用时的降级方案）
func (s *Store) fallbackSearch(query string, limit int) []*Article {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kw := strings.ToLower(query)
	var results []*Article

	for _, a := range s.articles {
		if strings.Contains(strings.ToLower(a.Title), kw) ||
			strings.Contains(strings.ToLower(a.Summary), kw) ||
			containsAnyKeyword(a.Keywords, kw) {
			results = append(results, a)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].PubDate.After(results[j].PubDate)
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func containsAnyKeyword(keywords []string, target string) bool {
	for _, k := range keywords {
		if strings.Contains(strings.ToLower(k), target) {
			return true
		}
	}
	return false
}

// tokenizeCJK 在中文字符间插入空格，使 FTS5 能按字索引
// "贵州茅台批发价上涨" → "贵 州 茅 台 批 发 价 上 涨"
func tokenizeCJK(text string) string {
	var b strings.Builder
	b.Grow(len(text) * 2)
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			b.WriteRune(' ')
			b.WriteRune(r)
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// buildFTSQuery 将用户查询转为 FTS5 短语匹配
// "茅台" → {title summary keywords}: "茅 台"
// "新能源 汽车" → {title summary keywords}: "新 能 源" OR {title summary keywords}: "汽 车"
func buildFTSQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// 按空格分割为多个搜索词
	terms := strings.Fields(query)
	var parts []string

	for _, term := range terms {
		tokenized := strings.TrimSpace(tokenizeCJK(term))
		words := strings.Fields(tokenized)
		if len(words) == 0 {
			continue
		}
		// 短语匹配：相邻 token 必须连续出现
		phrase := `"` + strings.Join(words, " ") + `"`
		parts = append(parts, phrase)
	}

	if len(parts) == 0 {
		return ""
	}

	// 多个词用 OR 连接
	return strings.Join(parts, " OR ")
}
