package generic

import (
	"fmt"
	"testing"
)

func TestMaxAbs_Generic(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		expected float64
	}{
		{"empty", []float64{}, 0},
		{"single positive", []float64{3.5}, 3.5},
		{"single negative", []float64{-4.2}, 4.2},
		{"all positive", []float64{1, 2, 3, 4, 5}, 5},
		{"all negative", []float64{-1, -2, -3, -4, -5}, 5},
		{"mixed", []float64{-1.5, 2.0, -3.5, 4.0, -5.5}, 5.5},
		{"zeros", []float64{0, 0, 0}, 0},
		{"with zero", []float64{-3, 0, 2}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxAbs(tt.input)
			if result != tt.expected {
				t.Errorf("MaxAbs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Note: NaN and Inf handling is implementation-defined.
// The generic implementation skips NaN in comparisons (standard Go float behavior)

func BenchmarkMaxAbs_Generic(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			x := make([]float64, n)
			for i := 0; i < n; i++ {
				x[i] = float64(i) - float64(n)/2
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				MaxAbs(x)
			}

			b.SetBytes(int64(n) * 8)
		})
	}
}
