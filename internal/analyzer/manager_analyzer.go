package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jay3cx/Quinfi/internal/datasource"
	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// DefaultManagerAnalyzer 基金经理分析器实现
type DefaultManagerAnalyzer struct {
	dataSource datasource.FundDataSource
	llmClient  llm.Client
}

// NewManagerAnalyzer 创建基金经理分析器
func NewManagerAnalyzer(ds datasource.FundDataSource, client llm.Client) *DefaultManagerAnalyzer {
	return &DefaultManagerAnalyzer{
		dataSource: ds,
		llmClient:  client,
	}
}

const managerPrompt = `你是一位专业的基金分析师。请分析以下基金经理的能力和风格。

## 基金信息
- 基金代码：%s
- 基金名称：%s

## 基金经理
- 姓名：%s
- 任职年限：%.1f年
- 管理规模：%.1f亿元
- 管理基金数：%d只
- 背景：%s

请以 JSON 格式输出分析，包含以下字段：
{
  "manager_name": "姓名",
  "years": 任职年限,
  "style": "投资风格描述（如价值型、成长型、均衡型等）",
  "strengths": ["优势1", "优势2"],
  "weaknesses": ["劣势1", "劣势2"],
  "best_performance": "最佳业绩表现",
  "analysis_text": "综合分析（100-200字）"
}

只输出 JSON，不要输出其他内容。`

// AnalyzeManager 分析基金经理
func (a *DefaultManagerAnalyzer) AnalyzeManager(ctx context.Context, code string) (*ManagerReport, error) {
	logger.Info("开始分析基金经理", zap.String("code", code))

	fund, err := a.dataSource.GetFundInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取基金信息失败: %w", err)
	}

	if fund.Manager == nil {
		return nil, fmt.Errorf("基金 %s 无经理信息", code)
	}

	m := fund.Manager
	prompt := fmt.Sprintf(managerPrompt, fund.Code, fund.Name,
		m.Name, m.Years, m.TotalScale, m.FundCount, m.Background)

	resp, err := a.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:       llm.GetDefaultModel(llm.TaskDaily),
		Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens:   0,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	jsonStr := extractJSON(resp.Content)
	var report ManagerReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	logger.Info("基金经理分析完成", zap.String("manager", report.ManagerName))
	return &report, nil
}
