package ir

import (
	"testing"
)

func BenchmarkSchroederIntegral(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.SchroederIntegral(ir)
	}
}

func BenchmarkRT60(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.RT60(ir)
	}
}

func BenchmarkAnalyze(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.Analyze(ir)
	}
}

func BenchmarkDefinition(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.Definition(ir, 50)
	}
}

func BenchmarkClarity(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.Clarity(ir, 80)
	}
}

func BenchmarkCenterTime(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.CenterTime(ir)
	}
}

func BenchmarkFindImpulseStart(b *testing.B) {
	ir := makeExponentialDecay(48000, 1.0, 3.0)
	a := NewAnalyzer(48000)

	b.ResetTimer()
	for b.Loop() {
		a.FindImpulseStart(ir)
	}
}
