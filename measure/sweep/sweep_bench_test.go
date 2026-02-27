package sweep

import (
	"testing"
)

func BenchmarkLogSweepGenerate(b *testing.B) {
	logSweep := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()

	for b.Loop() {
		_, err := logSweep.Generate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepInverseFilter(b *testing.B) {
	logSweep := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()

	for b.Loop() {
		_, err := logSweep.InverseFilter()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepDeconvolve(b *testing.B) {
	logSweep := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}
	sweep, _ := logSweep.Generate()

	b.ResetTimer()

	for b.Loop() {
		_, err := logSweep.Deconvolve(sweep)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepDeconvolve48k(b *testing.B) {
	logSweep := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}
	sweep, _ := logSweep.Generate()

	b.ResetTimer()

	for b.Loop() {
		_, err := logSweep.Deconvolve(sweep)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLinearSweepGenerate(b *testing.B) {
	linearSweep := &LinearSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()

	for b.Loop() {
		_, err := linearSweep.Generate()
		if err != nil {
			b.Fatal(err)
		}
	}
}
