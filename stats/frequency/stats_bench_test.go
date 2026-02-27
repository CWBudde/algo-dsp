package frequency

import (
	"fmt"
	"math"
	"testing"
)

// makeTestMagnitude creates a deterministic test magnitude spectrum.
func makeTestMagnitude(n int) []float64 {
	mag := make([]float64, n)
	for i := range mag {
		// Create a decaying spectrum with a few harmonics.
		f := float64(i) / float64(n)

		mag[i] = math.Exp(-3*f) + 0.1*math.Sin(2*math.Pi*5*f)
		if mag[i] < 0 {
			mag[i] = -mag[i]
		}
	}

	return mag
}

func BenchmarkCalculate(b *testing.B) {
	fftSizes := []int{64, 256, 1024, 4096, 16384}

	for _, fftSize := range fftSizes {
		n := fftSize/2 + 1
		mag := makeTestMagnitude(n)
		sampleRate := 48000.0

		b.Run(fmt.Sprintf("fft=%d", fftSize), func(b *testing.B) {
			b.SetBytes(int64(n * 8)) // 8 bytes per float64
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = Calculate(mag, sampleRate)
			}
		})
	}
}

func BenchmarkCentroid(b *testing.B) {
	fftSizes := []int{64, 256, 1024, 4096, 16384}

	for _, fftSize := range fftSizes {
		n := fftSize/2 + 1
		mag := makeTestMagnitude(n)
		sampleRate := 48000.0

		b.Run(fmt.Sprintf("fft=%d", fftSize), func(b *testing.B) {
			b.SetBytes(int64(n * 8))
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = Centroid(mag, sampleRate)
			}
		})
	}
}

func BenchmarkFlatness(b *testing.B) {
	fftSizes := []int{64, 256, 1024, 4096, 16384}

	for _, fftSize := range fftSizes {
		n := fftSize/2 + 1
		mag := makeTestMagnitude(n)

		b.Run(fmt.Sprintf("fft=%d", fftSize), func(b *testing.B) {
			b.SetBytes(int64(n * 8))
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = Flatness(mag)
			}
		})
	}
}
