// Package vision 提供图像识别能力（基于 Vision LLM）
package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jay3cx/Quinfi/pkg/llm"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// HoldingResult 持仓识别结果（单只基金）
type HoldingResult struct {
	Code            string  `json:"code"`                        // 6位基金代码（由 FundResolver 反查）
	Name            string  `json:"name"`                        // 基金名称（Vision 识别）
	Amount          float64 `json:"amount"`                      // 持有金额（元）
	DailyReturn     float64 `json:"daily_return,omitempty"`      // 当日收益（元）
	TotalProfit     float64 `json:"total_profit,omitempty"`      // 持有收益（元）
	TotalProfitRate float64 `json:"total_profit_rate,omitempty"` // 持有收益率（%）
}

// ScanResult 截图扫描结果
type ScanResult struct {
	Holdings   []HoldingResult `json:"holdings"`
	TotalValue float64         `json:"total_value"` // 账户总资产
	Error      string          `json:"error,omitempty"`
}

// Scanner 持仓截图识别器
// 流程：Gemini Vision 识别名称+金额 → FundResolver 反查基金代码
type Scanner struct {
	client   llm.Client
	model    llm.ModelID
	resolver *FundResolver
}

// NewScanner 创建截图识别器
func NewScanner(client llm.Client) *Scanner {
	return &Scanner{
		client:   client,
		model:    llm.ModelGemini3Flash, // Vision 需要支持多模态的模型
		resolver: NewFundResolver(), // 异步加载 26000+ 基金代码表
	}
}

// 只要求识别名称、金额和盈亏，不要求代码（Vision 模型读小数字不准）
const scanPrompt = `你是一个基金持仓截图识别专家。请仔细分析这张基金 App 的持仓截图。

提取每只基金的【完整名称】、【持有金额】、【当日收益】、【持有收益】和【持有收益率】。不需要识别基金代码。

请严格按以下 JSON 格式返回（不要添加任何其他文字）：

{
  "holdings": [
    {
      "name": "基金完整名称",
      "amount": 持有金额数值,
      "daily_return": 当日收益数值,
      "total_profit": 持有收益数值,
      "total_profit_rate": 持有收益率数值
    }
  ],
  "total_value": 账户总资产数值
}

注意事项：
1. name 必须是截图中显示的完整基金名称，包括份额类型（如A、C）
2. amount 是该基金的持有市值/金额（元），只写数字
3. daily_return 是当日收益（元），亏损为负数
4. total_profit 是持有收益/累计收益（元），亏损为负数
5. total_profit_rate 是持有收益率（%），亏损为负数，只写数字不要%符号
6. total_value 是截图中显示的账户总资产
7. 如果某个字段在截图中看不到，填 0
8. 如果图片不是基金持仓截图，返回 {"holdings": [], "total_value": 0, "error": "图片不是基金持仓截图"}`

// ScanPortfolio 识别持仓截图，返回基金持仓列表
// 两步流程：
// 1. Vision 模型识别基金名称和金额（准确率高）
// 2. FundResolver 用名称反查基金代码（本地全量表匹配）
func (s *Scanner) ScanPortfolio(ctx context.Context, imageBase64 string) (*ScanResult, error) {
	// 检测并修正图片 MIME 类型
	imageURL := ensureDataURI(imageBase64)

	logger.Info("开始识别持仓截图",
		zap.Int("image_size", len(imageBase64)),
		zap.String("model", string(s.model)),
		zap.Bool("resolver_ready", s.resolver.Ready()),
	)

	// Step 1: Vision 识别名称+金额
	msg := llm.NewVisionMessage(llm.RoleUser, scanPrompt, imageURL)

	resp, err := s.client.Chat(ctx, &llm.ChatRequest{
		Model:       s.model,
		Messages:    []llm.Message{msg},
		MaxTokens:   0,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("Vision API 调用失败: %w", err)
	}

	logger.Info("Vision 识别完成",
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
	)

	// 解析 JSON
	result, err := parseScanResult(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("识别结果解析失败: %w（原始响应: %s）", err, truncate(resp.Content, 200))
	}

	if result.Error != "" {
		return result, nil
	}

	// Step 2: 用名称反查基金代码
	s.resolveHoldingCodes(result)

	logger.Info("持仓截图识别完成",
		zap.Int("holdings", len(result.Holdings)),
		zap.Float64("total_value", result.TotalValue),
	)

	return result, nil
}

// resolveHoldingCodes 用 FundResolver 为每只基金反查代码
func (s *Scanner) resolveHoldingCodes(result *ScanResult) {
	if !s.resolver.Ready() {
		logger.Warn("基金代码表尚未加载，跳过代码反查")
		return
	}

	for i, h := range result.Holdings {
		code, fullName := s.resolver.ResolveCode(h.Name)
		if code != "" {
			result.Holdings[i].Code = code
			// 如果全量表的全称更完整，用全称替换
			if len(fullName) > len(h.Name) {
				result.Holdings[i].Name = fullName
			}
			logger.Info("基金代码反查成功",
				zap.String("scan_name", h.Name),
				zap.String("code", code),
				zap.String("full_name", fullName),
			)
		} else {
			logger.Warn("基金代码反查失败",
				zap.String("scan_name", h.Name),
			)
		}
	}
}

// ensureDataURI 确保图片有正确的 data URI 前缀
// 自动检测实际图片格式（解决 .png 文件实际是 JPEG 的问题）
func ensureDataURI(imageData string) string {
	// 已经是 data URI 或 HTTP URL
	if strings.HasPrefix(imageData, "data:") || strings.HasPrefix(imageData, "http") {
		// 检查已有 data URI 的 MIME 是否正确
		if strings.HasPrefix(imageData, "data:") {
			return fixDataURIMime(imageData)
		}
		return imageData
	}

	// 裸 base64 → 检测格式并添加前缀
	mime := detectImageMime(imageData)
	return "data:" + mime + ";base64," + imageData
}

// fixDataURIMime 修正 data URI 中错误的 MIME 类型
func fixDataURIMime(dataURI string) string {
	// 提取 base64 部分
	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return dataURI
	}

	actualMime := detectImageMime(parts[1])
	// 替换 MIME
	header := parts[0]
	if strings.Contains(header, "image/png") && actualMime == "image/jpeg" {
		header = strings.Replace(header, "image/png", "image/jpeg", 1)
	} else if strings.Contains(header, "image/jpeg") && actualMime == "image/png" {
		header = strings.Replace(header, "image/jpeg", "image/png", 1)
	}
	return header + "," + parts[1]
}

// detectImageMime 从 base64 数据的头部字节检测实际图片格式
func detectImageMime(b64Data string) string {
	// 解码前几个字节即可判断
	raw, err := base64.StdEncoding.DecodeString(b64Data[:min(100, len(b64Data))])
	if err != nil || len(raw) < 4 {
		return "image/png" // 默认
	}

	mime := http.DetectContentType(raw)
	if strings.HasPrefix(mime, "image/") {
		return mime
	}
	return "image/png"
}

// parseScanResult 从 LLM 响应中解析 JSON 结果
func parseScanResult(content string) (*ScanResult, error) {
	content = strings.TrimSpace(content)

	// 移除 markdown 代码块标记
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// 提取 JSON 对象
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var result ScanResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
