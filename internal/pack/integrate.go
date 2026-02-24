package pack

import (
	"context"

	"github.com/nvandessel/feedback-loop/internal/edges"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// IntegratePackBehaviors derives edges between newly installed pack behaviors
// and existing behaviors. Only computes new<->new and new<->existing pairs,
// skipping existing<->existing pairs for efficiency.
func IntegratePackBehaviors(ctx context.Context, s store.GraphStore, newNodeIDs []string) (*edges.SubsetResult, error) {
	if len(newNodeIDs) == 0 {
		return &edges.SubsetResult{}, nil
	}

	// Load all behaviors
	allBehaviors, err := edges.LoadBehaviorsFromStore(ctx, s)
	if err != nil {
		return nil, err
	}

	return edges.DeriveEdgesForSubset(ctx, s, newNodeIDs, allBehaviors)
}
