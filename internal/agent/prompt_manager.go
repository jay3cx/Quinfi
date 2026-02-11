// Package agent 提供 Prompt 模板管理
package agent

import (
	"bytes"
	"os"
	"sync"
	"text/template"
)

// PromptManager Prompt 模板管理器接口
type PromptManager interface {
	// Register 注册模板
	Register(name, content string) error

	// RegisterFromFile 从文件注册模板
	RegisterFromFile(name, filepath string) error

	// Render 渲染模板
	Render(name string, data any) (string, error)

	// List 列出所有模板名称
	List() []string
}

// DefaultPromptManager 默认 Prompt 模板管理器
type DefaultPromptManager struct {
	mu        sync.RWMutex
	templates map[string]*template.Template
	funcs     template.FuncMap
}

// NewPromptManager 创建 Prompt 模板管理器
func NewPromptManager() *DefaultPromptManager {
	return &DefaultPromptManager{
		templates: make(map[string]*template.Template),
		funcs:     defaultTemplateFuncs(),
	}
}

// defaultTemplateFuncs 默认模板函数
func defaultTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"join": func(sep string, items []string) string {
			result := ""
			for i, item := range items {
				if i > 0 {
					result += sep
				}
				result += item
			}
			return result
		},
		"default": func(defaultVal, val string) string {
			if val == "" {
				return defaultVal
			}
			return val
		},
	}
}

// Register 注册模板
func (m *DefaultPromptManager) Register(name, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tmpl, err := template.New(name).Funcs(m.funcs).Parse(content)
	if err != nil {
		return err
	}

	m.templates[name] = tmpl
	return nil
}

// RegisterFromFile 从文件注册模板
func (m *DefaultPromptManager) RegisterFromFile(name, filepath string) error {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	return m.Register(name, string(content))
}

// Render 渲染模板
func (m *DefaultPromptManager) Render(name string, data any) (string, error) {
	m.mu.RLock()
	tmpl, ok := m.templates[name]
	m.mu.RUnlock()

	if !ok {
		return "", ErrTemplateNotFound{Name: name}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// List 列出所有模板名称
func (m *DefaultPromptManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.templates))
	for name := range m.templates {
		names = append(names, name)
	}
	return names
}

// AddFunc 添加自定义模板函数
func (m *DefaultPromptManager) AddFunc(name string, fn any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.funcs[name] = fn
}

// ErrTemplateNotFound 模板未找到错误
type ErrTemplateNotFound struct {
	Name string
}

func (e ErrTemplateNotFound) Error() string {
	return "模板未找到: " + e.Name
}
