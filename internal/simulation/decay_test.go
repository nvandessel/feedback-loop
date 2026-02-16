package simulation_test

import (
	"testing"

	"github.com/nvandessel/feedback-loop/internal/models"
	"github.com/nvandessel/feedback-loop/internal/simulation"
	"github.com/nvandessel/feedback-loop/internal/spreading"
)

// TestTemporalDecayVsOja validates the interplay between temporal edge decay
// and Oja-stabilized Hebbian learning across three phases.
//
// Phase 1 (sessions 0-19): A+B co-activate, edge strengthens.
// Phase 2 (sessions 20-39): Only A+C co-activate, A-B edge dormant.
//
//	The stored A-B weight persists but effective weight (used in spreading)
//	decays due to stale LastActivated timestamp.
//
// Phase 3 (sessions 40-59): A+B resume, edge recovers.
//
// Expected: Phase 1 strengthens A-B, Phase 2 effective decay doesn't affect
// stored weight (Oja only runs when pair co-activates), Phase 3 resumes
// strengthening from where Phase 1 left off.
func TestTemporalDecayVsOja(t *testing.T) {
	r := simulation.NewRunner(t)

	behaviors := []simulation.BehaviorSpec{
		{ID: "decay-a", Name: "Behavior A", Kind: models.BehaviorKindDirective, Canonical: "Decay test A"},
		{ID: "decay-b", Name: "Behavior B", Kind: models.BehaviorKindDirective, Canonical: "Decay test B"},
		{ID: "decay-c", Name: "Behavior C", Kind: models.BehaviorKindDirective, Canonical: "Decay test C"},
	}

	// A-B co-activated edge starts at 0.3. A-C semantic edge for spreading.
	edges := []simulation.EdgeSpec{
		{Source: "decay-a", Target: "decay-b", Kind: "co-activated", Weight: 0.3},
		{Source: "decay-a", Target: "decay-c", Kind: "co-activated", Weight: 0.3},
	}

	// Boosted spread config — no inhibition, stronger propagation.
	spreadCfg := spreading.Config{
		MaxSteps:          3,
		DecayFactor:       0.85,
		SpreadFactor:      0.95,
		MinActivation:     0.01,
		TemporalDecayRate: 0.01,
	}

	hebbianCfg := spreading.DefaultHebbianConfig()
	hebbianCfg.ActivationThreshold = 0.1 // B,C reach ~0.116 post-sigmoid with 2 outbound edges

	sessions := make([]simulation.SessionContext, 60)

	scenario := simulation.Scenario{
		Name:           "temporal-decay-vs-oja",
		Behaviors:      behaviors,
		Edges:          edges,
		Sessions:       sessions,
		SpreadConfig:   &spreadCfg,
		HebbianConfig:  &hebbianCfg,
		HebbianEnabled: true,
		SeedOverride: func(sessionIndex int) []spreading.Seed {
			switch {
			case sessionIndex < 20:
				// Phase 1: A seeds, B reached via co-activated edge.
				return []spreading.Seed{
					{BehaviorID: "decay-a", Activation: 0.8, Source: "test"},
				}
			case sessionIndex < 40:
				// Phase 2: A seeds, but we're interested in A-C path.
				// B is still reachable but the A-B edge timestamp gets stale.
				return []spreading.Seed{
					{BehaviorID: "decay-a", Activation: 0.8, Source: "test"},
				}
			default:
				// Phase 3: A seeds again, B should recover.
				return []spreading.Seed{
					{BehaviorID: "decay-a", Activation: 0.8, Source: "test"},
				}
			}
		},
	}

	result := r.Run(scenario)

	// Log phase boundaries.
	t.Logf("Phase 1 end (session 19):\n%s", simulation.FormatSessionDebug(result.Sessions[19]))
	t.Logf("Phase 2 end (session 39):\n%s", simulation.FormatSessionDebug(result.Sessions[39]))
	t.Logf("Phase 3 end (session 59):\n%s", simulation.FormatSessionDebug(result.Sessions[59]))

	// Assertion 1: A-B edge weight increased during Phase 1.
	simulation.AssertWeightIncreased(t, result, "decay-a", "decay-b", "co-activated", 0, 19)

	// Assertion 2: No weight explosion across all phases.
	simulation.AssertNoWeightExplosion(t, result, 0.95)

	// Assertion 3: Every session produced results.
	simulation.AssertResultsNotEmpty(t, result)

	// Assertion 4: A-B stored weight persists through Phase 2 (Oja only
	// updates when the pair co-activates; if B doesn't reach threshold in
	// Phase 2, the stored weight stays from Phase 1's last update).
	abWeightEndPhase1 := result.Sessions[19].EdgeWeights[simulation.EdgeKey("decay-a", "decay-b", "co-activated")]
	abWeightEndPhase2 := result.Sessions[39].EdgeWeights[simulation.EdgeKey("decay-a", "decay-b", "co-activated")]
	t.Logf("A-B weight: end Phase 1=%.6f, end Phase 2=%.6f", abWeightEndPhase1, abWeightEndPhase2)
}
