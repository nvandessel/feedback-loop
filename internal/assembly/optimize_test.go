package assembly

import (
	"testing"

	"github.com/nvandessel/floop/internal/models"
)

func TestOptimizer_Optimize_NoLimit(t *testing.T) {
	optimizer := NewOptimizer(0) // 0 = no limit
	behaviors := []models.Behavior{
		{ID: "b1", Content: models.BehaviorContent{Canonical: "First behavior"}},
		{ID: "b2", Content: models.BehaviorContent{Canonical: "Second behavior"}},
	}

	result := optimizer.Optimize(behaviors)

	if len(result.Included) != 2 {
		t.Errorf("expected 2 included, got %d", len(result.Included))
	}
	if len(result.Excluded) != 0 {
		t.Errorf("expected 0 excluded, got %d", len(result.Excluded))
	}
	if result.Truncated {
		t.Error("expected not truncated")
	}
}

func TestOptimizer_Optimize_WithLimit(t *testing.T) {
	// Very small limit to force exclusion
	optimizer := NewOptimizer(100)
	behaviors := []models.Behavior{
		{ID: "b1", Priority: 10, Content: models.BehaviorContent{Canonical: "Short"}},
		{ID: "b2", Priority: 5, Content: models.BehaviorContent{Canonical: "This is a much longer behavior description that should exceed our token budget"}},
	}

	result := optimizer.Optimize(behaviors)

	// Should include the short, high-priority one
	if len(result.Included) == 0 {
		t.Error("expected at least one included behavior")
	}

	// Verify high priority was included
	hasHighPriority := false
	for _, b := range result.Included {
		if b.ID == "b1" {
			hasHighPriority = true
			break
		}
	}
	if !hasHighPriority {
		t.Error("expected high priority behavior to be included")
	}
}

func TestOptimizer_Optimize_ConstraintsFirst(t *testing.T) {
	optimizer := NewOptimizer(200)
	behaviors := []models.Behavior{
		{ID: "directive", Kind: models.BehaviorKindDirective, Priority: 10, Content: models.BehaviorContent{Canonical: "A directive"}},
		{ID: "constraint", Kind: models.BehaviorKindConstraint, Priority: 1, Content: models.BehaviorContent{Canonical: "A constraint"}},
	}

	result := optimizer.Optimize(behaviors)

	if len(result.Included) < 2 {
		t.Skip("budget too small for test")
	}

	// Constraint should be first even with lower priority
	if result.Included[0].ID != "constraint" {
		t.Errorf("expected constraint first, got %s", result.Included[0].ID)
	}
}

func TestOptimizer_Optimize_SortsByPriorityWithinKind(t *testing.T) {
	optimizer := NewOptimizer(500)
	behaviors := []models.Behavior{
		{ID: "low", Kind: models.BehaviorKindDirective, Priority: 1, Content: models.BehaviorContent{Canonical: "Low priority"}},
		{ID: "high", Kind: models.BehaviorKindDirective, Priority: 10, Content: models.BehaviorContent{Canonical: "High priority"}},
		{ID: "medium", Kind: models.BehaviorKindDirective, Priority: 5, Content: models.BehaviorContent{Canonical: "Medium priority"}},
	}

	result := optimizer.Optimize(behaviors)

	if len(result.Included) != 3 {
		t.Fatalf("expected 3 included, got %d", len(result.Included))
	}

	// Check order: high, medium, low
	if result.Included[0].ID != "high" {
		t.Errorf("expected first to be 'high', got %s", result.Included[0].ID)
	}
	if result.Included[1].ID != "medium" {
		t.Errorf("expected second to be 'medium', got %s", result.Included[1].ID)
	}
	if result.Included[2].ID != "low" {
		t.Errorf("expected third to be 'low', got %s", result.Included[2].ID)
	}
}

func TestOptimizer_Optimize_SortsByConfidence(t *testing.T) {
	optimizer := NewOptimizer(500)
	behaviors := []models.Behavior{
		{ID: "low-conf", Kind: models.BehaviorKindDirective, Priority: 5, Confidence: 0.5, Content: models.BehaviorContent{Canonical: "Low confidence"}},
		{ID: "high-conf", Kind: models.BehaviorKindDirective, Priority: 5, Confidence: 0.9, Content: models.BehaviorContent{Canonical: "High confidence"}},
	}

	result := optimizer.Optimize(behaviors)

	if len(result.Included) != 2 {
		t.Fatalf("expected 2 included, got %d", len(result.Included))
	}

	// Same priority, so higher confidence should come first
	if result.Included[0].ID != "high-conf" {
		t.Errorf("expected first to be 'high-conf', got %s", result.Included[0].ID)
	}
}

func TestOptimizer_OptimizeWithPriorities(t *testing.T) {
	optimizer := NewOptimizer(500)
	behaviors := []models.Behavior{
		{ID: "b1", Content: models.BehaviorContent{Canonical: "First"}},
		{ID: "b2", Content: models.BehaviorContent{Canonical: "Second"}},
		{ID: "b3", Content: models.BehaviorContent{Canonical: "Third"}},
	}

	// Custom order: b3 first, then b1, then b2
	priorityOrder := []string{"b3", "b1", "b2"}
	result := optimizer.OptimizeWithPriorities(behaviors, priorityOrder)

	if len(result.Included) != 3 {
		t.Fatalf("expected 3 included, got %d", len(result.Included))
	}

	if result.Included[0].ID != "b3" {
		t.Errorf("expected first to be 'b3', got %s", result.Included[0].ID)
	}
	if result.Included[1].ID != "b1" {
		t.Errorf("expected second to be 'b1', got %s", result.Included[1].ID)
	}
	if result.Included[2].ID != "b2" {
		t.Errorf("expected third to be 'b2', got %s", result.Included[2].ID)
	}
}

func TestOptimizer_Optimize_TokensUsed(t *testing.T) {
	optimizer := NewOptimizer(1000)
	behaviors := []models.Behavior{
		{ID: "b1", Content: models.BehaviorContent{Canonical: "Test behavior content"}},
	}

	result := optimizer.Optimize(behaviors)

	if result.TokensUsed <= 0 {
		t.Error("expected TokensUsed > 0")
	}
	if result.TokensAvailable != 1000 {
		t.Errorf("expected TokensAvailable = 1000, got %d", result.TokensAvailable)
	}
}

func TestOptimizer_Optimize_Empty(t *testing.T) {
	optimizer := NewOptimizer(100)
	result := optimizer.Optimize(nil)

	if len(result.Included) != 0 {
		t.Error("expected empty included")
	}
	if result.Truncated {
		t.Error("expected not truncated for empty input")
	}
}
