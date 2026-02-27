package pack

import (
	"context"
	"testing"
	"time"

	"github.com/nvandessel/floop/internal/config"
	"github.com/nvandessel/floop/internal/store"
)

func TestFindByPack(t *testing.T) {
	s := store.NewInMemoryGraphStore()
	ctx := context.Background()

	// Add behaviors from two different packs
	s.AddNode(ctx, store.Node{
		ID:   "b-q-1",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "pack-a-behavior-1",
		},
		Metadata: map[string]interface{}{
			"provenance": map[string]interface{}{
				"package":         "org-a/pack-a",
				"package_version": "1.0.0",
			},
		},
	})
	s.AddNode(ctx, store.Node{
		ID:   "b-q-2",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "pack-a-behavior-2",
		},
		Metadata: map[string]interface{}{
			"provenance": map[string]interface{}{
				"package":         "org-a/pack-a",
				"package_version": "1.0.0",
			},
		},
	})
	s.AddNode(ctx, store.Node{
		ID:   "b-q-3",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "pack-b-behavior-1",
		},
		Metadata: map[string]interface{}{
			"provenance": map[string]interface{}{
				"package":         "org-b/pack-b",
				"package_version": "2.0.0",
			},
		},
	})
	// Node without provenance
	s.AddNode(ctx, store.Node{
		ID:   "b-q-4",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "no-provenance",
		},
		Metadata: map[string]interface{}{},
	})

	// Query for pack-a
	result, err := FindByPack(ctx, s, "org-a/pack-a")
	if err != nil {
		t.Fatalf("FindByPack() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("found %d behaviors, want 2", len(result))
	}

	// Query for pack-b
	result, err = FindByPack(ctx, s, "org-b/pack-b")
	if err != nil {
		t.Fatalf("FindByPack() error = %v", err)
	}
	if len(result) != 1 {
		t.Errorf("found %d behaviors, want 1", len(result))
	}

	// Query for nonexistent pack
	result, err = FindByPack(ctx, s, "nonexistent/pack")
	if err != nil {
		t.Fatalf("FindByPack() error = %v", err)
	}
	if len(result) != 0 {
		t.Errorf("found %d behaviors, want 0", len(result))
	}
}

func TestListInstalled(t *testing.T) {
	cfg := config.Default()
	cfg.Packs.Installed = []config.InstalledPack{
		{
			ID:            "org-a/pack-a",
			Version:       "1.0.0",
			InstalledAt:   time.Now(),
			BehaviorCount: 5,
			EdgeCount:     2,
		},
		{
			ID:            "org-b/pack-b",
			Version:       "2.0.0",
			InstalledAt:   time.Now(),
			BehaviorCount: 3,
			EdgeCount:     0,
		},
	}

	result := ListInstalled(cfg)
	if len(result) != 2 {
		t.Fatalf("ListInstalled() = %d, want 2", len(result))
	}
	if result[0].ID != "org-a/pack-a" {
		t.Errorf("result[0].ID = %q, want %q", result[0].ID, "org-a/pack-a")
	}
	if result[1].ID != "org-b/pack-b" {
		t.Errorf("result[1].ID = %q, want %q", result[1].ID, "org-b/pack-b")
	}
}

func TestListInstalled_NilConfig(t *testing.T) {
	result := ListInstalled(nil)
	if result != nil {
		t.Errorf("ListInstalled(nil) = %v, want nil", result)
	}
}

func TestListInstalled_EmptyPacks(t *testing.T) {
	cfg := config.Default()
	result := ListInstalled(cfg)
	if len(result) != 0 {
		t.Errorf("ListInstalled() = %d, want 0", len(result))
	}
}
