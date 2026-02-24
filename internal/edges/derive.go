package edges

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nvandessel/feedback-loop/internal/constants"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/ranking"
	"github.com/nvandessel/feedback-loop/internal/similarity"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// MinSharedTagsForEdge is the minimum number of shared tags between two
// behaviors to create a similar-to edge, regardless of overall similarity
// score. Tag co-occurrence is a strong signal for conceptual relatedness --
// if two behaviors both have "git" and "worktree" tags, spreading activation
// needs that edge to associate related concepts.
const MinSharedTagsForEdge = 2

// DeriveResult holds the output for one store's edge derivation.
type DeriveResult struct {
	Scope           string           `json:"scope"`
	Behaviors       int              `json:"behaviors"`
	ExistingEdges   int              `json:"existing_edges"`
	ClearedEdges    int              `json:"cleared_edges"`
	ProposedEdges   []ProposedEdge   `json:"proposed_edges"`
	CreatedEdges    int              `json:"created_edges"`
	SkippedExisting int              `json:"skipped_existing"`
	Histogram       [10]int          `json:"score_histogram"`
	Connectivity    ConnectivityInfo `json:"connectivity"`
}

// ProposedEdge represents a single proposed edge.
type ProposedEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Kind   string  `json:"kind"`
	Weight float64 `json:"weight"`
	Score  float64 `json:"score"`
}

// ConnectivityInfo describes graph connectivity after edge derivation.
type ConnectivityInfo struct {
	TotalNodes int `json:"total_nodes"`
	Islands    int `json:"islands"`
	Connected  int `json:"connected"`
}

// SubsetResult reports edge derivation for a subset of behaviors.
type SubsetResult struct {
	NewBehaviors   int
	TotalBehaviors int
	PairsCompared  int
	EdgesCreated   int
	EdgesSkipped   int
	ProposedEdges  []ProposedEdge
}

// DeriveEdgesForStore runs the all-pairs edge derivation algorithm on a single store.
// Extracted from cmd/floop/cmd_derive_edges.go:deriveEdgesForStore.
func DeriveEdgesForStore(ctx context.Context, graphStore store.GraphStore, scope string, dryRun, clear bool) (DeriveResult, error) {
	result := DeriveResult{Scope: scope}

	// Load all non-forgotten behaviors
	behaviors, err := LoadBehaviorsFromStore(ctx, graphStore)
	if err != nil {
		return result, fmt.Errorf("failed to load behaviors: %w", err)
	}
	result.Behaviors = len(behaviors)

	if len(behaviors) == 0 {
		return result, nil
	}

	// Clear existing derived edges if requested
	if clear && !dryRun {
		result.ClearedEdges = ClearDerivedEdges(ctx, graphStore, behaviors)
	}

	// Build existing edge set for dedup.
	// For bidirectional edge kinds (similar-to), register both directions
	// so the check works regardless of behavior iteration order.
	existingEdges := make(map[string]bool)
	for _, b := range behaviors {
		edges, err := graphStore.GetEdges(ctx, b.ID, store.DirectionOutbound, "")
		if err != nil {
			continue
		}
		for _, e := range edges {
			key := e.Source + ":" + e.Target + ":" + e.Kind
			existingEdges[key] = true
			if e.Kind == "similar-to" {
				reverseKey := e.Target + ":" + e.Source + ":" + e.Kind
				existingEdges[reverseKey] = true
			}
		}
		result.ExistingEdges += len(edges)
	}

	// All-pairs comparison
	now := time.Now()
	for i := 0; i < len(behaviors); i++ {
		for j := i + 1; j < len(behaviors); j++ {
			a := &behaviors[i]
			b := &behaviors[j]

			// Compute similarity (no LLM)
			score := ComputeBehaviorSimilarity(a, b, nil, false, nil)

			// Record in histogram (10 buckets: [0.0,0.1), [0.1,0.2), ..., [0.9,1.0])
			bucket := int(score * 10)
			if bucket >= 10 {
				bucket = 9
			}
			result.Histogram[bucket]++

			// Check for overrides edges (specificity)
			if similarity.IsMoreSpecific(a.When, b.When) {
				pe := ProposedEdge{Source: a.ID, Target: b.ID, Kind: "overrides", Weight: 1.0, Score: score}
				key := a.ID + ":" + b.ID + ":overrides"
				if existingEdges[key] {
					result.SkippedExisting++
				} else {
					result.ProposedEdges = append(result.ProposedEdges, pe)
					existingEdges[key] = true
				}
			}
			if similarity.IsMoreSpecific(b.When, a.When) {
				pe := ProposedEdge{Source: b.ID, Target: a.ID, Kind: "overrides", Weight: 1.0, Score: score}
				key := b.ID + ":" + a.ID + ":overrides"
				if existingEdges[key] {
					result.SkippedExisting++
				} else {
					result.ProposedEdges = append(result.ProposedEdges, pe)
					existingEdges[key] = true
				}
			}

			// Check for similar-to edges:
			// 1. Score-based: similarity in [0.5, 0.9)
			// 2. Tag-based: behaviors sharing >= 2 tags are conceptually related
			//    and need edges for spreading activation (git -> branch, worktree, etc.)
			similarToKey := a.ID + ":" + b.ID + ":similar-to"
			shouldConnect := (score >= constants.SimilarToThreshold && score < constants.SimilarToUpperBound) ||
				similarity.CountSharedTags(a.Content.Tags, b.Content.Tags) >= MinSharedTagsForEdge
			if shouldConnect {
				pe := ProposedEdge{Source: a.ID, Target: b.ID, Kind: "similar-to", Weight: 0.8, Score: score}
				if existingEdges[similarToKey] {
					result.SkippedExisting++
				} else {
					result.ProposedEdges = append(result.ProposedEdges, pe)
					existingEdges[similarToKey] = true
				}
			}
		}
	}

	// Create proposed edges (unless dry-run)
	if !dryRun && len(result.ProposedEdges) > 0 {
		for _, pe := range result.ProposedEdges {
			edge := store.Edge{
				Source:    pe.Source,
				Target:    pe.Target,
				Kind:      pe.Kind,
				Weight:    pe.Weight,
				CreatedAt: now,
			}
			if err := graphStore.AddEdge(ctx, edge); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to add edge %s -> %s: %v\n", pe.Source, pe.Target, err)
				continue
			}
			result.CreatedEdges++
		}

		if err := graphStore.Sync(ctx); err != nil {
			return result, fmt.Errorf("failed to sync store: %w", err)
		}

		// Refresh PageRank
		if _, err := ranking.ComputePageRank(ctx, graphStore, ranking.DefaultPageRankConfig()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to refresh PageRank: %v\n", err)
		}
	}

	// Compute connectivity
	result.Connectivity = ComputeConnectivity(ctx, graphStore, behaviors)

	return result, nil
}

// DeriveEdgesForSubset derives edges for a subset of behaviors against all behaviors.
// Only computes pairs where at least one behavior is in newIDs.
// This is O(new * all) not O(all * all).
func DeriveEdgesForSubset(ctx context.Context, graphStore store.GraphStore, newIDs []string, allBehaviors []models.Behavior) (*SubsetResult, error) {
	// Performance guard
	newSet := make(map[string]bool, len(newIDs))
	for _, id := range newIDs {
		newSet[id] = true
	}

	existingCount := len(allBehaviors) - len(newIDs)
	if existingCount < 0 {
		existingCount = 0
	}
	if len(newIDs)*existingCount > 10000 {
		fmt.Fprintf(os.Stderr, "warning: large comparison set (%d new x %d existing = %d pairs)\n",
			len(newIDs), existingCount, len(newIDs)*existingCount)
	}

	// Build existing edge set
	existingEdges := make(map[string]bool)
	for _, b := range allBehaviors {
		edges, err := graphStore.GetEdges(ctx, b.ID, store.DirectionOutbound, "")
		if err != nil {
			continue
		}
		for _, e := range edges {
			key := e.Source + ":" + e.Target + ":" + e.Kind
			existingEdges[key] = true
			if e.Kind == "similar-to" {
				existingEdges[e.Target+":"+e.Source+":similar-to"] = true
			}
		}
	}

	// Pairwise comparison -- only pairs where at least one is new
	var proposed []ProposedEdge
	skipped := 0
	now := time.Now()

	for i := 0; i < len(allBehaviors); i++ {
		for j := i + 1; j < len(allBehaviors); j++ {
			a := &allBehaviors[i]
			b := &allBehaviors[j]

			// Skip if NEITHER is new (existing<->existing)
			if !newSet[a.ID] && !newSet[b.ID] {
				continue
			}

			score := ComputeBehaviorSimilarity(a, b, nil, false, nil)

			// Similar-to edges
			if (score >= constants.SimilarToThreshold && score < constants.SimilarToUpperBound) ||
				similarity.CountSharedTags(a.Content.Tags, b.Content.Tags) >= MinSharedTagsForEdge {
				key := a.ID + ":" + b.ID + ":similar-to"
				if !existingEdges[key] {
					proposed = append(proposed, ProposedEdge{Source: a.ID, Target: b.ID, Kind: "similar-to", Weight: 0.8, Score: score})
					existingEdges[key] = true
				} else {
					skipped++
				}
			}

			// Overrides edges
			if similarity.IsMoreSpecific(a.When, b.When) {
				key := a.ID + ":" + b.ID + ":overrides"
				if !existingEdges[key] {
					proposed = append(proposed, ProposedEdge{Source: a.ID, Target: b.ID, Kind: "overrides", Weight: 1.0, Score: score})
					existingEdges[key] = true
				} else {
					skipped++
				}
			}
			if similarity.IsMoreSpecific(b.When, a.When) {
				key := b.ID + ":" + a.ID + ":overrides"
				if !existingEdges[key] {
					proposed = append(proposed, ProposedEdge{Source: b.ID, Target: a.ID, Kind: "overrides", Weight: 1.0, Score: score})
					existingEdges[key] = true
				} else {
					skipped++
				}
			}
		}
	}

	// Create edges
	created := 0
	for _, pe := range proposed {
		edge := store.Edge{Source: pe.Source, Target: pe.Target, Kind: pe.Kind, Weight: pe.Weight, CreatedAt: now}
		if err := graphStore.AddEdge(ctx, edge); err != nil {
			continue
		}
		created++
	}

	if created > 0 {
		if err := graphStore.Sync(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to sync after subset derivation: %v\n", err)
		}
	}

	// Compute pairs compared: new*existing + new*(new-1)/2
	pairsCompared := len(newIDs)*existingCount + len(newIDs)*(len(newIDs)-1)/2

	return &SubsetResult{
		NewBehaviors:   len(newIDs),
		TotalBehaviors: len(allBehaviors),
		PairsCompared:  pairsCompared,
		EdgesCreated:   created,
		EdgesSkipped:   skipped,
		ProposedEdges:  proposed,
	}, nil
}

// ClearDerivedEdges removes all similar-to and overrides outbound edges for behaviors.
// Returns the number of edges removed. Logs warnings on individual failures but
// continues clearing remaining edges.
func ClearDerivedEdges(ctx context.Context, graphStore store.GraphStore, behaviors []models.Behavior) int {
	cleared := 0
	for _, b := range behaviors {
		for _, kind := range []string{"similar-to", "overrides"} {
			edges, err := graphStore.GetEdges(ctx, b.ID, store.DirectionOutbound, kind)
			if err != nil {
				continue
			}
			for _, e := range edges {
				if err := graphStore.RemoveEdge(ctx, e.Source, e.Target, e.Kind); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to remove edge %s -> %s (%s): %v\n", e.Source, e.Target, e.Kind, err)
					continue
				}
				cleared++
			}
		}
	}
	return cleared
}

// ComputeConnectivity counts how many behaviors have edges vs. are isolated islands.
func ComputeConnectivity(ctx context.Context, graphStore store.GraphStore, behaviors []models.Behavior) ConnectivityInfo {
	info := ConnectivityInfo{TotalNodes: len(behaviors)}

	for _, b := range behaviors {
		hasEdge := false
		// Check outbound edges
		outEdges, err := graphStore.GetEdges(ctx, b.ID, store.DirectionOutbound, "")
		if err == nil && len(outEdges) > 0 {
			hasEdge = true
		}
		// Check inbound edges
		if !hasEdge {
			inEdges, err := graphStore.GetEdges(ctx, b.ID, store.DirectionInbound, "")
			if err == nil && len(inEdges) > 0 {
				hasEdge = true
			}
		}
		if hasEdge {
			info.Connected++
		} else {
			info.Islands++
		}
	}

	return info
}
