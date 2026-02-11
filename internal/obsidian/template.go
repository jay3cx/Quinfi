// Package obsidian 提供笔记模板系统
package obsidian

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/jay3cx/fundmind/internal/datasource"
)

// FundNote 基金笔记数据
type FundNote struct {
	Code        string    // 基金代码
	Name        string    // 基金名称
	Type        string    // 基金类型
	Scale       string    // 基金规模
	Manager     string    // 基金经理
	Company     string    // 基金公司
	NetValue    float64   // 最新净值
	DailyReturn float64   // 日涨跌幅
	WeekReturn  float64   // 近一周收益
	MonthReturn float64   // 近一月收益
	YearReturn  float64   // 近一年收益
	TopHoldings []string  // 重仓股票
	Analysis    string    // AI 分析摘要
	UpdatedAt   time.Time // 更新时间
	Tags        []string  // 标签
}

// ManagerNote 基金经理笔记数据
type ManagerNote struct {
	Name          string    // 经理姓名
	Company       string    // 所属公司
	WorkYears     float64   // 从业年限
	TotalScale    string    // 管理规模
	BestReturn    float64   // 最佳业绩
	ManagedFunds  []string  // 管理基金列表（代码）
	InvestStyle   string    // 投资风格
	Introduction  string    // 简介
	Analysis      string    // AI 分析
	UpdatedAt     time.Time // 更新时间
	Tags          []string  // 标签
}

// fundNoteTemplate 基金笔记模板
const fundNoteTemplate = `---
tags:
{{- range .Tags }}
  - {{ . }}
{{- end }}
aliases:
  - {{ .Code }}
  - {{ .Name }}
created: {{ .UpdatedAt.Format "2006-01-02" }}
updated: {{ .UpdatedAt.Format "2006-01-02T15:04:05" }}
type: fund
code: "{{ .Code }}"
---

# {{ .Name }}

## 基本信息

| 属性 | 值 |
|------|-----|
| 基金代码 | {{ .Code }} |
| 基金类型 | {{ .Type }} |
| 基金规模 | {{ .Scale }} |
| 基金经理 | [[{{ .Manager }}]] |
| 基金公司 | {{ .Company }} |

## 业绩表现

| 指标 | 数值 |
|------|------|
| 最新净值 | {{ printf "%.4f" .NetValue }} |
| 日涨跌幅 | {{ printf "%.2f%%" .DailyReturn }} |
| 近一周 | {{ printf "%.2f%%" .WeekReturn }} |
| 近一月 | {{ printf "%.2f%%" .MonthReturn }} |
| 近一年 | {{ printf "%.2f%%" .YearReturn }} |

## 重仓股票

{{- if .TopHoldings }}
{{- range .TopHoldings }}
- {{ . }}
{{- end }}
{{- else }}
暂无数据
{{- end }}

## AI 分析

{{- if .Analysis }}

{{ .Analysis }}
{{- else }}

暂无分析
{{- end }}

---
*自动生成于 {{ .UpdatedAt.Format "2006-01-02 15:04" }}*
`

// managerNoteTemplate 基金经理笔记模板
const managerNoteTemplate = `---
tags:
{{- range .Tags }}
  - {{ . }}
{{- end }}
aliases:
  - {{ .Name }}
created: {{ .UpdatedAt.Format "2006-01-02" }}
updated: {{ .UpdatedAt.Format "2006-01-02T15:04:05" }}
type: manager
---

# {{ .Name }}

## 基本信息

| 属性 | 值 |
|------|-----|
| 姓名 | {{ .Name }} |
| 所属公司 | {{ .Company }} |
| 从业年限 | {{ printf "%.1f" .WorkYears }} 年 |
| 管理规模 | {{ .TotalScale }} |
| 最佳业绩 | {{ printf "%.2f%%" .BestReturn }} |

## 管理基金

{{- if .ManagedFunds }}
{{- range .ManagedFunds }}
- [[{{ . }}]]
{{- end }}
{{- else }}
暂无数据
{{- end }}

## 投资风格

{{ .InvestStyle }}

## 简介

{{ .Introduction }}

## AI 分析

{{- if .Analysis }}

{{ .Analysis }}
{{- else }}

暂无分析
{{- end }}

---
*自动生成于 {{ .UpdatedAt.Format "2006-01-02 15:04" }}*
`

// TemplateRenderer 模板渲染器
type TemplateRenderer struct {
	fundTmpl    *template.Template
	managerTmpl *template.Template
}

// NewTemplateRenderer 创建模板渲染器
func NewTemplateRenderer() (*TemplateRenderer, error) {
	fundTmpl, err := template.New("fund").Parse(fundNoteTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析基金模板失败: %w", err)
	}

	managerTmpl, err := template.New("manager").Parse(managerNoteTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析经理模板失败: %w", err)
	}

	return &TemplateRenderer{
		fundTmpl:    fundTmpl,
		managerTmpl: managerTmpl,
	}, nil
}

// RenderFundNote 渲染基金笔记
func (r *TemplateRenderer) RenderFundNote(note *FundNote) (string, error) {
	// 添加默认标签
	if len(note.Tags) == 0 {
		note.Tags = []string{"基金", "投资"}
	}

	var buf bytes.Buffer
	if err := r.fundTmpl.Execute(&buf, note); err != nil {
		return "", fmt.Errorf("渲染基金笔记失败: %w", err)
	}

	return buf.String(), nil
}

// RenderManagerNote 渲染经理笔记
func (r *TemplateRenderer) RenderManagerNote(note *ManagerNote) (string, error) {
	// 添加默认标签
	if len(note.Tags) == 0 {
		note.Tags = []string{"基金经理", "投资"}
	}

	var buf bytes.Buffer
	if err := r.managerTmpl.Execute(&buf, note); err != nil {
		return "", fmt.Errorf("渲染经理笔记失败: %w", err)
	}

	return buf.String(), nil
}

// FundNoteFromFund 从 Fund 创建笔记数据
func FundNoteFromFund(info *datasource.Fund) *FundNote {
	managerName := ""
	if info.Manager != nil {
		managerName = info.Manager.Name
	}

	note := &FundNote{
		Code:        info.Code,
		Name:        info.Name,
		Type:        string(info.Type),
		Manager:     managerName,
		UpdatedAt:   time.Now(),
		Tags:        []string{"基金", "投资"},
	}

	// 添加类型标签
	if info.Type != "" {
		note.Tags = append(note.Tags, string(info.Type))
	}

	return note
}

// FundNotePath 生成基金笔记路径
func FundNotePath(code, name string) string {
	// 清理文件名中的非法字符
	safeName := strings.ReplaceAll(name, "/", "-")
	safeName = strings.ReplaceAll(safeName, "\\", "-")
	safeName = strings.ReplaceAll(safeName, ":", "-")
	return fmt.Sprintf("基金/%s-%s.md", code, safeName)
}

// ManagerNotePath 生成经理笔记路径
func ManagerNotePath(name string) string {
	safeName := strings.ReplaceAll(name, "/", "-")
	safeName = strings.ReplaceAll(safeName, "\\", "-")
	return fmt.Sprintf("基金经理/%s.md", safeName)
}
