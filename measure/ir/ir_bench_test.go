package ir

import (
	"testing"
)

func BenchmarkSchroederIntegral(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.SchroederIntegral(ir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRT60(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.RT60(ir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyze(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.Analyze(ir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefinition(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.Definition(ir, 50); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClarity(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.Clarity(ir, 80); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCenterTime(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.CenterTime(ir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindImpulseStart(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()

	for b.Loop() {
		if _, err := a.FindImpulseStart(ir); err != nil {
			b.Fatal(err)
		}
	}
}
