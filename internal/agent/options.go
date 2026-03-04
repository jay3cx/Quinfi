// Package agent 提供 Agent 配置选项
package agent

import "github.com/jay3cx/Quinfi/pkg/llm"

// Options Agent 配置选项
type Options struct {
	Model       llm.ModelID // 使用的模型
	MaxTokens   int         // 最大 Token 数
	Temperature float64     // 温度参数
	SystemPrompt string     // 系统提示词
}

// DefaultOptions 默认配置
func DefaultOptions() *Options {
	return &Options{
		Model:       llm.ModelClaudeSonnet46,
		MaxTokens:   0,
		Temperature: 0.7,
		SystemPrompt: "",
	}
}

// Option 配置函数类型
type Option func(*Options)

// WithModel 设置模型
func WithModel(model llm.ModelID) Option {
	return func(o *Options) {
		o.Model = model
	}
}

// WithMaxTokens 设置最大 Token 数
func WithMaxTokens(maxTokens int) Option {
	return func(o *Options) {
		o.MaxTokens = maxTokens
	}
}

// WithTemperature 设置温度参数
func WithTemperature(temperature float64) Option {
	return func(o *Options) {
		o.Temperature = temperature
	}
}

// WithSystemPrompt 设置系统提示词
func WithSystemPrompt(prompt string) Option {
	return func(o *Options) {
		o.SystemPrompt = prompt
	}
}

// ApplyOptions 应用配置选项
func ApplyOptions(opts ...Option) *Options {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return options
}
