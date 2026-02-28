package vectorindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Compile-time check that TieredIndex satisfies the VectorIndex interface.
var _ VectorIndex = (*TieredIndex)(nil)

// axisVec returns an 8-dim unit vector along the given axis (0-7).
func axisVec(axis int) []float32 {
	v := make([]float32, 8)
	v[axis%8] = 1.0
	return v
}

func TestTieredIndex_StartsInBruteForce(t *testing.T) {
	idx, err := NewTieredIndex(TieredConfig{Threshold: 5})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Add 3 vectors (below threshold of 5).
	for i := 0; i < 3; i++ {
		if err := idx.Add(ctx, fmt.Sprintf("b%d", i), axisVec(i)); err != nil {
			t.Fatalf("Add b%d: %v", i, err)
		}
	}

	if idx.Len() != 3 {
		t.Fatalf("expected Len()=3, got %d", idx.Len())
	}

	// Search should work in brute-force mode.
	results, err := idx.Search(ctx, axisVec(0), 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].BehaviorID != "b0" {
		t.Errorf("expected b0 as top result, got %v", results)
	}

	// Verify not promoted.
	idx.mu.Lock()
	promoted := idx.promoted
	idx.mu.Unlock()
	if promoted {
		t.Error("expected index to still be in brute-force mode")
	}
}

func TestTieredIndex_PromotesToHNSW(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewTieredIndex(TieredConfig{
		Threshold: 3,
		HNSW:      HNSWConfig{Dir: dir},
	})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Add 4 vectors (exceeds threshold of 3).
	for i := 0; i < 4; i++ {
		if err := idx.Add(ctx, fmt.Sprintf("b%d", i), axisVec(i)); err != nil {
			t.Fatalf("Add b%d: %v", i, err)
		}
	}

	if idx.Len() != 4 {
		t.Fatalf("expected Len()=4, got %d", idx.Len())
	}

	// Search should return the correct nearest neighbor.
	results, err := idx.Search(ctx, axisVec(2), 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].BehaviorID != "b2" {
		t.Errorf("expected b2 as top result, got %v", results)
	}

	// Verify promoted.
	idx.mu.Lock()
	promoted := idx.promoted
	idx.mu.Unlock()
	if !promoted {
		t.Error("expected index to be promoted to HNSW")
	}

	// Save should create a file on disk.
	if err := idx.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, hnswFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected HNSW file at %s after Save", path)
	}
}

func TestTieredIndex_SearchConsistencyAcrossTiers(t *testing.T) {
	idx, err := NewTieredIndex(TieredConfig{Threshold: 3})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Add vectors one at a time, checking search at each step.
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("b%d", i)
		if err := idx.Add(ctx, id, axisVec(i)); err != nil {
			t.Fatalf("Add %s: %v", id, err)
		}

		// Search for the vector we just added; it should be the top-1.
		results, err := idx.Search(ctx, axisVec(i), 1)
		if err != nil {
			t.Fatalf("Search for %s: %v", id, err)
		}
		if len(results) == 0 {
			t.Fatalf("no results for %s", id)
		}
		if results[0].BehaviorID != id {
			t.Errorf("step %d: expected %s as top result, got %s (score=%f)",
				i, id, results[0].BehaviorID, results[0].Score)
		}
	}
}

func TestTieredIndex_NoDemotion(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewTieredIndex(TieredConfig{
		Threshold: 3,
		HNSW:      HNSWConfig{Dir: dir},
	})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Add enough vectors to promote.
	for i := 0; i < 5; i++ {
		if err := idx.Add(ctx, fmt.Sprintf("b%d", i), axisVec(i)); err != nil {
			t.Fatalf("Add b%d: %v", i, err)
		}
	}

	// Verify promoted.
	idx.mu.Lock()
	if !idx.promoted {
		idx.mu.Unlock()
		t.Fatal("expected promoted after adding 5 vectors with threshold=3")
	}
	idx.mu.Unlock()

	// Remove vectors below threshold.
	for i := 0; i < 4; i++ {
		if err := idx.Remove(ctx, fmt.Sprintf("b%d", i)); err != nil {
			t.Fatalf("Remove b%d: %v", i, err)
		}
	}

	if idx.Len() != 1 {
		t.Fatalf("expected Len()=1, got %d", idx.Len())
	}

	// Should still be promoted (no demotion).
	idx.mu.Lock()
	promoted := idx.promoted
	idx.mu.Unlock()
	if !promoted {
		t.Error("expected index to remain promoted after removals")
	}

	// Save should still write to disk (HNSW mode).
	if err := idx.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(dir, hnswFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected HNSW file at %s after Save (no demotion)", path)
	}
}

func TestTieredIndex_LoadExistingHNSW(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create an HNSW index, add vectors, and save.
	h, err := NewHNSWIndex(HNSWConfig{Dir: dir})
	if err != nil {
		t.Fatalf("NewHNSWIndex: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := h.Add(ctx, fmt.Sprintf("b%d", i), axisVec(i)); err != nil {
			t.Fatalf("Add b%d: %v", i, err)
		}
	}
	if err := h.Save(ctx); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Create a TieredIndex pointing to the same directory.
	idx, err := NewTieredIndex(TieredConfig{
		Threshold: 5,
		HNSW:      HNSWConfig{Dir: dir},
	})
	if err != nil {
		t.Fatalf("NewTieredIndex with existing HNSW: %v", err)
	}
	defer idx.Close()

	// Should start promoted with all vectors.
	idx.mu.Lock()
	promoted := idx.promoted
	idx.mu.Unlock()
	if !promoted {
		t.Error("expected index to start promoted when HNSW file exists")
	}

	if idx.Len() != 3 {
		t.Fatalf("expected Len()=3 from loaded HNSW, got %d", idx.Len())
	}

	// Search should work.
	results, err := idx.Search(ctx, axisVec(1), 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].BehaviorID != "b1" {
		t.Errorf("expected b1 as top result, got %v", results)
	}
}

func TestTieredIndex_DefaultThreshold(t *testing.T) {
	if DefaultTierThreshold != 1000 {
		t.Errorf("expected DefaultTierThreshold=1000, got %d", DefaultTierThreshold)
	}

	idx, err := NewTieredIndex(TieredConfig{})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	idx.mu.Lock()
	threshold := idx.threshold
	idx.mu.Unlock()

	if threshold != 1000 {
		t.Errorf("expected default threshold=1000, got %d", threshold)
	}
}

func TestTieredIndex_ConcurrentAddPromote(t *testing.T) {
	idx, err := NewTieredIndex(TieredConfig{Threshold: 5})
	if err != nil {
		t.Fatalf("NewTieredIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("g%d", n)
			vec := axisVec(n)
			if err := idx.Add(ctx, id, vec); err != nil {
				t.Errorf("goroutine %d Add: %v", n, err)
				return
			}
			// Search to exercise concurrent read path.
			_, _ = idx.Search(ctx, vec, 3)
		}(g)
	}
	wg.Wait()

	// Verify the index is consistent (no panic, no race).
	if idx.Len() != 10 {
		t.Errorf("expected Len()=10, got %d", idx.Len())
	}
}
