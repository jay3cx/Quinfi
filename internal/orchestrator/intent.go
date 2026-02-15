package orchestrator

import "strings"

// Intent 用户意图类型
type Intent string

const (
	IntentDeepAnalysis Intent = "deep_analysis" // 深度分析
	IntentQuickQuery   Intent = "quick_query"   // 快速查询
	IntentNewsSummary  Intent = "news_summary"  // 新闻摘要
	IntentChat         Intent = "chat"          // 普通对话
)

// DetectIntent 意图识别（基于关键词，后续可升级为 LLM 意图分类）
func DetectIntent(input string) (Intent, string) {
	lower := strings.ToLower(input)

	// 深度分析关键词
	deepKeywords := []string{"深度分析", "详细分析", "全面分析", "深入分析", "研报"}
	for _, kw := range deepKeywords {
		if strings.Contains(lower, kw) {
			code := extractFundCode(input)
			return IntentDeepAnalysis, code
		}
	}

	return IntentChat, ""
}

// extractFundCode 从文本中提取 6 位基金代码
func extractFundCode(text string) string {
	for _, word := range strings.Fields(text) {
		word = strings.TrimSpace(word)
		if len(word) == 6 && isAllDigits(word) {
			return word
		}
	}
	return ""
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
