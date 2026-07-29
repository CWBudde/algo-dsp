package spatial

import "testing"

func BenchmarkStereoPannerProcessMono(b *testing.B) {
	p, _ := NewStereoPanner(48000, WithPanPosition(0.3))
	sample := 0.5

	var outL, outR float64

	b.ReportAllocs()

	for b.Loop() {
		outL, outR = p.ProcessMono(sample)
	}

	_, _ = outL, outR
}

// BenchmarkStereoPannerProcessMonoAutoPan measures the auto-pan path, which
// evaluates the pan law per sample instead of reusing the cached static gains.
func BenchmarkStereoPannerProcessMonoAutoPan(b *testing.B) {
	p, _ := NewStereoPanner(48000, WithAutoPanRate(2))
	sample := 0.5

	var outL, outR float64

	b.ReportAllocs()

	for b.Loop() {
		outL, outR = p.ProcessMono(sample)
	}

	_, _ = outL, outR
}

func BenchmarkStereoPannerProcessStereo(b *testing.B) {
	p, _ := NewStereoPanner(48000, WithPanPosition(0.3))

	const l, r = 0.5, -0.3

	var outL, outR float64

	b.ReportAllocs()

	for b.Loop() {
		outL, outR = p.ProcessStereo(l, r)
	}

	_, _ = outL, outR
}

// benchmarkStereoPannerInPlace measures the buffer path. The panner sits hard
// left, where the balance gains are exactly (1, 0): re-processing the same
// buffer every iteration then leaves the samples unchanged instead of decaying
// them into denormals, which would measure FP fallback cost rather than the
// loop itself.
func benchmarkStereoPannerInPlace(b *testing.B, n int) {
	b.Helper()

	p, _ := NewStereoPanner(48000, WithPanPosition(-1))
	left := make([]float64, n)
	right := make([]float64, n)

	for i := range left {
		left[i] = 0.5
		right[i] = -0.3
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = p.ProcessStereoInPlace(left, right)
	}
}

func BenchmarkStereoPannerInPlace256(b *testing.B)  { benchmarkStereoPannerInPlace(b, 256) }
func BenchmarkStereoPannerInPlace1024(b *testing.B) { benchmarkStereoPannerInPlace(b, 1024) }
