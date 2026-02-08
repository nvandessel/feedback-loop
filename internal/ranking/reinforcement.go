// Package ranking provides scoring and ranking logic for behaviors.
package ranking

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConfidenceReinforcementConfig configures the confidence reinforcement parameters.
type ConfidenceReinforcementConfig struct {
	// BoostAmount is the confidence increase for each activation (+0.02 default).
	BoostAmount float64
	// DecayAmount is the confidence decrease for non-activation (-0.005 default).
	DecayAmount float64
	// Ceiling is the maximum confidence value (0.95 default).
	Ceiling float64
	// Floor is the minimum confidence value (0.3 default).
	Floor float64
}

// DefaultReinforcementConfig returns the default reinforcement configuration.
func DefaultReinforcementConfig() ConfidenceReinforcementConfig {
	return ConfidenceReinforcementConfig{
		BoostAmount: 0.02,
		DecayAmount: 0.005,
		Ceiling:     0.95,
		Floor:       0.3,
	}
}

// ConfidenceUpdater is the interface for updating behavior confidence.
type ConfidenceUpdater interface {
	UpdateConfidence(ctx context.Context, behaviorID string, newConfidence float64) error
}

// BoostTracker limits how many confidence boosts a behavior can receive within a time window.
// A nil BoostTracker means no rate limiting (backward-compatible).
type BoostTracker struct {
	mu         sync.Mutex
	maxBoosts  int
	window     time.Duration
	boostTimes map[string][]time.Time // behaviorID -> list of boost timestamps
}

// NewBoostTracker creates a BoostTracker with the given limits.
func NewBoostTracker(maxBoosts int, window time.Duration) *BoostTracker {
	return &BoostTracker{
		maxBoosts:  maxBoosts,
		window:     window,
		boostTimes: make(map[string][]time.Time),
	}
}

// DefaultBoostTracker returns a tracker allowing 3 boosts per hour.
func DefaultBoostTracker() *BoostTracker {
	return NewBoostTracker(3, time.Hour)
}

// AllowBoost checks if a behavior is allowed to be boosted.
// Returns true if under the rate limit, false if rate limited.
// When it returns true, it records the boost.
func (bt *BoostTracker) AllowBoost(behaviorID string) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-bt.window)

	// Filter to only recent boosts within the window
	times := bt.boostTimes[behaviorID]
	recent := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= bt.maxBoosts {
		bt.boostTimes[behaviorID] = recent
		return false
	}

	bt.boostTimes[behaviorID] = append(recent, now)
	return true
}

// ApplyReinforcement adjusts confidence for active and inactive behaviors.
// Active behaviors get a boost; all other behaviors decay slightly.
// If tracker is non-nil, boosts are rate-limited per behavior.
// A nil tracker disables rate limiting (backward-compatible).
func ApplyReinforcement(ctx context.Context, updater ConfidenceUpdater, activeIDs map[string]float64, allIDs map[string]float64, cfg ConfidenceReinforcementConfig, tracker *BoostTracker) error {
	for id, currentConf := range allIDs {
		var newConf float64
		if _, isActive := activeIDs[id]; isActive {
			// Check rate limit before boosting
			if tracker == nil || tracker.AllowBoost(id) {
				newConf = currentConf + cfg.BoostAmount
				if newConf > cfg.Ceiling {
					newConf = cfg.Ceiling
				}
			} else {
				newConf = currentConf // Rate limited, no change
			}
		} else {
			// Decay inactive behaviors
			newConf = currentConf - cfg.DecayAmount
			if newConf < cfg.Floor {
				newConf = cfg.Floor
			}
		}

		// Only update if confidence actually changed
		if newConf != currentConf {
			if err := updater.UpdateConfidence(ctx, id, newConf); err != nil {
				return fmt.Errorf("failed to update confidence for %s: %w", id, err)
			}
		}
	}

	return nil
}
