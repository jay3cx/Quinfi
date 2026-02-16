// Package gemini 提供 Gemini API 客户端实现
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

const (
	defaultBaseURL = "http://localhost:8317"
)

// Client Gemini API 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient 创建 Gemini 客户端
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// geminiRequest Gemini API 请求格式
type geminiRequest struct {
	Contents         []geminiContent   `json:"contents"`
	GenerationConfig *generationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string        `json:"role"`
	Parts []geminiPart  `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type generationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// geminiResponse Gemini API 响应格式
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// Chat 同步对话调用
func (c *Client) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	// 构建请求
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == llm.RoleAssistant {
			role = "model"
		} else if m.Role == llm.RoleSystem {
			// Gemini 不支持 system role，作为 user 消息处理
			role = "user"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	geminiReq := geminiRequest{
		Contents: contents,
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		geminiReq.GenerationConfig = &generationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 URL: /v1beta/models/{model}:generateContent
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, req.Model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	logger.Info("Gemini API 请求", zap.String("model", string(req.Model)))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	content := ""
	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		content = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	logger.Info("Gemini API 响应成功",
		zap.Int("input_tokens", geminiResp.UsageMetadata.PromptTokenCount),
		zap.Int("output_tokens", geminiResp.UsageMetadata.CandidatesTokenCount),
	)

	return &llm.ChatResponse{
		Content:      content,
		Model:        string(req.Model),
		InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
		OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
	}, nil
}

// ChatStream 流式对话调用
func (c *Client) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	// 构建请求
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == llm.RoleAssistant {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	geminiReq := geminiRequest{
		Contents: contents,
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		geminiReq.GenerationConfig = &generationConfig{
			MaxOutputTokens: req.MaxTokens,
			Temperature:     req.Temperature,
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 URL: /v1beta/models/{model}:streamGenerateContent
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, req.Model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan llm.StreamChunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var streamResp geminiResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Candidates) > 0 && len(streamResp.Candidates[0].Content.Parts) > 0 {
				text := streamResp.Candidates[0].Content.Parts[0].Text
				ch <- llm.StreamChunk{Content: text}
			}
		}

		ch <- llm.StreamChunk{Done: true}

		if err := scanner.Err(); err != nil {
			ch <- llm.StreamChunk{Error: err}
		}
	}()

	return ch, nil
}

// 确保 Client 实现 llm.Client 接口
var _ llm.Client = (*Client)(nil)
