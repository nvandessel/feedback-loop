package ranking

import (
	"math"
	"time"
)

// ACTRConfig configures the ACT-R base-level activation calculation.
type ACTRConfig struct {
	// Decay is the power-law decay parameter (d in the ACT-R formula).
	// Default: 0.5 (standard ACT-R value).
	Decay float64

	// SigmoidOffset shifts the sigmoid center for normalization.
	// Default: 1.0 (centers sigmoid at B_i = -1).
	SigmoidOffset float64
}

// DefaultACTRConfig returns the standard ACT-R parameters.
func DefaultACTRConfig() ACTRConfig {
	return ACTRConfig{
		Decay:         0.5,
		SigmoidOffset: 1.0,
	}
}

// BaseLevelActivation computes the ACT-R approximate base-level activation.
//
// Formula: B_i = ln(n * L^(-d) / (1-d))
//
// Where:
//   - n = number of activations (practice count)
//   - L = age in hours since creation
//   - d = decay parameter (typically 0.5)
//
// Returns a raw activation value (typically in range [-4, +2]).
// For n=0 or L<=0, returns a large negative value (effectively zero after normalization).
func BaseLevelActivation(n int, age time.Duration, d float64) float64 {
	if n <= 0 || age <= 0 {
		return -10.0 // Effectively zero after sigmoid normalization
	}

	hours := age.Hours()
	if hours < 0.001 {
		hours = 0.001 // Floor to avoid log(inf)
	}

	// B_i = ln(n * L^(-d) / (1-d))
	nf := float64(n)
	numerator := nf * math.Pow(hours, -d)
	denominator := 1.0 - d

	if denominator <= 0 {
		return -10.0
	}

	ratio := numerator / denominator
	if ratio <= 0 {
		return -10.0
	}

	return math.Log(ratio)
}

// NormalizeActivation maps a raw ACT-R activation to [0, 1] via shifted sigmoid.
//
// Formula: 1 / (1 + e^(-(B_i + offset)))
//
// The offset (default 1.0) centers the sigmoid at B_i = -1,
// mapping the practical range [-4, +2] evenly across [0, 1].
func NormalizeActivation(activation float64, offset float64) float64 {
	return 1.0 / (1.0 + math.Exp(-(activation + offset)))
}

// BaseLevelScore computes the normalized ACT-R base-level activation score.
// This combines frequency (n activations) and recency (age since creation)
// into a single [0, 1] score.
func BaseLevelScore(n int, age time.Duration, cfg ACTRConfig) float64 {
	raw := BaseLevelActivation(n, age, cfg.Decay)
	return NormalizeActivation(raw, cfg.SigmoidOffset)
}
