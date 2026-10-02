package resample

import "testing"

func BenchmarkProcessInto(b *testing.B) {
	for _, quality := range []Quality{QualityFast, QualityBalanced, QualityBest} {
		b.Run([]string{"fast", "balanced", "best"}[quality], func(b *testing.B) {
			r, err := NewRational(160, 147, WithQuality(quality))
			if err != nil {
				b.Fatal(err)
			}

			input := sine(1000, 44100, 1024)
			dst := make([]float64, r.PredictOutputLen(len(input)))

			b.ReportAllocs()
			b.SetBytes(int64(len(input) * 8))
			b.ResetTimer()

			for range b.N {
				if _, err := r.ProcessInto(dst, input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
