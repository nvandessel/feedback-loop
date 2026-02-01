package dedup

import (
	"context"
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func TestNewBehaviorMerger(t *testing.T) {
	t.Run("with no LLM", func(t *testing.T) {
		merger := NewBehaviorMerger(MergerConfig{})

		if merger.llmClient != nil {
			t.Error("expected llmClient to be nil")
		}
		if merger.useLLM {
			t.Error("expected useLLM to be false")
		}
	})

	t.Run("with LLM config", func(t *testing.T) {
		merger := NewBehaviorMerger(MergerConfig{
			UseLLM: true,
		})

		if !merger.useLLM {
			t.Error("expected useLLM to be true")
		}
	})
}

func TestBehaviorMerger_Merge(t *testing.T) {
	merger := NewBehaviorMerger(MergerConfig{})
	ctx := context.Background()

	t.Run("empty input returns error", func(t *testing.T) {
		_, err := merger.Merge(ctx, []*models.Behavior{})
		if err == nil {
			t.Error("expected error for empty input")
		}
		if err.Error() != "no behaviors to merge" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("single behavior returns same behavior", func(t *testing.T) {
		b := &models.Behavior{
			ID:   "b1",
			Name: "Test",
		}

		result, err := merger.Merge(ctx, []*models.Behavior{b})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != b {
			t.Error("expected same behavior returned for single input")
		}
	})

	t.Run("multiple behaviors merged", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{
				ID:         "b1",
				Name:       "First",
				Kind:       models.BehaviorKindDirective,
				Content:    models.BehaviorContent{Canonical: "first content"},
				Confidence: 0.8,
				Priority:   1,
			},
			{
				ID:         "b2",
				Name:       "Second",
				Kind:       models.BehaviorKindConstraint,
				Content:    models.BehaviorContent{Canonical: "second content"},
				Confidence: 0.6,
				Priority:   3,
			},
		}

		result, err := merger.Merge(ctx, behaviors)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("expected non-nil result")
		}

		// Check that merged behavior has expected properties
		if result.Priority != 3 {
			t.Errorf("Priority = %d, want 3 (max)", result.Priority)
		}
		if result.Confidence != 0.7 {
			t.Errorf("Confidence = %v, want 0.7 (average)", result.Confidence)
		}
		// Constraint has higher priority than Directive
		if result.Kind != models.BehaviorKindConstraint {
			t.Errorf("Kind = %q, want constraint (higher priority)", result.Kind)
		}
	})
}

func TestGenerateMergedID(t *testing.T) {
	t.Run("with behavior ID", func(t *testing.T) {
		behaviors := []*models.Behavior{{ID: "b1"}}
		id := generateMergedID(behaviors)
		if id != "b1-merged" {
			t.Errorf("generateMergedID() = %q, want b1-merged", id)
		}
	})

	t.Run("with empty ID", func(t *testing.T) {
		behaviors := []*models.Behavior{{ID: ""}}
		id := generateMergedID(behaviors)
		if id == "" {
			t.Error("generateMergedID() should not return empty string")
		}
		if id == "-merged" {
			t.Error("generateMergedID() should handle empty ID gracefully")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		id := generateMergedID([]*models.Behavior{})
		if id == "" {
			t.Error("generateMergedID() should not return empty string")
		}
	})
}

func TestGenerateMergedName(t *testing.T) {
	tests := []struct {
		name      string
		behaviors []*models.Behavior
		want      string
	}{
		{
			name:      "empty input",
			behaviors: []*models.Behavior{},
			want:      "Merged Behavior",
		},
		{
			name:      "single behavior with name",
			behaviors: []*models.Behavior{{Name: "Test"}},
			want:      "Test (merged)",
		},
		{
			name:      "multiple behaviors uses first name",
			behaviors: []*models.Behavior{{Name: "First"}, {Name: "Second"}},
			want:      "First (merged)",
		},
		{
			name:      "skip empty names",
			behaviors: []*models.Behavior{{Name: ""}, {Name: "Second"}},
			want:      "Second (merged)",
		},
		{
			name:      "all empty names",
			behaviors: []*models.Behavior{{Name: ""}, {Name: ""}},
			want:      "Merged Behavior",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMergedName(tt.behaviors)
			if got != tt.want {
				t.Errorf("generateMergedName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectBestKind(t *testing.T) {
	tests := []struct {
		name      string
		behaviors []*models.Behavior
		want      models.BehaviorKind
	}{
		{
			name:      "empty input returns directive",
			behaviors: []*models.Behavior{},
			want:      models.BehaviorKindDirective,
		},
		{
			name:      "single directive",
			behaviors: []*models.Behavior{{Kind: models.BehaviorKindDirective}},
			want:      models.BehaviorKindDirective,
		},
		{
			name:      "procedure beats constraint",
			behaviors: []*models.Behavior{{Kind: models.BehaviorKindConstraint}, {Kind: models.BehaviorKindProcedure}},
			want:      models.BehaviorKindProcedure,
		},
		{
			name:      "constraint beats directive",
			behaviors: []*models.Behavior{{Kind: models.BehaviorKindDirective}, {Kind: models.BehaviorKindConstraint}},
			want:      models.BehaviorKindConstraint,
		},
		{
			name:      "directive beats preference",
			behaviors: []*models.Behavior{{Kind: models.BehaviorKindPreference}, {Kind: models.BehaviorKindDirective}},
			want:      models.BehaviorKindDirective,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestKind(tt.behaviors)
			if got != tt.want {
				t.Errorf("selectBestKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeWhenConditions(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := mergeWhenConditions([]*models.Behavior{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("single behavior", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{When: map[string]interface{}{"language": "python"}},
		}
		result := mergeWhenConditions(behaviors)
		if result["language"] != "python" {
			t.Errorf("expected language=python, got %v", result["language"])
		}
	})

	t.Run("merge different keys", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{When: map[string]interface{}{"language": "python"}},
			{When: map[string]interface{}{"task": "testing"}},
		}
		result := mergeWhenConditions(behaviors)
		if result["language"] != "python" {
			t.Errorf("expected language=python, got %v", result["language"])
		}
		if result["task"] != "testing" {
			t.Errorf("expected task=testing, got %v", result["task"])
		}
	})

	t.Run("same key same value", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{When: map[string]interface{}{"language": "python"}},
			{When: map[string]interface{}{"language": "python"}},
		}
		result := mergeWhenConditions(behaviors)
		if result["language"] != "python" {
			t.Errorf("expected language=python, got %v", result["language"])
		}
	})

	t.Run("same key different values creates slice", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{When: map[string]interface{}{"language": "python"}},
			{When: map[string]interface{}{"language": "go"}},
		}
		result := mergeWhenConditions(behaviors)
		langs, ok := result["language"].([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result["language"])
		}
		if len(langs) != 2 {
			t.Errorf("expected 2 languages, got %d", len(langs))
		}
	})
}

func TestMergeConditionValues(t *testing.T) {
	t.Run("equal strings", func(t *testing.T) {
		result := mergeConditionValues("a", "a")
		if result != "a" {
			t.Errorf("expected 'a', got %v", result)
		}
	})

	t.Run("different strings create slice", func(t *testing.T) {
		result := mergeConditionValues("a", "b")
		slice, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}
		if len(slice) != 2 || slice[0] != "a" || slice[1] != "b" {
			t.Errorf("expected [a, b], got %v", slice)
		}
	})

	t.Run("merge two slices", func(t *testing.T) {
		result := mergeConditionValues([]string{"a", "b"}, []string{"b", "c"})
		slice, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}
		// Should dedupe: [a, b, c]
		if len(slice) != 3 {
			t.Errorf("expected 3 items (deduped), got %v", slice)
		}
	})

	t.Run("add string to slice", func(t *testing.T) {
		result := mergeConditionValues([]string{"a"}, "b")
		slice, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}
		if len(slice) != 2 {
			t.Errorf("expected 2 items, got %v", slice)
		}
	})

	t.Run("add slice to string", func(t *testing.T) {
		result := mergeConditionValues("a", []string{"b"})
		slice, ok := result.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T", result)
		}
		if len(slice) != 2 {
			t.Errorf("expected 2 items, got %v", slice)
		}
	})
}

func TestMergeCanonicalContent(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := mergeCanonicalContent([]*models.Behavior{})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("single behavior", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{Content: models.BehaviorContent{Canonical: "content"}},
		}
		result := mergeCanonicalContent(behaviors)
		if result != "content" {
			t.Errorf("expected 'content', got %q", result)
		}
	})

	t.Run("multiple different contents joined with semicolon", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{Content: models.BehaviorContent{Canonical: "first"}},
			{Content: models.BehaviorContent{Canonical: "second"}},
		}
		result := mergeCanonicalContent(behaviors)
		if result != "first; second" {
			t.Errorf("expected 'first; second', got %q", result)
		}
	})

	t.Run("duplicate content deduplicated", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{Content: models.BehaviorContent{Canonical: "same"}},
			{Content: models.BehaviorContent{Canonical: "same"}},
		}
		result := mergeCanonicalContent(behaviors)
		if result != "same" {
			t.Errorf("expected 'same', got %q", result)
		}
	})

	t.Run("empty content skipped", func(t *testing.T) {
		behaviors := []*models.Behavior{
			{Content: models.BehaviorContent{Canonical: ""}},
			{Content: models.BehaviorContent{Canonical: "content"}},
		}
		result := mergeCanonicalContent(behaviors)
		if result != "content" {
			t.Errorf("expected 'content', got %q", result)
		}
	})
}

func TestAverageConfidence(t *testing.T) {
	tests := []struct {
		name       string
		behaviors  []*models.Behavior
		wantResult float64
	}{
		{
			name:       "empty input",
			behaviors:  []*models.Behavior{},
			wantResult: 0.0,
		},
		{
			name:       "single behavior",
			behaviors:  []*models.Behavior{{Confidence: 0.8}},
			wantResult: 0.8,
		},
		{
			name:       "multiple behaviors",
			behaviors:  []*models.Behavior{{Confidence: 0.6}, {Confidence: 0.8}},
			wantResult: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := averageConfidence(tt.behaviors)
			if got != tt.wantResult {
				t.Errorf("averageConfidence() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestMaxPriority(t *testing.T) {
	tests := []struct {
		name      string
		behaviors []*models.Behavior
		want      int
	}{
		{
			name:      "empty input",
			behaviors: []*models.Behavior{},
			want:      0,
		},
		{
			name:      "single behavior",
			behaviors: []*models.Behavior{{Priority: 5}},
			want:      5,
		},
		{
			name:      "multiple behaviors",
			behaviors: []*models.Behavior{{Priority: 3}, {Priority: 7}, {Priority: 2}},
			want:      7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxPriority(tt.behaviors)
			if got != tt.want {
				t.Errorf("maxPriority() = %d, want %d", got, tt.want)
			}
		})
	}
}
