package vecmath

import "testing"

func BenchmarkPower(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			dst := make([]float64, tc.size)
			re := make([]float64, tc.size)
			im := make([]float64, tc.size)

			// Fill with some test values
			for i := range re {
				re[i] = float64(i%100) / 10.0
				im[i] = float64((i+1)%100) / 10.0
			}

			b.SetBytes(int64(tc.size * 8 * 3)) // 3 slices of float64
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				Power(dst, re, im)
			}
		})
	}
}
