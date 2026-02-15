package ranking

import (
	"math"
	"testing"
	"time"
)

func TestBaseLevelActivation(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		age     time.Duration
		d       float64
		wantMin float64
		wantMax float64
	}{
		{
			name:    "zero activations",
			n:       0,
			age:     24 * time.Hour,
			d:       0.5,
			wantMin: -10.0,
			wantMax: -10.0,
		},
		{
			name:    "negative activations",
			n:       -1,
			age:     24 * time.Hour,
			d:       0.5,
			wantMin: -10.0,
			wantMax: -10.0,
		},
		{
			name:    "zero age",
			n:       10,
			age:     0,
			d:       0.5,
			wantMin: -10.0,
			wantMax: -10.0,
		},
		{
			name: "1 activation, 1 hour old",
			n:    1,
			age:  1 * time.Hour,
			d:    0.5,
			// B = ln(1 * 1^(-0.5) / 0.5) = ln(2) = 0.693
			wantMin: 0.68,
			wantMax: 0.71,
		},
		{
			name: "10 activations, 1 hour old",
			n:    10,
			age:  1 * time.Hour,
			d:    0.5,
			// B = ln(10 * 1^(-0.5) / 0.5) = ln(20) = 2.996
			wantMin: 2.99,
			wantMax: 3.01,
		},
		{
			name: "1 activation, 24 hours old",
			n:    1,
			age:  24 * time.Hour,
			d:    0.5,
			// B = ln(1 * 24^(-0.5) / 0.5) = ln(2/sqrt(24)) = ln(0.4082) = -0.896
			wantMin: -0.90,
			wantMax: -0.88,
		},
		{
			name: "10 activations, 24 hours old",
			n:    10,
			age:  24 * time.Hour,
			d:    0.5,
			// B = ln(10 * 24^(-0.5) / 0.5) = ln(10 * 0.2041 / 0.5) = ln(4.082) = 1.406
			wantMin: 1.40,
			wantMax: 1.42,
		},
		{
			name: "1 activation, 720 hours (30 days)",
			n:    1,
			age:  720 * time.Hour,
			d:    0.5,
			// B = ln(1 * 720^(-0.5) / 0.5) = ln(2/sqrt(720)) = ln(0.0745) = -2.597
			wantMin: -2.60,
			wantMax: -2.58,
		},
		{
			name: "100 activations, 720 hours (30 days)",
			n:    100,
			age:  720 * time.Hour,
			d:    0.5,
			// B = ln(100 * 720^(-0.5) / 0.5) = ln(100 * 0.03727 / 0.5) = ln(7.454) = 2.008
			wantMin: 2.00,
			wantMax: 2.02,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BaseLevelActivation(tt.n, tt.age, tt.d)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("BaseLevelActivation(%d, %v, %f) = %f, want in [%f, %f]",
					tt.n, tt.age, tt.d, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestNormalizeActivation(t *testing.T) {
	tests := []struct {
		name       string
		activation float64
		offset     float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "very negative activation",
			activation: -10.0,
			offset:     1.0,
			wantMin:    0.0,
			wantMax:    0.001, // Near zero
		},
		{
			name:       "at sigmoid center (B_i = -1, offset = 1)",
			activation: -1.0,
			offset:     1.0,
			wantMin:    0.5,
			wantMax:    0.5,
		},
		{
			name:       "moderately positive",
			activation: 1.0,
			offset:     1.0,
			// sigmoid(2) = 1/(1+e^-2) = 0.881
			wantMin: 0.88,
			wantMax: 0.89,
		},
		{
			name:       "highly active",
			activation: 3.0,
			offset:     1.0,
			// sigmoid(4) = 1/(1+e^-4) = 0.982
			wantMin: 0.98,
			wantMax: 0.99,
		},
		{
			name:       "zero activation",
			activation: 0.0,
			offset:     1.0,
			// sigmoid(1) = 1/(1+e^-1) = 0.731
			wantMin: 0.73,
			wantMax: 0.74,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeActivation(tt.activation, tt.offset)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("NormalizeActivation(%f, %f) = %f, want in [%f, %f]",
					tt.activation, tt.offset, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBaseLevelScore(t *testing.T) {
	cfg := DefaultACTRConfig()

	tests := []struct {
		name    string
		n       int
		age     time.Duration
		wantMin float64
		wantMax float64
	}{
		{
			name:    "never activated",
			n:       0,
			age:     24 * time.Hour,
			wantMin: 0.0,
			wantMax: 0.001,
		},
		{
			name: "frequently used, recent",
			n:    50,
			age:  1 * time.Hour,
			// B = ln(50/0.5) = ln(100) = 4.605, sigmoid(5.605) ≈ 0.996
			wantMin: 0.99,
			wantMax: 1.0,
		},
		{
			name: "rarely used, old",
			n:    1,
			age:  720 * time.Hour,
			// B ≈ -2.60, sigmoid(-1.60) ≈ 0.168
			wantMin: 0.16,
			wantMax: 0.18,
		},
		{
			name: "moderately used, moderate age",
			n:    10,
			age:  24 * time.Hour,
			// B ≈ 1.41, sigmoid(2.41) ≈ 0.918
			wantMin: 0.91,
			wantMax: 0.93,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BaseLevelScore(tt.n, tt.age, cfg)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("BaseLevelScore(%d, %v) = %f, want in [%f, %f]",
					tt.n, tt.age, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBaseLevelScore_FrequencyBeatsRecency(t *testing.T) {
	cfg := DefaultACTRConfig()

	// A frequently-used old behavior should score higher than a rarely-used recent one
	frequentOld := BaseLevelScore(50, 720*time.Hour, cfg)  // 50 activations, 30 days old
	rareRecent := BaseLevelScore(1, 1*time.Hour, cfg)       // 1 activation, 1 hour old

	if frequentOld <= rareRecent {
		t.Errorf("frequent+old (%f) should score higher than rare+recent (%f)",
			frequentOld, rareRecent)
	}
}

func TestBaseLevelScore_RecencyMatters(t *testing.T) {
	cfg := DefaultACTRConfig()

	// Same frequency, newer should score higher
	recent := BaseLevelScore(10, 1*time.Hour, cfg)
	old := BaseLevelScore(10, 720*time.Hour, cfg)

	if recent <= old {
		t.Errorf("recent (%f) should score higher than old (%f)", recent, old)
	}
}

func TestBaseLevelScore_Monotonicity(t *testing.T) {
	cfg := DefaultACTRConfig()
	age := 24 * time.Hour

	// Score should increase monotonically with more activations
	var prevScore float64
	for _, n := range []int{1, 2, 5, 10, 20, 50, 100} {
		score := BaseLevelScore(n, age, cfg)
		if score <= prevScore {
			t.Errorf("score at n=%d (%f) should be > score at previous n (%f)",
				n, score, prevScore)
		}
		prevScore = score
	}
}

func TestBaseLevelScore_BoundedOutput(t *testing.T) {
	cfg := DefaultACTRConfig()

	// Test extreme values to ensure output stays in [0, 1]
	extremes := []struct {
		n   int
		age time.Duration
	}{
		{0, 0},
		{1, time.Millisecond},
		{1000000, time.Millisecond},
		{1, 8760 * time.Hour}, // 1 year
		{1000000, 8760 * time.Hour},
	}

	for _, e := range extremes {
		score := BaseLevelScore(e.n, e.age, cfg)
		if score < 0 || score > 1 {
			t.Errorf("BaseLevelScore(%d, %v) = %f, out of [0, 1] range", e.n, e.age, score)
		}
		if math.IsNaN(score) || math.IsInf(score, 0) {
			t.Errorf("BaseLevelScore(%d, %v) = %f, expected finite value", e.n, e.age, score)
		}
	}
}

func TestDefaultACTRConfig(t *testing.T) {
	cfg := DefaultACTRConfig()
	if cfg.Decay != 0.5 {
		t.Errorf("default decay = %f, want 0.5", cfg.Decay)
	}
	if cfg.SigmoidOffset != 1.0 {
		t.Errorf("default sigmoid offset = %f, want 1.0", cfg.SigmoidOffset)
	}
}
