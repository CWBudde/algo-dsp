package signal

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

func TestSineLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))
	s, err := g.Sine(1000, 1, 64)
	if err != nil {
		t.Fatalf("Sine() error = %v", err)
	}
	if len(s) != 64 {
		t.Fatalf("len = %d, want 64", len(s))
	}
}

func TestWhiteNoiseDeterministic(t *testing.T) {
	g1 := NewGeneratorWithOptions(nil, WithSeed(42))
	g2 := NewGeneratorWithOptions(nil, WithSeed(42))

	n1, err := g1.WhiteNoise(1, 16)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}
	n2, err := g2.WhiteNoise(1, 16)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	for i := range n1 {
		if n1[i] != n2[i] {
			t.Fatalf("noise mismatch at %d: %v != %v", i, n1[i], n2[i])
		}
	}
}

func TestNormalize(t *testing.T) {
	out, err := Normalize([]float64{-0.5, 1.0, -0.25}, 0.5)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if out[1] != 0.5 {
		t.Fatalf("peak = %v, want 0.5", out[1])
	}
}
