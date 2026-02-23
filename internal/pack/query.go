package pack

import (
	"context"
	"fmt"

	"github.com/nvandessel/feedback-loop/internal/config"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// ListInstalled returns all installed packs from config.
func ListInstalled(cfg *config.FloopConfig) []config.InstalledPack {
	if cfg == nil {
		return nil
	}
	return cfg.Packs.Installed
}

// FindByPack returns all behaviors from a specific pack in the store.
func FindByPack(ctx context.Context, s store.GraphStore, packID string) ([]store.Node, error) {
	nodes, err := s.QueryNodes(ctx, map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("querying nodes: %w", err)
	}

	var result []store.Node
	for _, node := range nodes {
		if models.ExtractPackageName(node.Metadata) == packID {
			result = append(result, node)
		}
	}
	return result, nil
}
