package debate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDebateResult_ConfidenceGateFields_JSON(t *testing.T) {
	result := DebateResult{
		DecisionGate:      DecisionGateReview,
		SystemConfidence:  68,
		ConfidenceReasons: []string{"insufficient_evidence"},
		ReviewAttempted:   true,
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal DebateResult failed: %v", err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("unmarshal DebateResult JSON failed: %v", err)
	}

	keys := []string{
		"decision_gate",
		"system_confidence",
		"confidence_reasons",
		"review_attempted",
	}
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			t.Fatalf("expected JSON field %q to exist, json=%s", key, string(b))
		}
	}

	if string(obj["decision_gate"]) != `"review"` {
		t.Fatalf("unexpected decision_gate: %s", string(obj["decision_gate"]))
	}
	if string(obj["system_confidence"]) != `68` {
		t.Fatalf("unexpected system_confidence: %s", string(obj["system_confidence"]))
	}
	if string(obj["review_attempted"]) != `true` {
		t.Fatalf("unexpected review_attempted: %s", string(obj["review_attempted"]))
	}
	if !strings.Contains(string(obj["confidence_reasons"]), "insufficient_evidence") {
		t.Fatalf("unexpected confidence_reasons: %s", string(obj["confidence_reasons"]))
	}
}

func TestDebateResult_ConfidenceGateFields_JSON_ZeroValueOmitempty(t *testing.T) {
	result := DebateResult{}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal DebateResult failed: %v", err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatalf("unmarshal DebateResult JSON failed: %v", err)
	}

	// 新增门控字段在零值下应被省略，避免输出无效空枚举/空数组。
	omittedKeys := []string{
		"decision_gate",
		"system_confidence",
		"confidence_reasons",
	}
	for _, key := range omittedKeys {
		if _, ok := obj[key]; ok {
			t.Fatalf("expected key %q omitted at zero value, json=%s", key, string(b))
		}
	}

	// review_attempted 是显式布尔开关，零值 false 也应保留。
	if v, ok := obj["review_attempted"]; !ok || string(v) != "false" {
		t.Fatalf("expected review_attempted=false, json=%s", string(b))
	}
}
