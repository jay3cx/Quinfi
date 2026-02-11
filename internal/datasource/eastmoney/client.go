// Package eastmoney 提供东方财富基金数据 API 客户端
package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jay3cx/fundmind/internal/datasource"
	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

const (
	// 基金基本信息 API
	fundInfoURL = "https://fundgz.1234567.com.cn/js/%s.js"
	// 基金详情页 API
	fundDetailURL = "https://fund.eastmoney.com/pingzhongdata/%s.js"
	// 基金持仓 API
	fundHoldingURL = "https://fundf10.eastmoney.com/FundArchivesDatas.aspx?type=jjcc&code=%s&topline=10"
)

// Client 东方财富 API 客户端
type Client struct {
	httpClient *http.Client
}

// NewClient 创建新的东方财富客户端
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetFundInfo 获取基金基本信息
// 组合两个 API：天天基金估值接口（基础信息）+ 东方财富详情页（经理、规模等）
func (c *Client) GetFundInfo(ctx context.Context, code string) (*datasource.Fund, error) {
	// 1. 从天天基金 API 获取基础信息（名称、代码）
	fund, err := c.fetchBasicInfo(ctx, code)
	if err != nil {
		return nil, err
	}

	// 2. 从东方财富详情页补充详细信息（经理、规模、类型、收益率）
	c.enrichFundDetail(ctx, fund)

	logger.Info("获取基金信息成功",
		zap.String("code", code),
		zap.String("name", fund.Name),
		zap.String("type", string(fund.Type)),
	)
	return fund, nil
}

// fetchBasicInfo 从天天基金估值接口获取基础信息
func (c *Client) fetchBasicInfo(ctx context.Context, code string) (*datasource.Fund, error) {
	url := fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", code)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &datasource.ErrDataSourceUnavailable{Source: "eastmoney", Reason: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &datasource.ErrFundNotFound{Code: code}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(body)
	if !strings.Contains(content, "jsonpgz") {
		return nil, &datasource.ErrFundNotFound{Code: code}
	}

	start := strings.Index(content, "(")
	end := strings.LastIndex(content, ")")
	if start == -1 || end == -1 || start >= end {
		return nil, &datasource.ErrFundNotFound{Code: code}
	}

	jsonStr := content[start+1 : end]
	var data struct {
		FundCode string `json:"fundcode"`
		Name     string `json:"name"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, err
	}

	return &datasource.Fund{
		Code: data.FundCode,
		Name: data.Name,
		Type: datasource.FundTypeUnknown,
	}, nil
}

// enrichFundDetail 从东方财富 pingzhongdata 接口补充详细信息
// 解析 JS 变量提取基金经理、规模、收益率等数据
func (c *Client) enrichFundDetail(ctx context.Context, fund *datasource.Fund) {
	url := fmt.Sprintf(fundDetailURL, fund.Code)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	content := string(body)

	// 提取基金类型（从名称推断）
	fund.Type = inferFundType(fund.Name)

	// 提取基金经理信息: var Data_currentFundManager = [{...}];
	if managerJSON := extractJSVar(content, "Data_currentFundManager"); managerJSON != "" {
		var managers []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			WorkTime string `json:"workTime"`
			FundSize string `json:"fundSize"`
		}
		if json.Unmarshal([]byte(managerJSON), &managers) == nil && len(managers) > 0 {
			m := managers[0]
			fund.Manager = &datasource.Manager{
				ID:   m.ID,
				Name: m.Name,
			}
			// 解析管理规模: "483.83亿(4只基金)"
			if sizeStr := m.FundSize; sizeStr != "" {
				var scale float64
				fmt.Sscanf(sizeStr, "%f", &scale)
				fund.Manager.TotalScale = scale
				// 提取基金数量
				if idx := strings.Index(sizeStr, "("); idx >= 0 {
					var count int
					fmt.Sscanf(sizeStr[idx+1:], "%d", &count)
					fund.Manager.FundCount = count
				}
			}
			// 解析从业年限: "13年又135天"
			if workTime := m.WorkTime; workTime != "" {
				var years int
				fmt.Sscanf(workTime, "%d", &years)
				fund.Manager.Years = float64(years)
			}
			// 基金规模 ≈ 经理管理规模 / 管理基金数
			if fund.Manager.FundCount > 0 {
				fund.Scale = fund.Manager.TotalScale / float64(fund.Manager.FundCount)
			}
		}
	}
}

// extractJSVar 从 JS 文本中提取指定变量的值
// 支持各种空格变体: var name = value; / var name =value; / var name= value; / var name=value;
func extractJSVar(js, varName string) string {
	// 找到 "var varName" 然后跳过空格和等号
	marker := "var " + varName
	idx := strings.Index(js, marker)
	if idx < 0 {
		return ""
	}

	// 从变量名后开始，跳过空格和等号
	start := idx + len(marker)
	for start < len(js) && (js[start] == ' ' || js[start] == '=' || js[start] == '\t' || js[start] == '\n') {
		start++
	}
	if start >= len(js) {
		return ""
	}

	// 找到值的结束位置（分号），处理嵌套的 [] {} 和字符串
	depth := 0
	inString := false
	stringChar := byte(0)
	for i := start; i < len(js); i++ {
		ch := js[i]
		if inString {
			if ch == stringChar && (i == 0 || js[i-1] != '\\') {
				inString = false
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inString = true
			stringChar = ch
			continue
		}
		if ch == '[' || ch == '{' {
			depth++
		} else if ch == ']' || ch == '}' {
			depth--
		} else if ch == ';' && depth == 0 {
			return strings.TrimSpace(js[start:i])
		}
	}
	return ""
}

// inferFundType 从基金名称推断基金类型
func inferFundType(name string) datasource.FundType {
	switch {
	case strings.Contains(name, "指数") || strings.Contains(name, "ETF"):
		return datasource.FundTypeIndex
	case strings.Contains(name, "混合"):
		return datasource.FundTypeMixed
	case strings.Contains(name, "股票"):
		return datasource.FundTypeStock
	case strings.Contains(name, "债") || strings.Contains(name, "纯债"):
		return datasource.FundTypeBond
	case strings.Contains(name, "货币") || strings.Contains(name, "现金"):
		return datasource.FundTypeMoney
	case strings.Contains(name, "QDII") || strings.Contains(name, "海外"):
		return datasource.FundTypeQDII
	case strings.Contains(name, "FOF"):
		return datasource.FundTypeFOF
	default:
		return datasource.FundTypeUnknown
	}
}

// GetFundNAV 获取基金净值历史（自动翻页，东方财富 API 每页最多 20 条）
func (c *Client) GetFundNAV(ctx context.Context, code string, days int) ([]datasource.NAV, error) {
	if days <= 0 {
		days = 30
	}

	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// 估算需要的交易日数（自然日 × 70%）和页数（每页 20 条）
	tradingDays := days * 7 / 10
	if tradingDays < 20 {
		tradingDays = 20
	}
	maxPages := (tradingDays / 20) + 1
	if maxPages > 20 {
		maxPages = 20 // 最多拉 20 页（400 个交易日 ≈ 1.6 年）
	}

	const pageSize = 20
	var allNavs []datasource.NAV

	for page := 1; page <= maxPages; page++ {
		navURL := fmt.Sprintf(
			"https://api.fund.eastmoney.com/f10/lsjz?fundCode=%s&pageIndex=%d&pageSize=%d&startDate=%s&endDate=%s",
			code, page, pageSize, startDate, endDate,
		)

		req, err := http.NewRequestWithContext(ctx, "GET", navURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", "https://fund.eastmoney.com/")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if len(allNavs) > 0 {
				break // 已有部分数据，不报错
			}
			return nil, &datasource.ErrDataSourceUnavailable{Source: "eastmoney", Reason: err.Error()}
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			break
		}

		var result struct {
			Data struct {
				LSJZList []struct {
					FSRQ  string `json:"FSRQ"`
					DWJZ  string `json:"DWJZ"`
					LJJZ  string `json:"LJJZ"`
					JZZZL string `json:"JZZZL"`
				} `json:"LSJZList"`
			} `json:"Data"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			break
		}

		if len(result.Data.LSJZList) == 0 {
			break // 没有更多数据
		}

		for _, item := range result.Data.LSJZList {
			unitNAV, _ := strconv.ParseFloat(item.DWJZ, 64)
			accumNAV, _ := strconv.ParseFloat(item.LJJZ, 64)
			dailyReturn, _ := strconv.ParseFloat(item.JZZZL, 64)

			allNavs = append(allNavs, datasource.NAV{
				Date:        item.FSRQ,
				UnitNAV:     unitNAV,
				AccumNAV:    accumNAV,
				DailyReturn: dailyReturn,
			})
		}

		if len(result.Data.LSJZList) < pageSize {
			break // 最后一页，不满 20 条
		}
	}

	logger.Info("获取净值历史成功", zap.String("code", code), zap.Int("days", days), zap.Int("count", len(allNavs)))
	return allNavs, nil
}

// GetFundManager 获取基金经理信息
func (c *Client) GetFundManager(ctx context.Context, code string) (*datasource.Manager, error) {
	// 通过 GetFundInfo 获取完整信息（内含经理数据）
	fund, err := c.GetFundInfo(ctx, code)
	if err != nil {
		return nil, err
	}
	if fund.Manager != nil {
		return fund.Manager, nil
	}
	return &datasource.Manager{Name: "未知"}, nil
}

// GetFundHoldings 获取基金前十大重仓股
// 解析东方财富 FundArchivesDatas 接口返回的 HTML 表格
func (c *Client) GetFundHoldings(ctx context.Context, code string) ([]datasource.Holding, error) {
	url := fmt.Sprintf(fundHoldingURL, code)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://fund.eastmoney.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &datasource.ErrDataSourceUnavailable{Source: "eastmoney", Reason: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	holdings := parseHoldingsHTML(string(body))
	logger.Info("获取基金持仓成功", zap.String("code", code), zap.Int("count", len(holdings)))
	return holdings, nil
}

// parseHoldingsHTML 从东方财富持仓页面的 HTML 中提取重仓股数据
// 响应格式: var apidata={ content:"<div>...<table>...<tbody><tr>...</tr></tbody></table>...</div>", ... }
// 表格行: <tr><td>序号</td><td><a>股票代码</a></td><td><a>股票名称</a></td><td>...</td><td>占比%</td><td>持股数</td><td>持仓市值</td></tr>
func parseHoldingsHTML(body string) []datasource.Holding {
	// 只解析第一个 <tbody>...</tbody>（最新一期持仓）
	tbodyStart := strings.Index(body, "<tbody>")
	if tbodyStart < 0 {
		return nil
	}
	tbodyEnd := strings.Index(body[tbodyStart:], "</tbody>")
	if tbodyEnd < 0 {
		return nil
	}
	tbody := body[tbodyStart : tbodyStart+tbodyEnd]

	var holdings []datasource.Holding

	// 逐行解析 <tr>...</tr>
	remaining := tbody
	for {
		trStart := strings.Index(remaining, "<tr>")
		if trStart < 0 {
			break
		}
		trEnd := strings.Index(remaining[trStart:], "</tr>")
		if trEnd < 0 {
			break
		}
		row := remaining[trStart : trStart+trEnd]
		remaining = remaining[trStart+trEnd+5:]

		h := parseHoldingRow(row)
		if h != nil {
			holdings = append(holdings, *h)
		}
	}

	return holdings
}

// parseHoldingRow 解析单行持仓数据
// <tr><td>1</td><td><a>00700</a></td><td><a>腾讯控股</a></td><td>...</td><td>...</td><td>...</td><td>9.98%</td><td>572.00</td><td>309,468.46</td></tr>
func parseHoldingRow(row string) *datasource.Holding {
	cells := extractTDValues(row)
	if len(cells) < 9 {
		return nil
	}

	// cells[0]=序号, cells[1]=股票代码, cells[2]=股票名称, ..., cells[6]=占比, cells[7]=持股数, cells[8]=持仓市值
	stockCode := extractTextContent(cells[1])
	stockName := extractTextContent(cells[2])
	ratioStr := extractTextContent(cells[6])
	shareStr := extractTextContent(cells[7])
	valueStr := extractTextContent(cells[8])

	if stockCode == "" || stockName == "" {
		return nil
	}

	ratio := parseHoldingFloat(strings.TrimSuffix(ratioStr, "%"))
	shares := parseHoldingFloat(shareStr)
	value := parseHoldingFloat(valueStr)

	return &datasource.Holding{
		StockCode:   stockCode,
		StockName:   stockName,
		Ratio:       ratio,
		ShareCount:  shares,
		MarketValue: value,
	}
}

// extractTDValues 提取 <tr> 中所有 <td>...</td> 的内容
func extractTDValues(row string) []string {
	var values []string
	remaining := row
	for {
		tdStart := strings.Index(remaining, "<td")
		if tdStart < 0 {
			break
		}
		// 跳过 <td ...> 中的属性
		tagEnd := strings.Index(remaining[tdStart:], ">")
		if tagEnd < 0 {
			break
		}
		contentStart := tdStart + tagEnd + 1
		tdEnd := strings.Index(remaining[contentStart:], "</td>")
		if tdEnd < 0 {
			break
		}
		values = append(values, remaining[contentStart:contentStart+tdEnd])
		remaining = remaining[contentStart+tdEnd+5:]
	}
	return values
}

// extractTextContent 从 HTML 片段中提取纯文本（去除标签）
func extractTextContent(html string) string {
	// 先尝试提取 <a> 标签中的文本
	aStart := strings.LastIndex(html, ">")
	if aStart >= 0 {
		aEnd := strings.Index(html[aStart:], "<")
		if aEnd > 0 {
			text := strings.TrimSpace(html[aStart+1 : aStart+aEnd])
			if text != "" {
				return text
			}
		}
	}

	// 移除所有 HTML 标签
	result := html
	for {
		start := strings.Index(result, "<")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end < 0 {
			break
		}
		result = result[:start] + result[start+end+1:]
	}
	return strings.TrimSpace(result)
}

// parseHoldingFloat 解析持仓数值（支持千分位逗号）
func parseHoldingFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "") // 移除千分位逗号: "309,468.46" → "309468.46"
	if s == "" || s == "---" || s == "--" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// 确保 Client 实现 FundDataSource 接口
var _ datasource.FundDataSource = (*Client)(nil)
