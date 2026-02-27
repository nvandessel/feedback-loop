package vectorindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DefaultTierThreshold is the vector count at which the index promotes
// from brute-force to HNSW.
const DefaultTierThreshold = 1000

// TieredIndex automatically selects between BruteForceIndex and HNSWIndex
// based on vector count. Thread-safe.
//
// Starts in brute-force mode. Promotes to HNSW when count exceeds threshold.
// Once promoted, stays in HNSW mode (no demotion to avoid oscillation).
type TieredIndex struct {
	mu        sync.Mutex
	bf        *BruteForceIndex
	hnsw      *HNSWIndex
	hnswCfg   HNSWConfig
	threshold int
	promoted  bool
}

// TieredConfig holds configuration for TieredIndex.
type TieredConfig struct {
	// Threshold is the vector count at which brute-force promotes to HNSW.
	// Default: DefaultTierThreshold (1000).
	Threshold int

	// HNSW holds the HNSW configuration used when promotion occurs.
	HNSW HNSWConfig
}

// NewTieredIndex creates a TieredIndex. If cfg.HNSW.Dir is set and an HNSW
// index file already exists there, the index starts in promoted (HNSW) mode
// with the persisted data. Otherwise it starts in brute-force mode.
func NewTieredIndex(cfg TieredConfig) (*TieredIndex, error) {
	threshold := cfg.Threshold
	if threshold <= 0 {
		threshold = DefaultTierThreshold
	}

	bf := NewBruteForceIndex()

	t := &TieredIndex{
		bf:        bf,
		hnswCfg:   cfg.HNSW,
		threshold: threshold,
	}

	// If HNSW dir is configured and an index file already exists, load it.
	if cfg.HNSW.Dir != "" {
		path := filepath.Join(cfg.HNSW.Dir, hnswFileName)
		if _, err := os.Stat(path); err == nil {
			h, err := NewHNSWIndex(cfg.HNSW)
			if err != nil {
				return nil, fmt.Errorf("loading existing HNSW index: %w", err)
			}
			t.hnsw = h
			t.promoted = true
		}
	}

	return t, nil
}

// active returns the currently active index. Caller must hold t.mu.
func (t *TieredIndex) active() VectorIndex {
	if t.promoted {
		return t.hnsw
	}
	return t.bf
}

// Add inserts or updates the vector for the given behavior ID.
// If the addition causes the vector count to exceed the threshold,
// the index promotes from brute-force to HNSW.
func (t *TieredIndex) Add(ctx context.Context, behaviorID string, vector []float32) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.active().Add(ctx, behaviorID, vector); err != nil {
		return err
	}

	if !t.promoted && t.active().Len() > t.threshold {
		if err := t.promote(ctx); err != nil {
			return fmt.Errorf("promoting to HNSW: %w", err)
		}
	}

	return nil
}

// Remove deletes the vector for the given behavior ID.
func (t *TieredIndex) Remove(ctx context.Context, behaviorID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active().Remove(ctx, behaviorID)
}

// Search returns the topK most similar vectors to query, sorted by descending score.
func (t *TieredIndex) Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active().Search(ctx, query, topK)
}

// Len returns the number of vectors currently in the index.
func (t *TieredIndex) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active().Len()
}

// Save persists the index state to its backing store.
func (t *TieredIndex) Save(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active().Save(ctx)
}

// Close releases resources. If promoted, closes the HNSW index.
func (t *TieredIndex) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.promoted {
		return t.hnsw.Close()
	}
	return t.bf.Close()
}

// promote migrates all vectors from brute-force to a new HNSW index.
// Caller must hold t.mu.
func (t *TieredIndex) promote(ctx context.Context) error {
	h, err := NewHNSWIndex(t.hnswCfg)
	if err != nil {
		return err
	}

	// Copy all vectors from bf to hnsw. Access bf.vectors directly
	// (same package). Lock bf.mu for safe reading.
	t.bf.mu.RLock()
	for id, vec := range t.bf.vectors {
		if err := h.Add(ctx, id, vec); err != nil {
			t.bf.mu.RUnlock()
			return fmt.Errorf("copying vector %s to HNSW: %w", id, err)
		}
	}
	t.bf.mu.RUnlock()

	t.hnsw = h
	t.promoted = true
	return nil
}
