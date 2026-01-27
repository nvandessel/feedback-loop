package activation

import (
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func TestResolver_Resolve(t *testing.T) {
	resolver := NewResolver()

	tests := []struct {
		name           string
		matches        []ActivationResult
		wantActive     int
		wantOverridden int
		wantExcluded   int
	}{
		{
			name:           "empty matches",
			matches:        []ActivationResult{},
			wantActive:     0,
			wantOverridden: 0,
			wantExcluded:   0,
		},
		{
			name: "single behavior",
			matches: []ActivationResult{
				{Behavior: models.Behavior{ID: "b1"}, Specificity: 1},
			},
			wantActive:     1,
			wantOverridden: 0,
			wantExcluded:   0,
		},
		{
			name: "no conflicts",
			matches: []ActivationResult{
				{Behavior: models.Behavior{ID: "b1"}, Specificity: 1},
				{Behavior: models.Behavior{ID: "b2"}, Specificity: 2},
			},
			wantActive:     2,
			wantOverridden: 0,
			wantExcluded:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.Resolve(tt.matches)

			if len(result.Active) != tt.wantActive {
				t.Errorf("Active = %d, want %d", len(result.Active), tt.wantActive)
			}
			if len(result.Overridden) != tt.wantOverridden {
				t.Errorf("Overridden = %d, want %d", len(result.Overridden), tt.wantOverridden)
			}
			if len(result.Excluded) != tt.wantExcluded {
				t.Errorf("Excluded = %d, want %d", len(result.Excluded), tt.wantExcluded)
			}
		})
	}
}

func TestResolver_Overrides(t *testing.T) {
	resolver := NewResolver()

	// b2 overrides b1
	matches := []ActivationResult{
		{
			Behavior: models.Behavior{
				ID:   "b1",
				Name: "general-rule",
			},
			Specificity: 1,
		},
		{
			Behavior: models.Behavior{
				ID:        "b2",
				Name:      "specific-rule",
				Overrides: []string{"b1"},
			},
			Specificity: 2,
		},
	}

	result := resolver.Resolve(matches)

	if len(result.Active) != 1 {
		t.Fatalf("Expected 1 active, got %d", len(result.Active))
	}
	if result.Active[0].ID != "b2" {
		t.Errorf("Expected b2 to be active, got %s", result.Active[0].ID)
	}
	if len(result.Overridden) != 1 {
		t.Fatalf("Expected 1 overridden, got %d", len(result.Overridden))
	}
	if result.Overridden[0].Behavior.ID != "b1" {
		t.Errorf("Expected b1 to be overridden, got %s", result.Overridden[0].Behavior.ID)
	}
}

func TestResolver_Conflicts(t *testing.T) {
	resolver := NewResolver()

	// b1 and b2 conflict, b2 has higher specificity so wins
	matches := []ActivationResult{
		{
			Behavior: models.Behavior{
				ID:        "b1",
				Name:      "rule-a",
				Conflicts: []string{"b2"},
			},
			Specificity: 1,
		},
		{
			Behavior: models.Behavior{
				ID:        "b2",
				Name:      "rule-b",
				Conflicts: []string{"b1"},
			},
			Specificity: 2,
		},
	}

	result := resolver.Resolve(matches)

	if len(result.Active) != 1 {
		t.Fatalf("Expected 1 active, got %d", len(result.Active))
	}
	if result.Active[0].ID != "b2" {
		t.Errorf("Expected b2 to win conflict, got %s", result.Active[0].ID)
	}
	if len(result.Excluded) != 1 {
		t.Fatalf("Expected 1 excluded, got %d", len(result.Excluded))
	}
	if result.Excluded[0].Behavior.ID != "b1" {
		t.Errorf("Expected b1 to be excluded, got %s", result.Excluded[0].Behavior.ID)
	}
}

func TestResolver_ConflictPriorityWins(t *testing.T) {
	resolver := NewResolver()

	// Same specificity, b1 has higher priority
	matches := []ActivationResult{
		{
			Behavior: models.Behavior{
				ID:        "b1",
				Name:      "high-priority",
				Priority:  10,
				Conflicts: []string{"b2"},
			},
			Specificity: 1,
		},
		{
			Behavior: models.Behavior{
				ID:        "b2",
				Name:      "low-priority",
				Priority:  1,
				Conflicts: []string{"b1"},
			},
			Specificity: 1,
		},
	}

	result := resolver.Resolve(matches)

	if len(result.Active) != 1 {
		t.Fatalf("Expected 1 active, got %d", len(result.Active))
	}
	if result.Active[0].ID != "b1" {
		t.Errorf("Expected b1 (higher priority) to win, got %s", result.Active[0].ID)
	}
}

func TestResolver_ConflictConfidenceWins(t *testing.T) {
	resolver := NewResolver()

	// Same specificity and priority, b1 has higher confidence
	matches := []ActivationResult{
		{
			Behavior: models.Behavior{
				ID:         "b1",
				Name:       "high-confidence",
				Priority:   5,
				Confidence: 0.9,
				Conflicts:  []string{"b2"},
			},
			Specificity: 1,
		},
		{
			Behavior: models.Behavior{
				ID:         "b2",
				Name:       "low-confidence",
				Priority:   5,
				Confidence: 0.5,
				Conflicts:  []string{"b1"},
			},
			Specificity: 1,
		},
	}

	result := resolver.Resolve(matches)

	if len(result.Active) != 1 {
		t.Fatalf("Expected 1 active, got %d", len(result.Active))
	}
	if result.Active[0].ID != "b1" {
		t.Errorf("Expected b1 (higher confidence) to win, got %s", result.Active[0].ID)
	}
}

func TestResolver_CheckDependencies(t *testing.T) {
	resolver := NewResolver()

	allBehaviors := []models.Behavior{
		{ID: "b1", Name: "base"},
		{ID: "b2", Name: "dependent", Requires: []string{"b1"}},
		{ID: "b3", Name: "missing-dep", Requires: []string{"b99"}},
	}

	tests := []struct {
		name       string
		active     []models.Behavior
		wantErrors int
	}{
		{
			name:       "no dependencies",
			active:     []models.Behavior{{ID: "b1"}},
			wantErrors: 0,
		},
		{
			name:       "dependency satisfied",
			active:     []models.Behavior{{ID: "b1"}, {ID: "b2", Requires: []string{"b1"}}},
			wantErrors: 0,
		},
		{
			name:       "dependency not active",
			active:     []models.Behavior{{ID: "b2", Requires: []string{"b1"}}},
			wantErrors: 1,
		},
		{
			name:       "dependency does not exist",
			active:     []models.Behavior{{ID: "b3", Requires: []string{"b99"}}},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := resolver.CheckDependencies(tt.active, allBehaviors)
			if len(errors) != tt.wantErrors {
				t.Errorf("CheckDependencies() returned %d errors, want %d", len(errors), tt.wantErrors)
			}
		})
	}
}
