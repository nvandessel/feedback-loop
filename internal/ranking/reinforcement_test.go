package ranking

import (
	"context"
	"math"
	"testing"
	"time"
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

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
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

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
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

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
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
	allIDs := map[string]float64{"b1": 0.3}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
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

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
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

	err := ApplyReinforcement(ctx, updater, map[string]float64{}, map[string]float64{}, cfg, nil)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	if len(updater.updates) != 0 {
		t.Errorf("expected no updates for empty sets, got %d", len(updater.updates))
	}
}

func TestDefaultReinforcementConfig_FloorIs03(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	if cfg.Floor != 0.3 {
		t.Errorf("DefaultReinforcementConfig().Floor = %v, want 0.3", cfg.Floor)
	}
}

func TestApplyReinforcement_DecayBelowOldFloor(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	updater := newMockUpdater()
	ctx := context.Background()

	// Start at 0.5, which is below the old floor of 0.6 but above the new floor of 0.3
	activeIDs := map[string]float64{}
	allIDs := map[string]float64{"b1": 0.5}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	// Should decay to 0.495 (below old 0.6 floor)
	if !floatEquals(updater.updates["b1"], 0.495) {
		t.Errorf("b1 confidence = %v, want 0.495", updater.updates["b1"])
	}
}

func TestBoostTracker_AllowBoost(t *testing.T) {
	tests := []struct {
		name       string
		maxBoosts  int
		window     time.Duration
		boostCount int
		wantAllow  bool
	}{
		{
			name:       "first boost allowed",
			maxBoosts:  3,
			window:     time.Hour,
			boostCount: 0,
			wantAllow:  true,
		},
		{
			name:       "second boost allowed",
			maxBoosts:  3,
			window:     time.Hour,
			boostCount: 1,
			wantAllow:  true,
		},
		{
			name:       "third boost allowed",
			maxBoosts:  3,
			window:     time.Hour,
			boostCount: 2,
			wantAllow:  true,
		},
		{
			name:       "fourth boost rate limited",
			maxBoosts:  3,
			window:     time.Hour,
			boostCount: 3,
			wantAllow:  false,
		},
		{
			name:       "single boost limit exceeded",
			maxBoosts:  1,
			window:     time.Hour,
			boostCount: 1,
			wantAllow:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := NewBoostTracker(tt.maxBoosts, tt.window)
			// Pre-fill boosts
			for range tt.boostCount {
				bt.AllowBoost("b1")
			}
			got := bt.AllowBoost("b1")
			if got != tt.wantAllow {
				t.Errorf("AllowBoost() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestBoostTracker_IndependentBehaviors(t *testing.T) {
	bt := NewBoostTracker(1, time.Hour)

	// First behavior gets one boost
	if !bt.AllowBoost("b1") {
		t.Error("b1 first boost should be allowed")
	}
	// b1 is now rate limited
	if bt.AllowBoost("b1") {
		t.Error("b1 second boost should be rate limited")
	}
	// b2 is independent, should still be allowed
	if !bt.AllowBoost("b2") {
		t.Error("b2 first boost should be allowed (independent of b1)")
	}
}

func TestBoostTracker_WindowExpiry(t *testing.T) {
	bt := NewBoostTracker(1, 100*time.Millisecond)

	if !bt.AllowBoost("b1") {
		t.Fatal("first boost should be allowed")
	}
	if bt.AllowBoost("b1") {
		t.Fatal("second boost should be rate limited")
	}

	// Wait for the window to expire
	time.Sleep(150 * time.Millisecond)

	// After the window expires, boost should be allowed again
	if !bt.AllowBoost("b1") {
		t.Error("boost should be allowed after window expires")
	}
}

func TestDefaultBoostTracker(t *testing.T) {
	bt := DefaultBoostTracker()
	if bt.maxBoosts != 3 {
		t.Errorf("DefaultBoostTracker().maxBoosts = %d, want 3", bt.maxBoosts)
	}
	if bt.window != time.Hour {
		t.Errorf("DefaultBoostTracker().window = %v, want %v", bt.window, time.Hour)
	}
}

func TestApplyReinforcement_WithTracker_RateLimitsBoosts(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	ctx := context.Background()

	// Tracker that allows only 1 boost per hour
	tracker := NewBoostTracker(1, time.Hour)

	// First call: boost should be applied
	updater1 := newMockUpdater()
	activeIDs := map[string]float64{"b1": 0.7}
	allIDs := map[string]float64{"b1": 0.7}

	err := ApplyReinforcement(ctx, updater1, activeIDs, allIDs, cfg, tracker)
	if err != nil {
		t.Fatalf("first ApplyReinforcement() error = %v", err)
	}
	if !floatEquals(updater1.updates["b1"], 0.72) {
		t.Errorf("first call: b1 confidence = %v, want 0.72", updater1.updates["b1"])
	}

	// Second call: boost should be rate limited, no update
	updater2 := newMockUpdater()
	err = ApplyReinforcement(ctx, updater2, activeIDs, allIDs, cfg, tracker)
	if err != nil {
		t.Fatalf("second ApplyReinforcement() error = %v", err)
	}
	if _, updated := updater2.updates["b1"]; updated {
		t.Errorf("second call: b1 should not be updated when rate limited, got %v", updater2.updates["b1"])
	}
}

func TestApplyReinforcement_NilTracker_NoRateLimit(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	ctx := context.Background()

	activeIDs := map[string]float64{"b1": 0.7}
	allIDs := map[string]float64{"b1": 0.7}

	// Call many times with nil tracker - should always boost
	for i := range 10 {
		updater := newMockUpdater()
		err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, nil)
		if err != nil {
			t.Fatalf("iteration %d: ApplyReinforcement() error = %v", i, err)
		}
		if _, updated := updater.updates["b1"]; !updated {
			t.Errorf("iteration %d: b1 should be updated with nil tracker", i)
		}
	}
}

func TestApplyReinforcement_TrackerDoesNotAffectDecay(t *testing.T) {
	cfg := DefaultReinforcementConfig()
	ctx := context.Background()

	// Tracker with very restrictive limit
	tracker := NewBoostTracker(0, time.Hour)

	updater := newMockUpdater()
	activeIDs := map[string]float64{} // b1 is NOT active
	allIDs := map[string]float64{"b1": 0.7}

	err := ApplyReinforcement(ctx, updater, activeIDs, allIDs, cfg, tracker)
	if err != nil {
		t.Fatalf("ApplyReinforcement() error = %v", err)
	}

	// Decay should still work even with tracker
	if !floatEquals(updater.updates["b1"], 0.695) {
		t.Errorf("b1 confidence = %v, want 0.695 (decay should not be affected by tracker)", updater.updates["b1"])
	}
}
