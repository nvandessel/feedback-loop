package ranking

import (
	"context"
	"math"
	"testing"
)

const floatEpsilon = 1e-9

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < floatEpsilon
}

// mockUpdater tracks confidence updates for testing.
type mockUpdater struct {
	updates map[string]float64
}

func newMockUpdater() *mockUpdater {
	return &mockUpdater{updates: make(map[string]float64)}
}

func (m *mockUpdater) UpdateConfidence(_ context.Context, id string, conf float64) error {
	m.updates[id] = conf
	return nil
}

func TestApplyReinforcement_Boost(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	activeIDs := map[string]float64{"b1": 0.7}
	allIDs := map[string]float64{"b1": 0.7}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	if !floatEquals(updater.updates["b1"], 0.72) {
		t.Errorf("b1 confidence = %v, want 0.72", updater.updates["b1"])
	}
}

func TestApplyReinforcement_Decay(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	activeIDs := map[string]float64{} // b1 is NOT active
	allIDs := map[string]float64{"b1": 0.7}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	if !floatEquals(updater.updates["b1"], 0.695) {
		t.Errorf("b1 confidence = %v, want 0.695", updater.updates["b1"])
	}
}

func TestApplyReinforcement_Ceiling(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	activeIDs := map[string]float64{"b1": 0.94}
	allIDs := map[string]float64{"b1": 0.94}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	if updater.updates["b1"] != cfg.Ceiling {
		t.Errorf("b1 confidence = %v, want ceiling %v", updater.updates["b1"], cfg.Ceiling)
	}
}

func TestApplyReinforcement_Floor(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	activeIDs := map[string]float64{} // b1 is NOT active
	allIDs := map[string]float64{"b1": 0.6}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	// Already at floor, no update should happen
	if _, updated := updater.updates["b1"]; updated {
		t.Errorf("b1 should not be updated when already at floor")
	}
}

func TestApplyReinforcement_MixedSet(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	activeIDs := map[string]float64{"b1": 0.7, "b3": 0.8}
	allIDs := map[string]float64{"b1": 0.7, "b2": 0.75, "b3": 0.8}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	// b1: active, boosted
	if !floatEquals(updater.updates["b1"], 0.72) {
		t.Errorf("b1 confidence = %v, want 0.72", updater.updates["b1"])
	}

	// b2: inactive, decayed
	if !floatEquals(updater.updates["b2"], 0.745) {
		t.Errorf("b2 confidence = %v, want 0.745", updater.updates["b2"])
	}

	// b3: active, boosted
	if !floatEquals(updater.updates["b3"], 0.82) {
		t.Errorf("b3 confidence = %v, want 0.82", updater.updates["b3"])
	}
}

func TestApplyReinforcement_EmptySets(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	err := ApplyReinforcement(ctx, updater, map[string]float64{}, map[string]float64{}, cfg)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	if len(updater.updates) != 0 {
		t.Errorf("expected no updates for empty sets, got %d", len(updater.updates))
	}
}
