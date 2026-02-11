// Package datasource 提供基金数据缓存层
package datasource

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// Cache 简单的内存缓存
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// NewCache 创建新的缓存实例
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get 获取缓存
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Data, true
}

// Set 设置缓存
func (c *Cache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// CachedDataSource 带缓存的数据源装饰器
type CachedDataSource struct {
	source FundDataSource
	cache  *Cache
}

// NewCachedDataSource 创建带缓存的数据源
func NewCachedDataSource(source FundDataSource, ttl time.Duration) *CachedDataSource {
	return &CachedDataSource{
		source: source,
		cache:  NewCache(ttl),
	}
}

// GetFundInfo 获取基金信息（带缓存）
func (c *CachedDataSource) GetFundInfo(ctx context.Context, code string) (*Fund, error) {
	key := "fund_info:" + code

	if cached, ok := c.cache.Get(key); ok {
		return cached.(*Fund), nil
	}

	fund, err := c.source.GetFundInfo(ctx, code)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, fund)
	return fund, nil
}

// GetFundNAV 获取净值历史（带缓存，按 code+days 分别缓存）
func (c *CachedDataSource) GetFundNAV(ctx context.Context, code string, days int) ([]NAV, error) {
	key := fmt.Sprintf("fund_nav:%s:%d", code, days)

	if cached, ok := c.cache.Get(key); ok {
		return cached.([]NAV), nil
	}

	navList, err := c.source.GetFundNAV(ctx, code, days)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, navList)
	return navList, nil
}

// GetFundManager 获取基金经理（带缓存）
func (c *CachedDataSource) GetFundManager(ctx context.Context, code string) (*Manager, error) {
	key := "fund_manager:" + code

	if cached, ok := c.cache.Get(key); ok {
		return cached.(*Manager), nil
	}

	manager, err := c.source.GetFundManager(ctx, code)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, manager)
	return manager, nil
}

// GetFundHoldings 获取持仓明细（带缓存）
func (c *CachedDataSource) GetFundHoldings(ctx context.Context, code string) ([]Holding, error) {
	key := "fund_holdings:" + code

	if cached, ok := c.cache.Get(key); ok {
		return cached.([]Holding), nil
	}

	holdings, err := c.source.GetFundHoldings(ctx, code)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, holdings)
	return holdings, nil
}

// 确保 CachedDataSource 实现 FundDataSource 接口
var _ FundDataSource = (*CachedDataSource)(nil)
