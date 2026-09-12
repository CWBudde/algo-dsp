package time

import "testing"

// BenchmarkPeakScalarReference measures the loop Peak used before vecmath, in the
// same binary and build as BenchmarkPeak, to locate the crossover where the
// vector kernel starts to pay for its call overhead.
func BenchmarkPeakScalarReference(b *testing.B) {
	sizes := []int{8, 16, 24, 32, 48, 64, 96, 128, 192, 256, 512, 1024}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			var sink float64
			for i := 0; i < b.N; i++ {
				sink = peakReference(signal)
			}

			_ = sink
		})
	}
}

func BenchmarkPeakVecmath(b *testing.B) {
	sizes := []int{8, 16, 24, 32, 48, 64, 96, 128, 192, 256, 512, 1024}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			var sink float64
			for i := 0; i < b.N; i++ {
				sink = Peak(signal)
			}

			_ = sink
		})
	}
}
