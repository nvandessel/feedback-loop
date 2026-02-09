// Package seed provides pre-seeded meta-behaviors for bootstrapping floop.
package seed

import (
	"github.com/nvandessel/feedback-loop/internal/store"
)

// SeedVersion is the version of the seed behavior definitions.
// Bump this when seed content changes to trigger updates.
const SeedVersion = "0.2.0"

// coreBehaviors returns the seed behaviors that bootstrap floop's self-teaching.
func coreBehaviors() []store.Node {
	return []store.Node{
		{
			ID:   "seed-capture-corrections",
			Kind: "behavior",
			Content: map[string]interface{}{
				"name": "core/capture-corrections-proactively",
				"kind": "directive",
				"content": map[string]interface{}{
					"canonical": "When corrected or when you discover insights, immediately call floop_learn(wrong='what happened', right='what to do instead'). Capture learnings proactively without waiting for permission.",
				},
			},
			Metadata: map[string]interface{}{
				"confidence": 1.0,
				"priority":   100,
				"provenance": map[string]interface{}{
					"source_type":     "imported",
					"package":         "floop-core",
					"package_version": SeedVersion,
				},
			},
		},
		{
			ID:   "seed-know-floop-tools",
			Kind: "behavior",
			Content: map[string]interface{}{
				"name": "core/know-your-floop-tools",
				"kind": "directive",
				"content": map[string]interface{}{
					"canonical": "You have persistent memory via floop. Use floop_active to check active behaviors for context, floop_learn to capture corrections and insights, and floop_list to see all stored behaviors.",
				},
			},
			Metadata: map[string]interface{}{
				"confidence": 1.0,
				"priority":   100,
				"provenance": map[string]interface{}{
					"source_type":     "imported",
					"package":         "floop-core",
					"package_version": SeedVersion,
				},
			},
		},
	}
}
