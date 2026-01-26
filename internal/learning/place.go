package learning

import (
	"context"
	"strings"

	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// PlacementDecision describes where a new behavior should go in the graph.
type PlacementDecision struct {
	// Action indicates what to do: "create", "merge", or "specialize"
	Action string

	// TargetID is set for merge/specialize actions to indicate the existing behavior
	TargetID string

	// ProposedEdges are the edges to add when placing the behavior
	ProposedEdges []ProposedEdge

	// SimilarBehaviors lists existing behaviors that are similar
	SimilarBehaviors []SimilarityMatch

	// Confidence indicates how confident the placer is in this decision (0.0-1.0)
	Confidence float64
}

// ProposedEdge represents a proposed edge to add to the graph.
type ProposedEdge struct {
	From string
	To   string
	Kind string // "requires", "overrides", "conflicts", "similar-to"
}

// SimilarityMatch represents a similar existing behavior.
type SimilarityMatch struct {
	ID    string
	Score float64
}

// GraphPlacer determines where a new behavior fits in the graph.
// It analyzes existing behaviors to find relationships and detect
// potential duplicates or merge opportunities.
type GraphPlacer interface {
	// Place determines where a behavior should be placed in the graph.
	// It returns a PlacementDecision indicating whether to create a new
	// behavior, merge with an existing one, or specialize an existing one.
	Place(ctx context.Context, behavior *models.Behavior) (*PlacementDecision, error)
}

// graphPlacer is the concrete implementation of GraphPlacer.
type graphPlacer struct {
	store store.GraphStore
}

// NewGraphPlacer creates a new GraphPlacer with the given store.
func NewGraphPlacer(s store.GraphStore) GraphPlacer {
	return &graphPlacer{store: s}
}

// Place determines where a behavior should be placed in the graph.
func (p *graphPlacer) Place(ctx context.Context, behavior *models.Behavior) (*PlacementDecision, error) {
	decision := &PlacementDecision{
		Action:           "create",
		ProposedEdges:    make([]ProposedEdge, 0),
		SimilarBehaviors: make([]SimilarityMatch, 0),
		Confidence:       0.7, // Default confidence for new behaviors
	}

	// Find existing behaviors with overlapping 'when' conditions
	existingBehaviors, err := p.findRelatedBehaviors(ctx, behavior)
	if err != nil {
		return nil, err
	}

	// If no existing behaviors, create with high confidence
	if len(existingBehaviors) == 0 {
		decision.Confidence = 0.9
		return decision, nil
	}

	// Track the most similar behavior for potential merge
	var mostSimilar *models.Behavior
	var highestSimilarity float64

	// Check for high similarity (potential duplicates or merges)
	for i := range existingBehaviors {
		existing := &existingBehaviors[i]
		similarity := p.computeSimilarity(behavior, existing)

		if similarity > 0.5 {
			decision.SimilarBehaviors = append(decision.SimilarBehaviors, SimilarityMatch{
				ID:    existing.ID,
				Score: similarity,
			})
		}

		if similarity > highestSimilarity {
			highestSimilarity = similarity
			mostSimilar = existing
		}
	}

	// Decide action based on similarity
	if highestSimilarity > 0.9 && mostSimilar != nil {
		// Very high similarity - suggest merge
		decision.Action = "merge"
		decision.TargetID = mostSimilar.ID
		decision.Confidence = 0.5 // Lower confidence for merges (needs review)
	} else if highestSimilarity > 0.7 && mostSimilar != nil {
		// High similarity but not duplicate - check if we should specialize
		if p.isMoreSpecific(behavior.When, mostSimilar.When) {
			decision.Action = "specialize"
			decision.TargetID = mostSimilar.ID
			decision.Confidence = 0.6
		}
	}

	// Determine edges based on relationships with existing behaviors
	decision.ProposedEdges = p.determineEdges(behavior, existingBehaviors)

	return decision, nil
}

// findRelatedBehaviors finds behaviors with overlapping activation conditions.
func (p *graphPlacer) findRelatedBehaviors(ctx context.Context, behavior *models.Behavior) ([]models.Behavior, error) {
	// Query for all behavior nodes
	nodes, err := p.store.QueryNodes(ctx, map[string]interface{}{
		"kind": "behavior",
	})
	if err != nil {
		return nil, err
	}

	related := make([]models.Behavior, 0)
	for _, node := range nodes {
		// Skip self if somehow present
		if node.ID == behavior.ID {
			continue
		}

		// Check for overlapping conditions
		if p.hasOverlappingConditions(behavior.When, node.Content) {
			b := p.nodeToBehavior(node)
			related = append(related, b)
		}
	}

	return related, nil
}

// computeSimilarity calculates similarity between two behaviors.
// Uses Jaccard word overlap on canonical content combined with when-condition overlap.
func (p *graphPlacer) computeSimilarity(a, b *models.Behavior) float64 {
	score := 0.0

	// Check 'when' overlap (40% weight)
	whenOverlap := p.computeWhenOverlap(a.When, b.When)
	score += whenOverlap * 0.4

	// Check content similarity using Jaccard word overlap (60% weight)
	contentSim := p.computeContentSimilarity(a.Content.Canonical, b.Content.Canonical)
	score += contentSim * 0.6

	return score
}

// computeWhenOverlap calculates overlap between two when predicates.
func (p *graphPlacer) computeWhenOverlap(a, b map[string]interface{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0 // Both empty = perfect overlap
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0 // One empty = no overlap
	}

	matches := 0
	total := len(a) + len(b)

	for key, valueA := range a {
		if valueB, exists := b[key]; exists {
			if valuesEqual(valueA, valueB) {
				matches += 2 // Count both sides as matched
			}
		}
	}

	if total == 0 {
		return 0.0
	}
	return float64(matches) / float64(total)
}

// computeContentSimilarity calculates Jaccard similarity between two strings.
func (p *graphPlacer) computeContentSimilarity(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0
	}
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0.0
	}

	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[strings.ToLower(w)] = true
	}

	setB := make(map[string]bool)
	for _, w := range wordsB {
		setB[strings.ToLower(w)] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits a string into word tokens.
func tokenize(s string) []string {
	words := make([]string, 0)
	current := ""
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			current += string(r)
		} else if current != "" {
			words = append(words, current)
			current = ""
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

// hasOverlappingConditions checks if a behavior's when conditions overlap with node content.
func (p *graphPlacer) hasOverlappingConditions(when map[string]interface{}, content map[string]interface{}) bool {
	existingWhen, ok := content["when"].(map[string]interface{})
	if !ok {
		// If the existing behavior has no when conditions, it applies everywhere
		// so there is overlap with any new behavior
		return len(when) == 0
	}

	// Check if any conditions match
	for key, value := range when {
		if existingValue, exists := existingWhen[key]; exists {
			if valuesEqual(value, existingValue) {
				return true
			}
		}
	}

	return false
}

// valuesEqual compares two interface{} values for equality.
func valuesEqual(a, b interface{}) bool {
	// Handle string comparison
	aStr, aIsStr := a.(string)
	bStr, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return aStr == bStr
	}

	// Handle slice comparison (both must contain at least one common element)
	aSlice, aIsSlice := a.([]interface{})
	bSlice, bIsSlice := b.([]interface{})
	if aIsSlice && bIsSlice {
		for _, av := range aSlice {
			for _, bv := range bSlice {
				if valuesEqual(av, bv) {
					return true
				}
			}
		}
		return false
	}

	// Handle string slice comparison
	aStrSlice, aIsStrSlice := a.([]string)
	bStrSlice, bIsStrSlice := b.([]string)
	if aIsStrSlice && bIsStrSlice {
		for _, av := range aStrSlice {
			for _, bv := range bStrSlice {
				if av == bv {
					return true
				}
			}
		}
		return false
	}

	// Fallback to direct equality
	return a == b
}

// determineEdges proposes edges for the new behavior based on relationships with existing behaviors.
func (p *graphPlacer) determineEdges(behavior *models.Behavior, existing []models.Behavior) []ProposedEdge {
	edges := make([]ProposedEdge, 0)

	for _, e := range existing {
		// If new behavior has more specific 'when' conditions, it overrides the existing one
		if p.isMoreSpecific(behavior.When, e.When) {
			edges = append(edges, ProposedEdge{
				From: behavior.ID,
				To:   e.ID,
				Kind: "overrides",
			})
		}

		// If existing behavior has more specific 'when' conditions,
		// the new behavior might be overridden by it (inverse relationship)
		if p.isMoreSpecific(e.When, behavior.When) {
			edges = append(edges, ProposedEdge{
				From: e.ID,
				To:   behavior.ID,
				Kind: "overrides",
			})
		}

		// Add similar-to edges for behaviors with moderate similarity
		similarity := p.computeSimilarity(behavior, &e)
		if similarity >= 0.5 && similarity < 0.9 {
			edges = append(edges, ProposedEdge{
				From: behavior.ID,
				To:   e.ID,
				Kind: "similar-to",
			})
		}
	}

	return edges
}

// isMoreSpecific returns true if a has all of b's conditions plus additional ones.
func (p *graphPlacer) isMoreSpecific(a, b map[string]interface{}) bool {
	// a is more specific than b if:
	// 1. a has more conditions than b
	// 2. a includes all of b's conditions with the same values
	if len(a) <= len(b) {
		return false
	}

	for key, valueB := range b {
		valueA, exists := a[key]
		if !exists {
			return false
		}
		if !valuesEqual(valueA, valueB) {
			return false
		}
	}

	return true
}

// nodeToBehavior converts a store.Node to a models.Behavior.
func (p *graphPlacer) nodeToBehavior(node store.Node) models.Behavior {
	b := models.Behavior{
		ID: node.ID,
	}

	// Extract kind
	if kind, ok := node.Content["kind"].(string); ok {
		b.Kind = models.BehaviorKind(kind)
	}

	// Extract name
	if name, ok := node.Content["name"].(string); ok {
		b.Name = name
	}

	// Extract when conditions
	if when, ok := node.Content["when"].(map[string]interface{}); ok {
		b.When = when
	}

	// Extract content
	if content, ok := node.Content["content"].(map[string]interface{}); ok {
		if canonical, ok := content["canonical"].(string); ok {
			b.Content.Canonical = canonical
		}
		if expanded, ok := content["expanded"].(string); ok {
			b.Content.Expanded = expanded
		}
		if structured, ok := content["structured"].(map[string]interface{}); ok {
			b.Content.Structured = structured
		}
	} else if content, ok := node.Content["content"].(models.BehaviorContent); ok {
		b.Content = content
	}

	// Extract confidence from metadata
	if confidence, ok := node.Metadata["confidence"].(float64); ok {
		b.Confidence = confidence
	}

	// Extract priority from metadata
	if priority, ok := node.Metadata["priority"].(int); ok {
		b.Priority = priority
	}

	return b
}
