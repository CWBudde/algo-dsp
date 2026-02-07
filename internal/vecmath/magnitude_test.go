package vecmath

import (
	"math"
	"testing"
)

func TestMagnitude(t *testing.T) {
	tests := []struct {
		name string
		re   []float64
		im   []float64
		want []float64
	}{
		{
			name: "simple values",
			re:   []float64{3, 4, 0, 1},
			im:   []float64{4, 3, 1, 0},
			want: []float64{5, 5, 1, 1},
		},
		{
			name: "zeros",
			re:   []float64{0, 0, 0, 0},
			im:   []float64{0, 0, 0, 0},
			want: []float64{0, 0, 0, 0},
		},
		{
			name: "negative values",
			re:   []float64{-3, -4, 5, -6},
			im:   []float64{-4, 3, -12, 8},
			want: []float64{5, 5, 13, 10},
		},
		{
			name: "unit circle",
			re:   []float64{1, 0, -1, 0},
			im:   []float64{0, 1, 0, -1},
			want: []float64{1, 1, 1, 1},
		},
		{
			name: "size 1",
			re:   []float64{3},
			im:   []float64{4},
			want: []float64{5},
		},
		{
			name: "size 2",
			re:   []float64{3, 4},
			im:   []float64{4, 3},
			want: []float64{5, 5},
		},
		{
			name: "size 3",
			re:   []float64{3, 4, 0},
			im:   []float64{4, 3, 1},
			want: []float64{5, 5, 1},
		},
		{
			name: "size 5 (triggers both SIMD and scalar paths)",
			re:   []float64{3, 4, 0, 5, 12},
			im:   []float64{4, 3, 1, 12, 5},
			want: []float64{5, 5, 1, 13, 13},
		},
		{
			name: "size 7",
			re:   []float64{3, 4, 0, 5, 12, 8, 15},
			im:   []float64{4, 3, 1, 12, 5, 15, 8},
			want: []float64{5, 5, 1, 13, 13, 17, 17},
		},
		{
			name: "large values",
			re:   []float64{1e10, 2e10, 3e10, 4e10},
			im:   []float64{2e10, 1e10, 4e10, 3e10},
			want: []float64{math.Sqrt(5e20), math.Sqrt(5e20), 5e10, 5e10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]float64, len(tt.want))
			Magnitude(dst, tt.re, tt.im)

			for i := range dst {
				if !floatEqual(dst[i], tt.want[i], 1e-9) {
					t.Errorf("Magnitude()[%d] = %v, want %v", i, dst[i], tt.want[i])
				}
			}
		})
	}
}

func TestMagnitudePanic(t *testing.T) {
	tests := []struct {
		name string
		dst  []float64
		re   []float64
		im   []float64
	}{
		{
			name: "dst length mismatch",
			dst:  make([]float64, 3),
			re:   make([]float64, 4),
			im:   make([]float64, 4),
		},
		{
			name: "re length mismatch",
			dst:  make([]float64, 4),
			re:   make([]float64, 3),
			im:   make([]float64, 4),
		},
		{
			name: "im length mismatch",
			dst:  make([]float64, 4),
			re:   make([]float64, 4),
			im:   make([]float64, 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("Expected panic, got none")
				}
			}()
			Magnitude(tt.dst, tt.re, tt.im)
		})
	}
}

func TestMagnitudeEmpty(t *testing.T) {
	dst := []float64{}
	re := []float64{}
	im := []float64{}
	Magnitude(dst, re, im)
	if len(dst) != 0 {
		t.Errorf("Expected empty result, got length %d", len(dst))
	}
}

// floatEqual checks if two float64 values are equal within a tolerance
func floatEqual(a, b, tolerance float64) bool {
	diff := math.Abs(a - b)
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	if math.IsInf(a, 0) && math.IsInf(b, 0) {
		return a == b
	}
	return diff <= tolerance
}
