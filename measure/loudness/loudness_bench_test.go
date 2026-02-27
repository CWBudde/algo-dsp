package loudness

import (
	"fmt"
	"testing"
)

func BenchmarkMeter_ProcessBlock(b *testing.B) {
	sizes := []int{64, 256, 1024}

	channels := []int{1, 2}
	for _, size := range sizes {
		for _, ch := range channels {
			b.Run(fmt.Sprintf("%dx%d", size, ch), func(b *testing.B) {
				meter := NewMeter(WithChannels(ch))
				block := make([]float64, size*ch)
				b.SetBytes(int64(size * ch * 8))
				b.ResetTimer()

				for range b.N {
					meter.ProcessBlock(block)
				}
			})
		}
	}
}
