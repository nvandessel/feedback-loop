package assembly

import (
	"sort"
	"strings"

	"github.com/nvandessel/feedback-loop/internal/models"
)

// CoalesceConfig controls behavior grouping.
type CoalesceConfig struct {
	// MinClusterSize is the minimum number of behaviors to trigger coalescing. Default: 3.
	// If fewer than this many related behaviors activate, show individually.
	MinClusterSize int

	// MaxIndividualPerCluster is how many behaviors in a cluster to show at full detail;
	// the rest are summarized. Default: 1.
	MaxIndividualPerCluster int
}

// DefaultCoalesceConfig returns sensible defaults.
func DefaultCoalesceConfig() CoalesceConfig {
	return CoalesceConfig{
		MinClusterSize:          3,
		MaxIndividualPerCluster: 1,
	}
}

// BehaviorCluster is a group of related behaviors that can be compactly represented.
type BehaviorCluster struct {
	// Representative is the most specific/actionable behavior shown at full detail.
	Representative models.InjectedBehavior

	// Members are the other cluster members (shown as summary or omitted).
	Members []models.InjectedBehavior

	// ClusterLabel describes the cluster (e.g., "Python File Handling" or "Git Operations").
	ClusterLabel string

	// SharedTags are tags common to all members.
	SharedTags []string
}

// Coalescer groups related behaviors to reduce context bloat.
type Coalescer struct {
	config CoalesceConfig
}

// NewCoalescer creates a new behavior coalescer.
func NewCoalescer(config CoalesceConfig) *Coalescer {
	return &Coalescer{config: config}
}

// Coalesce groups related behaviors by shared tags and kind.
// Returns individual behaviors and clusters.
//
// Algorithm:
//  1. Group behaviors by kind (directives together, constraints together)
//  2. Within each kind group, cluster by tag overlap (Jaccard > 0.5)
//  3. Clusters with >= MinClusterSize members get coalesced:
//     - Pick the highest-activation member as representative (full detail)
//     - Others become summary or omitted
//     - Generate cluster label from shared tags
//  4. Behaviors not in any cluster remain individual
func (c *Coalescer) Coalesce(behaviors []models.InjectedBehavior) (individuals []models.InjectedBehavior, clusters []BehaviorCluster) {
	if len(behaviors) == 0 {
		return nil, nil
	}

	// Group behaviors by kind.
	kindGroups := make(map[models.BehaviorKind][]models.InjectedBehavior)
	for _, b := range behaviors {
		if b.Behavior == nil {
			continue
		}
		kindGroups[b.Behavior.Kind] = append(kindGroups[b.Behavior.Kind], b)
	}

	// Process each kind group independently.
	for _, group := range kindGroups {
		ind, cls := c.clusterByTags(group)
		individuals = append(individuals, ind...)
		clusters = append(clusters, cls...)
	}

	// Sort clusters by label for deterministic output.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ClusterLabel < clusters[j].ClusterLabel
	})

	return individuals, clusters
}

// clusterByTags clusters a group of same-kind behaviors by tag overlap.
func (c *Coalescer) clusterByTags(behaviors []models.InjectedBehavior) (individuals []models.InjectedBehavior, clusters []BehaviorCluster) {
	if len(behaviors) < c.config.MinClusterSize {
		return behaviors, nil
	}

	// Build adjacency: behaviors with Jaccard tag similarity > 0.5 are "connected".
	n := len(behaviors)
	assigned := make([]bool, n)

	// Use a simple greedy clustering: for each unassigned behavior,
	// find all unassigned neighbors (Jaccard > 0.5) to form a candidate cluster.
	for i := 0; i < n; i++ {
		if assigned[i] {
			continue
		}

		tagsI := behaviorTags(behaviors[i])
		if len(tagsI) == 0 {
			continue
		}

		// Collect all behaviors similar to i.
		candidate := []int{i}
		for j := i + 1; j < n; j++ {
			if assigned[j] {
				continue
			}
			tagsJ := behaviorTags(behaviors[j])
			if jaccardSimilarity(tagsI, tagsJ) > 0.5 {
				candidate = append(candidate, j)
			}
		}

		if len(candidate) < c.config.MinClusterSize {
			// Not enough for a cluster; leave unassigned for now.
			continue
		}

		// Verify pairwise: only keep members that have Jaccard > 0.5 with the shared tags.
		// Compute shared tags across all candidate members.
		shared := intersectTags(tagsI, behaviorTags(behaviors[candidate[1]]))
		for _, idx := range candidate[2:] {
			shared = intersectTags(shared, behaviorTags(behaviors[idx]))
		}

		// Mark all candidate members as assigned.
		for _, idx := range candidate {
			assigned[idx] = true
		}

		// Pick representative: highest activation score.
		repIdx := candidate[0]
		for _, idx := range candidate[1:] {
			if behaviors[idx].Score > behaviors[repIdx].Score {
				repIdx = idx
			}
		}

		// Build members list (everyone except representative).
		var members []models.InjectedBehavior
		for _, idx := range candidate {
			if idx != repIdx {
				members = append(members, behaviors[idx])
			}
		}

		cluster := BehaviorCluster{
			Representative: behaviors[repIdx],
			Members:        members,
			SharedTags:     shared,
			ClusterLabel:   generateClusterLabel(shared),
		}
		clusters = append(clusters, cluster)
	}

	// All unassigned behaviors become individuals.
	for i := 0; i < n; i++ {
		if !assigned[i] {
			individuals = append(individuals, behaviors[i])
		}
	}

	return individuals, clusters
}

// behaviorTags returns the tags for an InjectedBehavior, or nil if the behavior is nil.
func behaviorTags(ib models.InjectedBehavior) []string {
	if ib.Behavior == nil {
		return nil
	}
	return ib.Behavior.Content.Tags
}

// jaccardSimilarity computes the Jaccard index between two string slices.
// Returns 0.0 if both are empty.
func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0.0
	}

	setA := make(map[string]bool, len(a))
	for _, s := range a {
		setA[s] = true
	}

	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[s] = true
	}

	intersection := 0
	for s := range setA {
		if setB[s] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// intersectTags returns the intersection of two tag slices, preserving order of the first slice.
func intersectTags(a, b []string) []string {
	setB := make(map[string]bool, len(b))
	for _, s := range b {
		setB[s] = true
	}

	var result []string
	for _, s := range a {
		if setB[s] {
			result = append(result, s)
		}
	}
	return result
}

// generateClusterLabel creates a human-readable label from shared tags.
// Tags are title-cased and joined with spaces.
func generateClusterLabel(tags []string) string {
	if len(tags) == 0 {
		return "Related Behaviors"
	}

	titled := make([]string, len(tags))
	for i, tag := range tags {
		titled[i] = titleCase(tag)
	}

	return strings.Join(titled, " ")
}

// titleCase capitalizes the first letter of a string.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
