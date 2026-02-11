// Package config 提供应用配置管理
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Log     LogConfig     `yaml:"log"`
	LLM     LLMConfig     `yaml:"llm"`
	DB      DBConfig      `yaml:"db"`
	Session SessionConfig `yaml:"session"`
	RSS     RSSConfig     `yaml:"rss"`
}

// RSSFeedConfig 单个 RSS 订阅源配置
type RSSFeedConfig struct {
	URL      string `yaml:"url"`      // 订阅地址
	Name     string `yaml:"name"`     // 显示名称
	Interval string `yaml:"interval"` // 抓取间隔（Go duration），默认 15m
}

// RSSConfig RSS 配置
type RSSConfig struct {
	Enabled bool            `yaml:"enabled"` // 是否启用 RSS 抓取
	Feeds   []RSSFeedConfig `yaml:"feeds"`   // 订阅源列表
}

// GetFeedInterval 解析 RSS 订阅源的抓取间隔
func (f *RSSFeedConfig) GetFeedInterval() time.Duration {
	if f.Interval == "" {
		return 15 * time.Minute
	}
	d, err := time.ParseDuration(f.Interval)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

// DBConfig 数据库配置
type DBConfig struct {
	Path string `yaml:"path"` // SQLite 文件路径，空字符串则纯内存模式
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release, test
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
	Env   string `yaml:"env"`   // development, production
}

// LLMConfig LLM 配置
type LLMConfig struct {
	BaseURL    string `yaml:"base_url"`    // 中转服务地址
	APIKey     string `yaml:"api_key"`     // API Key
	MaxRetries int    `yaml:"max_retries"` // 最大重试次数，默认 2
}

// SessionConfig 会话配置
type SessionConfig struct {
	TTL string `yaml:"ttl"` // 会话过期时间，如 "24h"、"1h"
}

// GetTTL 解析 TTL 为 time.Duration，默认 24h
func (c *SessionConfig) GetTTL() time.Duration {
	if c.TTL == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(c.TTL)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: "8080",
			Mode: "debug",
		},
		Log: LogConfig{
			Level: "info",
			Env:   "development",
		},
		LLM: LLMConfig{
			BaseURL:    "http://127.0.0.1:8045/v1",
			APIKey:     "",
			MaxRetries: 2,
		},
		DB: DBConfig{
			Path: "data/fundmind.db",
		},
		Session: SessionConfig{
			TTL: "24h",
		},
		RSS: RSSConfig{
			Enabled: true,
			Feeds: []RSSFeedConfig{
				// 专业金融
				{URL: "https://cn.investing.com/rss/news.rss", Name: "Investing.com财经", Interval: "10m"},
				{URL: "https://feedx.net/rss/wsj.xml", Name: "华尔街日报(中文)", Interval: "30m"},
				// 科技财经
				{URL: "https://36kr.com/feed-newsflash", Name: "36氪快讯", Interval: "10m"},
				// 宏观经济
				{URL: "https://feedx.net/rss/jingjiribao.xml", Name: "经济日报", Interval: "30m"},
				{URL: "https://feedx.net/rss/nikkei.xml", Name: "日经中文网", Interval: "30m"},
			},
		},
	}
}

// Validate 校验配置合法性
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server.port 不能为空")
	}
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.base_url 不能为空")
	}
	if c.Session.TTL != "" {
		if _, err := time.ParseDuration(c.Session.TTL); err != nil {
			return fmt.Errorf("session.ttl 格式无效 (%q): %w", c.Session.TTL, err)
		}
	}
	if c.LLM.MaxRetries < 0 {
		return fmt.Errorf("llm.max_retries 不能为负数")
	}
	return nil
}

// Load 从文件加载配置，环境变量可覆盖
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// 尝试从文件加载
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// 环境变量覆盖
	if port := os.Getenv("FUNDMIND_PORT"); port != "" {
		cfg.Server.Port = port
	}
	if mode := os.Getenv("FUNDMIND_MODE"); mode != "" {
		cfg.Server.Mode = mode
	}
	if env := os.Getenv("FUNDMIND_ENV"); env != "" {
		cfg.Log.Env = env
	}

	// LLM 配置环境变量覆盖
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if apiKey := os.Getenv("LLM_API_KEY"); apiKey != "" {
		cfg.LLM.APIKey = apiKey
	}

	// DB 配置环境变量覆盖
	if dbPath := os.Getenv("FUNDMIND_DB_PATH"); dbPath != "" {
		cfg.DB.Path = dbPath
	}

	// 校验
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}

	return cfg, nil
}
