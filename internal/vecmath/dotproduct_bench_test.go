package vecmath

import (
	"testing"
)

func BenchmarkDotProduct(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096, 16384, 65536}
	for _, size := range sizes {
		a := make([]float64, size)
		c := make([]float64, size)
		for i := range a {
			a[i] = float64(i)
			c[i] = float64(i) * 0.5
		}

		b.Run(sizeStr(size), func(b *testing.B) {
			b.SetBytes(int64(size * 8 * 2)) // Two slices read
			for i := 0; i < b.N; i++ {
				_ = DotProduct(a, c)
			}
		})
	}
}

func BenchmarkDotProductGeneric(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}
	for _, size := range sizes {
		a := make([]float64, size)
		c := make([]float64, size)
		for i := range a {
			a[i] = float64(i)
			c[i] = float64(i) * 0.5
		}

		b.Run(sizeStr(size), func(b *testing.B) {
			b.SetBytes(int64(size * 8 * 2)) // Two slices read
			for i := 0; i < b.N; i++ {
				_ = dotProductRef(a, c)
			}
		})
	}
}
