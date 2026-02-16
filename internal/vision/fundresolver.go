// Package vision 提供基金名称→代码解析器
// 从东方财富下载全量基金代码表（约 26000 只），在本地做名称模糊匹配
package vision

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// FundEntry 基金条目
type FundEntry struct {
	Code string // 6位基金代码
	Name string // 基金全称
	Type string // 基金类型
}

// FundResolver 基金名称→代码解析器
// 使用东方财富全量基金表做本地匹配，无需网络请求
type FundResolver struct {
	mu    sync.RWMutex
	funds []FundEntry
}

// NewFundResolver 创建解析器并加载基金表
func NewFundResolver() *FundResolver {
	r := &FundResolver{}
	go r.load() // 异步加载，不阻塞启动
	return r
}

const fundCodeListURL = "https://fund.eastmoney.com/js/fundcode_search.js"

// load 从东方财富下载全量基金代码表
func (r *FundResolver) load() {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fundCodeListURL, nil)
	if err != nil {
		logger.Error("创建基金表请求失败", zap.Error(err))
		return
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("下载基金代码表失败", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取基金代码表失败", zap.Error(err))
		return
	}

	funds := parseFundCodeList(string(body))

	r.mu.Lock()
	r.funds = funds
	r.mu.Unlock()

	logger.Info("基金代码表加载完成", zap.Int("funds", len(funds)))
}

// 解析格式: var r = [["000001","HXCZHH","华夏成长混合","混合型-灵活","HUAXIACHENGZHANGHUNHE"],...]
var fundEntryRegex = regexp.MustCompile(`\["(\d{6})","[^"]*","([^"]*)","([^"]*)","[^"]*"\]`)

func parseFundCodeList(js string) []FundEntry {
	matches := fundEntryRegex.FindAllStringSubmatch(js, -1)
	funds := make([]FundEntry, 0, len(matches))
	for _, m := range matches {
		funds = append(funds, FundEntry{
			Code: m[1],
			Name: m[2],
			Type: m[3],
		})
	}
	return funds
}

// ResolveCode 根据基金名称查找代码
// 返回最匹配的基金代码，如果找不到返回空字符串
func (r *FundResolver) ResolveCode(name string) (code string, fullName string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.funds) == 0 {
		return "", ""
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}

	// 1. 精确匹配
	for _, f := range r.funds {
		if f.Name == name {
			return f.Code, f.Name
		}
	}

	// 2. 去掉份额标识后匹配（"永赢科技智选混合A" → 匹配 "永赢科技智选混合发起A"）
	// 截图名称往往是简称，全量表是全称（多了"发起"、"发起式"等词）
	baseName := stripShareClass(name)
	shareClass := getShareClass(name)

	var bestMatch FundEntry
	bestScore := 0

	for _, f := range r.funds {
		score := matchScore(baseName, shareClass, f.Name)
		if score > bestScore {
			bestScore = score
			bestMatch = f
		}
	}

	// 匹配度阈值：基础名称至少 60% 的字符被包含
	minScore := utf8.RuneCountInString(baseName) * 60 / 100
	if bestScore >= minScore && bestScore > 3 {
		return bestMatch.Code, bestMatch.Name
	}

	return "", ""
}

// matchScore 计算名称匹配得分
func matchScore(baseName, shareClass, fullName string) int {
	// 全称必须包含基础名称的核心部分
	score := 0

	// 逐字匹配（连续匹配加分更多）
	baseRunes := []rune(baseName)
	fullRunes := []rune(fullName)
	j := 0
	consecutive := 0
	for i := 0; i < len(baseRunes) && j < len(fullRunes); j++ {
		if baseRunes[i] == fullRunes[j] {
			score++
			consecutive++
			if consecutive > 1 {
				score++ // 连续匹配额外加分
			}
			i++
		} else {
			consecutive = 0
		}
	}

	// 份额类型匹配加分（A/B/C）
	if shareClass != "" && strings.HasSuffix(fullName, shareClass) {
		score += 5
	}

	return score
}

// stripShareClass 去掉份额标识（A/B/C/E/H）
func stripShareClass(name string) string {
	if len(name) == 0 {
		return name
	}
	last := name[len(name)-1]
	if last == 'A' || last == 'B' || last == 'C' || last == 'E' || last == 'H' {
		return strings.TrimSpace(name[:len(name)-1])
	}
	return name
}

// getShareClass 获取份额标识
func getShareClass(name string) string {
	if len(name) == 0 {
		return ""
	}
	last := name[len(name)-1]
	if last == 'A' || last == 'B' || last == 'C' || last == 'E' || last == 'H' {
		return string(last)
	}
	return ""
}

// ResolveBatch 批量解析基金名称→代码
func (r *FundResolver) ResolveBatch(names []string) map[string]FundEntry {
	result := make(map[string]FundEntry)
	for _, name := range names {
		code, fullName := r.ResolveCode(name)
		if code != "" {
			result[name] = FundEntry{Code: code, Name: fullName}
		}
	}
	return result
}

// Count 返回已加载的基金数量
func (r *FundResolver) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.funds)
}

// Ready 检查是否已加载完成
func (r *FundResolver) Ready() bool {
	return r.Count() > 0
}

// String 调试用
func (r *FundResolver) String() string {
	return fmt.Sprintf("FundResolver{funds=%d}", r.Count())
}
