package effects

import (
	"math"
	"testing"
)

func TestDelayProcessInPlaceMatchesSample(t *testing.T) {
	d1, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}
	d2, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 29)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = d1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	d2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestDelayResetRestoresState(t *testing.T) {
	d, err := NewDelay(48000)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}

	in := make([]float64, 96)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = d.ProcessSample(in[i])
	}

	d.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = d.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestDelayImpulseAtConfiguredTime(t *testing.T) {
	const sampleRate = 1000.0
	d, err := NewDelay(sampleRate)
	if err != nil {
		t.Fatalf("NewDelay() error = %v", err)
	}
	if err := d.SetTime(0.01); err != nil {
		t.Fatalf("SetTime() error = %v", err)
	}
	if err := d.SetMix(1); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}
	if err := d.SetFeedback(0); err != nil {
		t.Fatalf("SetFeedback() error = %v", err)
	}

	in := make([]float64, 20)
	in[0] = 1
	out := make([]float64, len(in))
	for i := range in {
		out[i] = d.ProcessSample(in[i])
	}

	for i := range out {
		want := 0.0
		if i == 10 {
			want = 1
		}
		if diff := math.Abs(out[i] - want); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g", i, out[i], want)
		}
	}
}
