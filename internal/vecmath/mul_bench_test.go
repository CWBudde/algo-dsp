package vecmath

import "testing"

func BenchmarkMulBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3)) // 3 arrays accessed
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				MulBlock(dst, a, c)
			}
		})
	}
}

func BenchmarkMulBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mulBlockRef(dst, a, c)
			}
		})
	}
}

func BenchmarkMulBlockInPlace(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset dst for fair comparison
				for j := range dst {
					dst[j] = float64(j) * 0.1
				}
				MulBlockInPlace(dst, src)
			}
		})
	}
}
