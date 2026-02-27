package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportImportPreservesEmbeddings(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewSQLiteGraphStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore() error = %v", err)
	}

	ctx := context.Background()

	// Add a behavior and store an embedding
	_, err = s.AddNode(ctx, Node{
		ID:   "emb-round-trip",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "Embedding Round Trip",
			"kind": "directive",
			"content": map[string]interface{}{
				"canonical": "Test embedding round-trip through JSONL",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddNode() error = %v", err)
	}

	originalVec := []float32{0.1, -0.2, 0.3, 0.0, 0.5}
	modelName := "text-embedding-3-small"
	err = s.StoreEmbedding(ctx, "emb-round-trip", originalVec, modelName)
	if err != nil {
		t.Fatalf("StoreEmbedding() error = %v", err)
	}

	// Sync to export to JSONL
	if err := s.Sync(ctx); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	s.Close()

	// Create a new store in a different directory and import the JSONL
	tmpDir2 := t.TempDir()
	floopDir2 := filepath.Join(tmpDir2, ".floop")
	if err := os.MkdirAll(floopDir2, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Copy the nodes.jsonl from the first store
	nodesFile := filepath.Join(tmpDir, ".floop", "nodes.jsonl")
	nodesData, err := os.ReadFile(nodesFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	destNodesFile := filepath.Join(floopDir2, "nodes.jsonl")
	if err := os.WriteFile(destNodesFile, nodesData, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Open the new store which auto-imports
	s2, err := NewSQLiteGraphStore(tmpDir2)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore() for import error = %v", err)
	}
	defer s2.Close()

	// Verify the embedding was restored
	embeddings, err := s2.GetAllEmbeddings(ctx)
	if err != nil {
		t.Fatalf("GetAllEmbeddings() error = %v", err)
	}

	if len(embeddings) != 1 {
		t.Fatalf("GetAllEmbeddings() returned %d, want 1", len(embeddings))
	}

	if embeddings[0].BehaviorID != "emb-round-trip" {
		t.Errorf("BehaviorID = %s, want emb-round-trip", embeddings[0].BehaviorID)
	}

	if len(embeddings[0].Embedding) != len(originalVec) {
		t.Fatalf("embedding length = %d, want %d", len(embeddings[0].Embedding), len(originalVec))
	}

	for i, v := range embeddings[0].Embedding {
		if v != originalVec[i] {
			t.Errorf("embedding[%d] = %v, want %v", i, v, originalVec[i])
		}
	}

	// Verify embedding_model was stored
	var storedModel string
	err = s2.db.QueryRowContext(ctx,
		`SELECT embedding_model FROM behaviors WHERE id = ?`,
		"emb-round-trip").Scan(&storedModel)
	if err != nil {
		t.Fatalf("query embedding_model error = %v", err)
	}
	if storedModel != modelName {
		t.Errorf("embedding_model = %s, want %s", storedModel, modelName)
	}
}

func TestImportJSONL_NoEmbedding(t *testing.T) {
	// Test that old JSONL files without embedding fields import cleanly
	tmpDir := t.TempDir()
	floopDir := filepath.Join(tmpDir, ".floop")
	if err := os.MkdirAll(floopDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Write a JSONL file without any embedding fields (old format)
	nodesFile := filepath.Join(floopDir, "nodes.jsonl")
	f, err := os.Create(nodesFile)
	if err != nil {
		t.Fatalf("os.Create() error = %v", err)
	}
	f.WriteString(`{"id":"old-node","kind":"behavior","content":{"name":"Old Node","kind":"directive","content":{"canonical":"A behavior from old JSONL"}},"metadata":{"confidence":0.7}}`)
	f.WriteString("\n")
	f.Close()

	// Import should succeed without errors
	s, err := NewSQLiteGraphStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore() error = %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Verify node was imported
	got, err := s.GetNode(ctx, "old-node")
	if err != nil {
		t.Fatalf("GetNode() error = %v", err)
	}
	if got == nil {
		t.Fatal("imported node not found")
	}
	if got.Content["name"] != "Old Node" {
		t.Errorf("node name = %v, want Old Node", got.Content["name"])
	}

	// Verify no embedding was stored
	embeddings, err := s.GetAllEmbeddings(ctx)
	if err != nil {
		t.Fatalf("GetAllEmbeddings() error = %v", err)
	}
	if len(embeddings) != 0 {
		t.Errorf("GetAllEmbeddings() returned %d, want 0 (old JSONL has no embeddings)", len(embeddings))
	}
}

func TestExportEmbedding_Base64Format(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewSQLiteGraphStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore() error = %v", err)
	}

	ctx := context.Background()

	// Add a behavior and store an embedding
	_, err = s.AddNode(ctx, Node{
		ID:   "base64-test",
		Kind: "behavior",
		Content: map[string]interface{}{
			"name": "Base64 Test",
			"kind": "directive",
			"content": map[string]interface{}{
				"canonical": "Test base64 encoding correctness",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddNode() error = %v", err)
	}

	vec := []float32{1.0, 2.0, 3.0}
	modelName := "test-model-v1"
	err = s.StoreEmbedding(ctx, "base64-test", vec, modelName)
	if err != nil {
		t.Fatalf("StoreEmbedding() error = %v", err)
	}

	// Sync to export to JSONL
	if err := s.Sync(ctx); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Read the raw JSONL and verify the base64 encoding
	nodesFile := filepath.Join(tmpDir, ".floop", "nodes.jsonl")
	data, err := os.ReadFile(nodesFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var node Node
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify embedding field is present and is base64
	embStr, ok := node.Metadata["embedding"].(string)
	if !ok || embStr == "" {
		t.Fatal("embedding not found in metadata or not a string")
	}

	// Decode the base64 and verify it matches the original binary encoding
	embBytes, err := base64.StdEncoding.DecodeString(embStr)
	if err != nil {
		t.Fatalf("base64.DecodeString() error = %v", err)
	}

	expectedBlob := encodeEmbedding(vec)
	if len(embBytes) != len(expectedBlob) {
		t.Fatalf("decoded blob length = %d, want %d", len(embBytes), len(expectedBlob))
	}
	for i := range embBytes {
		if embBytes[i] != expectedBlob[i] {
			t.Errorf("blob[%d] = %d, want %d", i, embBytes[i], expectedBlob[i])
		}
	}

	// Verify embedding_model field
	embModelStr, ok := node.Metadata["embedding_model"].(string)
	if !ok || embModelStr != modelName {
		t.Errorf("embedding_model = %v, want %s", node.Metadata["embedding_model"], modelName)
	}

	// Also verify round-trip: decode the blob to float32 and compare
	decoded := decodeEmbedding(embBytes)
	if len(decoded) != len(vec) {
		t.Fatalf("decoded vector length = %d, want %d", len(decoded), len(vec))
	}
	for i, v := range decoded {
		if v != vec[i] {
			t.Errorf("decoded[%d] = %v, want %v", i, v, vec[i])
		}
	}

	s.Close()
}
