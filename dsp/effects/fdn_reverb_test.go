package effects

import (
	"math"
	"testing"
)

func TestFDNReverbProcessInPlaceMatchesSample(t *testing.T) {
	r1, err := NewFDNReverb(44100)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}
	r2, err := NewFDNReverb(44100)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}

	input := make([]float64, 128)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 31)
	}

	want := make([]float64, len(input))
	copy(want, input)
	for i := range want {
		want[i] = r1.ProcessSample(want[i])
	}

	got := make([]float64, len(input))
	copy(got, input)
	r2.ProcessInPlace(got)

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestFDNReverbResetRestoresState(t *testing.T) {
	r, err := NewFDNReverb(44100)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}

	in := make([]float64, 128)
	in[0] = 1

	out1 := make([]float64, len(in))
	for i := range in {
		out1[i] = r.ProcessSample(in[i])
	}

	r.Reset()

	out2 := make([]float64, len(in))
	for i := range in {
		out2[i] = r.ProcessSample(in[i])
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestFDNReverbImpulseTailExists(t *testing.T) {
	r, err := NewFDNReverb(44100)
	if err != nil {
		t.Fatalf("NewFDNReverb: %v", err)
	}
	if err := r.SetDry(0); err != nil {
		t.Fatalf("SetDry: %v", err)
	}

	const n = 4096
	var nonZero bool
	for i := 0; i < n; i++ {
		x := 0.0
		if i == 0 {
			x = 1
		}
		y := r.ProcessSample(x)
		if i > 0 && math.Abs(y) > 1e-10 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero reverb tail")
	}
}
