package dynamics

import "testing"

func BenchmarkGateProcessSample(b *testing.B) {
	g, _ := NewGate(48000)
	sample := 0.5

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = g.ProcessSample(sample)
	}
}

func BenchmarkGateProcessInPlace64(b *testing.B) {
	g, _ := NewGate(48000)

	buf := make([]float64, 64)
	for i := range buf {
		buf[i] = 0.5
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g.ProcessInPlace(buf)
	}
}

func BenchmarkGateProcessInPlace128(b *testing.B) {
	g, _ := NewGate(48000)

	buf := make([]float64, 128)
	for i := range buf {
		buf[i] = 0.5
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g.ProcessInPlace(buf)
	}
}

func BenchmarkGateProcessInPlace256(b *testing.B) {
	g, _ := NewGate(48000)

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.5
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g.ProcessInPlace(buf)
	}
}

func BenchmarkGateProcessInPlace512(b *testing.B) {
	g, _ := NewGate(48000)

	buf := make([]float64, 512)
	for i := range buf {
		buf[i] = 0.5
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g.ProcessInPlace(buf)
	}
}

func BenchmarkGateProcessInPlace1024(b *testing.B) {
	g, _ := NewGate(48000)

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = 0.5
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		g.ProcessInPlace(buf)
	}
}
