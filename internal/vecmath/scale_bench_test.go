package vecmath

import "testing"

func BenchmarkScaleBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 1.5

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ScaleBlock(dst, src, scale)
			}
		})
	}
}

func BenchmarkScaleBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			src := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 1.5

			for i := range src {
				src[i] = float64(i) + 0.5
			}

			b.SetBytes(int64(tc.size * 8 * 2))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				scaleBlockRef(dst, src, scale)
			}
		})
	}
}

func BenchmarkScaleBlockInPlace(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			dst := make([]float64, tc.size)
			scale := 1.5

			b.SetBytes(int64(tc.size * 8))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset for fair comparison
				for j := range dst {
					dst[j] = float64(j) + 0.5
				}
				ScaleBlockInPlace(dst, scale)
			}
		})
	}
}
