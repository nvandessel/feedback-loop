// Package vectorindex provides approximate nearest neighbor search over behavior embeddings.
package vectorindex

import "context"

// SearchResult pairs a behavior ID with its cosine similarity score.
type SearchResult struct {
	BehaviorID string
	Score      float64 // cosine similarity in [-1, 1], higher = more similar
}

// VectorIndex provides approximate nearest neighbor search over embeddings.
// Implementations must be safe for concurrent use from multiple goroutines.
type VectorIndex interface {
	// Add inserts or updates the vector for the given behavior ID.
	// If the ID already exists, the vector is replaced.
	Add(ctx context.Context, behaviorID string, vector []float32) error

	// Remove deletes the vector for the given behavior ID.
	// Returns nil if the ID does not exist (idempotent).
	Remove(ctx context.Context, behaviorID string) error

	// Search returns the topK most similar vectors to query, sorted by descending score.
	// Returns fewer than topK results if the index contains fewer vectors.
	Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error)

	// Len returns the number of vectors currently in the index.
	Len() int

	// Save persists the index state to its backing store.
	// Implementations without persistence should no-op.
	Save(ctx context.Context) error

	// Close releases resources. Implementations should save before closing.
	Close() error
}
