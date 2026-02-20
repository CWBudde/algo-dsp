package effects

import (
	"math"
	"testing"
)

func TestFlangerProcessInPlaceMatchesProcess(t *testing.T) {
	f1, err := NewFlanger(48000)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}
	f2, err := NewFlanger(48000)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 29)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = f1.Process(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	if err := f2.ProcessInPlace(got); err != nil {
		t.Fatalf("ProcessInPlace() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestFlangerResetRestoresState(t *testing.T) {
	f, err := NewFlanger(48000)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = f.Process(in[i])
	}

	f.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = f.Process(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestFlangerImpulseAtConfiguredDelayWhenDepthZero(t *testing.T) {
	f, err := NewFlanger(1000,
		WithFlangerBaseDelaySeconds(0.005),
		WithFlangerDepthSeconds(0),
		WithFlangerMix(1),
		WithFlangerFeedback(0),
	)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}

	in := make([]float64, 16)
	in[0] = 1
	out := make([]float64, len(in))
	for i := range in {
		out[i] = f.Process(in[i])
	}

	for i := range out {
		want := 0.0
		if i == 5 {
			want = 1
		}
		if diff := math.Abs(out[i] - want); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out[i], want)
		}
	}
}

func TestFlangerValidation(t *testing.T) {
	if _, err := NewFlanger(0); err == nil {
		t.Fatal("NewFlanger() expected error for invalid sample rate")
	}

	if _, err := NewFlanger(48000, WithFlangerMix(2)); err == nil {
		t.Fatal("NewFlanger() expected error for invalid mix")
	}

	if _, err := NewFlanger(48000,
		WithFlangerBaseDelaySeconds(0.009),
		WithFlangerDepthSeconds(0.002),
	); err == nil {
		t.Fatal("NewFlanger() expected error for base+depth > max")
	}
}
