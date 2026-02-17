package ranking

import (
	"time"

	"github.com/nvandessel/feedback-loop/internal/constants"
	"github.com/nvandessel/feedback-loop/internal/models"
)

// ScorerConfig configures the relevance scorer.
//
// The 4-signal model replaces the original 5-signal model:
//   - ContextScore (context match specificity)
//   - BaseLevelScore (ACT-R: frequency + recency in one principled formula)
//   - FeedbackScore (quality ratio: confirmed vs overridden)
//   - PriorityScore (user-assigned priority + kind boost)
type ScorerConfig struct {
	// Weight for context match specificity (0.0-1.0)
	ContextWeight float64

	// Weight for ACT-R base-level activation (frequency + recency) (0.0-1.0)
	BaseLevelWeight float64

	// Weight for feedback quality ratio (confirmed/overridden) (0.0-1.0)
	FeedbackWeight float64

	// Weight for priority and kind (0.0-1.0)
	PriorityWeight float64

	// ACT-R configuration for base-level activation
	ACTR ACTRConfig

	// FeedbackMinSample is the minimum number of feedback signals required
	// before the feedback ratio is used. Below this, FeedbackScore is neutral (0.5).
	FeedbackMinSample int

	// KindBoosts are score multipliers for behavior kinds
	KindBoosts map[models.BehaviorKind]float64
}

// DefaultScorerConfig returns the default scoring configuration.
// Weights: Context 35%, BaseLevel 30%, Feedback 15%, Priority 20%
func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		ContextWeight:     0.35,
		BaseLevelWeight:   0.30,
		FeedbackWeight:    0.15,
		PriorityWeight:    0.20,
		ACTR:              DefaultACTRConfig(),
		FeedbackMinSample: 3,
		KindBoosts: map[models.BehaviorKind]float64{
			models.BehaviorKindConstraint: 2.0, // Constraints are safety-critical
			models.BehaviorKindDirective:  1.5,
			models.BehaviorKindProcedure:  1.2,
			models.BehaviorKindPreference: 1.0,
		},
	}
}

// RelevanceScorer calculates relevance scores for behaviors
type RelevanceScorer struct {
	config ScorerConfig
}

// NewRelevanceScorer creates a new relevance scorer with the given config
func NewRelevanceScorer(config ScorerConfig) *RelevanceScorer {
	// Validate and normalize weights
	totalWeight := config.ContextWeight + config.BaseLevelWeight +
		config.FeedbackWeight + config.PriorityWeight

	if totalWeight > 0 && totalWeight != 1.0 {
		config.ContextWeight /= totalWeight
		config.BaseLevelWeight /= totalWeight
		config.FeedbackWeight /= totalWeight
		config.PriorityWeight /= totalWeight
	}

	if config.FeedbackMinSample <= 0 {
		config.FeedbackMinSample = 3
	}

	if config.KindBoosts == nil {
		config.KindBoosts = DefaultScorerConfig().KindBoosts
	}

	return &RelevanceScorer{config: config}
}

// ScoredBehavior represents a behavior with its calculated relevance score
type ScoredBehavior struct {
	Behavior *models.Behavior
	Score    float64

	// Component scores for debugging/transparency
	ContextScore   float64
	BaseLevelScore float64
	FeedbackScore  float64
	PriorityScore  float64
	KindBoost      float64

	// Deprecated: kept for backward compatibility with tests that reference old fields.
	// These map to new signals: UsageScore→BaseLevelScore, RecencyScore→0, ConfidenceScore→FeedbackScore.
	UsageScore      float64
	RecencyScore    float64
	ConfidenceScore float64
}

// Score calculates the relevance score for a single behavior
func (s *RelevanceScorer) Score(behavior *models.Behavior, ctx *models.ContextSnapshot) ScoredBehavior {
	if behavior == nil {
		return ScoredBehavior{}
	}

	scored := ScoredBehavior{
		Behavior:       behavior,
		ContextScore:   s.contextScore(behavior, ctx),
		BaseLevelScore: s.baseLevelScore(behavior),
		FeedbackScore:  s.feedbackScore(behavior),
		PriorityScore:  s.priorityScore(behavior),
		KindBoost:      s.kindBoost(behavior.Kind),
	}

	// Backward-compat aliases
	scored.UsageScore = scored.BaseLevelScore
	scored.RecencyScore = 0
	scored.ConfidenceScore = scored.FeedbackScore

	// Calculate weighted score
	baseScore := scored.ContextScore*s.config.ContextWeight +
		scored.BaseLevelScore*s.config.BaseLevelWeight +
		scored.FeedbackScore*s.config.FeedbackWeight +
		scored.PriorityScore*s.config.PriorityWeight

	// Apply kind boost
	scored.Score = baseScore * scored.KindBoost

	return scored
}

// ScoreBatch calculates relevance scores for multiple behaviors
func (s *RelevanceScorer) ScoreBatch(behaviors []models.Behavior, ctx *models.ContextSnapshot) []ScoredBehavior {
	results := make([]ScoredBehavior, len(behaviors))
	for i := range behaviors {
		results[i] = s.Score(&behaviors[i], ctx)
	}
	return results
}

// contextScore calculates how specifically the behavior matches the context
func (s *RelevanceScorer) contextScore(behavior *models.Behavior, ctx *models.ContextSnapshot) float64 {
	if behavior.When == nil || len(behavior.When) == 0 {
		return constants.NeutralScore
	}

	matches := 0
	total := len(behavior.When)

	for key := range behavior.When {
		if ctx != nil && s.predicateMatches(key, ctx) {
			matches++
		}
	}

	if total == 0 {
		return constants.NeutralScore
	}

	specificityBonus := float64(total) * constants.ContextSpecificityFactor
	if specificityBonus > constants.MaxContextSpecificityBonus {
		specificityBonus = constants.MaxContextSpecificityBonus
	}

	matchRatio := float64(matches) / float64(total)
	score := matchRatio + specificityBonus

	if score > 1.0 {
		score = 1.0
	}
	return score
}

// predicateMatches checks if a predicate key matches the context
func (s *RelevanceScorer) predicateMatches(key string, ctx *models.ContextSnapshot) bool {
	switch key {
	case "file", "file_path":
		return ctx.FilePath != ""
	case "language":
		return ctx.FileLanguage != ""
	case "task":
		return ctx.Task != ""
	case "environment", "env":
		return ctx.Environment != ""
	case "repo", "repository":
		return ctx.RepoRoot != ""
	default:
		if ctx.Custom != nil {
			_, ok := ctx.Custom[key]
			return ok
		}
		return false
	}
}

// baseLevelScore computes the ACT-R base-level activation score,
// combining frequency (TimesActivated) and recency (age since CreatedAt)
// into a single principled signal.
func (s *RelevanceScorer) baseLevelScore(behavior *models.Behavior) float64 {
	n := behavior.Stats.TimesActivated
	if n <= 0 {
		// New behavior with no activations — give a fair starting score.
		// ACT-R would return near-zero for n=0, but new behaviors shouldn't
		// be penalized before they've had a chance to be used.
		return constants.NeutralScore
	}

	age := time.Since(behavior.Stats.CreatedAt)
	if age <= 0 {
		// CreatedAt is zero or in the future — fall back to neutral
		return constants.NeutralScore
	}

	return BaseLevelScore(n, age, s.config.ACTR)
}

// feedbackScore calculates score based on explicit feedback quality.
// This is the ratio of positive signals (followed + confirmed) to total feedback
// (followed + confirmed + overridden). Requires a minimum sample size to avoid
// noise from sparse data; returns neutral (0.5) below the threshold.
func (s *RelevanceScorer) feedbackScore(behavior *models.Behavior) float64 {
	stats := behavior.Stats
	totalFeedback := stats.TimesFollowed + stats.TimesConfirmed + stats.TimesOverridden

	if totalFeedback < s.config.FeedbackMinSample {
		return constants.NeutralScore
	}

	positiveSignals := stats.TimesFollowed + stats.TimesConfirmed
	ratio := float64(positiveSignals) / float64(totalFeedback)

	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// priorityScore normalizes priority to a 0-1 score
func (s *RelevanceScorer) priorityScore(behavior *models.Behavior) float64 {
	priority := behavior.Priority
	if priority < 0 {
		priority = 0
	}
	if priority > 10 {
		priority = 10
	}
	return float64(priority) / 10.0
}

// kindBoost returns the score multiplier for a behavior kind
func (s *RelevanceScorer) kindBoost(kind models.BehaviorKind) float64 {
	if boost, ok := s.config.KindBoosts[kind]; ok {
		return boost
	}
	return 1.0
}
