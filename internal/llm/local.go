//go:build llamacpp

package llm

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/nvandessel/feedback-loop/internal/models"
	llama "github.com/tcpipuk/llama-go"
)

// LocalClient implements Client and EmbeddingComparer using an embedded GGUF model
// via llama-go. It provides embedding-based similarity comparison without external
// API dependencies. Thread-safe: all model/context access is serialized via mutex.
type LocalClient struct {
	embeddingModelPath string
	gpuLayers          int
	contextSize        int

	// mu serializes all llama context access (contexts are not thread-safe)
	mu sync.Mutex

	// Lazy-loaded resources
	model   *llama.Model
	embCtx  *llama.Context
	loadErr error
	once    sync.Once

	// fallback handles MergeBehaviors until Phase 2 (generation)
	fallback *FallbackClient
}

// LocalConfig configures the local LLM client.
type LocalConfig struct {
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
		embeddingModelPath: embPath,
		gpuLayers:          cfg.GPULayers,
		contextSize:        ctxSize,
		fallback:           NewFallbackClient(),
	}
}

// loadModel lazy-loads the embedding model and context on first use.
func (c *LocalClient) loadModel() error {
	c.once.Do(func() {
		path := c.embeddingModelPath
		if path == "" {
			c.loadErr = fmt.Errorf("no model path configured")
			return
		}

		model, err := llama.LoadModel(path,
			llama.WithGPULayers(c.gpuLayers),
			llama.WithMMap(true),
			llama.WithSilentLoading(),
		)
		if err != nil {
			c.loadErr = fmt.Errorf("loading model %s: %w", path, err)
			return
		}
		c.model = model

		ctx, err := model.NewContext(
			llama.WithEmbeddings(),
			llama.WithContext(c.contextSize),
			llama.WithThreads(runtime.NumCPU()),
		)
		if err != nil {
			model.Close()
			c.model = nil
			c.loadErr = fmt.Errorf("creating embedding context: %w", err)
			return
		}
		c.embCtx = ctx
	})
	return c.loadErr
}

// Available returns true if the embedding model file exists on disk.
// This is a cheap check that does not load the model.
func (c *LocalClient) Available() bool {
	path := c.embeddingModelPath
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// Embed returns a dense vector embedding for the given text.
func (c *LocalClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := c.loadModel(); err != nil {
		return nil, fmt.Errorf("local embed: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	emb, err := c.embCtx.GetEmbeddings(text)
	if err != nil {
		return nil, fmt.Errorf("getting embeddings: %w", err)
	}
	return emb, nil
}

// CompareEmbeddings embeds both texts and returns their cosine similarity.
func (c *LocalClient) CompareEmbeddings(ctx context.Context, a, b string) (float64, error) {
	embA, err := c.Embed(ctx, a)
	if err != nil {
		return 0, fmt.Errorf("embedding text a: %w", err)
	}
	embB, err := c.Embed(ctx, b)
	if err != nil {
		return 0, fmt.Errorf("embedding text b: %w", err)
	}
	return CosineSimilarity(embA, embB), nil
}

// CompareBehaviors compares two behaviors using embedding-based cosine similarity.
func (c *LocalClient) CompareBehaviors(ctx context.Context, a, b *models.Behavior) (*ComparisonResult, error) {
	similarity, err := c.CompareEmbeddings(ctx, a.Content.Canonical, b.Content.Canonical)
	if err != nil {
		return nil, fmt.Errorf("comparing behaviors: %w", err)
	}

	return &ComparisonResult{
		SemanticSimilarity: similarity,
		IntentMatch:        similarity > 0.8,
		MergeCandidate:     similarity > 0.7,
		Reasoning:          "Local embedding-based cosine similarity comparison",
	}, nil
}

// MergeBehaviors delegates to the rule-based FallbackClient.
// Phase 2 will add generation-based merging with a local text model.
func (c *LocalClient) MergeBehaviors(ctx context.Context, behaviors []*models.Behavior) (*MergeResult, error) {
	return c.fallback.MergeBehaviors(ctx, behaviors)
}

// Close releases the model and context resources.
// Safe to call multiple times.
func (c *LocalClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.embCtx != nil {
		c.embCtx.Close()
		c.embCtx = nil
	}
	if c.model != nil {
		c.model.Close()
		c.model = nil
	}
	return nil
}
