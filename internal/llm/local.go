package llm

import (
	"context"
	"fmt"

	"github.com/nvandessel/feedback-loop/internal/models"
)

// LocalClient implements Client and EmbeddingComparer using a local GGUF model.
// The actual model backend (yzma) is wired in a separate PR; this file provides
// the struct, config, constructor, and stub methods so the rest of the codebase
// compiles and tests pass.
type LocalClient struct {
	libPath            string
	embeddingModelPath string
	gpuLayers          int
	contextSize        int

	// fallback handles MergeBehaviors until local generation is implemented.
	fallback *FallbackClient
}

// LocalConfig configures the local LLM client.
type LocalConfig struct {
	// LibPath is the directory containing yzma shared libraries (.so/.dylib).
	// Falls back to YZMA_LIB env var at runtime.
	LibPath string

	// ModelPath is the path to the GGUF model file for text generation.
	ModelPath string

	// EmbeddingModelPath is the path to the GGUF model file for embeddings.
	// If empty, ModelPath is used for embeddings as well.
	EmbeddingModelPath string

	// GPULayers is the number of layers to offload to GPU (0 = CPU only).
	GPULayers int

	// ContextSize is the context window size in tokens.
	ContextSize int
}

// NewLocalClient creates a new LocalClient. The model is not loaded until first use.
func NewLocalClient(cfg LocalConfig) *LocalClient {
	embPath := cfg.EmbeddingModelPath
	if embPath == "" {
		embPath = cfg.ModelPath
	}
	ctxSize := cfg.ContextSize
	if ctxSize <= 0 {
		ctxSize = 512
	}
	return &LocalClient{
		libPath:            cfg.LibPath,
		embeddingModelPath: embPath,
		gpuLayers:          cfg.GPULayers,
		contextSize:        ctxSize,
		fallback:           NewFallbackClient(),
	}
}

// Available returns true if both the library directory and model file exist on disk.
func (c *LocalClient) Available() bool {
	return false // Stub — yzma backend not yet wired
}

// Embed returns a dense vector embedding for the given text.
func (c *LocalClient) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, fmt.Errorf("local LLM not available: yzma backend not yet wired")
}

// CompareEmbeddings embeds both texts and returns their cosine similarity.
func (c *LocalClient) CompareEmbeddings(_ context.Context, _, _ string) (float64, error) {
	return 0, fmt.Errorf("local LLM not available: yzma backend not yet wired")
}

// CompareBehaviors compares two behaviors using embedding-based cosine similarity.
func (c *LocalClient) CompareBehaviors(_ context.Context, _, _ *models.Behavior) (*ComparisonResult, error) {
	return nil, fmt.Errorf("local LLM not available: yzma backend not yet wired")
}

// MergeBehaviors delegates to the rule-based FallbackClient.
func (c *LocalClient) MergeBehaviors(ctx context.Context, behaviors []*models.Behavior) (*MergeResult, error) {
	return c.fallback.MergeBehaviors(ctx, behaviors)
}

// Close is a no-op until the yzma backend is wired.
func (c *LocalClient) Close() error {
	return nil
}
