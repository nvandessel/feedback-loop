// Package ranking provides scoring and ranking logic for behaviors.
package ranking

import (
	"context"
	"fmt"
)

// ConfidenceReinforcementConfig configures the confidence reinforcement parameters.
type ConfidenceReinforcementConfig struct {
	// BoostAmount is the confidence increase for each activation (+0.02 default).
	BoostAmount float64
	// DecayAmount is the confidence decrease for non-activation (-0.005 default).
	DecayAmount float64
	// Ceiling is the maximum confidence value (0.95 default).
	Ceiling float64
	// Floor is the minimum confidence value (0.6 default).
	Floor float64
}

// DefaultReinforcementConfig returns the default reinforcement configuration.
func DefaultReinforcementConfig() ConfidenceReinforcementConfig {
	return ConfidenceReinforcementConfig{
		BoostAmount: 0.02,
		DecayAmount: 0.005,
		Ceiling:     0.95,
		Floor:       0.6,
	}
}

// ConfidenceUpdater is the interface for updating behavior confidence.
type ConfidenceUpdater interface {
	UpdateConfidence(ctx context.Context, behaviorID string, newConfidence float64) error
}

// ApplyReinforcement adjusts confidence for active and inactive behaviors.
// Active behaviors get a boost; all other behaviors decay slightly.
func ApplyReinforcement(ctx context.Context, updater ConfidenceUpdater, activeIDs map[string]float64, allIDs map[string]float64, cfg ConfidenceReinforcementConfig) error {
	for id, currentConf := range allIDs {
		var newConf float64
		if _, isActive := activeIDs[id]; isActive {
			// Boost active behaviors
			newConf = currentConf + cfg.BoostAmount
			if newConf > cfg.Ceiling {
				newConf = cfg.Ceiling
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
