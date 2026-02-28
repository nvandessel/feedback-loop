package vectorindex

import (
	"context"
	"sort"
	"sync"

	"github.com/nvandessel/feedback-loop/internal/vecmath"
)

// BruteForceIndex performs exhaustive nearest neighbor search using cosine similarity.
// Thread-safe. Suitable for small to medium vector counts (up to ~1000).
type BruteForceIndex struct {
	mu      sync.RWMutex
	vectors map[string][]float32
}

// NewBruteForceIndex creates an empty BruteForceIndex.
func NewBruteForceIndex() *BruteForceIndex {
	return &BruteForceIndex{
		vectors: make(map[string][]float32),
	}
}

// Add inserts or replaces the vector for the given behavior ID.
func (b *BruteForceIndex) Add(_ context.Context, behaviorID string, vector []float32) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]float32, len(vector))
	copy(cp, vector)
	b.vectors[behaviorID] = cp
	return nil
}

// Remove deletes the vector for the given behavior ID. No-op if not found.
func (b *BruteForceIndex) Remove(_ context.Context, behaviorID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.vectors, behaviorID)
	return nil
}

// Search returns the topK most similar vectors to query, sorted by descending score.
func (b *BruteForceIndex) Search(_ context.Context, query []float32, topK int) ([]SearchResult, error) {
	if len(query) == 0 || topK <= 0 {
		return nil, nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.vectors) == 0 {
		return nil, nil
	}

	results := make([]SearchResult, 0, len(b.vectors))
	for id, vec := range b.vectors {
		score := vecmath.CosineSimilarity(query, vec)
		results = append(results, SearchResult{
			BehaviorID: id,
			Score:      score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > len(results) {
		topK = len(results)
	}

	return results[:topK], nil
}

// Len returns the number of vectors in the index.
func (b *BruteForceIndex) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.vectors)
}

// Save is a no-op for the in-memory brute-force index.
func (b *BruteForceIndex) Save(_ context.Context) error {
	return nil
}

// Close is a no-op for the in-memory brute-force index.
func (b *BruteForceIndex) Close() error {
	return nil
}
