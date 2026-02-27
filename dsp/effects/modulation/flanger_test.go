package modulation

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

	err = f2.ProcessInPlace(got)
	if err != nil {
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
	_, err := NewFlanger(0)
	if err == nil {
		t.Fatal("NewFlanger() expected error for invalid sample rate")
	}

	_, err = NewFlanger(48000, WithFlangerMix(2))
	if err == nil {
		t.Fatal("NewFlanger() expected error for invalid mix")
	}

	_, err = NewFlanger(48000,
		WithFlangerBaseDelaySeconds(0.009),
		WithFlangerDepthSeconds(0.002),
	)
	if err == nil {
		t.Fatal("NewFlanger() expected error for base+depth > max")
	}
}

func TestFlangerSetDepthSecondsRollsBackOnInvalidCombination(t *testing.T) {
	f, err := NewFlanger(48000,
		WithFlangerBaseDelaySeconds(0.0087),
		WithFlangerDepthSeconds(0.0012),
	)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}

	prevBase := f.BaseDelaySeconds()
	prevDepth := f.DepthSeconds()

	err = f.SetDepthSeconds(0.0014)
	if err == nil {
		t.Fatal("SetDepthSeconds() expected error for base+depth > max")
	}

	if got := f.BaseDelaySeconds(); math.Abs(got-prevBase) > 1e-12 {
		t.Fatalf("base delay changed after failed update: got=%g want=%g", got, prevBase)
	}

	if got := f.DepthSeconds(); math.Abs(got-prevDepth) > 1e-12 {
		t.Fatalf("depth changed after failed update: got=%g want=%g", got, prevDepth)
	}

	err = f.SetDepthSeconds(0.0011)
	if err != nil {
		t.Fatalf("SetDepthSeconds() error after rollback = %v", err)
	}
}

func TestFlangerSetBaseDelaySecondsRollsBackOnInvalidCombination(t *testing.T) {
	f, err := NewFlanger(48000,
		WithFlangerBaseDelaySeconds(0.004),
		WithFlangerDepthSeconds(0.004),
	)
	if err != nil {
		t.Fatalf("NewFlanger() error = %v", err)
	}

	prevBase := f.BaseDelaySeconds()
	prevDepth := f.DepthSeconds()

	err = f.SetBaseDelaySeconds(0.007)
	if err == nil {
		t.Fatal("SetBaseDelaySeconds() expected error for base+depth > max")
	}

	if got := f.BaseDelaySeconds(); math.Abs(got-prevBase) > 1e-12 {
		t.Fatalf("base delay changed after failed update: got=%g want=%g", got, prevBase)
	}

	if got := f.DepthSeconds(); math.Abs(got-prevDepth) > 1e-12 {
		t.Fatalf("depth changed after failed update: got=%g want=%g", got, prevDepth)
	}

	err = f.SetBaseDelaySeconds(0.0055)
	if err != nil {
		t.Fatalf("SetBaseDelaySeconds() error after rollback = %v", err)
	}
}
