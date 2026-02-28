package seed

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nvandessel/floop/internal/backup"
	"github.com/nvandessel/floop/internal/pack"
)

// repoRoot walks up from the working directory to find the go.mod sentinel.
// This is immune to -trimpath and to the test file being moved.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

// committedPackPath returns the absolute path to the committed floop-core.fpack.
func committedPackPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".floop", "packs", "floop-core.fpack")
}

func TestExportCorePack(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	out := filepath.Join(dir, "floop-core.fpack")

	if err := ExportCorePack(ctx, out); err != nil {
		t.Fatalf("ExportCorePack() error = %v", err)
	}

	// Read back and verify
	data, manifest, err := pack.ReadPackFile(out)
	if err != nil {
		t.Fatalf("ReadPackFile() error = %v", err)
	}

	// Manifest fields
	if manifest.ID != "floop/core" {
		t.Errorf("manifest.ID = %q, want %q", manifest.ID, "floop/core")
	}
	if manifest.Version != SeedVersion {
		t.Errorf("manifest.Version = %q, want %q", manifest.Version, SeedVersion)
	}
	if manifest.Description != "Core behaviors that teach agents how to use floop" {
		t.Errorf("manifest.Description = %q", manifest.Description)
	}
	if manifest.Author != "floop" {
		t.Errorf("manifest.Author = %q, want %q", manifest.Author, "floop")
	}

	// Behavior count matches seeds
	seeds := coreBehaviors()
	if len(data.Nodes) != len(seeds) {
		t.Errorf("behavior count = %d, want %d", len(data.Nodes), len(seeds))
	}

	// Every seed ID is present
	nodeIDs := make(map[string]bool)
	for _, n := range data.Nodes {
		nodeIDs[n.Node.ID] = true
	}
	for _, s := range seeds {
		if !nodeIDs[s.ID] {
			t.Errorf("seed %q missing from exported pack", s.ID)
		}
	}

	// Update committed artifact if UPDATE_GOLDEN env is set
	if os.Getenv("UPDATE_GOLDEN") != "" {
		committed := committedPackPath(t)
		if err := os.MkdirAll(filepath.Dir(committed), 0o755); err != nil {
			t.Fatalf("creating packs dir: %v", err)
		}
		if err := ExportCorePack(ctx, committed); err != nil {
			t.Fatalf("updating committed pack: %v", err)
		}
		t.Logf("updated committed pack at %s", committed)
	}
}

func TestCorePackFreshness(t *testing.T) {
	ctx := context.Background()
	committed := committedPackPath(t)
	if _, err := os.Stat(committed); os.IsNotExist(err) {
		t.Fatalf("committed pack not found at %s — generate with: UPDATE_GOLDEN=1 go test -run TestExportCorePack ./internal/seed/", committed)
	}

	// Read committed pack
	committedData, committedManifest, err := pack.ReadPackFile(committed)
	if err != nil {
		t.Fatalf("reading committed pack: %v", err)
	}

	// Generate fresh pack
	dir := t.TempDir()
	freshPath := filepath.Join(dir, "floop-core.fpack")
	if err := ExportCorePack(ctx, freshPath); err != nil {
		t.Fatalf("generating fresh pack: %v", err)
	}

	freshData, freshManifest, err := pack.ReadPackFile(freshPath)
	if err != nil {
		t.Fatalf("reading fresh pack: %v", err)
	}

	// Compare manifest version
	if committedManifest.Version != freshManifest.Version {
		t.Errorf("committed pack version %q != current SeedVersion %q — regenerate with: UPDATE_GOLDEN=1 go test -run TestExportCorePack ./internal/seed/",
			committedManifest.Version, freshManifest.Version)
	}

	// Compare node IDs
	committedIDs := sortedNodeIDs(committedData.Nodes)
	freshIDs := sortedNodeIDs(freshData.Nodes)

	if len(committedIDs) != len(freshIDs) {
		t.Fatalf("committed pack has %d nodes, fresh has %d — regenerate with: UPDATE_GOLDEN=1 go test -run TestExportCorePack ./internal/seed/",
			len(committedIDs), len(freshIDs))
	}

	for i := range committedIDs {
		if committedIDs[i] != freshIDs[i] {
			t.Errorf("node ID mismatch at index %d: committed=%q fresh=%q — regenerate with: UPDATE_GOLDEN=1 go test -run TestExportCorePack ./internal/seed/",
				i, committedIDs[i], freshIDs[i])
		}
	}

	// Compare full node content (Content + Metadata) via JSON serialization
	committedContent := nodeSerializedMap(committedData.Nodes)
	freshContent := nodeSerializedMap(freshData.Nodes)

	for id, freshJSON := range freshContent {
		if committedJSON, ok := committedContent[id]; ok {
			if committedJSON != freshJSON {
				t.Errorf("content drift for %q — committed floop-core.fpack is stale — regenerate with: UPDATE_GOLDEN=1 go test -run TestExportCorePack ./internal/seed/", id)
			}
		}
	}
}

func sortedNodeIDs(nodes []backup.BackupNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.Node.ID
	}
	sort.Strings(ids)
	return ids
}

// nodeSerializedMap returns a map from node ID to a stable JSON serialization
// of the full node (Content + Metadata), enabling deep drift detection beyond
// just the canonical text.
func nodeSerializedMap(nodes []backup.BackupNode) map[string]string {
	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		// Serialize both Content and Metadata for full comparison
		payload := map[string]interface{}{
			"content":  n.Node.Content,
			"metadata": n.Node.Metadata,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			// Fallback: use node ID as placeholder so comparison still works
			m[n.Node.ID] = n.Node.ID
			continue
		}
		m[n.Node.ID] = string(b)
	}
	return m
}
