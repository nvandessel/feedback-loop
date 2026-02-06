package ranking

import (
	"math"
	"testing"
	"time"
)

func TestEdgeDecay(t *testing.T) {
	tests := []struct {
		name          string
		weight        float64
		lastActivated time.Time
		rho           float64
		wantMin       float64
		wantMax       float64
	}{
		{
			name:          "just activated",
			weight:        1.0,
			lastActivated: time.Now(),
			rho:           DefaultDecayRate,
			wantMin:       0.99,
			wantMax:       1.0,
		},
		{
			name:          "one day ago",
			weight:        1.0,
			lastActivated: time.Now().Add(-24 * time.Hour),
			rho:           DefaultDecayRate,
			wantMin:       0.78,
			wantMax:       0.79,
		},
		{
			name:          "one week ago",
			weight:        1.0,
			lastActivated: time.Now().Add(-168 * time.Hour),
			rho:           DefaultDecayRate,
			wantMin:       0.18,
			wantMax:       0.19,
		},
		{
			name:          "weighted edge",
			weight:        0.5,
			lastActivated: time.Now(),
			rho:           DefaultDecayRate,
			wantMin:       0.49,
			wantMax:       0.50,
		},
		{
			name:          "zero weight",
			weight:        0.0,
			lastActivated: time.Now(),
			rho:           DefaultDecayRate,
			wantMin:       0.0,
			wantMax:       0.0,
		},
		{
			name:          "zero time returns full weight",
			weight:        0.8,
			lastActivated: time.Time{},
			rho:           DefaultDecayRate,
			wantMin:       0.8,
			wantMax:       0.8,
		},
		{
			name:          "high decay rate",
			weight:        1.0,
			lastActivated: time.Now().Add(-24 * time.Hour),
			rho:           0.1,
			wantMin:       0.09,
			wantMax:       0.10,
		},
		{
			name:          "zero decay rate preserves weight",
			weight:        1.0,
			lastActivated: time.Now().Add(-168 * time.Hour),
			rho:           0.0,
			wantMin:       1.0,
			wantMax:       1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EdgeDecay(tt.weight, tt.lastActivated, tt.rho)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EdgeDecay(%v, %v, %v) = %v, want [%v, %v]",
					tt.weight, tt.lastActivated, tt.rho, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDefaultDecayRate(t *testing.T) {
	// Verify the default decay rate constant matches expected behavior
	// At DefaultDecayRate=0.01, after 1 hour: e^(-0.01*1) ~ 0.99
	oneHourDecay := math.Exp(-DefaultDecayRate * 1)
	if oneHourDecay < 0.989 || oneHourDecay > 0.991 {
		t.Errorf("1-hour decay at DefaultDecayRate = %v, want ~0.99", oneHourDecay)
	}

	// After 24 hours: e^(-0.01*24) ~ 0.786
	oneDayDecay := math.Exp(-DefaultDecayRate * 24)
	if oneDayDecay < 0.785 || oneDayDecay > 0.787 {
		t.Errorf("1-day decay at DefaultDecayRate = %v, want ~0.786", oneDayDecay)
	}
}
