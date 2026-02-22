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
		if _, err := logSweep.Generate(); err != nil {
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
		if _, err := logSweep.InverseFilter(); err != nil {
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
		if _, err := logSweep.Deconvolve(sweep); err != nil {
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
		if _, err := logSweep.Deconvolve(sweep); err != nil {
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
		if _, err := linearSweep.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}
