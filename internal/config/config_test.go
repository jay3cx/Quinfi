package config

import "testing"

func TestDefaultConfig_DebateConfidenceDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Debate.Confidence.ReviewThreshold != 70 {
		t.Fatalf("expected review_threshold=70, got %d", cfg.Debate.Confidence.ReviewThreshold)
	}
	if cfg.Debate.Confidence.PassThreshold != 75 {
		t.Fatalf("expected pass_threshold=75, got %d", cfg.Debate.Confidence.PassThreshold)
	}
	if cfg.Debate.Confidence.HardCap != 60 {
		t.Fatalf("expected hard_cap=60, got %d", cfg.Debate.Confidence.HardCap)
	}
}

func TestValidate_DebateConfidenceRejectsInvalidRange(t *testing.T) {
	cfg := DefaultConfig()

	cfg.Debate.Confidence.PassThreshold = 101
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for pass_threshold=101")
	}

	cfg = DefaultConfig()
	cfg.Debate.Confidence.ReviewThreshold = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for review_threshold=0")
	}

	cfg = DefaultConfig()
	cfg.Debate.Confidence.HardCap = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for hard_cap=-1")
	}

	cfg = DefaultConfig()
	cfg.Debate.Confidence.ReviewThreshold = 80
	cfg.Debate.Confidence.PassThreshold = 70
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for review_threshold >= pass_threshold")
	}
}
