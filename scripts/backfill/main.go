// scripts/backfill/main.go
//
// One-time backfill script for graph connectivity fix.
// - Strips "environment" from all behavior when-maps
// - Backfills tags on tagless behaviors using dictionary extraction
// - Clears all stale edges
// - Re-derives edges using tag-enhanced WeightedScoreWithTags
// - Validates and syncs to JSONL
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nvandessel/feedback-loop/internal/constants"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/similarity"
	"github.com/nvandessel/feedback-loop/internal/store"
	"github.com/nvandessel/feedback-loop/internal/tagging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "backfill failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Determine store paths
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	stores := []struct {
		name string
		root string
	}{
		{"local", projectRoot},
		{"global", homeDir},
	}

	for _, st := range stores {
		fmt.Printf("\n=== Processing %s store (%s) ===\n", st.name, filepath.Join(st.root, ".floop"))
		if err := processStore(ctx, st.name, st.root); err != nil {
			return fmt.Errorf("process %s store: %w", st.name, err)
		}
	}

	fmt.Println("\nBackfill complete!")
	return nil
}

func processStore(ctx context.Context, name, root string) error {
	// Delete existing DB so we get a clean import from JSONL
	dbPath := filepath.Join(root, ".floop", "floop.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old DB: %w", err)
	}
	fmt.Printf("  Removed old DB (will reimport from JSONL)\n")

	s, err := store.NewSQLiteGraphStore(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	// Step 1: Load all behaviors
	nodes, err := s.QueryNodes(ctx, map[string]interface{}{"kind": "behavior"})
	if err != nil {
		return fmt.Errorf("query behaviors: %w", err)
	}
	fmt.Printf("  Found %d behaviors\n", len(nodes))

	// Step 2: Strip "environment" from when-maps
	envStripped := 0
	for _, node := range nodes {
		when, ok := node.Content["when"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasEnv := when["environment"]; hasEnv {
			delete(when, "environment")
			node.Content["when"] = when
			if err := s.UpdateNode(ctx, node); err != nil {
				return fmt.Errorf("update node %s: %w", node.ID, err)
			}
			envStripped++
		}
	}
	fmt.Printf("  Stripped 'environment' from %d behaviors\n", envStripped)

	// Step 3: Backfill tags on tagless behaviors
	// Uses the same ExtractTags(canonical, dictionary) as the learn pipeline
	dict := tagging.NewDictionary()
	tagsBackfilled := 0
	for _, node := range nodes {
		contentMap, ok := node.Content["content"].(map[string]interface{})
		if !ok {
			continue
		}

		// Skip behaviors that already have tags
		if existingTags, ok := contentMap["tags"].([]interface{}); ok && len(existingTags) > 0 {
			continue
		}

		canonical, _ := contentMap["canonical"].(string)
		if canonical == "" {
			continue
		}

		tags := tagging.ExtractTags(canonical, dict)
		if len(tags) == 0 {
			continue
		}

		// Convert []string to []interface{} for the content map
		tagIfaces := make([]interface{}, len(tags))
		for i, t := range tags {
			tagIfaces[i] = t
		}
		contentMap["tags"] = tagIfaces
		node.Content["content"] = contentMap
		if err := s.UpdateNode(ctx, node); err != nil {
			return fmt.Errorf("update node tags %s: %w", node.ID, err)
		}
		tagsBackfilled++
	}
	fmt.Printf("  Backfilled tags on %d behaviors\n", tagsBackfilled)

	// Step 4: Clear all stale edges
	edgesRemoved := 0
	for _, node := range nodes {
		edges, err := s.GetEdges(ctx, node.ID, store.DirectionBoth, "")
		if err != nil {
			return fmt.Errorf("get edges for %s: %w", node.ID, err)
		}
		for _, edge := range edges {
			if err := s.RemoveEdge(ctx, edge.Source, edge.Target, edge.Kind); err != nil {
				// Ignore errors from already-removed edges (dedup from bidirectional query)
				continue
			}
			edgesRemoved++
		}
	}
	fmt.Printf("  Removed %d edges\n", edgesRemoved)

	// Step 5: Re-derive edges using new scoring
	// Reload nodes after updates
	nodes, err = s.QueryNodes(ctx, map[string]interface{}{"kind": "behavior"})
	if err != nil {
		return fmt.Errorf("reload behaviors: %w", err)
	}

	behaviors := make([]models.Behavior, len(nodes))
	for i, node := range nodes {
		behaviors[i] = models.NodeToBehavior(node)
	}

	edgesAdded := 0
	overridesCount := 0
	similarToCount := 0
	now := time.Now()

	for i := 0; i < len(behaviors); i++ {
		for j := i + 1; j < len(behaviors); j++ {
			a := &behaviors[i]
			b := &behaviors[j]

			// Compute similarity using tag-enhanced scoring
			whenOverlap := similarity.ComputeWhenOverlap(a.When, b.When)
			contentSim := similarity.ComputeContentSimilarity(a.Content.Canonical, b.Content.Canonical)
			tagSim := similarity.ComputeTagSimilarity(a.Content.Tags, b.Content.Tags)
			score := similarity.WeightedScoreWithTags(whenOverlap, contentSim, tagSim)

			// Check for overrides (more specific when-conditions)
			if isMoreSpecific(a.When, b.When) {
				edge := store.Edge{
					Source:    a.ID,
					Target:    b.ID,
					Kind:      "overrides",
					Weight:    0.8,
					CreatedAt: now,
				}
				if err := s.AddEdge(ctx, edge); err != nil {
					return fmt.Errorf("add override edge %s→%s: %w", a.ID, b.ID, err)
				}
				edgesAdded++
				overridesCount++
			}
			if isMoreSpecific(b.When, a.When) {
				edge := store.Edge{
					Source:    b.ID,
					Target:    a.ID,
					Kind:      "overrides",
					Weight:    0.8,
					CreatedAt: now,
				}
				if err := s.AddEdge(ctx, edge); err != nil {
					return fmt.Errorf("add override edge %s→%s: %w", b.ID, a.ID, err)
				}
				edgesAdded++
				overridesCount++
			}

			// Check for similar-to edges
			if score >= constants.SimilarToThreshold && score < constants.SimilarToUpperBound {
				edge := store.Edge{
					Source:    a.ID,
					Target:    b.ID,
					Kind:      "similar-to",
					Weight:    0.8,
					CreatedAt: now,
				}
				if err := s.AddEdge(ctx, edge); err != nil {
					return fmt.Errorf("add similar-to edge %s→%s: %w", a.ID, b.ID, err)
				}
				edgesAdded++
				similarToCount++
			}
		}
	}
	fmt.Printf("  Added %d edges (%d overrides, %d similar-to)\n", edgesAdded, overridesCount, similarToCount)

	// Step 6: Validate
	errors, err := s.ValidateBehaviorGraph(ctx)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if len(errors) > 0 {
		fmt.Printf("  WARNING: %d validation errors:\n", len(errors))
		for _, e := range errors {
			fmt.Printf("    %s\n", e.String())
		}
	} else {
		fmt.Printf("  Validation: 0 errors\n")
	}

	// Step 7: Sync to JSONL
	if err := s.Sync(ctx); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	fmt.Printf("  Synced to JSONL\n")

	return nil
}

// isMoreSpecific returns true if a has all of b's conditions plus additional ones.
// Matches the logic in learning/place.go.
func isMoreSpecific(a, b map[string]interface{}) bool {
	if len(a) <= len(b) {
		return false
	}
	if len(b) == 0 {
		return false
	}
	for key, valueB := range b {
		valueA, exists := a[key]
		if !exists {
			return false
		}
		if !similarity.ValuesEqual(valueA, valueB) {
			return false
		}
	}
	return true
}
