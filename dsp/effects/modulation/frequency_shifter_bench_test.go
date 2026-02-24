package modulation

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

func BenchmarkFrequencyShifterProcessSample(b *testing.B) {
	f, err := NewFrequencyShifter(48000,
		WithFrequencyShiftHz(120),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		b.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
		_, _ = f.ProcessSample(x)
	}
}

func BenchmarkFrequencyShifterProcessBlock(b *testing.B) {
	f, err := NewFrequencyShifter(48000,
		WithFrequencyShiftHz(120),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		b.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	const n = 1024
	input := make([]float64, n)
	up := make([]float64, n)
	down := make([]float64, n)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * 700 * float64(i) / 48000)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := f.ProcessBlock(input, up, down); err != nil {
			b.Fatalf("ProcessBlock() error = %v", err)
		}
	}
}
