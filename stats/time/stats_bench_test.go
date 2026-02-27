//nolint:revive
package time

import (
	"math"
	"testing"
)

func makeBenchSignal(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.Sin(2 * math.Pi * float64(i) / float64(n))
	}

	return out
}

func BenchmarkCalculate(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096, 16384, 65536}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))

			for range b.N {
				Calculate(signal)
			}
		})
	}
}

func BenchmarkRMS(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096, 16384, 65536}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))

			for range b.N {
				RMS(signal)
			}
		})
	}
}

func BenchmarkStreamingUpdate(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096, 16384, 65536}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))

			ss := NewStreamingStats()
			for range b.N {
				ss.Reset()
				ss.Update(signal)
			}
		})
	}
}
