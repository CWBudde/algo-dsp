package dynamics

import (
	"math"
	"testing"
)

func BenchmarkMultibandProcessSample2Band(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{1000}, 4, 48000)
	sample := 0.5

	b.ResetTimer()

	for range b.N {
		_ = mc.ProcessSample(sample)
	}
}

func BenchmarkMultibandProcessSample3Band(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)
	sample := 0.5

	b.ResetTimer()

	for range b.N {
		_ = mc.ProcessSample(sample)
	}
}

func BenchmarkMultibandProcessSample4Band(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{200, 2000, 10000}, 4, 48000)
	sample := 0.5

	b.ResetTimer()

	for range b.N {
		_ = mc.ProcessSample(sample)
	}
}

func BenchmarkMultibandProcessSample3BandLR8(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 8, 48000)
	sample := 0.5

	b.ResetTimer()

	for range b.N {
		_ = mc.ProcessSample(sample)
	}
}

func BenchmarkMultibandProcessInPlace128(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	buf := make([]float64, 128)
	for i := range buf {
		buf[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	b.ResetTimer()

	for range b.N {
		mc.ProcessInPlace(buf)
	}
}

func BenchmarkMultibandProcessInPlace512(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	buf := make([]float64, 512)
	for i := range buf {
		buf[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	b.ResetTimer()

	for range b.N {
		mc.ProcessInPlace(buf)
	}
}

func BenchmarkMultibandProcessInPlace1024(b *testing.B) {
	mc, _ := NewMultibandCompressor([]float64{500, 5000}, 4, 48000)

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}

	b.ResetTimer()

	for range b.N {
		mc.ProcessInPlace(buf)
	}
}
