package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

const (
	defaultBaseURL = "http://127.0.0.1:8045/v1"
)

// Client OpenAI 兼容协议客户端实现（支持 Function Calling）
type Client struct {
	client *openai.Client
}

// NewClient 创建 OpenAI 兼容客户端
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL

	return &Client{
		client: openai.NewClientWithConfig(cfg),
	}
}

// Chat 同步对话调用（支持 Tool Calling）
func (c *Client) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	messages := convertMessages(req.Messages)

	logger.Info("OpenAI API 请求",
		zap.String("model", string(req.Model)),
		zap.Int("tools", len(req.Tools)),
		zap.Int("messages", len(messages)),
	)

	openaiReq := openai.ChatCompletionRequest{
		Model:       string(req.Model),
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Stream:      false,
	}

	// 添加工具定义
	if len(req.Tools) > 0 {
		openaiReq.Tools = convertTools(req.Tools)
	}

	resp, err := c.client.CreateChatCompletion(ctx, openaiReq)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("OpenAI API 返回空响应")
	}

	choice := resp.Choices[0]

	result := &llm.ChatResponse{
		Content:      choice.Message.Content,
		Model:        resp.Model,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}

	// 解析工具调用
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]llm.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = llm.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
		logger.Info("OpenAI API 返回工具调用",
			zap.Int("tool_calls", len(result.ToolCalls)),
		)
	} else {
		logger.Info("OpenAI API 响应成功",
			zap.Int("input_tokens", resp.Usage.PromptTokens),
			zap.Int("output_tokens", resp.Usage.CompletionTokens),
		)
	}

	return result, nil
}

// ChatStream 流式对话调用
func (c *Client) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	messages := convertMessages(req.Messages)

	openaiReq := openai.ChatCompletionRequest{
		Model:       string(req.Model),
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Stream:      true,
	}

	// 流式模式不传工具定义（工具轮次使用同步调用）
	stream, err := c.client.CreateChatCompletionStream(ctx, openaiReq)
	if err != nil {
		return nil, err
	}

	ch := make(chan llm.StreamChunk, 100)

	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- llm.StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- llm.StreamChunk{Error: err}
				return
			}

			if len(response.Choices) > 0 {
				ch <- llm.StreamChunk{
					Content: response.Choices[0].Delta.Content,
					Done:    false,
				}
			}
		}
	}()

	return ch, nil
}

// convertMessages 将 llm.Message 转换为 OpenAI SDK 格式
// 支持纯文本消息和多模态消息（Vision）
func convertMessages(msgs []llm.Message) []openai.ChatCompletionMessage {
	messages := make([]openai.ChatCompletionMessage, 0, len(msgs))

	for _, msg := range msgs {
		var m openai.ChatCompletionMessage

		// 多模态消息（含图片）
		if len(msg.MultiContent) > 0 {
			m = openai.ChatCompletionMessage{
				Role:         string(msg.Role),
				MultiContent: convertMultiContent(msg.MultiContent),
			}
		} else {
			m = openai.ChatCompletionMessage{
				Role:    string(msg.Role),
				Content: msg.Content,
			}
		}

		// 处理 assistant 消息中的 tool_calls
		if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				m.ToolCalls[i] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}

		// 处理 tool 角色消息
		if msg.Role == llm.RoleTool {
			m.ToolCallID = msg.ToolCallID
		}

		messages = append(messages, m)
	}

	return messages
}

// convertMultiContent 将 llm.ContentPart 转为 OpenAI SDK 的 ChatMessagePart
func convertMultiContent(parts []llm.ContentPart) []openai.ChatMessagePart {
	result := make([]openai.ChatMessagePart, len(parts))
	for i, p := range parts {
		switch p.Type {
		case "image_url":
			result[i] = openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: p.ImageURL.URL,
				},
			}
		default: // "text"
			result[i] = openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: p.Text,
			}
		}
	}
	return result
}

// convertTools 将 llm.ToolDef 转换为 OpenAI SDK 格式
func convertTools(tools []llm.ToolDef) []openai.Tool {
	result := make([]openai.Tool, len(tools))

	for i, t := range tools {
		// 将 Parameters map 转为 JSON Schema
		paramsJSON, _ := json.Marshal(t.Parameters)

		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(paramsJSON),
			},
		}
	}

	return result
}

// 确保 Client 实现 llm.Client 接口
var _ llm.Client = (*Client)(nil)
