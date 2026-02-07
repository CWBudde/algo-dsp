package window

import "testing"

func BenchmarkGenerate(b *testing.B) {
	sizes := []int{256, 1024, 4096, 16384}
	for _, n := range sizes {
		b.Run("hann/"+itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Generate(TypeHann, n)
			}
		})
		b.Run("bh4/"+itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Generate(TypeBlackmanHarris4Term, n)
			}
		})
		b.Run("kaiser/"+itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Generate(TypeKaiser, n, WithAlpha(8))
			}
		})
	}
}

func BenchmarkApply(b *testing.B) {
	sizes := []int{256, 1024, 4096, 16384}
	for _, n := range sizes {
		b.Run("hann/"+itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			buf := make([]float64, n)
			for i := 0; i < b.N; i++ {
				Apply(TypeHann, buf)
			}
		})
	}
}

func BenchmarkApplyCoefficientsInPlace(b *testing.B) {
	sizes := []int{256, 1024, 4096, 16384}
	for _, n := range sizes {
		b.Run(itoa(n), func(b *testing.B) {
			coeffs := Generate(TypeHann, n)
			buf := make([]float64, n)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(n * 8))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ApplyCoefficientsInPlace(buf, coeffs)
			}
		})
	}
}

func BenchmarkApplyCoefficients(b *testing.B) {
	sizes := []int{256, 1024, 4096, 16384}
	for _, n := range sizes {
		b.Run(itoa(n), func(b *testing.B) {
			coeffs := Generate(TypeHann, n)
			buf := make([]float64, n)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(n * 8))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = ApplyCoefficients(buf, coeffs)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
