package signal

import (
	"math"
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

func TestSetSeed(t *testing.T) {
	g := NewGenerator()
	g.SetSeed(99)
	if g.Seed() != 99 {
		t.Fatalf("Seed()=%d, want 99", g.Seed())
	}

	a, err := g.WhiteNoise(1, 8)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}
	g.SetSeed(100)
	b, err := g.WhiteNoise(1, 8)
	if err != nil {
		t.Fatalf("WhiteNoise() error = %v", err)
	}

	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("expected different seeds to produce different noise")
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

func TestMultisineLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))
	out, err := g.Multisine([]float64{1000, 2000}, 1, 64)
	if err != nil {
		t.Fatalf("Multisine() error = %v", err)
	}
	if len(out) != 64 {
		t.Fatalf("len = %d, want 64", len(out))
	}
}

func TestImpulse(t *testing.T) {
	g := NewGenerator()
	out, err := g.Impulse(0.75, 8, 3)
	if err != nil {
		t.Fatalf("Impulse() error = %v", err)
	}
	for i, v := range out {
		want := 0.0
		if i == 3 {
			want = 0.75
		}
		if v != want {
			t.Fatalf("out[%d]=%v, want %v", i, v, want)
		}
	}
}

func TestLinearSweepLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))
	out, err := g.LinearSweep(20, 20000, 1, 128)
	if err != nil {
		t.Fatalf("LinearSweep() error = %v", err)
	}
	if len(out) != 128 {
		t.Fatalf("len = %d, want 128", len(out))
	}
}

func TestLogSweepLength(t *testing.T) {
	g := NewGenerator(core.WithSampleRate(48000))
	out, err := g.LogSweep(20, 20000, 1, 128)
	if err != nil {
		t.Fatalf("LogSweep() error = %v", err)
	}
	if len(out) != 128 {
		t.Fatalf("len = %d, want 128", len(out))
	}
}

func TestClip(t *testing.T) {
	out, err := Clip([]float64{-2, -0.5, 0.25, 2}, -1, 1)
	if err != nil {
		t.Fatalf("Clip() error = %v", err)
	}
	want := []float64{-1, -0.5, 0.25, 1}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%v, want %v", i, out[i], want[i])
		}
	}
}

func TestRemoveDC(t *testing.T) {
	out, err := RemoveDC([]float64{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("RemoveDC() error = %v", err)
	}
	sum := 0.0
	for _, v := range out {
		sum += v
	}
	if math.Abs(sum) > 1e-12 {
		t.Fatalf("sum=%v, want near 0", sum)
	}
}

func TestEnvelopeFollower(t *testing.T) {
	in := []float64{0, 1, 0, 1, 0}
	out, err := EnvelopeFollower(in, 1.0, 0.5)
	if err != nil {
		t.Fatalf("EnvelopeFollower() error = %v", err)
	}
	if out[0] != 0 || out[1] != 1 {
		t.Fatalf("unexpected attack behavior: %+v", out)
	}
	if !(out[2] < out[1] && out[2] > 0) {
		t.Fatalf("unexpected release behavior: %+v", out)
	}
}
