package vecmath

import (
	"testing"
)

func BenchmarkSum(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096, 16384, 65536}
	for _, size := range sizes {
		x := make([]float64, size)
		for i := range x {
			x[i] = float64(i)
		}

		b.Run(sizeStr(size), func(b *testing.B) {
			b.SetBytes(int64(size * 8))
			for i := 0; i < b.N; i++ {
				_ = Sum(x)
			}
		})
	}
}

func BenchmarkSumGeneric(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096}
	for _, size := range sizes {
		x := make([]float64, size)
		for i := range x {
			x[i] = float64(i)
		}

		b.Run(sizeStr(size), func(b *testing.B) {
			b.SetBytes(int64(size * 8))
			for i := 0; i < b.N; i++ {
				_ = sumRef(x)
			}
		})
	}
}
