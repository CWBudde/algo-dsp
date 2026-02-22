package fir

import (
	"fmt"
	"testing"
)

// processBlockBaseline is the old implementation for comparison.
func (f *Filter) processBlockBaseline(buf []float64) {
	for i, x := range buf {
		buf[i] = f.ProcessSample(x)
	}
}

func BenchmarkProcessSample(b *testing.B) {
	for _, taps := range []int{8, 32, 128, 512} {
		b.Run(fmt.Sprintf("taps=%d", taps), func(b *testing.B) {
			coeffs := make([]float64, taps)
			for i := range coeffs {
				coeffs[i] = 1.0 / float64(taps)
			}

			f := New(coeffs)

			x := 1.0
			for b.Loop() {
				x = f.ProcessSample(x)
			}

			_ = x
		})
	}
}

func BenchmarkProcessBlock(b *testing.B) {
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
