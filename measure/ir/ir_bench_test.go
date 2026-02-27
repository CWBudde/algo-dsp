package ir

import (
	"testing"
)

func BenchmarkSchroederIntegral(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.SchroederIntegral(impulseResponse)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRT60(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.RT60(impulseResponse)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyze(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.Analyze(impulseResponse)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefinition(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.Definition(impulseResponse, 50)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClarity(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.Clarity(impulseResponse, 80)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCenterTime(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.CenterTime(impulseResponse)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindImpulseStart(b *testing.B) {
	impulseResponse := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		_, err := a.FindImpulseStart(impulseResponse)
		if err != nil {
			b.Fatal(err)
		}
	}
}
