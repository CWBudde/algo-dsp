//go:build amd64 && !purego

package avx2

import (
	"fmt"
	"testing"
)

func TestAddMulBlock_AVX2(t *testing.T) {
	sizes := []int{0, 1, 4, 8, 16, 32, 64, 100}
	scalars := []float64{0, 1, -1, 2.0}

	for _, n := range sizes {
		for _, scalar := range scalars {
			t.Run(fmt.Sprintf("n=%d_scale=%.2f", n, scalar), func(t *testing.T) {
				a := make([]float64, n)
				b := make([]float64, n)
				dst := make([]float64, n)
				expected := make([]float64, n)

				for i := 0; i < n; i++ {
					a[i] = float64(i) + 1.0
					b[i] = float64(i) * 2.0
					expected[i] = (a[i] + b[i]) * scalar
				}

				AddMulBlock(dst, a, b, scalar)

				for i := 0; i < n; i++ {
					if dst[i] != expected[i] {
						t.Errorf("AddMulBlock[%d] = %v, want %v", i, dst[i], expected[i])
					}
				}
			})
		}
	}
}

func TestMulAddBlock_AVX2(t *testing.T) {
	sizes := []int{0, 1, 4, 8, 16, 32, 64, 100}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			c := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := 0; i < n; i++ {
				a[i] = float64(i) + 1.0
				b[i] = float64(i) * 2.0
				c[i] = float64(i) * 3.0
				expected[i] = a[i]*b[i] + c[i]
			}

			MulAddBlock(dst, a, b, c)

			for i := 0; i < n; i++ {
				if dst[i] != expected[i] {
					t.Errorf("MulAddBlock[%d] = %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func BenchmarkAddMulBlock_AVX2(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}
	scalar := 2.5

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dst := make([]float64, n)
			a := make([]float64, n)
			src := make([]float64, n)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				AddMulBlock(dst, a, src, scalar)
			}

			bytes := int64(n) * 8 * 3
			b.SetBytes(bytes)
		})
	}
}
