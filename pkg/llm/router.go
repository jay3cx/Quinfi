// Package llm 提供模型路由功能
package llm

// Router 模型路由器
// 封装 Factory，提供便捷的模型路由接口
type Router struct {
	factory *Factory
}

// NewRouter 创建模型路由器
func NewRouter(baseURL, apiKey string) *Router {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8045"
	}

	return &Router{
		factory: NewFactory(baseURL, apiKey),
	}
}

// Factory 返回内部工厂实例，用于注册 Provider
func (r *Router) Factory() *Factory {
	return r.factory
}

// GetClient 根据模型 ID 获取客户端
func (r *Router) GetClient(model ModelID) (Client, error) {
	return r.factory.GetClient(model)
}

// GetClientForTask 根据任务类型获取客户端和推荐模型
func (r *Router) GetClientForTask(task TaskType) (Client, ModelID, error) {
	return r.factory.GetClientForTask(task)
}

// IsClaudeModel 判断是否为 Claude 模型
func IsClaudeModel(model ModelID) bool {
	return DetectProvider(model) == ProviderClaude
}

// IsGeminiModel 判断是否为 Gemini 模型
func IsGeminiModel(model ModelID) bool {
	return DetectProvider(model) == ProviderGemini
}

// IsOpenAIModel 判断是否为 OpenAI 模型
func IsOpenAIModel(model ModelID) bool {
	return DetectProvider(model) == ProviderOpenAI
}
