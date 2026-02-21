package modulation

import (
	"math"
	"testing"
)

func TestPhaserProcessInPlaceMatchesProcess(t *testing.T) {
	p1, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}
	p2, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	input := make([]float64, 256)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 37)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = p1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	if err := p2.ProcessInPlace(got); err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestPhaserResetRestoresState(t *testing.T) {
	p, err := NewPhaser(48000)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	in := make([]float64, 128)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = p.Process(in[i])
	}

	p.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = p.Process(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestPhaserValidation(t *testing.T) {
	if _, err := NewPhaser(0); err == nil {
		t.Fatal("NewPhaser() expected error for invalid sample rate")
	}

	if _, err := NewPhaser(48000, WithPhaserStages(0)); err == nil {
		t.Fatal("NewPhaser() expected error for invalid stage count")
	}

	if _, err := NewPhaser(48000, WithPhaserFrequencyRangeHz(1000, 800)); err == nil {
		t.Fatal("NewPhaser() expected error for invalid frequency range")
	}

	if _, err := NewPhaser(48000, WithPhaserFrequencyRangeHz(1000, 30000)); err == nil {
		t.Fatal("NewPhaser() expected error for above-nyquist frequency")
	}
}

func TestPhaserFiniteOutputUnderFeedback(t *testing.T) {
	p, err := NewPhaser(48000,
		WithPhaserFeedback(0.85),
		WithPhaserMix(0.8),
		WithPhaserStages(8),
	)
	if err != nil {
		t.Fatalf("NewPhaser() error = %v", err)
	}

	for i := 0; i < 12000; i++ {
		in := 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
		out := p.Process(in)
		if math.IsNaN(out) || math.IsInf(out, 0) {
			t.Fatalf("non-finite output at sample %d: %v", i, out)
		}
	}
}
