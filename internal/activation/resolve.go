package activation

import (
	"github.com/nvandessel/feedback-loop/internal/models"
)

// Resolver handles conflicts between active behaviors
type Resolver struct{}

// NewResolver creates a new conflict resolver
func NewResolver() *Resolver {
	return &Resolver{}
}

// ResolveResult contains the final active behaviors after conflict resolution
type ResolveResult struct {
	// Active behaviors after resolution
	Active []models.Behavior

	// Overridden behaviors (superseded by another behavior)
	Overridden []OverrideInfo

	// Conflicting behaviors that were excluded
	Excluded []ConflictInfo
}

// OverrideInfo describes why a behavior was overridden
type OverrideInfo struct {
	Behavior   models.Behavior `json:"behavior"`
	OverrideBy string          `json:"override_by"`
	Reason     string          `json:"reason"`
}

// ConflictInfo describes a conflict between behaviors
type ConflictInfo struct {
	Behavior      models.Behavior `json:"behavior"`
	ConflictsWith string          `json:"conflicts_with"`
	Winner        string          `json:"winner"`
	Reason        string          `json:"reason"`
}

// Resolve takes a list of matching behaviors and resolves conflicts
func (r *Resolver) Resolve(matches []ActivationResult) ResolveResult {
	result := ResolveResult{
		Active:     make([]models.Behavior, 0),
		Overridden: make([]OverrideInfo, 0),
		Excluded:   make([]ConflictInfo, 0),
	}

	if len(matches) == 0 {
		return result
	}

	// Build lookup maps
	behaviorByID := make(map[string]models.Behavior)
	resultByID := make(map[string]ActivationResult)
	for _, m := range matches {
		behaviorByID[m.Behavior.ID] = m.Behavior
		resultByID[m.Behavior.ID] = m
	}

	// Track which behaviors are excluded
	excluded := make(map[string]bool)
	overridden := make(map[string]string) // ID -> overriding ID

	// Process overrides
	for _, m := range matches {
		for _, overriddenID := range m.Behavior.Overrides {
			if _, exists := behaviorByID[overriddenID]; exists {
				overridden[overriddenID] = m.Behavior.ID
			}
		}
	}

	// Process conflicts - higher priority/specificity wins
	for i, m1 := range matches {
		if excluded[m1.Behavior.ID] {
			continue
		}

		for _, conflictID := range m1.Behavior.Conflicts {
			// Check if conflicting behavior is also in matches
			if m2, exists := resultByID[conflictID]; exists {
				if excluded[conflictID] {
					continue
				}

				// Determine winner based on specificity, then priority, then confidence
				winner := r.pickWinner(m1, m2)
				loser := m1.Behavior.ID
				if winner == m1.Behavior.ID {
					loser = conflictID
				}

				excluded[loser] = true
				result.Excluded = append(result.Excluded, ConflictInfo{
					Behavior:      behaviorByID[loser],
					ConflictsWith: winner,
					Winner:        winner,
					Reason:        "Lost conflict resolution",
				})
			}
		}

		// Also check if any later behavior conflicts with this one
		for j := i + 1; j < len(matches); j++ {
			m2 := matches[j]
			if excluded[m2.Behavior.ID] {
				continue
			}

			for _, conflictID := range m2.Behavior.Conflicts {
				if conflictID == m1.Behavior.ID && !excluded[m1.Behavior.ID] {
					winner := r.pickWinner(m1, m2)
					loser := m1.Behavior.ID
					if winner == m1.Behavior.ID {
						loser = m2.Behavior.ID
					}

					excluded[loser] = true
					result.Excluded = append(result.Excluded, ConflictInfo{
						Behavior:      behaviorByID[loser],
						ConflictsWith: winner,
						Winner:        winner,
						Reason:        "Lost conflict resolution",
					})
				}
			}
		}
	}

	// Build final active list
	for _, m := range matches {
		id := m.Behavior.ID

		if excluded[id] {
			continue
		}

		if overrideBy, wasOverridden := overridden[id]; wasOverridden {
			result.Overridden = append(result.Overridden, OverrideInfo{
				Behavior:   m.Behavior,
				OverrideBy: overrideBy,
				Reason:     "Superseded by more specific behavior",
			})
			continue
		}

		result.Active = append(result.Active, m.Behavior)
	}

	return result
}

// pickWinner determines which behavior wins a conflict
func (r *Resolver) pickWinner(a, b ActivationResult) string {
	// Higher specificity wins
	if a.Specificity > b.Specificity {
		return a.Behavior.ID
	}
	if b.Specificity > a.Specificity {
		return b.Behavior.ID
	}

	// Higher priority wins
	if a.Behavior.Priority > b.Behavior.Priority {
		return a.Behavior.ID
	}
	if b.Behavior.Priority > a.Behavior.Priority {
		return b.Behavior.ID
	}

	// Higher confidence wins
	if a.Behavior.Confidence > b.Behavior.Confidence {
		return a.Behavior.ID
	}
	if b.Behavior.Confidence > a.Behavior.Confidence {
		return b.Behavior.ID
	}

	// Tie-breaker: first one wins (stable sort)
	return a.Behavior.ID
}

// CheckDependencies verifies that all required behaviors are present
func (r *Resolver) CheckDependencies(active []models.Behavior, all []models.Behavior) []DependencyError {
	var errors []DependencyError

	// Build lookup of all behavior IDs
	allIDs := make(map[string]bool)
	for _, b := range all {
		allIDs[b.ID] = true
	}

	// Build lookup of active behavior IDs
	activeIDs := make(map[string]bool)
	for _, b := range active {
		activeIDs[b.ID] = true
	}

	// Check each active behavior's requirements
	for _, b := range active {
		for _, requiredID := range b.Requires {
			if !activeIDs[requiredID] {
				errors = append(errors, DependencyError{
					BehaviorID: b.ID,
					RequiredID: requiredID,
					Exists:     allIDs[requiredID],
				})
			}
		}
	}

	return errors
}

// DependencyError indicates a missing required behavior
type DependencyError struct {
	BehaviorID string `json:"behavior_id"`
	RequiredID string `json:"required_id"`
	Exists     bool   `json:"exists"` // true if behavior exists but isn't active
}
