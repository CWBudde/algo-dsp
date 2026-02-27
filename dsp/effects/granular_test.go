package effects

import (
	"math"
	"testing"
)

func TestNewGranularRejectsInvalidSampleRate(t *testing.T) {
	invalid := []float64{0, -1, math.NaN(), math.Inf(1)}
	for _, sampleRate := range invalid {
		_, err := NewGranular(sampleRate)
		if err == nil {
			t.Fatalf("NewGranular(%v) expected error", sampleRate)
		}
	}
}

func TestGranularProcessInPlaceMatchesSample(t *testing.T) {
	g1, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	g2, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	g1.SetRandomSeed(42)
	g2.SetRandomSeed(42)

	input := make([]float64, 512)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * 220 * float64(i) / 48000)
	}

	want := make([]float64, len(input))
	copy(want, input)

	for i := range want {
		want[i] = g1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	g2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestGranularResetRestoresState(t *testing.T) {
	g, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	g.SetRandomSeed(7)

	in := make([]float64, 1024)
	for i := range in {
		in[i] = math.Sin(2 * math.Pi * 330 * float64(i) / 48000)
	}

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = g.ProcessSample(in[i])
	}

	g.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = g.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestGranularMixZeroTransparent(t *testing.T) {
	g, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	err = g.SetMix(0)
	if err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}

	for i := range 512 {
		in := 0.8 * math.Sin(2*math.Pi*440*float64(i)/48000)

		out := g.ProcessSample(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out, in)
		}
	}
}

func TestGranularSustainsTailEnergy(t *testing.T) {
	g, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	_ = g.SetGrainSeconds(0.04)
	_ = g.SetOverlap(0.75)
	_ = g.SetPitch(1.0)
	_ = g.SetSpray(0.0)
	_ = g.SetBaseDelay(0.01)
	g.SetRandomSeed(1)

	in := make([]float64, 4096)
	for i := range 1024 {
		in[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 48000)
	}

	out := make([]float64, len(in))
	for i := range in {
		out[i] = g.ProcessSample(in[i])
	}

	var tailEnergy float64
	for i := 1200; i < 1800; i++ {
		tailEnergy += out[i] * out[i]
	}

	if tailEnergy <= 1e-6 {
		t.Fatalf("expected non-zero granular tail energy, got %g", tailEnergy)
	}
}

func TestGranularSettersValidation(t *testing.T) {
	g, err := NewGranular(48000)
	if err != nil {
		t.Fatalf("NewGranular() error = %v", err)
	}

	err = g.SetGrainSeconds(0)
	if err == nil {
		t.Fatalf("SetGrainSeconds(0) expected error")
	}

	err = g.SetOverlap(1)
	if err == nil {
		t.Fatalf("SetOverlap(1) expected error")
	}

	err = g.SetMix(-0.1)
	if err == nil {
		t.Fatalf("SetMix(-0.1) expected error")
	}

	err = g.SetPitch(10)
	if err == nil {
		t.Fatalf("SetPitch(10) expected error")
	}

	err = g.SetSpray(2)
	if err == nil {
		t.Fatalf("SetSpray(2) expected error")
	}

	err = g.SetBaseDelay(3)
	if err == nil {
		t.Fatalf("SetBaseDelay(3) expected error")
	}
}
