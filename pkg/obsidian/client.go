// Package obsidian 提供 Obsidian REST API 客户端
package obsidian

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Client Obsidian REST API 客户端
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// ClientOption 客户端配置选项
type ClientOption func(*Client)

// WithBaseURL 设置 API 地址
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithToken 设置认证 Token
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// WithHTTPClient 设置 HTTP 客户端
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient 创建 Obsidian 客户端
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL: "http://127.0.0.1:27123",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // 跳过自签名证书验证
				},
			},
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Ping 健康检查
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/", nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Obsidian 连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("认证失败: Token 无效")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("Obsidian 返回错误: %d", resp.StatusCode)
	}

	logger.Info("Obsidian 连接成功", zap.String("url", c.baseURL))
	return nil
}

// ReadFile 读取 Vault 文件
func (c *Client) ReadFile(ctx context.Context, path string) (string, error) {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/vault/%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "text/markdown")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("认证失败: Token 无效")
	}

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("文件不存在: %s", path)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("读取失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	return string(body), nil
}

// WriteFile 写入 Vault 文件
func (c *Client) WriteFile(ctx context.Context, path, content string) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/vault/%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBufferString(content))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "text/markdown")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("认证失败: Token 无效")
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("写入失败: HTTP %d, %s", resp.StatusCode, string(body))
	}

	logger.Info("Obsidian 文件写入成功", zap.String("path", path))
	return nil
}

// DeleteFile 删除 Vault 文件
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/vault/%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("认证失败: Token 无效")
	}

	if resp.StatusCode == 404 {
		return nil // 文件不存在视为删除成功
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("删除失败: HTTP %d", resp.StatusCode)
	}

	return nil
}

// FileExists 检查文件是否存在
func (c *Client) FileExists(ctx context.Context, path string) (bool, error) {
	path = strings.TrimPrefix(path, "/")
	url := fmt.Sprintf("%s/vault/%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("检查文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return false, nil
	}

	if resp.StatusCode == 200 {
		return true, nil
	}

	return false, fmt.Errorf("检查失败: HTTP %d", resp.StatusCode)
}

// ListFiles 列出目录文件
func (c *Client) ListFiles(ctx context.Context, path string) ([]string, error) {
	path = strings.TrimPrefix(path, "/")
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	url := fmt.Sprintf("%s/vault/%s", c.baseURL, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("列出失败: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Files []string `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result.Files, nil
}

// setHeaders 设置请求头
func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
