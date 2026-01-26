package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/store"
)

// LearningResult represents the result of processing a correction.
type LearningResult struct {
	// Correction is the original correction that was processed
	Correction models.Correction

	// CandidateBehavior is the behavior extracted from the correction
	CandidateBehavior models.Behavior

	// Placement describes where the behavior was/should be placed
	Placement PlacementDecision

	// AutoAccepted indicates whether the behavior was automatically accepted
	AutoAccepted bool

	// RequiresReview indicates whether human review is needed
	RequiresReview bool

	// ReviewReasons explains why review is required
	ReviewReasons []string
}

// LearningLoop orchestrates the correction -> behavior pipeline.
// It coordinates CorrectionCapture, BehaviorExtractor, and GraphPlacer
// to process corrections and produce learned behaviors.
type LearningLoop interface {
	// ProcessCorrection processes a single correction into a candidate behavior.
	// It extracts a behavior, determines graph placement, and optionally
	// auto-accepts the behavior if confidence is high enough.
	ProcessCorrection(ctx context.Context, correction models.Correction) (*LearningResult, error)

	// ApprovePending approves a pending behavior, updating its provenance.
	ApprovePending(ctx context.Context, behaviorID, approver string) error

	// RejectPending rejects a pending behavior with a reason.
	RejectPending(ctx context.Context, behaviorID, rejector, reason string) error
}

// LearningLoopConfig holds configuration for the learning loop.
type LearningLoopConfig struct {
	// AutoAcceptThreshold is the minimum confidence for auto-accepting behaviors.
	// Behaviors with confidence >= this threshold and no review flags are auto-accepted.
	// Default: 0.8
	AutoAcceptThreshold float64
}

// DefaultLearningLoopConfig returns sensible defaults for the learning loop.
func DefaultLearningLoopConfig() LearningLoopConfig {
	return LearningLoopConfig{
		AutoAcceptThreshold: 0.8,
	}
}

// NewLearningLoop creates a new learning loop with the given store and config.
// If config is nil, default configuration is used.
func NewLearningLoop(s store.GraphStore, config *LearningLoopConfig) LearningLoop {
	cfg := DefaultLearningLoopConfig()
	if config != nil {
		cfg = *config
	}
	return &learningLoop{
		store:               s,
		capturer:            NewCorrectionCapture(),
		extractor:           NewBehaviorExtractor(),
		placer:              NewGraphPlacer(s),
		autoAcceptThreshold: cfg.AutoAcceptThreshold,
	}
}

// learningLoop is the concrete implementation of LearningLoop.
type learningLoop struct {
	store               store.GraphStore
	capturer            CorrectionCapture
	extractor           BehaviorExtractor
	placer              GraphPlacer
	autoAcceptThreshold float64
}

// ProcessCorrection implements LearningLoop.
func (l *learningLoop) ProcessCorrection(ctx context.Context, correction models.Correction) (*LearningResult, error) {
	// Step 1: Extract candidate behavior
	candidate, err := l.extractor.Extract(correction)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	// Step 2: Determine graph placement
	placement, err := l.placer.Place(ctx, candidate)
	if err != nil {
		return nil, fmt.Errorf("placement failed: %w", err)
	}

	// Step 3: Decide if auto-accept or needs review
	requiresReview, reasons := l.needsReview(candidate, placement)
	autoAccepted := !requiresReview && placement.Confidence >= l.autoAcceptThreshold

	// Step 4: If auto-accepted, commit to graph
	if autoAccepted {
		if err := l.commitBehavior(ctx, candidate, placement); err != nil {
			return nil, fmt.Errorf("commit failed: %w", err)
		}
	}

	return &LearningResult{
		Correction:        correction,
		CandidateBehavior: *candidate,
		Placement:         *placement,
		AutoAccepted:      autoAccepted,
		RequiresReview:    requiresReview,
		ReviewReasons:     reasons,
	}, nil
}

// needsReview determines if human review is required.
func (l *learningLoop) needsReview(candidate *models.Behavior, placement *PlacementDecision) (bool, []string) {
	var reasons []string

	// Constraints always need review
	if candidate.Kind == models.BehaviorKindConstraint {
		reasons = append(reasons, "Constraints require human review")
	}

	// Merging into existing behavior needs review
	if placement.Action == "merge" {
		reasons = append(reasons, fmt.Sprintf("Would merge into existing behavior: %s", placement.TargetID))
	}

	// Conflicts need review
	if len(candidate.Conflicts) > 0 {
		reasons = append(reasons, fmt.Sprintf("Conflicts with: %v", candidate.Conflicts))
	}

	// Low confidence placements need review
	if placement.Confidence < 0.6 {
		reasons = append(reasons, fmt.Sprintf("Low placement confidence: %.2f", placement.Confidence))
	}

	// High similarity to existing might be duplicate
	for _, sim := range placement.SimilarBehaviors {
		if sim.Score > 0.85 {
			reasons = append(reasons, fmt.Sprintf("Very similar to existing: %s (%.2f)", sim.ID, sim.Score))
		}
	}

	return len(reasons) > 0, reasons
}

// commitBehavior saves the behavior to the graph.
func (l *learningLoop) commitBehavior(ctx context.Context, behavior *models.Behavior, placement *PlacementDecision) error {
	// Convert behavior to node
	node := store.Node{
		ID:   behavior.ID,
		Kind: "behavior",
		Content: map[string]interface{}{
			"name":       behavior.Name,
			"kind":       string(behavior.Kind),
			"when":       behavior.When,
			"content":    behavior.Content,
			"provenance": behavior.Provenance,
			"requires":   behavior.Requires,
			"overrides":  behavior.Overrides,
			"conflicts":  behavior.Conflicts,
		},
		Metadata: map[string]interface{}{
			"confidence": behavior.Confidence,
			"priority":   behavior.Priority,
			"stats":      behavior.Stats,
		},
	}

	// Add the node
	if _, err := l.store.AddNode(ctx, node); err != nil {
		return err
	}

	// Add edges
	for _, e := range placement.ProposedEdges {
		edge := store.Edge{
			Source: e.From,
			Target: e.To,
			Kind:   e.Kind,
		}
		if err := l.store.AddEdge(ctx, edge); err != nil {
			return err
		}
	}

	return l.store.Sync(ctx)
}

// ApprovePending implements LearningLoop.
func (l *learningLoop) ApprovePending(ctx context.Context, behaviorID, approver string) error {
	node, err := l.store.GetNode(ctx, behaviorID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("behavior not found: %s", behaviorID)
	}

	// Update provenance
	if prov, ok := node.Content["provenance"].(map[string]interface{}); ok {
		now := time.Now()
		prov["approved_by"] = approver
		prov["approved_at"] = now
	}

	// Increase confidence
	if conf, ok := node.Metadata["confidence"].(float64); ok {
		node.Metadata["confidence"] = minFloat(1.0, conf+0.2)
	}

	return l.store.UpdateNode(ctx, *node)
}

// RejectPending implements LearningLoop.
func (l *learningLoop) RejectPending(ctx context.Context, behaviorID, rejector, reason string) error {
	node, err := l.store.GetNode(ctx, behaviorID)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("behavior not found: %s", behaviorID)
	}

	node.Kind = "rejected-behavior"
	node.Metadata["rejected_by"] = rejector
	node.Metadata["rejected_at"] = time.Now()
	node.Metadata["rejection_reason"] = reason

	return l.store.UpdateNode(ctx, *node)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
