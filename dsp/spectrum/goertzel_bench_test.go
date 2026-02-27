package spectrum

import (
	"fmt"
	"strconv"
	"testing"
)

func BenchmarkGoertzel_ProcessBlock(b *testing.B) {
	sizes := []int{64, 256, 1024, 4096}
	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			g, _ := NewGoertzel(1000, 48000)

			sig := make([]float64, size)
			for i := range sig {
				sig[i] = float64(i) / float64(size)
			}

			b.SetBytes(int64(size * 8))
			b.ResetTimer()

			for range b.N {
				g.ProcessBlock(sig)
			}
		})
	}
}

func BenchmarkMultiGoertzel_ProcessBlock(b *testing.B) {
	sizes := []int{64, 256, 1024}

	numBins := []int{8, 16, 32}
	for _, size := range sizes {
		for _, bins := range numBins {
			b.Run(fmt.Sprintf("%dx%d", size, bins), func(b *testing.B) {
				freqs := make([]float64, bins)
				for i := range freqs {
					freqs[i] = float64(i+1) * 100
				}

				mg, _ := NewMultiGoertzel(freqs, 48000)
				sig := make([]float64, size)
				b.SetBytes(int64(size * 8))
				b.ResetTimer()

				for range b.N {
					mg.ProcessBlock(sig)
				}
			})
		}
	}
}
