package time

import "testing"

func BenchmarkNearestZeroCrossing(b *testing.B) {
	for _, length := range []int{64, 1024, 4096} {
		signal64 := makeBenchSignal(length)

		signal32 := make([]float32, length)
		for i, value := range signal64 {
			signal32[i] = float32(value)
		}

		b.Run("float32/"+itoa(length), func(b *testing.B) {
			b.ReportAllocs()

			var found bool
			for range b.N {
				_, found = NearestZeroCrossing(signal32, length/2, length)
			}

			if !found {
				b.Fatal("expected a crossing")
			}
		})
		b.Run("float64/"+itoa(length), func(b *testing.B) {
			b.ReportAllocs()

			var found bool
			for range b.N {
				_, found = NearestZeroCrossing(signal64, length/2, length)
			}

			if !found {
				b.Fatal("expected a crossing")
			}
		})
	}
}
