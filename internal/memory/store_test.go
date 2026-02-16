package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jay3cx/Quinfi/internal/db"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewStore(sqlDB)
}

func TestStore_SaveAndRecall(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// 存储记忆
	entries := []MemoryEntry{
		{UserID: "u1", Type: TypeProfile, Content: "用户风险偏好中等", Importance: 0.8},
		{UserID: "u1", Type: TypeFact, Content: "用户持有 005827 易方达蓝筹精选", Importance: 0.9},
		{UserID: "u1", Type: TypePreference, Content: "用户关注新能源板块", Importance: 0.6},
	}

	err := store.Save(ctx, entries)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// 召回 — 无查询（返回全部按评分排序）
	results, err := store.Recall(ctx, "u1", "", 10)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 memories, got %d", len(results))
	}

	// 召回 — 有查询（应该优先返回匹配的）
	results, err = store.Recall(ctx, "u1", "005827 基金", 10)
	if err != nil {
		t.Fatalf("recall with query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty recall")
	}
	// 第一条应该是包含 005827 的那条（关键词匹配权重最高）
	if results[0].Type != TypeFact {
		t.Errorf("expected first result to be fact type, got %s", results[0].Type)
	}
}

func TestStore_Dedup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	entry := MemoryEntry{UserID: "u1", Type: TypeFact, Content: "用户持有 005827", Importance: 0.9}

	// 存储两次相同内容
	store.Save(ctx, []MemoryEntry{entry})
	store.Save(ctx, []MemoryEntry{entry})

	results, _ := store.Recall(ctx, "u1", "", 10)
	if len(results) != 1 {
		t.Errorf("expected 1 after dedup, got %d", len(results))
	}
}

func TestStore_Invalidate(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Save(ctx, []MemoryEntry{
		{UserID: "u1", Type: TypeFact, Content: "用户持有 005827", Importance: 0.9},
	})

	results, _ := store.Recall(ctx, "u1", "", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}

	// 使其失效
	store.Invalidate(ctx, results[0].ID)

	// 再次召回应为空（valid_to 已设置）
	results, _ = store.Recall(ctx, "u1", "", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 after invalidation, got %d", len(results))
	}
}

func TestStore_UserIsolation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Save(ctx, []MemoryEntry{
		{UserID: "u1", Type: TypeFact, Content: "u1 持有基金A", Importance: 0.8},
		{UserID: "u2", Type: TypeFact, Content: "u2 持有基金B", Importance: 0.8},
	})

	r1, _ := store.Recall(ctx, "u1", "", 10)
	r2, _ := store.Recall(ctx, "u2", "", 10)

	if len(r1) != 1 || r1[0].Content != "u1 持有基金A" {
		t.Errorf("u1 isolation failed: %v", r1)
	}
	if len(r2) != 1 || r2[0].Content != "u2 持有基金B" {
		t.Errorf("u2 isolation failed: %v", r2)
	}
}

func TestScoreMemories_Ranking(t *testing.T) {
	now := time.Now()

	memories := []MemoryEntry{
		{Type: TypePreference, Content: "用户关注消费板块", Importance: 0.5, CreatedAt: now.Add(-7 * 24 * time.Hour)},
		{Type: TypeProfile, Content: "用户风险偏好中等", Importance: 0.8, CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{Type: TypeFact, Content: "用户持有 005827 易方达蓝筹精选", Importance: 0.9, CreatedAt: now.Add(-1 * 24 * time.Hour)},
	}

	// 查询与 005827 相关
	scored := scoreMemories(memories, "005827")

	// 第一条应该是 fact（关键词匹配 + 高重要性 + 最近）
	if scored[0].Type != TypeFact {
		t.Errorf("expected fact first, got %s: %s", scored[0].Type, scored[0].Content)
	}
}

func TestScoreMemories_NoQuery(t *testing.T) {
	now := time.Now()

	memories := []MemoryEntry{
		{Type: TypePreference, Content: "关注消费板块", Importance: 0.5, CreatedAt: now},
		{Type: TypeProfile, Content: "风险偏好中等", Importance: 0.8, CreatedAt: now},
	}

	// 空查询应该按 importance + type_boost 排序
	scored := scoreMemories(memories, "")

	// profile 应该排在前面（importance 0.8 + type_boost 1.0）
	if scored[0].Type != TypeProfile {
		t.Errorf("expected profile first with empty query, got %s", scored[0].Type)
	}
}

func TestFormatMemoriesForPrompt(t *testing.T) {
	memories := []MemoryEntry{
		{Type: TypeProfile, Content: "风险偏好中等"},
		{Type: TypeFact, Content: "持有 005827"},
	}

	result := FormatMemoriesForPrompt(memories)

	if result == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsStr(result, "[profile]") {
		t.Error("missing profile tag")
	}
	if !containsStr(result, "[fact]") {
		t.Error("missing fact tag")
	}
	if !containsStr(result, "关于这位用户") {
		t.Error("missing header")
	}
}

func TestFormatMemoriesForPrompt_Empty(t *testing.T) {
	result := FormatMemoriesForPrompt(nil)
	if result != "" {
		t.Errorf("expected empty for nil input, got %q", result)
	}
}

func TestParseExtractedMemories(t *testing.T) {
	json := `[
		{"type": "fact", "content": "用户持有 005827", "importance": 0.9},
		{"type": "preference", "content": "关注新能源", "importance": 0.6}
	]`

	entries := parseExtractedMemories(json, "sess-1")

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Type != TypeFact {
		t.Errorf("expected fact, got %s", entries[0].Type)
	}
	if entries[0].Importance != 0.9 {
		t.Errorf("expected importance 0.9, got %f", entries[0].Importance)
	}
	if entries[1].SourceSession != "sess-1" {
		t.Errorf("expected session sess-1, got %s", entries[1].SourceSession)
	}
}

func TestParseExtractedMemories_Empty(t *testing.T) {
	entries := parseExtractedMemories("[]", "s1")
	if len(entries) != 0 {
		t.Errorf("expected 0 for empty array, got %d", len(entries))
	}
}

func TestParseExtractedMemories_Invalid(t *testing.T) {
	entries := parseExtractedMemories("not json at all", "s1")
	if entries != nil {
		t.Errorf("expected nil for invalid JSON, got %v", entries)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
