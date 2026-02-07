package vecmath

import "testing"

func BenchmarkAddBlock(b *testing.B) {
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
				AddBlock(dst, a, c)
			}
		})
	}
}

func BenchmarkAddBlockRef(b *testing.B) {
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
				addBlockRef(dst, a, c)
			}
		})
	}
}
