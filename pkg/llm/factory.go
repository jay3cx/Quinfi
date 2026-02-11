// Package llm 提供 LLM 客户端工厂
package llm

import (
	"fmt"
	"strings"
	"sync"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Provider LLM 服务提供商
type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderGemini Provider = "gemini"
	ProviderOpenAI Provider = "openai"
)

// ClientConstructor 客户端构造函数签名
type ClientConstructor func(baseURL, apiKey string) Client

// Factory LLM 客户端工厂
// 根据 ModelID 自动选择对应 Provider 并缓存客户端实例
type Factory struct {
	baseURL      string
	apiKey       string
	constructors map[Provider]ClientConstructor
	clients      map[Provider]Client
	mu           sync.RWMutex
}

// NewFactory 创建客户端工厂
func NewFactory(baseURL, apiKey string) *Factory {
	return &Factory{
		baseURL:      baseURL,
		apiKey:       apiKey,
		constructors: make(map[Provider]ClientConstructor),
		clients:      make(map[Provider]Client),
	}
}

// Register 注册 Provider 的客户端构造函数
func (f *Factory) Register(provider Provider, constructor ClientConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.constructors[provider] = constructor
	logger.Info("注册 LLM Provider", zap.String("provider", string(provider)))
}

// GetClient 根据 ModelID 获取对应的客户端
// 客户端按 Provider 级别缓存，同一 Provider 只创建一个实例
func (f *Factory) GetClient(model ModelID) (Client, error) {
	provider := DetectProvider(model)

	// 快路径：读锁检查缓存
	f.mu.RLock()
	if client, ok := f.clients[provider]; ok {
		f.mu.RUnlock()
		return client, nil
	}
	f.mu.RUnlock()

	// 慢路径：写锁创建客户端
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check
	if client, ok := f.clients[provider]; ok {
		return client, nil
	}

	constructor, ok := f.constructors[provider]
	if !ok {
		return nil, fmt.Errorf("未注册的 LLM Provider: %s (model: %s)", provider, model)
	}

	client := constructor(f.baseURL, f.apiKey)
	f.clients[provider] = client

	logger.Info("创建 LLM 客户端",
		zap.String("provider", string(provider)),
		zap.String("model", string(model)),
	)

	return client, nil
}

// GetClientForTask 根据任务类型获取客户端和推荐模型
func (f *Factory) GetClientForTask(task TaskType) (Client, ModelID, error) {
	model := GetDefaultModel(task)
	client, err := f.GetClient(model)
	if err != nil {
		return nil, "", err
	}
	return client, model, nil
}

// DetectProvider 根据 ModelID 推断 Provider
func DetectProvider(model ModelID) Provider {
	m := strings.ToLower(string(model))

	switch {
	case strings.HasPrefix(m, "claude"):
		return ProviderClaude
	case strings.HasPrefix(m, "gemini"):
		return ProviderGemini
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"):
		return ProviderOpenAI
	default:
		// 未知模型默认走 OpenAI 兼容协议（最通用）
		return ProviderOpenAI
	}
}
