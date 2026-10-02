package time

import "testing"

func BenchmarkSummary(b *testing.B) {
	for _, length := range []int{64, 256, 1024, 4096, 16384, 65536} {
		signal64 := makeBenchSignal(length)

		signal32 := make([]float32, length)
		for i, value := range signal64 {
			signal32[i] = float32(value)
		}

		b.Run("float32/"+itoa(length), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(length * 4))

			var result SummaryStats
			for range b.N {
				result = Summary(signal32)
			}

			if result.Energy <= 0 {
				b.Fatal("unexpected zero energy")
			}
		})
		b.Run("float64/"+itoa(length), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(length * 8))

			var result SummaryStats
			for range b.N {
				result = Summary(signal64)
			}

			if result.Energy <= 0 {
				b.Fatal("unexpected zero energy")
			}
		})
	}
}
