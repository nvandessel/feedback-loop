//go:build windows

package vectorindex

import "context"

const hnswFileName = "hnsw.bin"

// HNSWConfig holds configuration parameters for HNSWIndex.
type HNSWConfig struct {
	// Dir is the directory where the HNSW graph is persisted.
	// If empty, the graph is in-memory only and Save is a no-op.
	Dir string

	// M is the maximum number of neighbors per node. Default: 16.
	M int

	// EfSearch is the number of candidates considered during search. Default: 100.
	EfSearch int

	// Ml is the level generation factor. Default: 0.25.
	Ml float64
}

func (c *HNSWConfig) withDefaults() HNSWConfig {
	out := *c
	if out.M == 0 {
		out.M = 16
	}
	if out.EfSearch == 0 {
		out.EfSearch = 100
	}
	if out.Ml == 0 {
		out.Ml = 0.25
	}
	return out
}

// HNSWIndex on Windows falls back to BruteForceIndex.
// The coder/hnsw library depends on google/renameio which is not
// available on Windows. All operations delegate to brute-force search.
type HNSWIndex struct {
	bf *BruteForceIndex
}

// NewHNSWIndex creates a BruteForceIndex-backed fallback on Windows.
func NewHNSWIndex(_ HNSWConfig) (*HNSWIndex, error) {
	return &HNSWIndex{bf: NewBruteForceIndex()}, nil
}

func (h *HNSWIndex) Add(ctx context.Context, behaviorID string, vector []float32) error {
	return h.bf.Add(ctx, behaviorID, vector)
}

func (h *HNSWIndex) Remove(ctx context.Context, behaviorID string) error {
	return h.bf.Remove(ctx, behaviorID)
}

func (h *HNSWIndex) Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error) {
	return h.bf.Search(ctx, query, topK)
}

func (h *HNSWIndex) Len() int {
	return h.bf.Len()
}

// Save is a no-op on Windows (no HNSW persistence).
func (h *HNSWIndex) Save(_ context.Context) error {
	return nil
}

// Close is a no-op on Windows.
func (h *HNSWIndex) Close() error {
	return nil
}

// Verify HNSWIndex satisfies the VectorIndex interface at compile time.
var _ VectorIndex = (*HNSWIndex)(nil)
