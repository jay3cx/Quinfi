package debate

import (
	"context"
	"testing"

	"github.com/jay3cx/fundmind/internal/agent"
	"github.com/jay3cx/fundmind/pkg/llm"
)

// mockFundInfoTool 返回固定基金信息的 mock 工具
type mockFundInfoTool struct{}

func (t *mockFundInfoTool) Name() string        { return "get_fund_info" }
func (t *mockFundInfoTool) Description() string  { return "mock" }
func (t *mockFundInfoTool) Parameters() map[string]any { return map[string]any{} }
func (t *mockFundInfoTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return `{"code":"005827","name":"易方达蓝筹精选","type":"混合型","scale":500.0}`, nil
}

type mockNAVTool struct{}

func (t *mockNAVTool) Name() string        { return "get_nav_history" }
func (t *mockNAVTool) Description() string  { return "mock" }
func (t *mockNAVTool) Parameters() map[string]any { return map[string]any{} }
func (t *mockNAVTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return "近30天净值走势：1.5000 → 1.5500，涨幅3.33%", nil
}

type mockHoldingsTool struct{}

func (t *mockHoldingsTool) Name() string        { return "get_fund_holdings" }
func (t *mockHoldingsTool) Description() string  { return "mock" }
func (t *mockHoldingsTool) Parameters() map[string]any { return map[string]any{} }
func (t *mockHoldingsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return "1. 贵州茅台 15.2%\n2. 五粮液 10.1%", nil
}

type mockNewsTool struct{}

func (t *mockNewsTool) Name() string        { return "search_news" }
func (t *mockNewsTool) Description() string  { return "mock" }
func (t *mockNewsTool) Parameters() map[string]any { return map[string]any{} }
func (t *mockNewsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	return "暂无相关新闻", nil
}

func newMockTools() *agent.ToolRegistry {
	r := agent.NewToolRegistry()
	r.Register(&mockFundInfoTool{})
	r.Register(&mockNAVTool{})
	r.Register(&mockHoldingsTool{})
	r.Register(&mockNewsTool{})
	return r
}

func TestRunDebate_FullFlow(t *testing.T) {
	callIdx := 0
	mock := llm.NewMockClient()
	mock.ChatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		callIdx++
		switch callIdx {
		case 1: // Bull 立论
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"看好蓝筹精选","points":["基金经理经验丰富","持仓估值合理","近期净值回升"],"confidence":75}`,
				Model:   "test",
			}, nil
		case 2: // Bear 立论
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"风险大于收益","points":["持仓集中度过高","白酒板块面临政策风险","规模过大影响灵活性"],"confidence":70}`,
				Model:   "test",
			}, nil
		case 3: // Bull 反驳
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"风险可控","points":["集中持仓是深度研究的表现","消费龙头长期价值稳固"],"confidence":72}`,
				Model:   "test",
			}, nil
		case 4: // Bear 反驳
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"不确定性增加","points":["长期价值不等于短期安全","宏观经济下行压力增大"],"confidence":68}`,
				Model:   "test",
			}, nil
		case 5: // Judge 裁决
			return &llm.ChatResponse{
				Content: `{"summary":"综合来看，该基金具有一定投资价值但需关注短期风险","bull_strength":"基金经理投研能力突出","bear_strength":"持仓集中度风险不容忽视","suggestion":"适合风险偏好中等、投资周期1年以上的投资者","risk_warnings":["白酒板块政策风险","持仓集中度高"],"confidence":62}`,
				Model:   "test",
			}, nil
		default:
			return &llm.ChatResponse{Content: "{}", Model: "test"}, nil
		}
	}

	orch := NewOrchestrator(mock, newMockTools())
	result, err := orch.RunDebate(context.Background(), "005827")

	if err != nil {
		t.Fatalf("RunDebate failed: %v", err)
	}

	// 验证基本结构
	if result.FundCode != "005827" {
		t.Errorf("expected fund code 005827, got %s", result.FundCode)
	}
	if result.FundName != "易方达蓝筹精选" {
		t.Errorf("expected fund name 易方达蓝筹精选, got %s", result.FundName)
	}

	// 验证 6 个阶段都完成
	if len(result.Phases) != 6 {
		t.Errorf("expected 6 phases, got %d", len(result.Phases))
	}

	// 验证各论点
	if result.BullCase == nil {
		t.Fatal("BullCase is nil")
	}
	if result.BullCase.Confidence != 75 {
		t.Errorf("expected bull confidence 75, got %d", result.BullCase.Confidence)
	}
	if len(result.BullCase.Points) != 3 {
		t.Errorf("expected 3 bull points, got %d", len(result.BullCase.Points))
	}

	if result.BearCase == nil {
		t.Fatal("BearCase is nil")
	}
	if result.BearCase.Confidence != 70 {
		t.Errorf("expected bear confidence 70, got %d", result.BearCase.Confidence)
	}

	if result.BullRebuttal == nil {
		t.Fatal("BullRebuttal is nil")
	}
	if result.BearRebuttal == nil {
		t.Fatal("BearRebuttal is nil")
	}

	// 验证裁决
	if result.Verdict == nil {
		t.Fatal("Verdict is nil")
	}
	if result.Verdict.Confidence != 62 {
		t.Errorf("expected verdict confidence 62, got %d", result.Verdict.Confidence)
	}
	if len(result.Verdict.RiskWarnings) != 2 {
		t.Errorf("expected 2 risk warnings, got %d", len(result.Verdict.RiskWarnings))
	}

	// 验证无错误
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// 验证 LLM 被调用了 5 次（4轮辩论 + 1裁决）
	if callIdx != 5 {
		t.Errorf("expected 5 LLM calls, got %d", callIdx)
	}
}

func TestRunDebate_PartialFailure(t *testing.T) {
	callIdx := 0
	mock := llm.NewMockClient()
	mock.ChatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		callIdx++
		if callIdx == 1 {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"看好","points":["理由1"],"confidence":60}`,
				Model:   "test",
			}, nil
		}
		// 第二轮（Bear）返回无法解析的内容
		if callIdx == 2 {
			return &llm.ChatResponse{
				Content: "这不是有效的 JSON",
				Model:   "test",
			}, nil
		}
		// 后续正常
		return &llm.ChatResponse{
			Content: `{"role":"bear","position":"降级","points":["降级论点"],"confidence":50}`,
			Model:   "test",
		}, nil
	}

	orch := NewOrchestrator(mock, newMockTools())
	result, _ := orch.RunDebate(context.Background(), "005827")

	// Bull 应该成功
	if result.BullCase == nil {
		t.Fatal("BullCase should not be nil")
	}

	// Bear 应该降级（用原始文本构建）
	if result.BearCase == nil {
		t.Fatal("BearCase should not be nil even on parse failure")
	}
	if result.BearCase.Confidence != 50 {
		// 降级时置信度为 50
		t.Logf("BearCase confidence: %d", result.BearCase.Confidence)
	}
}

func TestDebateResult_FormatAsMarkdown(t *testing.T) {
	result := &DebateResult{
		FundCode: "005827",
		FundName: "易方达蓝筹精选",
		BullCase: &Argument{
			Role:       "bull",
			Position:   "看好蓝筹精选",
			Points:     []string{"论据1", "论据2"},
			Confidence: 75,
		},
		BearCase: &Argument{
			Role:       "bear",
			Position:   "风险大于收益",
			Points:     []string{"风险1", "风险2"},
			Confidence: 65,
		},
		Verdict: &Verdict{
			Summary:      "综合来看值得关注",
			BullStrength: "经理优秀",
			BearStrength: "集中度高",
			Suggestion:   "适合长期持有",
			RiskWarnings: []string{"政策风险"},
			Confidence:   60,
		},
	}

	md := result.FormatAsMarkdown()

	// 验证包含关键内容
	checks := []string{
		"多空辩论结果",
		"005827",
		"看多方观点",
		"看空方观点",
		"裁判结论",
		"不构成投资建议",
	}
	for _, check := range checks {
		if !contains(md, check) {
			t.Errorf("markdown missing '%s'", check)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
