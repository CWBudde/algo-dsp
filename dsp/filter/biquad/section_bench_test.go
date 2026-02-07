package biquad

import (
	"fmt"
	"testing"
)

// benchCoeffs is a realistic lowpass-like biquad for benchmarking.
var benchCoeffs = Coefficients{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04}

func BenchmarkProcessSample(b *testing.B) {
	s := NewSection(benchCoeffs)
	x := 1.0
	for b.Loop() {
		x = s.ProcessSample(x)
	}
	_ = x
}

func BenchmarkProcessBlock(b *testing.B) {
	for _, size := range []int{256, 1024, 4096} {
		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			s := NewSection(benchCoeffs)
			buf := make([]float64, size)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(size * 8))
			b.ResetTimer()
			for range b.N {
				s.ProcessBlock(buf)
			}
		})
	}
}

func BenchmarkProcessBlockScalar(b *testing.B) {
	for _, size := range []int{256, 1024, 4096} {
		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			s := NewSection(benchCoeffs)
			buf := make([]float64, size)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(size * 8))
			b.ResetTimer()
			for range b.N {
				s.processBlockScalar(buf)
			}
		})
	}
}

func BenchmarkProcessBlockTo(b *testing.B) {
	for _, size := range []int{256, 1024, 4096} {
		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			s := NewSection(benchCoeffs)
			src := make([]float64, size)
			dst := make([]float64, size)
			for i := range src {
				src[i] = float64(i) * 0.001
			}
			b.SetBytes(int64(size * 8))
			b.ResetTimer()
			for range b.N {
				s.ProcessBlockTo(dst, src)
			}
		})
	}
}
