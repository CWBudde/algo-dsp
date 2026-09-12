package time

import "testing"

func BenchmarkPeak(b *testing.B) {
	sizes := []int{16, 64, 256, 1024, 4096, 65536}
	for _, n := range sizes {
		signal := makeBenchSignal(n)
		b.Run(itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))

			var sink float64

			for i := 0; i < b.N; i++ {
				sink = Peak(signal)
			}

			_ = sink
		})
	}
}
