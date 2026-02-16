package debate

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/jay3cx/Quinfi/pkg/llm"
)

// LLMReviewer 使用 LLM 对裁决进行一次复核。
type LLMReviewer struct {
	client llm.Client
}

// NewLLMReviewer 创建复核器。
func NewLLMReviewer(client llm.Client) *LLMReviewer {
	return &LLMReviewer{client: client}
}

// ReviewJudge 对同一裁决提示进行一次低温复核，返回结构化 Verdict。
func (r *LLMReviewer) ReviewJudge(ctx context.Context, prompt string) (*Verdict, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("reviewer unavailable")
	}
	resp, err := r.client.Chat(ctx, &llm.ChatRequest{
		Model: llm.ModelGLM5,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: JudgeSystemPrompt},
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   0,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}
	return parseVerdict(resp.Content)
}

// ComputeConsistencyScore 计算 base 与 review 两份裁决的一致性评分（0-100）。
// 评分维度：
// 1. confidence 差距（越接近越高）
// 2. risk_warnings 重叠度
// 3. suggestion 关键语义重叠
func ComputeConsistencyScore(base, review *Verdict) int {
	if base == nil || review == nil {
		return 0
	}

	confidenceScore := confidenceSimilarityScore(base.Confidence, review.Confidence)
	riskScore := riskWarningOverlapScore(base.RiskWarnings, review.RiskWarnings)
	suggestionScore := suggestionSemanticScore(base.Suggestion, review.Suggestion)

	score := int(math.Round(
		0.35*float64(confidenceScore) +
			0.35*float64(riskScore) +
			0.30*float64(suggestionScore),
	))
	return clampScore(score)
}

func confidenceSimilarityScore(baseConfidence, reviewConfidence int) int {
	diff := clampScore(baseConfidence) - clampScore(reviewConfidence)
	if diff < 0 {
		diff = -diff
	}
	return clampScore(100 - diff)
}

func riskWarningOverlapScore(baseWarnings, reviewWarnings []string) int {
	if len(baseWarnings) == 0 && len(reviewWarnings) == 0 {
		return 100
	}
	if len(baseWarnings) == 0 || len(reviewWarnings) == 0 {
		return 0
	}

	baseText := strings.Join(baseWarnings, " ")
	reviewText := strings.Join(reviewWarnings, " ")

	keywordScore := keywordOverlapScore(baseText, reviewText)
	runeScore := runeSetSimilarityScore(baseText, reviewText)

	return clampScore(int(math.Round(
		0.5*float64(keywordScore) + 0.5*float64(runeScore),
	)))
}

func suggestionSemanticScore(baseSuggestion, reviewSuggestion string) int {
	baseSuggestion = strings.TrimSpace(baseSuggestion)
	reviewSuggestion = strings.TrimSpace(reviewSuggestion)

	if baseSuggestion == "" && reviewSuggestion == "" {
		return 100
	}
	if baseSuggestion == "" || reviewSuggestion == "" {
		return 0
	}

	if normalizeComparableText(baseSuggestion) == normalizeComparableText(reviewSuggestion) {
		return 100
	}

	keywordScore := keywordOverlapScore(baseSuggestion, reviewSuggestion)
	runeScore := runeSetSimilarityScore(baseSuggestion, reviewSuggestion)
	intentScore := suggestionIntentSimilarityScore(baseSuggestion, reviewSuggestion)

	return clampScore(int(math.Round(
		0.35*float64(keywordScore) +
			0.35*float64(runeScore) +
			0.30*float64(intentScore),
	)))
}

func suggestionIntentSimilarityScore(baseSuggestion, reviewSuggestion string) int {
	baseIntent := detectSuggestionIntent(baseSuggestion)
	reviewIntent := detectSuggestionIntent(reviewSuggestion)

	if baseIntent == suggestionIntentUnknown || reviewIntent == suggestionIntentUnknown {
		return 50
	}
	if baseIntent == reviewIntent {
		return 100
	}
	return 0
}

func keywordOverlapScore(baseText, reviewText string) int {
	baseKeywords := extractKeywords(baseText)
	reviewKeywords := extractKeywords(reviewText)
	return jaccardSetScore(baseKeywords, reviewKeywords)
}

func runeSetSimilarityScore(baseText, reviewText string) int {
	baseSet := extractRuneSet(baseText)
	reviewSet := extractRuneSet(reviewText)
	return jaccardSetScore(baseSet, reviewSet)
}

func extractRuneSet(text string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, r := range normalizeComparableText(text) {
		if unicode.IsSpace(r) {
			continue
		}
		result[string(r)] = struct{}{}
	}
	return result
}

func extractKeywords(text string) map[string]struct{} {
	keywords := make(map[string]struct{})
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return keywords
	}

	var asciiToken []rune
	var hanSeq []rune

	flushASCIIToken := func() {
		if len(asciiToken) < 2 {
			asciiToken = asciiToken[:0]
			return
		}
		token := string(asciiToken)
		if _, stopWord := commonStopWords[token]; !stopWord {
			keywords[token] = struct{}{}
		}
		asciiToken = asciiToken[:0]
	}

	flushHanSeq := func() {
		if len(hanSeq) == 0 {
			return
		}
		if len(hanSeq) == 1 {
			keywords[string(hanSeq)] = struct{}{}
			hanSeq = hanSeq[:0]
			return
		}
		for i := 0; i < len(hanSeq)-1; i++ {
			keywords[string(hanSeq[i:i+2])] = struct{}{}
		}
		hanSeq = hanSeq[:0]
	}

	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushASCIIToken()
			hanSeq = append(hanSeq, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushHanSeq()
			asciiToken = append(asciiToken, r)
		default:
			flushASCIIToken()
			flushHanSeq()
		}
	}

	flushASCIIToken()
	flushHanSeq()
	return keywords
}

func normalizeComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}

	var b strings.Builder
	needSpace := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r), unicode.IsLetter(r), unicode.IsDigit(r):
			if needSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			needSpace = false
		case unicode.IsSpace(r):
			needSpace = true
		default:
			// 将标点等非词符号视为分隔符，避免 token 被无空格符号粘连。
			needSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func jaccardSetScore(left, right map[string]struct{}) int {
	if len(left) == 0 && len(right) == 0 {
		return 100
	}
	if len(left) == 0 || len(right) == 0 {
		return 0
	}

	intersection := 0
	union := len(left)
	for token := range right {
		if _, exists := left[token]; exists {
			intersection++
			continue
		}
		union++
	}

	return clampScore(int(math.Round(float64(intersection) * 100 / float64(union))))
}

type suggestionIntent int

const (
	suggestionIntentUnknown suggestionIntent = iota
	suggestionIntentBuy
	suggestionIntentHold
	suggestionIntentSell
)

func detectSuggestionIntent(suggestion string) suggestionIntent {
	s := normalizeComparableText(suggestion)
	if s == "" {
		return suggestionIntentUnknown
	}

	buyCount := countIntentKeywords(s, buyIntentKeywords)
	sellCount := countIntentKeywords(s, sellIntentKeywords)
	holdCount := countIntentKeywords(s, holdIntentKeywords)

	if buyCount > 0 || sellCount > 0 {
		if buyCount > sellCount {
			return suggestionIntentBuy
		}
		if sellCount > buyCount {
			return suggestionIntentSell
		}
	}

	if holdCount > 0 {
		return suggestionIntentHold
	}
	return suggestionIntentUnknown
}

func countIntentKeywords(text string, keywords []string) int {
	normalized := normalizeComparableText(text)
	if normalized == "" {
		return 0
	}

	tokenSet := make(map[string]struct{})
	for _, token := range strings.Fields(normalized) {
		tokenSet[token] = struct{}{}
	}

	count := 0
	for _, keyword := range keywords {
		kw := strings.ToLower(strings.TrimSpace(keyword))
		if kw == "" {
			continue
		}

		if containsHanRune(kw) {
			if strings.Contains(normalized, kw) {
				count++
			}
			continue
		}
		if _, ok := tokenSet[kw]; ok {
			count++
		}
	}
	return count
}

func containsHanRune(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

var buyIntentKeywords = []string{
	"买入", "加仓", "增持", "建仓", "配置", "布局",
	"buy", "accumulate", "overweight",
}

var holdIntentKeywords = []string{
	"持有", "观望", "中性", "等待", "继续跟踪",
	"hold", "neutral", "wait", "watch",
}

var sellIntentKeywords = []string{
	"减仓", "减持", "卖出", "回避", "止盈", "退出",
	"sell", "reduce", "underweight", "avoid", "trim",
}

var commonStopWords = map[string]struct{}{
	"the": {}, "and": {}, "or": {}, "to": {}, "of": {}, "for": {},
	"a": {}, "an": {}, "is": {}, "are": {}, "on": {}, "in": {}, "at": {},
	"with": {}, "from": {}, "by": {}, "be": {}, "as": {}, "it": {},
	"this": {}, "that": {}, "we": {}, "you": {}, "they": {},
}
