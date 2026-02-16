// Package agent 提供基金分析 Agent（ReAct 模式）
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

const (
	maxToolRounds = 25 // 最大工具调用轮次，防止死循环
	maxHistoryLen = 20 // 保留最近 N 条历史消息
)

// FundAgent 基金投研 Agent（ReAct 模式）
//
// 工作流程：
// 1. 构建消息序列：系统提示词 + 历史 + 用户查询
// 2. 调用 LLM（携带工具定义）
// 3. 如果 LLM 返回工具调用 → 执行工具 → 将结果追加到消息 → 回到步骤 2
// 4. 如果 LLM 返回文本 → 作为最终响应返回
type FundAgent struct {
	name    string
	client  llm.Client
	tools   *ToolRegistry
	options *Options
}

// NewFundAgent 创建基金投研 Agent
func NewFundAgent(client llm.Client, tools *ToolRegistry, opts ...Option) *FundAgent {
	options := ApplyOptions(opts...)

	// 如果没有指定系统提示词，使用默认的 Quinfi 提示词
	if options.SystemPrompt == "" {
		options.SystemPrompt = SystemPromptQuinfi
	}

	return &FundAgent{
		name:    "Quinfi",
		client:  client,
		tools:   tools,
		options: options,
	}
}

// Name 返回 Agent 名称
func (a *FundAgent) Name() string {
	return a.name
}

// Run 同步执行（ReAct 循环）
func (a *FundAgent) Run(ctx context.Context, input *AgentInput) (*AgentResponse, error) {
	logger.Info("FundAgent 开始执行",
		zap.String("query", input.Query),
		zap.String("session_id", input.Context.SessionID),
	)

	// 构建初始消息序列
	messages := a.buildMessages(input)

	var totalInputTokens, totalOutputTokens int

	// ReAct 循环
	for round := 0; round < maxToolRounds; round++ {
		resp, err := a.client.Chat(ctx, &llm.ChatRequest{
			Model:       a.options.Model,
			Messages:    messages,
			MaxTokens:   a.options.MaxTokens,
			Temperature: a.options.Temperature,
			Tools:       a.tools.ToLLMTools(),
		})
		if err != nil {
			return nil, fmt.Errorf("LLM 调用失败 (round %d): %w", round, err)
		}

		totalInputTokens += resp.InputTokens
		totalOutputTokens += resp.OutputTokens

		// 没有工具调用 → 最终响应
		if !resp.HasToolCalls() {
			logger.Info("FundAgent 执行完成",
				zap.Int("rounds", round+1),
				zap.Int("input_tokens", totalInputTokens),
				zap.Int("output_tokens", totalOutputTokens),
			)

			return &AgentResponse{
				Content:  resp.Content,
				Metadata: map[string]string{"rounds": fmt.Sprintf("%d", round+1)},
				TokensUsed: TokenUsage{
					InputTokens:  totalInputTokens,
					OutputTokens: totalOutputTokens,
				},
				GeneratedAt: time.Now().Format(time.RFC3339),
			}, nil
		}

		// 有工具调用 → 执行工具并追加结果
		logger.Info("FundAgent 工具调用",
			zap.Int("round", round+1),
			zap.Int("tool_calls", len(resp.ToolCalls)),
		)

		// 追加 assistant 消息（含 tool_calls）
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// 执行每个工具调用
		for _, tc := range resp.ToolCalls {
			result := a.executeTool(ctx, tc)
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return nil, fmt.Errorf("超过最大工具调用轮次 (%d)", maxToolRounds)
}

// RunStream 流式执行（ReAct 循环）
//
// 策略：
// - 所有轮次使用 ChatStream() 实现真正的 token-by-token 流式输出
// - 文本响应：直接转发 LLM 流式 chunk 给前端（真流式）
// - 工具调用：流式累积工具名+参数 → 执行 → 继续下一轮
func (a *FundAgent) RunStream(ctx context.Context, input *AgentInput) (<-chan StreamChunk, error) {
	logger.Info("FundAgent 开始流式执行",
		zap.String("query", input.Query),
	)

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)

		// 构建初始消息序列
		messages := a.buildMessages(input)

		// ReAct 循环
		for round := 0; round < maxToolRounds; round++ {
			// 使用 ChatStream 获取真流式响应
			streamCh, err := a.client.ChatStream(ctx, &llm.ChatRequest{
				Model:       a.options.Model,
				Messages:    messages,
				MaxTokens:   a.options.MaxTokens,
				Temperature: a.options.Temperature,
				Tools:       a.tools.ToLLMTools(),
			})
			if err != nil {
				chunks <- StreamChunk{Error: fmt.Errorf("LLM 流式调用失败: %w", err), Done: true}
				return
			}

			// 消费流式 chunks，累积完整内容和工具调用
			var fullContent string
			var toolCalls []llm.ToolCall

			for chunk := range streamCh {
				if chunk.Error != nil {
					chunks <- StreamChunk{Error: fmt.Errorf("LLM 流式错误: %w", chunk.Error), Done: true}
					return
				}

				// 文本内容：直接转发给前端（真流式！）
				if chunk.Content != "" {
					fullContent += chunk.Content
					chunks <- StreamChunk{Type: ChunkText, Content: chunk.Content}
				}

				// 工具调用（流结束时由 ChatStream 汇总返回）
				if len(chunk.ToolCalls) > 0 {
					toolCalls = chunk.ToolCalls
				}

				if chunk.Done {
					break
				}
			}

			// 判断本轮结果
			if len(toolCalls) == 0 {
				// 没有工具调用 → 最终响应，已经流式输出给前端
				logger.Info("FundAgent 流式执行完成",
					zap.Int("rounds", round+1),
				)
				chunks <- StreamChunk{Done: true}
				return
			}

			// 有工具调用 → 执行工具，推送中间状态
			logger.Info("FundAgent 流式工具调用",
				zap.Int("round", round+1),
				zap.Int("tool_calls", len(toolCalls)),
			)

			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   fullContent,
				ToolCalls: toolCalls,
			})

			for _, tc := range toolCalls {
				// 推送工具调用状态
				chunks <- StreamChunk{
					Type:     ChunkToolStart,
					ToolName: tc.Name,
					Content:  fmt.Sprintf("正在调用 %s...", toolDisplayName(tc.Name)),
				}

				// 注入进度回调，让工具可以推送中间状态（如辩论阶段）
				toolCtx := ContextWithToolProgress(ctx, func(chunk StreamChunk) {
					chunks <- chunk
				})

				result := a.executeTool(toolCtx, tc)
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Content:    result,
					ToolCallID: tc.ID,
				})

				// 推送工具结果状态（简要）
				chunks <- StreamChunk{
					Type:     ChunkToolResult,
					ToolName: tc.Name,
					Content:  fmt.Sprintf("%s 查询完成", toolDisplayName(tc.Name)),
				}
			}

			// 推送思考进度：让前端知道 Agent 正在综合分析
			chunks <- StreamChunk{
				Type:    ChunkThinking,
				Content: thinkingMessage(round, len(toolCalls)),
			}
		}

		chunks <- StreamChunk{
			Error: fmt.Errorf("超过最大工具调用轮次"),
			Done:  true,
		}
	}()

	return chunks, nil
}

// buildMessages 构建 LLM 消息序列
func (a *FundAgent) buildMessages(input *AgentInput) []llm.Message {
	messages := make([]llm.Message, 0, maxHistoryLen+2)

	// 系统提示词 + 记忆上下文注入
	systemPrompt := a.options.SystemPrompt
	baseSystemPromptLen := len([]rune(systemPrompt))
	memCtxLen := 0
	if input.Context != nil {
		if memCtx := input.Context.GetVariable("memory_context"); memCtx != "" {
			memCtxLen = len([]rune(memCtx))
			systemPrompt += "\n" + memCtx
		}
	}
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: systemPrompt,
	})

	// 历史消息（截取最近 N 条，避免超出 context window）
	historyTotal := 0
	historyUsed := 0
	historyTruncated := false
	if input.Context != nil && len(input.Context.History) > 0 {
		historyTotal = len(input.Context.History)
		history := input.Context.History
		if len(history) > maxHistoryLen {
			history = history[len(history)-maxHistoryLen:]
			historyTruncated = true
		}
		historyUsed = len(history)
		for _, msg := range history {
			role := llm.RoleUser
			switch msg.Role {
			case "assistant":
				role = llm.RoleAssistant
			case "system":
				role = llm.RoleSystem
			}
			messages = append(messages, llm.Message{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	// 当前用户查询（如果有图片，构建多模态消息）
	if len(input.Images) > 0 {
		messages = append(messages, llm.NewVisionMessage(llm.RoleUser, input.Query, input.Images...))
	} else {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: input.Query,
		})
	}

	a.maybeLogPromptSummary(input, messages, promptLogMeta{
		BaseSystemPromptLen: baseSystemPromptLen,
		MemoryContextLen:    memCtxLen,
		HistoryTotal:        historyTotal,
		HistoryUsed:         historyUsed,
		HistoryTruncated:    historyTruncated,
	})

	return messages
}

type promptLogMeta struct {
	BaseSystemPromptLen int
	MemoryContextLen    int
	HistoryTotal        int
	HistoryUsed         int
	HistoryTruncated    bool
}

// maybeLogPromptSummary logs a safe prompt summary to help debug prompt composition.
//
// Enable with env var:
//
//	QUINFI_LOG_PROMPT=1     # summary (info level)
//	QUINFI_LOG_PROMPT=full  # include per-message previews (debug level)
func (a *FundAgent) maybeLogPromptSummary(input *AgentInput, messages []llm.Message, meta promptLogMeta) {
	mode := strings.ToLower(strings.TrimSpace(logger.GetEnv("QUINFI_LOG_PROMPT", "")))
	if mode == "" || mode == "0" || mode == "false" || mode == "off" {
		return
	}

	roles := make([]string, 0, len(messages))
	for _, m := range messages {
		roles = append(roles, string(m.Role))
	}

	// Detect suspicious tail duplication: "... user(X), user(X)".
	dupTail := false
	if n := len(messages); n >= 2 {
		m1 := messages[n-1]
		m2 := messages[n-2]
		if m1.Role == llm.RoleUser && m2.Role == llm.RoleUser && promptMessageText(m1) == promptMessageText(m2) {
			dupTail = true
		}
	}

	fields := []zap.Field{
		zap.String("session_id", safeSessionID(input)),
		zap.Int("messages", len(messages)),
		zap.Strings("roles", roles),
		zap.Int("query_len", len([]rune(input.Query))),
		zap.Int("images", len(input.Images)),
		zap.Int("system_prompt_len", len([]rune(messages[0].Content))),
		zap.Int("base_system_prompt_len", meta.BaseSystemPromptLen),
		zap.Int("memory_context_len", meta.MemoryContextLen),
		zap.Int("history_total", meta.HistoryTotal),
		zap.Int("history_used", meta.HistoryUsed),
		zap.Bool("history_truncated", meta.HistoryTruncated),
		zap.Bool("dup_user_tail", dupTail),
	}

	// Summary is info-level so it shows up with default config (log.level=info).
	logger.Info("FundAgent prompt summary", fields...)

	if mode != "full" && mode != "2" && mode != "debug" {
		return
	}

	type msgPreview struct {
		Index       int    `json:"index"`
		Role        string `json:"role"`
		TextLen     int    `json:"text_len"`
		TextPreview string `json:"text_preview,omitempty"`
		Parts       int    `json:"parts,omitempty"`
		Images      int    `json:"images,omitempty"`
	}

	// Only preview the tail messages to reduce the chance of leaking long histories.
	const tailPreviewN = 6
	cutoff := len(messages) - tailPreviewN
	if cutoff < 0 {
		cutoff = 0
	}

	previews := make([]msgPreview, 0, len(messages))
	for i, m := range messages {
		text := promptMessageText(m)
		p := msgPreview{
			Index:   i,
			Role:    string(m.Role),
			TextLen: len([]rune(text)),
		}
		// Never print system prompt content (it may include injected memory context).
		// For non-system messages, only print previews for the tail N messages.
		if m.Role == llm.RoleSystem {
			p.TextPreview = "<system prompt redacted>"
		} else if i >= cutoff {
			p.TextPreview = truncateRunes(text, 160)
		}
		if len(m.MultiContent) > 0 {
			p.Parts = len(m.MultiContent)
			for _, part := range m.MultiContent {
				if part.Type == "image_url" {
					p.Images++
				}
			}
		}
		previews = append(previews, p)
	}

	// Full previews are debug-level to avoid spamming production logs.
	logger.Debug("FundAgent prompt messages", zap.Any("messages", previews))
}

func safeSessionID(input *AgentInput) string {
	if input == nil || input.Context == nil {
		return ""
	}
	return input.Context.SessionID
}

func promptMessageText(m llm.Message) string {
	if len(m.MultiContent) == 0 {
		return m.Content
	}
	var b strings.Builder
	for _, part := range m.MultiContent {
		if part.Type != "text" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(part.Text)
	}
	return b.String()
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// executeTool 执行单个工具调用
func (a *FundAgent) executeTool(ctx context.Context, tc llm.ToolCall) string {
	// 解析参数（容错：某些代理可能返回拼接的 JSON，只取第一个对象）
	argsStr := sanitizeToolArguments(tc.Arguments)

	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		errMsg := fmt.Sprintf("参数解析失败: %s", err.Error())
		logger.Error("工具参数解析失败",
			zap.String("tool", tc.Name),
			zap.String("arguments", tc.Arguments),
			zap.Error(err),
		)
		return errMsg
	}

	logger.Info("执行工具",
		zap.String("tool", tc.Name),
		zap.Any("args", args),
	)

	result, err := a.tools.Execute(ctx, tc.Name, args)
	if err != nil {
		errMsg := fmt.Sprintf("工具执行失败: %s", err.Error())
		logger.Error("工具执行失败",
			zap.String("tool", tc.Name),
			zap.Error(err),
		)
		return errMsg
	}

	logger.Info("工具执行成功",
		zap.String("tool", tc.Name),
		zap.Int("result_len", len(result)),
	)

	return result
}

// thinkingMessage 根据轮次和工具数量生成思考进度描述
func thinkingMessage(round, toolCount int) string {
	if round == 0 {
		return fmt.Sprintf("已完成 %d 项数据查询，正在综合分析...", toolCount)
	}
	return fmt.Sprintf("正在补充更多数据进行深入分析...（第 %d 轮）", round+1)
}

// toolDisplayName 工具名称的中文显示
func toolDisplayName(name string) string {
	switch name {
	case "get_fund_info":
		return "基金信息查询"
	case "get_nav_history":
		return "净值走势查询"
	case "get_fund_holdings":
		return "持仓数据查询"
	case "search_news":
		return "新闻资讯搜索"
	default:
		return name
	}
}

// sanitizeToolArguments 清理工具参数
// 某些代理（如 Antigravity Tools）可能返回拼接的 JSON：
//
//	{"code":"005827"}{"code":"005827","days":30}
//
// 只取第一个完整的 JSON 对象
func sanitizeToolArguments(args string) string {
	args = strings.TrimSpace(args)
	if args == "" || args[0] != '{' {
		return args
	}

	depth := 0
	for i, c := range args {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return args[:i+1]
			}
		}
	}
	return args
}
