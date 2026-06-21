package moog

import (
	"fmt"
	"testing"
)

func benchSignal(n int) []float64 {
	buf := make([]float64, n)
	for i := range buf {
		buf[i] = float64(i%17)*0.01 - 0.08
	}

	return buf
}

func BenchmarkProcessSample(b *testing.B) {
	for _, fast := range []bool{false, true} {
		name := "full"
		if fast {
			name = "fast"
		}

		b.Run(name, func(b *testing.B) {
			f, _ := New(1000, 48000, WithFastTanh(fast), WithResonance(2))

			x := 0.5
			for b.Loop() {
				x = f.ProcessSample(x)
			}

			_ = x
		})
	}
}

func BenchmarkProcessInPlace(b *testing.B) {
	for _, fast := range []bool{false, true} {
		for _, size := range []int{256, 1024, 4096} {
			name := fmt.Sprintf("full/N=%d", size)
			if fast {
				name = fmt.Sprintf("fast/N=%d", size)
			}

			b.Run(name, func(b *testing.B) {
				f, _ := New(1000, 48000, WithFastTanh(fast), WithResonance(2))

				buf := benchSignal(size)

				b.SetBytes(int64(size * 8))
				b.ReportAllocs()
				b.ResetTimer()

				for b.Loop() {
					f.ProcessInPlace(buf)
				}
			})
		}
	}
}
