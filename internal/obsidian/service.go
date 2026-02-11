// Package obsidian 提供笔记服务
package obsidian

import (
	"context"
	"fmt"
	"strings"

	obs "github.com/jay3cx/fundmind/pkg/obsidian"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Service Obsidian 笔记服务
type Service struct {
	client   *obs.Client
	renderer *TemplateRenderer
	basePath string // 笔记根目录
}

// ServiceOption 服务配置选项
type ServiceOption func(*Service)

// WithBasePath 设置笔记根目录
func WithBasePath(path string) ServiceOption {
	return func(s *Service) {
		s.basePath = strings.TrimSuffix(path, "/")
	}
}

// NewService 创建笔记服务
func NewService(client *obs.Client, opts ...ServiceOption) (*Service, error) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		return nil, fmt.Errorf("创建模板渲染器失败: %w", err)
	}

	s := &Service{
		client:   client,
		renderer: renderer,
		basePath: "FundMind", // 默认根目录
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// CreateFundNote 创建基金笔记
func (s *Service) CreateFundNote(ctx context.Context, note *FundNote) error {
	content, err := s.renderer.RenderFundNote(note)
	if err != nil {
		return fmt.Errorf("渲染笔记失败: %w", err)
	}

	path := s.fullPath(FundNotePath(note.Code, note.Name))

	if err := s.client.WriteFile(ctx, path, content); err != nil {
		return fmt.Errorf("写入笔记失败: %w", err)
	}

	logger.Info("创建基金笔记成功",
		zap.String("code", note.Code),
		zap.String("name", note.Name),
		zap.String("path", path),
	)

	return nil
}

// UpdateFundNote 更新基金笔记
func (s *Service) UpdateFundNote(ctx context.Context, note *FundNote) error {
	path := s.fullPath(FundNotePath(note.Code, note.Name))

	// 检查文件是否存在
	exists, err := s.client.FileExists(ctx, path)
	if err != nil {
		return fmt.Errorf("检查文件失败: %w", err)
	}

	if !exists {
		// 不存在则创建
		return s.CreateFundNote(ctx, note)
	}

	// 存在则更新（目前简单覆盖，后续可保留用户自定义内容）
	content, err := s.renderer.RenderFundNote(note)
	if err != nil {
		return fmt.Errorf("渲染笔记失败: %w", err)
	}

	if err := s.client.WriteFile(ctx, path, content); err != nil {
		return fmt.Errorf("更新笔记失败: %w", err)
	}

	logger.Info("更新基金笔记成功",
		zap.String("code", note.Code),
		zap.String("path", path),
	)

	return nil
}

// GetFundNote 获取基金笔记内容
func (s *Service) GetFundNote(ctx context.Context, code, name string) (string, error) {
	path := s.fullPath(FundNotePath(code, name))

	content, err := s.client.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("读取笔记失败: %w", err)
	}

	return content, nil
}

// CreateManagerNote 创建经理笔记
func (s *Service) CreateManagerNote(ctx context.Context, note *ManagerNote) error {
	content, err := s.renderer.RenderManagerNote(note)
	if err != nil {
		return fmt.Errorf("渲染笔记失败: %w", err)
	}

	path := s.fullPath(ManagerNotePath(note.Name))

	if err := s.client.WriteFile(ctx, path, content); err != nil {
		return fmt.Errorf("写入笔记失败: %w", err)
	}

	logger.Info("创建经理笔记成功",
		zap.String("name", note.Name),
		zap.String("path", path),
	)

	return nil
}

// UpdateManagerNote 更新经理笔记
func (s *Service) UpdateManagerNote(ctx context.Context, note *ManagerNote) error {
	path := s.fullPath(ManagerNotePath(note.Name))

	exists, err := s.client.FileExists(ctx, path)
	if err != nil {
		return fmt.Errorf("检查文件失败: %w", err)
	}

	if !exists {
		return s.CreateManagerNote(ctx, note)
	}

	content, err := s.renderer.RenderManagerNote(note)
	if err != nil {
		return fmt.Errorf("渲染笔记失败: %w", err)
	}

	if err := s.client.WriteFile(ctx, path, content); err != nil {
		return fmt.Errorf("更新笔记失败: %w", err)
	}

	logger.Info("更新经理笔记成功",
		zap.String("name", note.Name),
		zap.String("path", path),
	)

	return nil
}

// GetManagerNote 获取经理笔记内容
func (s *Service) GetManagerNote(ctx context.Context, name string) (string, error) {
	path := s.fullPath(ManagerNotePath(name))

	content, err := s.client.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("读取笔记失败: %w", err)
	}

	return content, nil
}

// Ping 检查 Obsidian 连接
func (s *Service) Ping(ctx context.Context) error {
	return s.client.Ping(ctx)
}

// fullPath 生成完整路径
func (s *Service) fullPath(path string) string {
	if s.basePath == "" {
		return path
	}
	return s.basePath + "/" + path
}
