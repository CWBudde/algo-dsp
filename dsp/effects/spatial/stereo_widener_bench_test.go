package spatial

import "testing"

func BenchmarkStereoWidenerProcessStereo(b *testing.B) {
	w, _ := NewStereoWidener(48000, WithWidth(1.5))
	l, r := 0.5, -0.3

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = w.ProcessStereo(l, r)
	}
}

func BenchmarkStereoWidenerProcessStereoBassMono(b *testing.B) {
	w, _ := NewStereoWidener(48000, WithWidth(1.5), WithBassMonoFreq(120))
	l, r := 0.5, -0.3

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = w.ProcessStereo(l, r)
	}
}

func benchmarkStereoWidenerInPlace(b *testing.B, n int) {
	w, _ := NewStereoWidener(48000, WithWidth(1.5))
	left := make([]float64, n)
	right := make([]float64, n)

	for i := range left {
		left[i] = 0.5
		right[i] = -0.3
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = w.ProcessStereoInPlace(left, right)
	}
}

func BenchmarkStereoWidenerInPlace64(b *testing.B)   { benchmarkStereoWidenerInPlace(b, 64) }
func BenchmarkStereoWidenerInPlace128(b *testing.B)  { benchmarkStereoWidenerInPlace(b, 128) }
func BenchmarkStereoWidenerInPlace256(b *testing.B)  { benchmarkStereoWidenerInPlace(b, 256) }
func BenchmarkStereoWidenerInPlace512(b *testing.B)  { benchmarkStereoWidenerInPlace(b, 512) }
func BenchmarkStereoWidenerInPlace1024(b *testing.B) { benchmarkStereoWidenerInPlace(b, 1024) }

func benchmarkStereoWidenerInterleaved(b *testing.B, n int) {
	w, _ := NewStereoWidener(48000, WithWidth(1.5))

	buf := make([]float64, 2*n)
	for i := 0; i < len(buf); i += 2 {
		buf[i] = 0.5
		buf[i+1] = -0.3
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = w.ProcessInterleavedInPlace(buf)
	}
}

func BenchmarkStereoWidenerInterleaved256(b *testing.B)  { benchmarkStereoWidenerInterleaved(b, 256) }
func BenchmarkStereoWidenerInterleaved1024(b *testing.B) { benchmarkStereoWidenerInterleaved(b, 1024) }
