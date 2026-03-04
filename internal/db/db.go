// Package db 提供 SQLite 持久化层
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jay3cx/Quinfi/pkg/logger"
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

-- 基金元数据缓存
CREATE TABLE IF NOT EXISTS funds (
	code TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT '',
	company TEXT NOT NULL DEFAULT '',
	scale REAL DEFAULT 0,
	establish_at DATETIME,
	manager_name TEXT NOT NULL DEFAULT '',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 净值历史（时序数据）
CREATE TABLE IF NOT EXISTS nav_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	fund_code TEXT NOT NULL,
	date TEXT NOT NULL,
	unit_nav REAL NOT NULL,
	accum_nav REAL NOT NULL,
	daily_return REAL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(fund_code, date)
);
CREATE INDEX IF NOT EXISTS idx_nav_history_code_date ON nav_history(fund_code, date DESC);

-- 持仓快照（按季度）
CREATE TABLE IF NOT EXISTS holdings_snapshot (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	fund_code TEXT NOT NULL,
	quarter TEXT NOT NULL,
	stock_code TEXT NOT NULL,
	stock_name TEXT NOT NULL,
	ratio REAL DEFAULT 0,
	share_count REAL DEFAULT 0,
	market_value REAL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_holdings_code_quarter ON holdings_snapshot(fund_code, quarter DESC);

-- 用户持仓表
CREATE TABLE IF NOT EXISTS user_holdings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	fund_code TEXT NOT NULL UNIQUE,
	fund_name TEXT NOT NULL DEFAULT '',
	shares REAL NOT NULL DEFAULT 0,
	cost REAL NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 异步任务表
CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	payload TEXT NOT NULL DEFAULT '{}',
	result TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	progress INTEGER NOT NULL DEFAULT 0,
	progress_msg TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_tasks_type_created ON tasks(type, created_at DESC);
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
