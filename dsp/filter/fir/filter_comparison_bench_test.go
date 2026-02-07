package fir

import (
	"fmt"
	"testing"
)

// BenchmarkProcessBlock_Baseline benchmarks the old sample-by-sample implementation.
func BenchmarkProcessBlock_Baseline(b *testing.B) {
	for _, taps := range []int{8, 32, 128, 512} {
		b.Run(fmt.Sprintf("taps=%d", taps), func(b *testing.B) {
			coeffs := make([]float64, taps)
			for i := range coeffs {
				coeffs[i] = 1.0 / float64(taps)
			}
			f := New(coeffs)
			buf := make([]float64, 1024)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(1024 * 8)
			b.ResetTimer()
			for range b.N {
				f.processBlockBaseline(buf)
			}
		})
	}
}

// BenchmarkProcessBlock_Optimized benchmarks the new SIMD-optimized implementation.
func BenchmarkProcessBlock_Optimized(b *testing.B) {
	for _, taps := range []int{8, 32, 128, 512} {
		b.Run(fmt.Sprintf("taps=%d", taps), func(b *testing.B) {
			coeffs := make([]float64, taps)
			for i := range coeffs {
				coeffs[i] = 1.0 / float64(taps)
			}
			f := New(coeffs)
			buf := make([]float64, 1024)
			for i := range buf {
				buf[i] = float64(i) * 0.001
			}
			b.SetBytes(1024 * 8)
			b.ResetTimer()
			for range b.N {
				f.ProcessBlock(buf)
			}
		})
	}
}
