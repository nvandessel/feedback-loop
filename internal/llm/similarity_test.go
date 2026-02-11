package llm

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float64
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 2, 3},
			b:    []float32{1, 2, 3},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{0, 1, 0},
			want: 0.0,
		},
		{
			name: "anti-parallel vectors",
			a:    []float32{1, 0, 0},
			b:    []float32{-1, 0, 0},
			want: -1.0,
		},
		{
			name: "scaled identical",
			a:    []float32{1, 2, 3},
			b:    []float32{2, 4, 6},
			want: 1.0,
		},
		{
			name: "partial similarity",
			a:    []float32{1, 1, 0},
			b:    []float32{1, 0, 0},
			want: 1.0 / math.Sqrt(2),
		},
		{
			name: "zero vector a",
			a:    []float32{0, 0, 0},
			b:    []float32{1, 2, 3},
			want: 0.0,
		},
		{
			name: "zero vector b",
			a:    []float32{1, 2, 3},
			b:    []float32{0, 0, 0},
			want: 0.0,
		},
		{
			name: "both zero vectors",
			a:    []float32{0, 0, 0},
			b:    []float32{0, 0, 0},
			want: 0.0,
		},
		{
			name: "different lengths",
			a:    []float32{1, 2},
			b:    []float32{1, 2, 3},
			want: 0.0,
		},
		{
			name: "empty vectors",
			a:    []float32{},
			b:    []float32{},
			want: 0.0,
		},
		{
			name: "nil vectors",
			a:    nil,
			b:    nil,
			want: 0.0,
		},
		{
			name: "single dimension identical",
			a:    []float32{5},
			b:    []float32{5},
			want: 1.0,
		},
		{
			name: "single dimension opposite",
			a:    []float32{5},
			b:    []float32{-3},
			want: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		vec  []float32
	}{
		{
			name: "unit vector",
			vec:  []float32{1, 0, 0},
		},
		{
			name: "non-unit vector",
			vec:  []float32{3, 4, 0},
		},
		{
			name: "all equal",
			vec:  []float32{1, 1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vec := make([]float32, len(tt.vec))
			copy(vec, tt.vec)
			normalize(vec)

			// Check that the L2 norm is 1.0
			var sum float64
			for _, v := range vec {
				sum += float64(v) * float64(v)
			}
			norm := math.Sqrt(sum)
			if math.Abs(norm-1.0) > 1e-6 {
				t.Errorf("normalize() L2 norm = %v, want 1.0", norm)
			}
		})
	}

	t.Run("zero vector unchanged", func(t *testing.T) {
		vec := []float32{0, 0, 0}
		normalize(vec)
		for i, v := range vec {
			if v != 0 {
				t.Errorf("normalize(zero)[%d] = %v, want 0", i, v)
			}
		}
	})
}
