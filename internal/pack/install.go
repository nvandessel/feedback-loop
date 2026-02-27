package pack

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nvandessel/feedback-loop/internal/config"
	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// InstallOptions configures pack installation.
type InstallOptions struct {
	DeriveEdges bool // Automatically derive edges between pack behaviors and existing behaviors
}

// InstallResult reports what was installed.
type InstallResult struct {
	PackID       string
	Version      string
	Added        []string // IDs of newly added behaviors
	Updated      []string // IDs of upgraded behaviors
	Skipped      []string // IDs of skipped (up-to-date or forgotten)
	EdgesAdded   int
	EdgesSkipped int
	DerivedEdges int // Edges automatically derived between new and existing behaviors
}

// Install loads a pack file and installs its behaviors into the store.
// Follows the seeder pattern: skip forgotten, version-gate updates, stamp provenance.
func Install(ctx context.Context, s store.GraphStore, filePath string, cfg *config.FloopConfig, opts InstallOptions) (*InstallResult, error) {
	// 1. Read pack file
	data, manifest, err := ReadPackFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading pack file: %w", err)
	}

	result := &InstallResult{
		PackID:  string(manifest.ID),
		Version: manifest.Version,
	}

	// 2. Install nodes
	for _, bn := range data.Nodes {
		node := bn.Node

		// Stamp provenance on each node
		stampProvenance(&node, manifest)

		existing, err := s.GetNode(ctx, node.ID)
		if err != nil {
			return nil, fmt.Errorf("checking node %s: %w", node.ID, err)
		}

		if existing == nil {
			// New node -- add it
			if _, err := s.AddNode(ctx, node); err != nil {
				return nil, fmt.Errorf("adding node %s: %w", node.ID, err)
			}
			result.Added = append(result.Added, node.ID)
			continue
		}

		// Respect user curation: don't re-add forgotten behaviors
		if existing.Kind == string(models.BehaviorKindForgotten) {
			result.Skipped = append(result.Skipped, node.ID)
			continue
		}

		// Check version for upgrade
		existingVersion := models.ExtractPackageVersion(existing.Metadata)
		if existingVersion == manifest.Version {
			// Already up-to-date
			result.Skipped = append(result.Skipped, node.ID)
			continue
		}

		// Version mismatch -- update content
		if err := s.UpdateNode(ctx, node); err != nil {
			return nil, fmt.Errorf("updating node %s: %w", node.ID, err)
		}
		result.Updated = append(result.Updated, node.ID)
	}

	// 3. Install edges
	for _, edge := range data.Edges {
		if err := s.AddEdge(ctx, edge); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to add edge %s -> %s (%s): %v\n",
				edge.Source, edge.Target, edge.Kind, err)
			result.EdgesSkipped++
			continue
		}
		result.EdgesAdded++
	}

	// 4. Sync store
	if err := s.Sync(ctx); err != nil {
		return nil, fmt.Errorf("syncing after install: %w", err)
	}

	// 4b. Derive edges between new/updated pack behaviors and existing behaviors
	if opts.DeriveEdges && (len(result.Added) > 0 || len(result.Updated) > 0) {
		newIDs := make([]string, 0, len(result.Added)+len(result.Updated))
		newIDs = append(newIDs, result.Added...)
		newIDs = append(newIDs, result.Updated...)
		intResult, intErr := IntegratePackBehaviors(ctx, s, newIDs)
		if intErr != nil {
			fmt.Fprintf(os.Stderr, "warning: edge derivation failed: %v\n", intErr)
		} else {
			result.DerivedEdges = intResult.EdgesCreated
		}
	}

	// 5. Record in config
	if cfg != nil {
		recordInstall(cfg, manifest, result)
	}

	return result, nil
}

// stampProvenance sets package and package_version in the node's provenance metadata.
func stampProvenance(node *store.Node, manifest *PackManifest) {
	if node.Metadata == nil {
		node.Metadata = make(map[string]interface{})
	}

	prov, ok := node.Metadata["provenance"].(map[string]interface{})
	if !ok {
		prov = make(map[string]interface{})
	}

	prov["package"] = string(manifest.ID)
	prov["package_version"] = manifest.Version
	node.Metadata["provenance"] = prov
}

// recordInstall updates the config's installed packs list.
func recordInstall(cfg *config.FloopConfig, manifest *PackManifest, result *InstallResult) {
	// Remove existing entry for this pack if present
	filtered := make([]config.InstalledPack, 0, len(cfg.Packs.Installed))
	for _, p := range cfg.Packs.Installed {
		if p.ID != string(manifest.ID) {
			filtered = append(filtered, p)
		}
	}

	// Add new entry
	filtered = append(filtered, config.InstalledPack{
		ID:            string(manifest.ID),
		Version:       manifest.Version,
		InstalledAt:   time.Now(),
		BehaviorCount: len(result.Added) + len(result.Updated) + len(result.Skipped),
		EdgeCount:     result.EdgesAdded,
	})

	cfg.Packs.Installed = filtered
}
