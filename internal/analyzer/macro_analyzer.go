package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jay3cx/fundmind/pkg/llm"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// DefaultMacroAnalyzer 宏观分析器实现
type DefaultMacroAnalyzer struct {
	llmClient llm.Client
}

// NewMacroAnalyzer 创建宏观分析器
func NewMacroAnalyzer(client llm.Client) *DefaultMacroAnalyzer {
	return &DefaultMacroAnalyzer{llmClient: client}
}

const macroPrompt = `你是一位宏观经济分析师。请基于以下新闻，分析宏观环境对基金「%s」的影响。

## 近期新闻
%s

请以 JSON 格式输出分析：
{
  "market_sentiment": "乐观/中性/悲观",
  "key_events": ["事件1", "事件2"],
  "impact": "对该基金的具体影响分析（50-100字）",
  "risk_factors": ["宏观风险1", "宏观风险2"],
  "analysis_text": "综合宏观研判（100-200字）"
}

只输出 JSON，不要输出其他内容。`

// AnalyzeMacro 宏观研判
func (a *DefaultMacroAnalyzer) AnalyzeMacro(ctx context.Context, fundName string, news []string) (*MacroReport, error) {
	logger.Info("开始宏观研判", zap.String("fund", fundName))

	if len(news) == 0 {
		return &MacroReport{
			MarketSentiment: "中性",
			AnalysisText:    "暂无相关新闻，无法进行宏观研判",
		}, nil
	}

	newsText := strings.Join(news, "\n- ")
	prompt := fmt.Sprintf(macroPrompt, fundName, "- "+newsText)

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:       llm.GetDefaultModel(llm.TaskDaily),
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	jsonStr := extractJSON(resp.Content)
	var report MacroReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	logger.Info("宏观研判完成", zap.String("sentiment", report.MarketSentiment))
	return &report, nil
}
