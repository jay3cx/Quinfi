// Package eastmoney 提供基金排行榜和搜索 API
package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

const (
	// 基金排行榜 API（东方财富开放接口）
	fundRankingURL = "https://fund.eastmoney.com/data/rankhandler.aspx"
	// 基金搜索 API
	fundSearchURL = "https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchPageAPI.ashx"
)

// FundRankItem 基金排行条目
type FundRankItem struct {
	Code       string  `json:"code"`        // 基金代码
	Name       string  `json:"name"`        // 基金名称
	Type       string  `json:"type"`        // 基金类型
	NAV        float64 `json:"nav"`         // 最新净值
	DailyReturn float64 `json:"daily_return"` // 日涨幅
	WeekReturn  float64 `json:"week_return"`  // 近1周
	MonthReturn float64 `json:"month_return"` // 近1月
	ThreeMonthReturn float64 `json:"three_month_return"` // 近3月
	YearReturn  float64 `json:"year_return"`  // 近1年
	Manager    string  `json:"manager"`     // 基金经理
	Scale      string  `json:"scale"`       // 基金规模
}

// FundSearchResult 基金搜索结果
type FundSearchResult struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// GetFundRanking 获取基金排行榜
// sortType: 近1周(1w)/近1月(1m)/近3月(3m)/近6月(6m)/近1年(1y)/近3年(3y)
// fundType: 全部(0)/股票型(1)/混合型(3)/债券型(2)/指数型(5)/QDII(6)
// count: 返回条数
func (c *Client) GetFundRanking(ctx context.Context, sortType string, fundType int, count int) ([]FundRankItem, error) {
	if count <= 0 {
		count = 20
	}
	if sortType == "" {
		sortType = "1y" // 默认按近1年排序
	}

	// 构建排行榜 API 参数
	sc := sortTypeToParam(sortType)

	params := url.Values{}
	params.Set("op", "ph")
	params.Set("dt", "kf")   // 开放式基金
	params.Set("rs", "")
	params.Set("gs", "0")
	params.Set("sc", sc)
	params.Set("st", "desc") // 降序
	params.Set("pi", "1")
	params.Set("pn", fmt.Sprintf("%d", count))
	if fundType > 0 {
		params.Set("ft", fmt.Sprintf("%d", fundType))
	}

	reqURL := fundRankingURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/data/fundranking.html")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求基金排行榜失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	items := parseRankingResponse(string(body))
	logger.Info("获取基金排行榜成功",
		zap.String("sort", sortType),
		zap.Int("count", len(items)),
	)

	return items, nil
}

// SearchFunds 搜索基金（按关键词）
func (c *Client) SearchFunds(ctx context.Context, keyword string) ([]FundSearchResult, error) {
	params := url.Values{}
	params.Set("m", "1")
	params.Set("key", keyword)
	params.Set("pageIndex", "1")
	params.Set("pageSize", "20")

	reqURL := fundSearchURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("搜索基金失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	results := parseSearchResponse(string(body))
	logger.Info("搜索基金成功",
		zap.String("keyword", keyword),
		zap.Int("results", len(results)),
	)

	return results, nil
}

// === 解析函数 ===

func sortTypeToParam(sortType string) string {
	switch sortType {
	case "1w":
		return "zzf"   // 近1周
	case "1m":
		return "1yzf"  // 近1月
	case "3m":
		return "3yzf"  // 近3月
	case "6m":
		return "6yzf"  // 近6月
	case "1y":
		return "1nzf"  // 近1年
	case "3y":
		return "3nzf"  // 近3年
	default:
		return "1nzf"
	}
}

// parseRankingResponse 解析排行榜 JS 响应
// 东方财富返回格式: var rankData = {datas:["code,name,...","code,name,..."],allRecords:1000,...}
func parseRankingResponse(body string) []FundRankItem {
	// 提取 datas 数组
	start := strings.Index(body, `"datas":[`)
	if start < 0 {
		start = strings.Index(body, `datas:[`)
	}
	if start < 0 {
		return nil
	}

	// 找到数组区域
	arrStart := strings.Index(body[start:], "[")
	if arrStart < 0 {
		return nil
	}
	arrStart += start

	arrEnd := strings.Index(body[arrStart:], "]")
	if arrEnd < 0 {
		return nil
	}
	arrEnd += arrStart

	arrStr := body[arrStart : arrEnd+1]

	var dataStrings []string
	if err := json.Unmarshal([]byte(arrStr), &dataStrings); err != nil {
		return nil
	}

	var items []FundRankItem
	for _, s := range dataStrings {
		item := parseRankingItem(s)
		if item != nil {
			items = append(items, *item)
		}
	}

	return items
}

// parseRankingItem 解析单条排行数据
// 格式: "代码,简称,字母缩写,日期,单位净值,累计净值,日涨幅,近1周,近1月,近3月,近6月,近1年,近2年,近3年,今年来,成立来,..."
func parseRankingItem(s string) *FundRankItem {
	parts := strings.Split(s, ",")
	if len(parts) < 15 {
		return nil
	}

	return &FundRankItem{
		Code:        parts[0],
		Name:        parts[1],
		NAV:         parseFloat(parts[4]),
		DailyReturn: parseFloat(parts[6]),
		WeekReturn:  parseFloat(parts[7]),
		MonthReturn: parseFloat(parts[8]),
		ThreeMonthReturn: parseFloat(parts[9]),
		YearReturn:  parseFloat(parts[11]),
	}
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "---" || s == "--" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// parseSearchResponse 解析搜索响应
func parseSearchResponse(body string) []FundSearchResult {
	// 尝试解析为 JSON
	var resp struct {
		Datas []struct {
			CODE string `json:"CODE"`
			NAME string `json:"NAME"`
			FundType string `json:"FundType"`
		} `json:"Datas"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		// 尝试其他格式
		return parseSearchFallback(body)
	}

	var results []FundSearchResult
	for _, d := range resp.Datas {
		results = append(results, FundSearchResult{
			Code: d.CODE,
			Name: d.NAME,
			Type: d.FundType,
		})
	}
	return results
}

func parseSearchFallback(body string) []FundSearchResult {
	// 简单提取模式：在响应文本中查找基金代码和名称
	var results []FundSearchResult
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.Contains(line, "CODE") || strings.Contains(line, "code") {
			// 尝试 JSON 行
			var item struct {
				Code string `json:"CODE"`
				Name string `json:"NAME"`
			}
			if json.Unmarshal([]byte(line), &item) == nil && item.Code != "" {
				results = append(results, FundSearchResult{Code: item.Code, Name: item.Name})
			}
		}
	}
	return results
}
