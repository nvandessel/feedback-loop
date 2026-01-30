package llm

import (
	"testing"
	"time"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func TestComparisonResult_Instantiation(t *testing.T) {
	result := ComparisonResult{
		SemanticSimilarity: 0.85,
		IntentMatch:        true,
		MergeCandidate:     true,
		Reasoning:          "Both behaviors describe git workflow patterns",
	}

	if result.SemanticSimilarity != 0.85 {
		t.Errorf("expected SemanticSimilarity 0.85, got %f", result.SemanticSimilarity)
	}
	if !result.IntentMatch {
		t.Error("expected IntentMatch to be true")
	}
	if !result.MergeCandidate {
		t.Error("expected MergeCandidate to be true")
	}
	if result.Reasoning == "" {
		t.Error("expected Reasoning to be set")
	}
}

func TestMergeResult_Instantiation(t *testing.T) {
	behavior := &models.Behavior{
		ID:   "merged-1",
		Name: "Combined git workflow",
		Kind: models.BehaviorKindDirective,
	}

	result := MergeResult{
		Merged:    behavior,
		SourceIDs: []string{"b1", "b2", "b3"},
		Reasoning: "Combined three related git behaviors",
	}

	if result.Merged == nil {
		t.Error("expected Merged to be set")
	}
	if len(result.SourceIDs) != 3 {
		t.Errorf("expected 3 source IDs, got %d", len(result.SourceIDs))
	}
	if result.Reasoning == "" {
		t.Error("expected Reasoning to be set")
	}
}

func TestClientConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.Provider != "fallback" {
		t.Errorf("expected Provider 'fallback', got '%s'", config.Provider)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", config.Timeout)
	}
	if !config.FallbackToRules {
		t.Error("expected FallbackToRules to be true by default")
	}
}

func TestClientConfig_Instantiation(t *testing.T) {
	config := ClientConfig{
		Provider:        "anthropic",
		APIKey:          "sk-test-key",
		Model:           "claude-3-haiku-20240307",
		Timeout:         5 * time.Second,
		FallbackToRules: true,
	}

	if config.Provider != "anthropic" {
		t.Errorf("expected Provider 'anthropic', got '%s'", config.Provider)
	}
	if config.APIKey != "sk-test-key" {
		t.Error("expected APIKey to be set")
	}
	if config.Model != "claude-3-haiku-20240307" {
		t.Errorf("expected Model 'claude-3-haiku-20240307', got '%s'", config.Model)
	}
}

func TestComparisonResult_ZeroValues(t *testing.T) {
	var result ComparisonResult

	if result.SemanticSimilarity != 0 {
		t.Errorf("expected zero value for SemanticSimilarity, got %f", result.SemanticSimilarity)
	}
	if result.IntentMatch {
		t.Error("expected zero value false for IntentMatch")
	}
	if result.MergeCandidate {
		t.Error("expected zero value false for MergeCandidate")
	}
}

func TestMergeResult_ZeroValues(t *testing.T) {
	var result MergeResult

	if result.Merged != nil {
		t.Error("expected nil Merged behavior")
	}
	if result.SourceIDs != nil {
		t.Error("expected nil SourceIDs")
	}
}
