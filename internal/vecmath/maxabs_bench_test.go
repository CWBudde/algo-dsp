package vecmath

import "testing"

func BenchmarkMaxAbs(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			x := make([]float64, tc.size)
			for i := range x {
				sign := 1.0
				if i%2 == 0 {
					sign = -1.0
				}
				x[i] = sign * (float64((i*37)%113) + 0.125)
			}

			b.SetBytes(int64(tc.size * 8))
			b.ResetTimer()

			var result float64
			for i := 0; i < b.N; i++ {
				result = MaxAbs(x)
			}
			_ = result
		})
	}
}

func BenchmarkMaxAbsRef(b *testing.B) {
	for _, tc := range benchSizes {
		b.Run(tc.name, func(b *testing.B) {
			x := make([]float64, tc.size)
			for i := range x {
				sign := 1.0
				if i%2 == 0 {
					sign = -1.0
				}
				x[i] = sign * (float64((i*37)%113) + 0.125)
			}

			b.SetBytes(int64(tc.size * 8))
			b.ResetTimer()

			var result float64
			for i := 0; i < b.N; i++ {
				result = maxAbsRef(x)
			}
			_ = result
		})
	}
}
