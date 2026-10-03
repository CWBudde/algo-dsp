package features_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

func BenchmarkExtract(b *testing.B) {
	channels := musicFixture(1)
	cfg := features.DefaultConfig()

	b.ReportAllocs()

	for b.Loop() {
		_, err := features.Extract(channels, cfg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractLogSpectrogram(b *testing.B) {
	channels := musicFixture(1)
	cfg := features.DefaultConfig()

	b.ReportAllocs()

	for b.Loop() {
		_, err := features.Extract(channels, cfg, features.WithLogSpectrogram(64, 25, 12000))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogScaleApply(b *testing.B) {
	s, err := features.NewLogScale(64, 25, 12000, SampleRate, FFTSize)
	if err != nil {
		b.Fatal(err)
	}

	power := make([]float64, FFTSize/2+1)
	dst := make([]float64, s.Bins())

	b.ReportAllocs()

	for b.Loop() {
		_ = s.Apply(dst, power)
	}
}

func TestLogScaleApplyZeroAlloc(t *testing.T) {
	s, err := features.NewLogScale(64, 25, 12000, SampleRate, FFTSize)
	if err != nil {
		t.Fatal(err)
	}

	power := make([]float64, FFTSize/2+1)
	dst := make([]float64, s.Bins())

	if allocs := testing.AllocsPerRun(10, func() { _ = s.Apply(dst, power) }); allocs != 0 {
		t.Fatalf("Apply allocates %v times", allocs)
	}
}
