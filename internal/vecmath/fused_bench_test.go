package vecmath

import "testing"

func BenchmarkAddMulBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 0.5

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				AddMulBlock(dst, a, c, scale)
			}
		})
	}
}

func BenchmarkAddMulBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			scale := 0.5

			for i := range a {
				a[i] = float64(i) + 0.5
				c[i] = float64(tc.size-i) * 0.1
			}

			b.SetBytes(int64(tc.size * 8 * 3))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				addMulBlockRef(dst, a, c, scale)
			}
		})
	}
}


func BenchmarkMulAddBlock(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			bslice := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				bslice[i] = float64(tc.size-i) * 0.1
				c[i] = float64(i*2) - 1.0
			}

			b.SetBytes(int64(tc.size * 8 * 4))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				MulAddBlock(dst, a, bslice, c)
			}
		})
	}
}

func BenchmarkMulAddBlockRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			a := make([]float64, tc.size)
			bslice := make([]float64, tc.size)
			c := make([]float64, tc.size)
			dst := make([]float64, tc.size)

			for i := range a {
				a[i] = float64(i) + 0.5
				bslice[i] = float64(tc.size-i) * 0.1
				c[i] = float64(i*2) - 1.0
			}

			b.SetBytes(int64(tc.size * 8 * 4))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				mulAddBlockRef(dst, a, bslice, c)
			}
		})
	}
}
