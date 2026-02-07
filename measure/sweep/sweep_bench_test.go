package sweep

import (
	"testing"
)

func BenchmarkLogSweepGenerate(b *testing.B) {
	s := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := s.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepInverseFilter(b *testing.B) {
	s := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := s.InverseFilter(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepDeconvolve(b *testing.B) {
	s := &LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.5,
		SampleRate: 16000,
	}
	sweep, _ := s.Generate()

	b.ResetTimer()
	for b.Loop() {
		if _, err := s.Deconvolve(sweep); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogSweepDeconvolve48k(b *testing.B) {
	s := &LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}
	sweep, _ := s.Generate()

	b.ResetTimer()
	for b.Loop() {
		if _, err := s.Deconvolve(sweep); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLinearSweepGenerate(b *testing.B) {
	s := &LinearSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	b.ResetTimer()
	for b.Loop() {
		if _, err := s.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}
