// Package db 提供 SQLite 持久化层
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// schema 数据库表结构（自动迁移）
const schema = `
-- 会话表
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_active_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 对话消息表
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);

-- RSS 订阅源表
CREATE TABLE IF NOT EXISTS feeds (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	last_fetched_at DATETIME,
	interval_seconds INTEGER NOT NULL DEFAULT 900,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- RSS 文章表
CREATE TABLE IF NOT EXISTS articles (
	guid TEXT PRIMARY KEY,
	feed_id TEXT NOT NULL,
	title TEXT NOT NULL,
	link TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	author TEXT NOT NULL DEFAULT '',
	pub_date DATETIME,
	fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	source TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	sentiment TEXT NOT NULL DEFAULT '',
	sentiment_reason TEXT NOT NULL DEFAULT '',
	keywords TEXT NOT NULL DEFAULT '[]',
	summarized_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_articles_feed ON articles(feed_id);
CREATE INDEX IF NOT EXISTS idx_articles_pub_date ON articles(pub_date DESC);
CREATE INDEX IF NOT EXISTS idx_articles_sentiment ON articles(sentiment);

-- 文章全文搜索索引（FTS5，中文逐字分词）
CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
	guid UNINDEXED,
	title,
	summary,
	keywords,
	content='articles',
	content_rowid='rowid'
);

-- 简报表
CREATE TABLE IF NOT EXISTS briefs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'daily',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 记忆表
CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL DEFAULT 'default',
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	source_session TEXT DEFAULT '',
	importance REAL DEFAULT 0.5,
	valid_from DATETIME,
	valid_to DATETIME,
	access_count INTEGER DEFAULT 0,
	last_accessed_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_memories_user_type ON memories(user_id, type);
`

// Open 打开或创建数据库
func Open(dbPath string) (*sql.DB, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据库目录失败: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(1) // SQLite 单写者模型
	db.SetMaxIdleConns(1)

	// 启用 WAL 模式（提升并发读性能）
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 WAL 模式失败: %w", err)
	}

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用外键约束失败: %w", err)
	}

	// 自动建表
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库 schema 失败: %w", err)
	}

	logger.Info("数据库初始化完成", zap.String("path", dbPath))
	return db, nil
}
