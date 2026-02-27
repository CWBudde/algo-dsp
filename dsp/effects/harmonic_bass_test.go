package effects

import (
	"math"
	"testing"
)

func TestNewHarmonicBass(t *testing.T) {
	_, err := NewHarmonicBass(0)
	if err == nil {
		t.Fatal("expected error for zero sample rate")
	}

	b, err := NewHarmonicBass(48000)
	if err != nil {
		t.Fatalf("NewHarmonicBass() error = %v", err)
	}

	if b == nil {
		t.Fatal("NewHarmonicBass() returned nil")
	}
}

func TestHarmonicBassDefaults(t *testing.T) {
	b, err := NewHarmonicBass(48000)
	if err != nil {
		t.Fatalf("NewHarmonicBass() error = %v", err)
	}

	if got := b.Frequency(); got != defaultHarmonicBassFrequency {
		t.Fatalf("Frequency() = %f, want %f", got, defaultHarmonicBassFrequency)
	}

	if got := b.Response(); got != defaultHarmonicBassResponseMs {
		t.Fatalf("Response() = %f, want %f", got, defaultHarmonicBassResponseMs)
	}

	if got := b.Ratio(); got != defaultHarmonicBassRatio {
		t.Fatalf("Ratio() = %f, want %f", got, defaultHarmonicBassRatio)
	}

	if got := b.Decay(); got != defaultHarmonicBassDecay {
		t.Fatalf("Decay() = %f, want %f", got, defaultHarmonicBassDecay)
	}

	if got := b.InputLevel(); got != 1.0 {
		t.Fatalf("InputLevel() = %f, want 1", got)
	}

	if got := b.HighFrequencyLevel(); got != 1.0 {
		t.Fatalf("HighFrequencyLevel() = %f, want 1", got)
	}

	if got := b.OriginalBassLevel(); got != 1.0 {
		t.Fatalf("OriginalBassLevel() = %f, want 1", got)
	}

	if got := b.HarmonicBassLevel(); got != 0.0 {
		t.Fatalf("HarmonicBassLevel() = %f, want 0", got)
	}
}

func TestHarmonicBassSetters(t *testing.T) {
	b, err := NewHarmonicBass(48000)
	if err != nil {
		t.Fatalf("NewHarmonicBass() error = %v", err)
	}

	err = b.SetFrequency(5)
	if err == nil {
		t.Fatal("expected error for frequency below min")
	}

	err = b.SetFrequency(200)
	if err != nil {
		t.Fatalf("SetFrequency() error = %v", err)
	}

	err = b.SetRatio(0)
	if err == nil {
		t.Fatal("expected error for non-positive ratio")
	}

	err = b.SetRatio(2)
	if err != nil {
		t.Fatalf("SetRatio() error = %v", err)
	}

	err = b.SetResponse(math.NaN())
	if err == nil {
		t.Fatal("expected error for NaN response")
	}

	err = b.SetResponse(10)
	if err != nil {
		t.Fatalf("SetResponse() error = %v", err)
	}

	err = b.SetDecay(math.Inf(1))
	if err == nil {
		t.Fatal("expected error for infinite decay")
	}

	err = b.SetDecay(0.25)
	if err != nil {
		t.Fatalf("SetDecay() error = %v", err)
	}
}

func TestHarmonicBassProcessSampleFinite(t *testing.T) {
	b, err := NewHarmonicBass(48000)
	if err != nil {
		t.Fatalf("NewHarmonicBass() error = %v", err)
	}

	err = b.SetHarmonicBassLevel(1)
	if err != nil {
		t.Fatalf("SetHarmonicBassLevel() error = %v", err)
	}

	inputs := []float64{-1, -0.5, 0, 0.5, 1}
	for _, in := range inputs {
		out := b.ProcessSample(in)
		if math.IsNaN(out) || math.IsInf(out, 0) {
			t.Fatalf("ProcessSample(%f) produced non-finite output", in)
		}
	}
}
