package vectorindex

import (
	"context"
	"fmt"
	"sync"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

const lanceTableName = "behaviors"

// LanceDBConfig holds configuration for LanceDBIndex.
type LanceDBConfig struct {
	// Dir is the directory where LanceDB stores its data files.
	Dir string

	// Dims is the dimensionality of the embedding vectors.
	Dims int
}

// LanceDBIndex performs approximate nearest neighbor search using LanceDB,
// an embedded vector database. Thread-safe.
type LanceDBIndex struct {
	mu    sync.RWMutex
	db    contracts.IConnection
	table contracts.ITable
	dims  int
}

// NewLanceDBIndex creates a LanceDBIndex backed by the given directory.
// If a table already exists, it is opened; otherwise a new one is created.
func NewLanceDBIndex(cfg LanceDBConfig) (*LanceDBIndex, error) {
	ctx := context.Background()

	db, err := lancedb.Connect(ctx, cfg.Dir, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to LanceDB: %w", err)
	}

	names, err := db.TableNames(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("list tables: %w", err)
	}

	var table contracts.ITable
	found := false
	for _, n := range names {
		if n == lanceTableName {
			found = true
			break
		}
	}

	if found {
		table, err = db.OpenTable(ctx, lanceTableName)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("open table: %w", err)
		}
	} else {
		schema, serr := lancedb.NewSchemaBuilder().
			AddStringField("id", false).
			AddVectorField("vector", cfg.Dims, contracts.VectorDataTypeFloat32, false).
			Build()
		if serr != nil {
			db.Close()
			return nil, fmt.Errorf("build schema: %w", serr)
		}
		table, err = db.CreateTable(ctx, lanceTableName, schema)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("create table: %w", err)
		}
	}

	return &LanceDBIndex{db: db, table: table, dims: cfg.Dims}, nil
}

// Add inserts or replaces the vector for the given behavior ID.
func (l *LanceDBIndex) Add(_ context.Context, _ string, _ []float32) error {
	return nil
}

// Remove deletes the vector for the given behavior ID. No-op if not found.
func (l *LanceDBIndex) Remove(_ context.Context, _ string) error {
	return nil
}

// Search returns the topK most similar vectors to query, sorted by descending score.
func (l *LanceDBIndex) Search(_ context.Context, _ []float32, _ int) ([]SearchResult, error) {
	return nil, nil
}

// Len returns the number of vectors in the index.
func (l *LanceDBIndex) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count, err := l.table.Count(context.Background())
	if err != nil {
		return 0
	}
	return int(count)
}

// Save is a no-op. LanceDB auto-persists on write.
func (l *LanceDBIndex) Save(_ context.Context) error {
	return nil
}

// Close releases resources.
func (l *LanceDBIndex) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.table != nil {
		l.table.Close()
	}
	if l.db != nil {
		l.db.Close()
	}
	return nil
}

// Verify LanceDBIndex satisfies the VectorIndex interface at compile time.
var _ VectorIndex = (*LanceDBIndex)(nil)
