package dynamics

import "testing"

func BenchmarkCompressorProcessSample(b *testing.B) {
	compressor, _ := NewCompressor(48000)
	sample := 0.5

	for b.Loop() {
		_ = compressor.ProcessSample(sample)
	}
}

func BenchmarkCompressorProcessInPlace64(b *testing.B) {
	compressor, _ := NewCompressor(48000)

	buf := make([]float64, 64)
	for i := range buf {
		buf[i] = 0.5
	}

	for b.Loop() {
		compressor.ProcessInPlace(buf)
	}
}

func BenchmarkCompressorProcessInPlace128(b *testing.B) {
	compressor, _ := NewCompressor(48000)

	buf := make([]float64, 128)
	for i := range buf {
		buf[i] = 0.5
	}

	for b.Loop() {
		compressor.ProcessInPlace(buf)
	}
}

func BenchmarkCompressorProcessInPlace256(b *testing.B) {
	compressor, _ := NewCompressor(48000)

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.5
	}

	for b.Loop() {
		compressor.ProcessInPlace(buf)
	}
}

func BenchmarkCompressorProcessInPlace512(b *testing.B) {
	compressor, _ := NewCompressor(48000)

	buf := make([]float64, 512)
	for i := range buf {
		buf[i] = 0.5
	}

	for b.Loop() {
		compressor.ProcessInPlace(buf)
	}
}

func BenchmarkCompressorProcessInPlace1024(b *testing.B) {
	compressor, _ := NewCompressor(48000)

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = 0.5
	}

	for b.Loop() {
		compressor.ProcessInPlace(buf)
	}
}
