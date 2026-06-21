package spatial

import "testing"

func BenchmarkHaasDelayProcessStereo(b *testing.B) {
	h, _ := NewHaasDelay(48000, WithHaasDelayMs(15))
	l, r := 0.5, -0.3

	b.ReportAllocs()

	for b.Loop() {
		l, r = h.ProcessStereo(l, r)
	}

	_, _ = l, r
}

func benchmarkHaasDelayInPlace(b *testing.B, n int) {
	b.Helper()

	h, _ := NewHaasDelay(48000, WithHaasDelayMs(15))
	left := make([]float64, n)
	right := make([]float64, n)

	for i := range left {
		left[i] = 0.5
		right[i] = -0.3
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = h.ProcessStereoInPlace(left, right)
	}
}

func BenchmarkHaasDelayInPlace256(b *testing.B)  { benchmarkHaasDelayInPlace(b, 256) }
func BenchmarkHaasDelayInPlace1024(b *testing.B) { benchmarkHaasDelayInPlace(b, 1024) }
