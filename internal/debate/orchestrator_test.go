package debate

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/jay3cx/Quinfi/internal/agent"
	"github.com/jay3cx/Quinfi/pkg/llm"
)

type safeMockClient struct {
	mu       sync.Mutex
	chatFunc func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error)
}

func (m *safeMockClient) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return &llm.ChatResponse{Content: "{}", Model: "test"}, nil
}

func (m *safeMockClient) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

var _ llm.Client = (*safeMockClient)(nil)

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
	mock := &safeMockClient{}
	mock.chatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		callIdx++
		system := req.Messages[0].Content
		user := req.Messages[1].Content
		if strings.Contains(system, "看多分析师") && !strings.Contains(user, "看空方的论点") {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"看好蓝筹精选","points":["近30天净值上涨3.3%","基金经理年化回报12%","持仓估值处历史30%分位"],"confidence":82}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "风险分析师") && !strings.Contains(user, "看多方") {
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"风险可控但需警惕","points":["前十大持仓占比58%","白酒板块波动率18%","规模500亿影响调仓灵活性"],"confidence":70}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "看多分析师") && strings.Contains(user, "看空方的论点") {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"风险可控","points":["过去3年同类排名前20%","消费龙头ROE稳定在15%以上"],"confidence":78}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "风险分析师") && strings.Contains(user, "看多方") {
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"短期不确定性仍在","points":["单一行业暴露超过40%","宏观下行周期中回撤或达10%"],"confidence":72}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "投资分析裁判") {
			return &llm.ChatResponse{
				Content: `{"summary":"综合来看，该基金具有一定投资价值但需关注短期风险","bull_strength":"基金经理投研能力突出","bear_strength":"持仓集中度风险不容忽视","suggestion":"适合风险偏好中等、投资周期1年以上的投资者","risk_warnings":["白酒板块政策风险","持仓集中度高"],"confidence":82}`,
				Model:   "test",
			}, nil
		}
		return &llm.ChatResponse{Content: "{}", Model: "test"}, nil
	}

	orch := NewOrchestratorWithConfidence(mock, newMockTools(), ConfidenceConfig{
		ReviewThreshold: 75,
		PassThreshold:   80,
		HardCap:         60,
	})
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
	if result.BullCase.Confidence != 82 {
		t.Errorf("expected bull confidence 82, got %d", result.BullCase.Confidence)
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
	if result.Verdict.Confidence != 82 {
		t.Errorf("expected verdict confidence 82, got %d", result.Verdict.Confidence)
	}
	if len(result.Verdict.RiskWarnings) != 2 {
		t.Errorf("expected 2 risk warnings, got %d", len(result.Verdict.RiskWarnings))
	}

	// 验证无错误
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.DecisionGate != DecisionGatePass {
		t.Errorf("expected decision gate pass, got %s", result.DecisionGate)
	}
	if result.ReviewAttempted {
		t.Errorf("expected no review on strong sample")
	}
	if result.SystemConfidence <= 0 {
		t.Errorf("expected positive system confidence, got %d", result.SystemConfidence)
	}

	// 验证 LLM 被调用了 5 次（4轮辩论 + 1裁决）
	if callIdx != 5 {
		t.Errorf("expected 5 LLM calls, got %d", callIdx)
	}
}

func TestRunDebate_PartialFailure(t *testing.T) {
	mock := &safeMockClient{}
	mock.chatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		system := req.Messages[0].Content
		user := req.Messages[1].Content
		if strings.Contains(system, "看多分析师") {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"看好","points":["近30天上涨2%","理由1"],"confidence":60}`,
				Model:   "test",
			}, nil
		}
		// Bear 立论故意返回无法解析内容，触发 parse_failure
		if strings.Contains(system, "风险分析师") && !strings.Contains(user, "看多方") {
			return &llm.ChatResponse{
				Content: "这不是有效的 JSON",
				Model:   "test",
			}, nil
		}
		// 其余阶段正常
		if strings.Contains(system, "风险分析师") {
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"降级","points":["回撤8%","降级论点"],"confidence":50}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "投资分析裁判") {
			return &llm.ChatResponse{
				Content: `{"summary":"存在争议","bull_strength":"收益改善","bear_strength":"波动提升","suggestion":"建议观察","risk_warnings":["波动风险"],"confidence":62}`,
				Model:   "test",
			}, nil
		}
		return &llm.ChatResponse{
			Content: "{}",
			Model:   "test",
		}, nil
	}

	orch := NewOrchestratorWithConfidence(mock, newMockTools(), ConfidenceConfig{
		ReviewThreshold: 75,
		PassThreshold:   80,
		HardCap:         60,
	})
	result, err := orch.RunDebate(context.Background(), "005827")
	if err != nil {
		t.Fatalf("RunDebate failed: %v", err)
	}

	// Bull 应该成功
	if result.BullCase == nil {
		t.Fatal("BullCase should not be nil")
	}

	// Bear 应该降级（用原始文本构建）
	if result.BearCase == nil {
		t.Fatal("BearCase should not be nil even on parse failure")
	}
	if result.BearCase.Confidence != 50 {
		t.Fatalf("expected bear fallback confidence 50, got %d", result.BearCase.Confidence)
	}
	if result.DecisionGate != DecisionGateDegrade {
		t.Fatalf("expected degrade on parse failure, got %s", result.DecisionGate)
	}
	if result.Verdict == nil {
		t.Fatal("expected verdict to exist")
	}
	if result.Verdict.Suggestion != "证据不足，建议观望" {
		t.Fatalf("expected degrade suggestion, got %s", result.Verdict.Suggestion)
	}
}

func TestRunDebate_ReviewPath_UsesConsistencyScore(t *testing.T) {
	callIdx := 0
	judgeCalls := 0
	mock := &safeMockClient{}
	mock.chatFunc = func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
		callIdx++
		system := req.Messages[0].Content
		user := req.Messages[1].Content
		if strings.Contains(system, "看多分析师") && !strings.Contains(user, "看空方的论点") {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"中性偏多","points":["近30天上涨2%","基本面稳定"],"confidence":72}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "风险分析师") && !strings.Contains(user, "看多方") {
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"中性偏空","points":["波动率上升","最大回撤8%"],"confidence":70}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "看多分析师") && strings.Contains(user, "看空方的论点") {
			return &llm.ChatResponse{
				Content: `{"role":"bull","position":"风险可控","points":["仓位管理完善","夏普比率1.2"],"confidence":71}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "风险分析师") && strings.Contains(user, "看多方") {
			return &llm.ChatResponse{
				Content: `{"role":"bear","position":"仍需谨慎","points":["行业轮动风险","集中度35%"],"confidence":69}`,
				Model:   "test",
			}, nil
		}
		if strings.Contains(system, "投资分析裁判") {
			judgeCalls++
			if judgeCalls == 1 {
				return &llm.ChatResponse{
					Content: `{"summary":"建议继续观察并分批参与","bull_strength":"估值回落后性价比改善","bear_strength":"行业轮动导致波动","suggestion":"可小仓位试探并设置止损","risk_warnings":["行业轮动风险","回撤风险"],"confidence":72}`,
					Model:   "test",
				}, nil
			}
			return &llm.ChatResponse{
				Content: `{"summary":"建议谨慎参与并分批布局","bull_strength":"估值修复空间仍在","bear_strength":"短期波动未完全释放","suggestion":"小仓位分批买入并严格止损","risk_warnings":["行业轮动风险","回撤风险"],"confidence":70}`,
				Model:   "test",
			}, nil
		}
		return &llm.ChatResponse{Content: "{}", Model: "test"}, nil
	}

	orch := NewOrchestratorWithConfidence(mock, newMockTools(), ConfidenceConfig{
		ReviewThreshold: 75,
		PassThreshold:   80,
		HardCap:         60,
	})
	result, err := orch.RunDebate(context.Background(), "005827")
	if err != nil {
		t.Fatalf("RunDebate failed: %v", err)
	}
	if !result.ReviewAttempted {
		t.Fatalf("expected review attempted on borderline sample")
	}
	if result.ConfidenceDetail == nil {
		t.Fatal("expected confidence detail")
	}
	if result.ConfidenceDetail.ConsistencyScore <= 0 {
		t.Fatalf("expected positive consistency score, got %d", result.ConfidenceDetail.ConsistencyScore)
	}
	if result.SystemConfidence != result.ConfidenceDetail.FinalScore {
		t.Fatalf("system confidence and final score mismatch: %d vs %d", result.SystemConfidence, result.ConfidenceDetail.FinalScore)
	}
	if result.DecisionGate == DecisionGateReview {
		t.Fatalf("final decision should not stay review")
	}
	if callIdx != 6 {
		t.Fatalf("expected 6 llm calls (including review), got %d", callIdx)
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
