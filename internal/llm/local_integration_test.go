//go:build llamacpp && integration

package llm

import (
	"context"
	"os"
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
)

// These tests require a real GGUF embedding model.
// Run with: FLOOP_TEST_MODEL_PATH=/path/to/model.gguf go test -tags "llamacpp integration" ./internal/llm/ -v
//
// Recommended models:
//   - all-MiniLM-L6-v2-Q8_0.gguf (~23MB)
//   - nomic-embed-text-v1.5.Q8_0.gguf (~137MB)

func modelPath(t *testing.T) string {
	t.Helper()
	path := os.Getenv("FLOOP_TEST_MODEL_PATH")
	if path == "" {
		t.Skip("FLOOP_TEST_MODEL_PATH not set, skipping integration test")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("model file not found at %s: %v", path, err)
	}
	return path
}

func TestLocalClient_Integration_Available(t *testing.T) {
	path := modelPath(t)
	client := NewLocalClient(LocalConfig{
		EmbeddingModelPath: path,
		ContextSize:        512,
	})
	defer client.Close()

	if !client.Available() {
		t.Error("Available() should return true when model file exists")
	}
}

func TestLocalClient_Integration_Embed(t *testing.T) {
	path := modelPath(t)
	client := NewLocalClient(LocalConfig{
		EmbeddingModelPath: path,
		GPULayers:          0,
		ContextSize:        512,
	})
	defer client.Close()

	ctx := context.Background()
	emb, err := client.Embed(ctx, "The quick brown fox jumps over the lazy dog")
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if len(emb) == 0 {
		t.Fatal("Embed() returned empty vector")
	}
	t.Logf("Embedding dimension: %d", len(emb))
}

func TestLocalClient_Integration_CompareEmbeddings(t *testing.T) {
	path := modelPath(t)
	client := NewLocalClient(LocalConfig{
		EmbeddingModelPath: path,
		GPULayers:          0,
		ContextSize:        512,
	})
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name    string
		a       string
		b       string
		wantMin float64
		wantMax float64
	}{
		{
			name:    "identical texts",
			a:       "use pathlib for file paths",
			b:       "use pathlib for file paths",
			wantMin: 0.99,
			wantMax: 1.0,
		},
		{
			name:    "semantically similar",
			a:       "always run tests before committing code",
			b:       "execute the test suite prior to making a commit",
			wantMin: 0.5,
			wantMax: 1.0,
		},
		{
			name:    "semantically different",
			a:       "use pathlib for file paths in Python",
			b:       "the weather is sunny today",
			wantMin: -1.0,
			wantMax: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim, err := client.CompareEmbeddings(ctx, tt.a, tt.b)
			if err != nil {
				t.Fatalf("CompareEmbeddings() error: %v", err)
			}
			t.Logf("similarity = %.4f", sim)
			if sim < tt.wantMin || sim > tt.wantMax {
				t.Errorf("similarity = %.4f, want [%.2f, %.2f]", sim, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestLocalClient_Integration_CompareBehaviors(t *testing.T) {
	path := modelPath(t)
	client := NewLocalClient(LocalConfig{
		EmbeddingModelPath: path,
		GPULayers:          0,
		ContextSize:        512,
	})
	defer client.Close()

	ctx := context.Background()

	tests := []struct {
		name            string
		aCanonical      string
		bCanonical      string
		wantIntentMatch bool
	}{
		{
			name:            "near-duplicate behaviors",
			aCanonical:      "Always run go test before committing changes",
			bCanonical:      "Run go test prior to each commit to catch regressions",
			wantIntentMatch: true,
		},
		{
			name:            "unrelated behaviors",
			aCanonical:      "Use table-driven tests with t.Run",
			bCanonical:      "Never commit secrets or API keys to the repository",
			wantIntentMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := testBehavior("a", tt.aCanonical)
			b := testBehavior("b", tt.bCanonical)

			result, err := client.CompareBehaviors(ctx, a, b)
			if err != nil {
				t.Fatalf("CompareBehaviors() error: %v", err)
			}
			t.Logf("similarity=%.4f intent=%v merge=%v",
				result.SemanticSimilarity, result.IntentMatch, result.MergeCandidate)

			if result.IntentMatch != tt.wantIntentMatch {
				t.Errorf("IntentMatch = %v, want %v (similarity=%.4f)",
					result.IntentMatch, tt.wantIntentMatch, result.SemanticSimilarity)
			}
		})
	}
}

func TestLocalClient_Integration_Close(t *testing.T) {
	path := modelPath(t)
	client := NewLocalClient(LocalConfig{
		EmbeddingModelPath: path,
		GPULayers:          0,
		ContextSize:        512,
	})

	// Load model by using it
	ctx := context.Background()
	_, err := client.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}

	// Close should free resources
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Double close should be safe
	if err := client.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func testBehavior(id, canonical string) *models.Behavior {
	return &models.Behavior{
		ID:      id,
		Name:    id,
		Kind:    models.BehaviorKindDirective,
		Content: models.BehaviorContent{Canonical: canonical},
	}
}
