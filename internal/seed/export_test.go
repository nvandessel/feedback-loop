package seed

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/nvandessel/floop/internal/backup"
	"github.com/nvandessel/floop/internal/pack"
)

// committedPackPath returns the absolute path to the committed floop-core.fpack.
func committedPackPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	// thisFile is internal/seed/export_test.go — repo root is two dirs up
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(repoRoot, ".floop", "packs", "floop-core.fpack")
}

func TestExportCorePack(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "floop-core.fpack")

	if err := ExportCorePack(out); err != nil {
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
		if err := ExportCorePack(committed); err != nil {
			t.Fatalf("updating committed pack: %v", err)
		}
		t.Logf("updated committed pack at %s", committed)
	}
}

func TestCorePackFreshness(t *testing.T) {
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
	if err := ExportCorePack(freshPath); err != nil {
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

	// Compare node content (canonical text)
	committedContent := nodeContentMap(committedData.Nodes)
	freshContent := nodeContentMap(freshData.Nodes)

	for id, freshCanonical := range freshContent {
		if committedCanonical, ok := committedContent[id]; ok {
			if committedCanonical != freshCanonical {
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

func nodeContentMap(nodes []backup.BackupNode) map[string]string {
	m := make(map[string]string, len(nodes))
	for _, n := range nodes {
		canonical := ""
		if content, ok := n.Node.Content["content"].(map[string]interface{}); ok {
			if c, ok := content["canonical"].(string); ok {
				canonical = c
			}
		}
		m[n.Node.ID] = canonical
	}
	return m
}
