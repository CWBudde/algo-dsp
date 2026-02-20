package effects

import (
	"math"
	"testing"
)

func TestTremoloProcessInPlaceMatchesProcess(t *testing.T) {
	t1, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}
	t2, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = t1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	if err := t2.ProcessInPlace(got); err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestTremoloResetRestoresState(t *testing.T) {
	tm, err := NewTremolo(48000)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = tm.Process(in[i])
	}

	tm.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = tm.Process(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestTremoloDepthZeroIsTransparent(t *testing.T) {
	tm, err := NewTremolo(48000,
		WithTremoloDepth(0),
		WithTremoloMix(1),
	)
	if err != nil {
		t.Fatalf("NewTremolo() error = %v", err)
	}

	for i := 0; i < 512; i++ {
		in := 0.5 * math.Sin(2*math.Pi*440*float64(i)/48000)
		out := tm.Process(in)
		if diff := math.Abs(out - in); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out, in)
		}
	}
}

func TestTremoloValidation(t *testing.T) {
	if _, err := NewTremolo(0); err == nil {
		t.Fatal("NewTremolo() expected error for invalid sample rate")
	}

	if _, err := NewTremolo(48000, WithTremoloDepth(1.2)); err == nil {
		t.Fatal("NewTremolo() expected error for invalid depth")
	}

	if _, err := NewTremolo(48000, WithTremoloSmoothingMs(-1)); err == nil {
		t.Fatal("NewTremolo() expected error for invalid smoothing")
	}
}
