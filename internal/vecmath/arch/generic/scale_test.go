package generic

import (
	"fmt"
	"testing"
)

func TestScaleBlock_Generic(t *testing.T) {
	sizes := []int{0, 1, 4, 8, 15, 16, 17, 32, 64, 100}
	scalars := []float64{0, 1, -1, 0.5, 2.0, 3.14159}

	for _, n := range sizes {
		for _, scalar := range scalars {
			t.Run(fmt.Sprintf("n=%d_scale=%.2f", n, scalar), func(t *testing.T) {
				src := make([]float64, n)
				dst := make([]float64, n)
				expected := make([]float64, n)

				for i := 0; i < n; i++ {
					src[i] = float64(i) + 0.5
					expected[i] = src[i] * scalar
				}

				ScaleBlock(dst, src, scalar)

				for i := 0; i < n; i++ {
					if dst[i] != expected[i] {
						t.Errorf("ScaleBlock[%d] = %v, want %v", i, dst[i], expected[i])
					}
				}
			})
		}
	}
}

func BenchmarkScaleBlock_Generic(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}
	scalar := 2.5

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dst := make([]float64, n)
			src := make([]float64, n)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ScaleBlock(dst, src, scalar)
			}

			bytes := int64(n) * 8 * 2
			b.SetBytes(bytes)
		})
	}
}
