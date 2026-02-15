package ranking

import (
	"math"
	"testing"
	"time"

	"github.com/nvandessel/feedback-loop/internal/models"
)

func TestNewRelevanceScorer(t *testing.T) {
	config := DefaultScorerConfig()
	scorer := NewRelevanceScorer(config)

	if scorer == nil {
		t.Fatal("expected scorer to be non-nil")
	}

	// Check weights are normalized (4-signal model)
	totalWeight := scorer.config.ContextWeight + scorer.config.BaseLevelWeight +
		scorer.config.FeedbackWeight + scorer.config.PriorityWeight

	if math.Abs(totalWeight-1.0) > 0.001 {
		t.Errorf("weights should sum to 1.0, got %f", totalWeight)
	}
}

func TestRelevanceScorer_Score_NilBehavior(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	result := scorer.Score(nil, nil)

	if result.Score != 0 {
		t.Errorf("nil behavior should have score 0, got %f", result.Score)
	}
}

func TestRelevanceScorer_Score_Basic(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	behavior := &models.Behavior{
		ID:         "test",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 10,
			TimesFollowed:  8,
			CreatedAt:      now.Add(-24 * time.Hour),
			UpdatedAt:      now.Add(-1 * time.Hour),
		},
	}

	result := scorer.Score(behavior, nil)

	if result.Score <= 0 {
		t.Error("expected positive score")
	}
	if result.Behavior != behavior {
		t.Error("expected behavior reference to be preserved")
	}
}

func TestRelevanceScorer_Score_ConstraintBoost(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	constraint := &models.Behavior{
		ID:         "constraint",
		Kind:       models.BehaviorKindConstraint,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 10,
			TimesFollowed:  8,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	directive := &models.Behavior{
		ID:         "directive",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 10,
			TimesFollowed:  8,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	constraintScore := scorer.Score(constraint, nil)
	directiveScore := scorer.Score(directive, nil)

	// Constraints should score higher due to kind boost
	if constraintScore.Score <= directiveScore.Score {
		t.Errorf("constraint score (%f) should be > directive score (%f)",
			constraintScore.Score, directiveScore.Score)
	}

	// Verify kind boosts are applied correctly
	if constraintScore.KindBoost != 2.0 {
		t.Errorf("constraint kind boost should be 2.0, got %f", constraintScore.KindBoost)
	}
	if directiveScore.KindBoost != 1.5 {
		t.Errorf("directive kind boost should be 1.5, got %f", directiveScore.KindBoost)
	}
}

func TestRelevanceScorer_Score_BaseLevelScore(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	// Frequently activated behavior should have higher base-level score
	frequentlyUsed := &models.Behavior{
		ID:         "frequent",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 100,
			CreatedAt:      now.Add(-24 * time.Hour),
			UpdatedAt:      now,
		},
	}

	rarelyUsed := &models.Behavior{
		ID:         "rare",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 1,
			CreatedAt:      now.Add(-24 * time.Hour),
			UpdatedAt:      now,
		},
	}

	frequentScore := scorer.Score(frequentlyUsed, nil)
	rareScore := scorer.Score(rarelyUsed, nil)

	if frequentScore.BaseLevelScore <= rareScore.BaseLevelScore {
		t.Errorf("frequent base-level (%f) should be > rare base-level (%f)",
			frequentScore.BaseLevelScore, rareScore.BaseLevelScore)
	}
}

func TestRelevanceScorer_Score_BaseLevelNewBehavior(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())

	// New behavior with no activations should get neutral score (0.5)
	newBehavior := &models.Behavior{
		ID:         "new",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 0,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	result := scorer.Score(newBehavior, nil)
	if result.BaseLevelScore != 0.5 {
		t.Errorf("new behavior BaseLevelScore = %f, want 0.5", result.BaseLevelScore)
	}
}

func TestRelevanceScorer_Score_Recency(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	// ACT-R captures recency: same activation count, different ages
	recent := &models.Behavior{
		ID:         "recent",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 10,
			CreatedAt:      now.Add(-1 * time.Hour),
			UpdatedAt:      now.Add(-1 * time.Hour),
		},
	}

	old := &models.Behavior{
		ID:         "old",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated: 10,
			CreatedAt:      now.Add(-30 * 24 * time.Hour),
			UpdatedAt:      now.Add(-30 * 24 * time.Hour),
		},
	}

	recentScore := scorer.Score(recent, nil)
	oldScore := scorer.Score(old, nil)

	// Recent behavior should have higher base-level score (ACT-R captures recency)
	if recentScore.BaseLevelScore <= oldScore.BaseLevelScore {
		t.Errorf("recent base-level (%f) should be > old base-level (%f)",
			recentScore.BaseLevelScore, oldScore.BaseLevelScore)
	}
}

func TestRelevanceScorer_Score_ContextSpecificity(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	ctx := &models.ContextSnapshot{
		FilePath:     "main.go",
		FileLanguage: "go",
		Task:         "development",
	}

	global := &models.Behavior{
		ID:         "global",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		When:       nil, // No predicate = global
		Stats: models.BehaviorStats{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	specific := &models.Behavior{
		ID:         "specific",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		When: map[string]interface{}{
			"language": "go",
			"task":     "development",
		},
		Stats: models.BehaviorStats{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	globalScore := scorer.Score(global, ctx)
	specificScore := scorer.Score(specific, ctx)

	if specificScore.ContextScore <= globalScore.ContextScore {
		t.Errorf("specific context score (%f) should be > global context score (%f)",
			specificScore.ContextScore, globalScore.ContextScore)
	}
}

func TestRelevanceScorer_ScoreBatch(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	behaviors := []models.Behavior{
		{ID: "b1", Kind: models.BehaviorKindDirective, Confidence: 0.8, Stats: models.BehaviorStats{CreatedAt: now, UpdatedAt: now}},
		{ID: "b2", Kind: models.BehaviorKindConstraint, Confidence: 0.9, Stats: models.BehaviorStats{CreatedAt: now, UpdatedAt: now}},
		{ID: "b3", Kind: models.BehaviorKindPreference, Confidence: 0.7, Stats: models.BehaviorStats{CreatedAt: now, UpdatedAt: now}},
	}

	results := scorer.ScoreBatch(behaviors, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Behavior == nil {
			t.Errorf("result %d has nil behavior", i)
		}
		if result.Score <= 0 {
			t.Errorf("result %d has non-positive score: %f", i, result.Score)
		}
	}
}

func TestFeedbackScore_NoFeedback(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	// Behavior with no feedback should get neutral score (0.5)
	behavior := &models.Behavior{
		ID:         "no-feedback",
		Kind:       models.BehaviorKindDirective,
		Confidence: 0.8,
		Priority:   5,
		Stats: models.BehaviorStats{
			TimesActivated:  5,
			TimesFollowed:   0,
			TimesConfirmed:  0,
			TimesOverridden: 0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}

	result := scorer.Score(behavior, nil)
	if result.FeedbackScore != 0.5 {
		t.Errorf("FeedbackScore = %f, want 0.5 (neutral when no feedback data)", result.FeedbackScore)
	}
}

func TestFeedbackScore_WithFeedback(t *testing.T) {
	scorer := NewRelevanceScorer(DefaultScorerConfig())
	now := time.Now()

	tests := []struct {
		name    string
		stats   models.BehaviorStats
		wantMin float64
		wantMax float64
	}{
		{
			name: "below min sample — neutral",
			stats: models.BehaviorStats{
				TimesActivated: 10,
				TimesFollowed:  1,
				TimesConfirmed: 1,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			wantMin: 0.5,
			wantMax: 0.5,
		},
		{
			name: "all positive feedback",
			stats: models.BehaviorStats{
				TimesActivated: 10,
				TimesFollowed:  2,
				TimesConfirmed: 3,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			wantMin: 1.0,
			wantMax: 1.0, // 5/5 = 1.0
		},
		{
			name: "all negative feedback",
			stats: models.BehaviorStats{
				TimesActivated:  10,
				TimesOverridden: 5,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			wantMin: 0.0,
			wantMax: 0.0, // 0/5 = 0.0
		},
		{
			name: "mixed feedback — 60% positive",
			stats: models.BehaviorStats{
				TimesActivated:  10,
				TimesFollowed:   3,
				TimesOverridden: 2,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			wantMin: 0.6,
			wantMax: 0.6, // 3/5 = 0.6
		},
		{
			name: "confirmed counts as positive",
			stats: models.BehaviorStats{
				TimesActivated:  10,
				TimesFollowed:   1,
				TimesConfirmed:  2,
				TimesOverridden: 2,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
			wantMin: 0.6,
			wantMax: 0.6, // (1+2)/5 = 0.6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			behavior := &models.Behavior{
				ID:         "feedback-test",
				Kind:       models.BehaviorKindDirective,
				Confidence: 0.8,
				Priority:   5,
				Stats:      tt.stats,
			}
			result := scorer.Score(behavior, nil)
			if result.FeedbackScore < tt.wantMin-0.001 || result.FeedbackScore > tt.wantMax+0.001 {
				t.Errorf("FeedbackScore = %f, want in [%f, %f]", result.FeedbackScore, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestExponentialDecay(t *testing.T) {
	halfLife := 7 * 24 * time.Hour

	tests := []struct {
		name    string
		time    time.Time
		wantMin float64
		wantMax float64
	}{
		{
			name:    "zero time",
			time:    time.Time{},
			wantMin: 0,
			wantMax: 0.001,
		},
		{
			name:    "now",
			time:    time.Now(),
			wantMin: 0.99,
			wantMax: 1.0,
		},
		{
			name:    "one half-life ago",
			time:    time.Now().Add(-halfLife),
			wantMin: 0.45,
			wantMax: 0.55,
		},
		{
			name:    "two half-lives ago",
			time:    time.Now().Add(-2 * halfLife),
			wantMin: 0.20,
			wantMax: 0.30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ExponentialDecay(tt.time, halfLife)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("ExponentialDecay() = %f, want in [%f, %f]", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDefaultScorerConfig_Weights(t *testing.T) {
	cfg := DefaultScorerConfig()

	if cfg.ContextWeight != 0.35 {
		t.Errorf("ContextWeight = %f, want 0.35", cfg.ContextWeight)
	}
	if cfg.BaseLevelWeight != 0.30 {
		t.Errorf("BaseLevelWeight = %f, want 0.30", cfg.BaseLevelWeight)
	}
	if cfg.FeedbackWeight != 0.15 {
		t.Errorf("FeedbackWeight = %f, want 0.15", cfg.FeedbackWeight)
	}
	if cfg.PriorityWeight != 0.20 {
		t.Errorf("PriorityWeight = %f, want 0.20", cfg.PriorityWeight)
	}
	if cfg.FeedbackMinSample != 3 {
		t.Errorf("FeedbackMinSample = %d, want 3", cfg.FeedbackMinSample)
	}
}
