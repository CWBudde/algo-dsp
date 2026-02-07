package generic

import (
	"fmt"
	"testing"
)

func TestMulBlock_Generic(t *testing.T) {
	sizes := []int{0, 1, 4, 8, 15, 16, 17, 32, 64, 100, 1000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			a := make([]float64, n)
			b := make([]float64, n)
			dst := make([]float64, n)
			expected := make([]float64, n)

			for i := 0; i < n; i++ {
				a[i] = float64(i) + 1.0
				b[i] = float64(i) * 2.0
				expected[i] = a[i] * b[i]
			}

			MulBlock(dst, a, b)

			for i := 0; i < n; i++ {
				if dst[i] != expected[i] {
					t.Errorf("MulBlock[%d] = %v, want %v", i, dst[i], expected[i])
				}
			}
		})
	}
}

func BenchmarkMulBlock_Generic(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			dst := make([]float64, n)
			a := make([]float64, n)
			src := make([]float64, n)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				MulBlock(dst, a, src)
			}

			bytes := int64(n) * 8 * 3
			b.SetBytes(bytes)
		})
	}
}
