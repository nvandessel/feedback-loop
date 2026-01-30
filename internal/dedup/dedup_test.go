package dedup

import (
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func TestDuplicateMatch_Instantiation(t *testing.T) {
	behavior := &models.Behavior{
		ID:   "test-behavior-1",
		Name: "Test Behavior",
	}

	match := DuplicateMatch{
		Behavior:         behavior,
		Similarity:       0.95,
		SimilarityMethod: "jaccard",
		MergeRecommended: true,
	}

	if match.Behavior == nil {
		t.Error("expected Behavior to be set")
	}
	if match.Behavior.ID != "test-behavior-1" {
		t.Errorf("expected Behavior.ID to be 'test-behavior-1', got '%s'", match.Behavior.ID)
	}
	if match.Similarity != 0.95 {
		t.Errorf("expected Similarity to be 0.95, got %f", match.Similarity)
	}
	if match.SimilarityMethod != "jaccard" {
		t.Errorf("expected SimilarityMethod to be 'jaccard', got '%s'", match.SimilarityMethod)
	}
	if !match.MergeRecommended {
		t.Error("expected MergeRecommended to be true")
	}
}

func TestDeduplicationReport_Instantiation(t *testing.T) {
	report := DeduplicationReport{
		TotalBehaviors:  100,
		DuplicatesFound: 10,
		MergesPerformed: 5,
		MergedBehaviors: []*models.Behavior{
			{ID: "merged-1", Name: "Merged Behavior 1"},
			{ID: "merged-2", Name: "Merged Behavior 2"},
		},
		Errors: []string{"error 1", "error 2"},
	}

	if report.TotalBehaviors != 100 {
		t.Errorf("expected TotalBehaviors to be 100, got %d", report.TotalBehaviors)
	}
	if report.DuplicatesFound != 10 {
		t.Errorf("expected DuplicatesFound to be 10, got %d", report.DuplicatesFound)
	}
	if report.MergesPerformed != 5 {
		t.Errorf("expected MergesPerformed to be 5, got %d", report.MergesPerformed)
	}
	if len(report.MergedBehaviors) != 2 {
		t.Errorf("expected 2 merged behaviors, got %d", len(report.MergedBehaviors))
	}
	if len(report.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(report.Errors))
	}
}

func TestDeduplicatorConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.SimilarityThreshold != 0.9 {
		t.Errorf("expected SimilarityThreshold to be 0.9, got %f", config.SimilarityThreshold)
	}
	if config.AutoMerge {
		t.Error("expected AutoMerge to be false by default")
	}
	if config.UseLLM {
		t.Error("expected UseLLM to be false by default")
	}
	if config.MaxBatchSize != 100 {
		t.Errorf("expected MaxBatchSize to be 100, got %d", config.MaxBatchSize)
	}
}

func TestDeduplicatorConfig_SensibleThreshold(t *testing.T) {
	config := DefaultConfig()

	// Threshold should be high enough to avoid false positives
	if config.SimilarityThreshold < 0.7 {
		t.Errorf("default threshold %f is too low, should be >= 0.7", config.SimilarityThreshold)
	}

	// Threshold should be less than 1.0 to allow some variance
	if config.SimilarityThreshold >= 1.0 {
		t.Errorf("default threshold %f should be less than 1.0", config.SimilarityThreshold)
	}
}

func TestDeduplicatorConfig_MaxBatchSizePositive(t *testing.T) {
	config := DefaultConfig()

	if config.MaxBatchSize <= 0 {
		t.Errorf("expected MaxBatchSize to be positive, got %d", config.MaxBatchSize)
	}
}

func TestDuplicateMatch_ZeroValues(t *testing.T) {
	// Test that zero-valued struct is valid
	match := DuplicateMatch{}

	if match.Behavior != nil {
		t.Error("expected zero-valued Behavior to be nil")
	}
	if match.Similarity != 0.0 {
		t.Errorf("expected zero-valued Similarity to be 0.0, got %f", match.Similarity)
	}
	if match.SimilarityMethod != "" {
		t.Errorf("expected zero-valued SimilarityMethod to be empty, got '%s'", match.SimilarityMethod)
	}
	if match.MergeRecommended {
		t.Error("expected zero-valued MergeRecommended to be false")
	}
}

func TestDeduplicationReport_ZeroValues(t *testing.T) {
	// Test that zero-valued struct is valid
	report := DeduplicationReport{}

	if report.TotalBehaviors != 0 {
		t.Errorf("expected zero-valued TotalBehaviors to be 0, got %d", report.TotalBehaviors)
	}
	if report.MergedBehaviors != nil {
		t.Error("expected zero-valued MergedBehaviors to be nil")
	}
	if report.Errors != nil {
		t.Error("expected zero-valued Errors to be nil")
	}
}
