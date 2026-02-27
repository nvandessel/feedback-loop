package edges

import (
	"context"
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
)

func TestLoadBehaviorsFromStore(t *testing.T) {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	// Add behavior nodes
	s.AddNode(ctx, store.Node{
		ID:   "b-1",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name":      "test behavior 1",
			"canonical": "do something",
		},
	})
	s.AddNode(ctx, store.Node{
		ID:   "b-2",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name":      "test behavior 2",
			"canonical": "do something else",
		},
	})

	// Add a non-behavior node (correction)
	s.AddNode(ctx, store.Node{
		ID:   "c-1",
		Kind: "correction",
		Content: map[string]interface{}{
			"wrong": "did X",
			"right": "do Y",
		},
	})

	behaviors, err := LoadBehaviorsFromStore(ctx, s)
	if err != nil {
		t.Fatalf("LoadBehaviorsFromStore() error = %v", err)
	}

	if len(behaviors) != 2 {
		t.Errorf("got %d behaviors, want 2 (should skip non-behavior nodes)", len(behaviors))
	}

	// Verify the IDs are correct
	ids := make(map[string]bool)
	for _, b := range behaviors {
		ids[b.ID] = true
	}
	if !ids["b-1"] || !ids["b-2"] {
		t.Errorf("expected b-1 and b-2, got %v", ids)
	}
}

func TestLoadBehaviorsFromStore_Empty(t *testing.T) {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	behaviors, err := LoadBehaviorsFromStore(ctx, s)
	if err != nil {
		t.Fatalf("LoadBehaviorsFromStore() error = %v", err)
	}
	if len(behaviors) != 0 {
		t.Errorf("got %d behaviors, want 0", len(behaviors))
	}
}

func TestComputeBehaviorSimilarity(t *testing.T) {
	// Two identical behaviors should have high similarity
	a := &models.Behavior{
		ID:   "b-1",
		Name: "Go error handling",
		When: map[string]interface{}{"language": "go"},
		Content: models.BehaviorContent{
			Canonical: "use error wrapping with fmt context",
			Tags:      []string{"go", "errors"},
		},
		Confidence: 0.8,
	}
	b := &models.Behavior{
		ID:   "b-2",
		Name: "Go error handling copy",
		When: map[string]interface{}{"language": "go"},
		Content: models.BehaviorContent{
			Canonical: "use error wrapping with fmt context",
			Tags:      []string{"go", "errors"},
		},
		Confidence: 0.8,
	}

	score := ComputeBehaviorSimilarity(a, b, nil, false, nil)
	if score < 0.8 {
		t.Errorf("identical behaviors similarity = %.4f, want >= 0.8", score)
	}

	// Two completely different behaviors should have low similarity
	c := &models.Behavior{
		ID:   "b-3",
		Name: "Python typing",
		When: map[string]interface{}{"language": "python"},
		Content: models.BehaviorContent{
			Canonical: "use type hints for function parameters",
			Tags:      []string{"python", "typing"},
		},
		Confidence: 0.8,
	}

	score2 := ComputeBehaviorSimilarity(a, c, nil, false, nil)
	if score2 > 0.5 {
		t.Errorf("different behaviors similarity = %.4f, want < 0.5", score2)
	}
}
