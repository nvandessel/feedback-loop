package pack

import (
	"context"
	"testing"

	"github.com/nvandessel/floop/internal/models"
	"github.com/nvandessel/floop/internal/store"
)

func addTestBehavior(t *testing.T, ctx context.Context, s store.GraphStore, b models.Behavior) {
	t.Helper()
	node := models.BehaviorToNode(&b)
	if _, err := s.AddNode(ctx, node); err != nil {
		t.Fatalf("failed to add node %s: %v", b.ID, err)
	}
}

func TestIntegratePackBehaviors_CreatesEdges(t *testing.T) {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	// Pre-existing behavior
	existing := models.Behavior{
		ID:   "b-existing",
		Name: "Go error conventions",
		When: map[string]interface{}{"language": "go"},
		Content: models.BehaviorContent{
			Canonical: "use error wrapping with fmt context propagation",
			Tags:      []string{"go", "errors"},
		},
		Confidence: 0.8,
	}
	addTestBehavior(t, ctx, s, existing)

	// New pack behavior that is similar
	newBehavior := models.Behavior{
		ID:   "b-pack-new",
		Name: "Go error API patterns",
		When: map[string]interface{}{"language": "go"},
		Content: models.BehaviorContent{
			Canonical: "use error wrapping and custom error types for API context",
			Tags:      []string{"go", "api"},
		},
		Confidence: 0.8,
	}
	addTestBehavior(t, ctx, s, newBehavior)

	result, err := IntegratePackBehaviors(ctx, s, []string{"b-pack-new"})
	if err != nil {
		t.Fatalf("IntegratePackBehaviors() error = %v", err)
	}

	// Should derive edges between the new and existing behaviors
	if result.EdgesCreated == 0 {
		t.Error("expected at least one edge to be created")
		for _, pe := range result.ProposedEdges {
			t.Logf("proposed: %s -> %s (%s, score=%.4f)", pe.Source, pe.Target, pe.Kind, pe.Score)
		}
	}

	if result.NewBehaviors != 1 {
		t.Errorf("NewBehaviors = %d, want 1", result.NewBehaviors)
	}
	if result.TotalBehaviors != 2 {
		t.Errorf("TotalBehaviors = %d, want 2", result.TotalBehaviors)
	}
}

func TestIntegratePackBehaviors_EmptyNewIDs(t *testing.T) {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	result, err := IntegratePackBehaviors(ctx, s, []string{})
	if err != nil {
		t.Fatalf("IntegratePackBehaviors() error = %v", err)
	}

	if result.EdgesCreated != 0 {
		t.Errorf("EdgesCreated = %d, want 0", result.EdgesCreated)
	}
	if result.NewBehaviors != 0 {
		t.Errorf("NewBehaviors = %d, want 0", result.NewBehaviors)
	}
}

func TestIntegratePackBehaviors_SkipsExistingExisting(t *testing.T) {
	ctx := context.Background()
	s := store.NewInMemoryGraphStore()

	// Two existing behaviors that are similar
	existing1 := models.Behavior{
		ID:   "b-existing-1",
		Name: "Git branching",
		When: map[string]interface{}{},
		Content: models.BehaviorContent{
			Canonical: "always create feature branches for new work",
			Tags:      []string{"git", "worktree", "branching"},
		},
		Confidence: 0.8,
	}
	existing2 := models.Behavior{
		ID:   "b-existing-2",
		Name: "Worktree cleanup",
		When: map[string]interface{}{},
		Content: models.BehaviorContent{
			Canonical: "remove stale worktrees after merging pull requests",
			Tags:      []string{"git", "worktree", "cleanup"},
		},
		Confidence: 0.8,
	}

	// One new behavior that is completely different
	newBehavior := models.Behavior{
		ID:   "b-new",
		Name: "Python typing",
		When: map[string]interface{}{"language": "python"},
		Content: models.BehaviorContent{
			Canonical: "use type hints for all function parameters and return values",
			Tags:      []string{"python", "typing"},
		},
		Confidence: 0.8,
	}

	for _, b := range []models.Behavior{existing1, existing2, newBehavior} {
		addTestBehavior(t, ctx, s, b)
	}

	result, err := IntegratePackBehaviors(ctx, s, []string{"b-new"})
	if err != nil {
		t.Fatalf("IntegratePackBehaviors() error = %v", err)
	}

	// Verify no edges between existing-1 and existing-2
	for _, pe := range result.ProposedEdges {
		if (pe.Source == "b-existing-1" && pe.Target == "b-existing-2") ||
			(pe.Source == "b-existing-2" && pe.Target == "b-existing-1") {
			t.Errorf("unexpected edge between existing behaviors: %s -> %s (%s)", pe.Source, pe.Target, pe.Kind)
		}
	}

	// PairsCompared should be new*existing + new*(new-1)/2 = 1*2 + 0 = 2
	if result.PairsCompared != 2 {
		t.Errorf("PairsCompared = %d, want 2 (only new<->existing pairs)", result.PairsCompared)
	}
}
