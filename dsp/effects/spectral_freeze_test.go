package effects

import (
	"math"
	"testing"
)

func TestNewSpectralFreezeRejectsInvalidSampleRate(t *testing.T) {
	invalid := []float64{0, -1, math.NaN(), math.Inf(1)}
	for _, sampleRate := range invalid {
		if _, err := NewSpectralFreeze(sampleRate); err == nil {
			t.Fatalf("NewSpectralFreeze(%v) expected error", sampleRate)
		}
	}
}

func TestSpectralFreezeMixZeroPassthrough(t *testing.T) {
	freeze, err := NewSpectralFreeze(48000)
	if err != nil {
		t.Fatalf("NewSpectralFreeze() error = %v", err)
	}

	if err := freeze.SetMix(0); err != nil {
		t.Fatalf("SetMix() error = %v", err)
	}
	freeze.Freeze()

	in := make([]float64, 512)
	for i := range in {
		in[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 48000)
	}

	out, err := freeze.ProcessWithError(in)
	if err != nil {
		t.Fatalf("ProcessWithError() error = %v", err)
	}

	for i := range in {
		if diff := math.Abs(out[i] - in[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, out[i], in[i], diff)
		}
	}
}

func TestSpectralFreezeInPlaceMatchesOutOfPlace(t *testing.T) {
	f1, err := NewSpectralFreeze(48000)
	if err != nil {
		t.Fatalf("NewSpectralFreeze() error = %v", err)
	}

	f2, err := NewSpectralFreeze(48000)
	if err != nil {
		t.Fatalf("NewSpectralFreeze() error = %v", err)
	}

	if err := f1.SetPhaseMode(SpectralFreezePhaseAdvance); err != nil {
		t.Fatalf("SetPhaseMode() error = %v", err)
	}
	if err := f2.SetPhaseMode(SpectralFreezePhaseAdvance); err != nil {
		t.Fatalf("SetPhaseMode() error = %v", err)
	}
	f1.Freeze()
	f2.Freeze()

	input := make([]float64, 1024)
	for i := range input {
		input[i] = math.Sin(2*math.Pi*220*float64(i)/48000) + 0.2*math.Sin(2*math.Pi*660*float64(i)/48000)
	}

	want, err := f1.ProcessWithError(input)
	if err != nil {
		t.Fatalf("ProcessWithError() error = %v", err)
	}

	got := make([]float64, len(input))
	copy(got, input)
	if err := f2.ProcessInPlaceWithError(got); err != nil {
		t.Fatalf("ProcessInPlaceWithError() error = %v", err)
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestSpectralFreezeResetDeterministic(t *testing.T) {
	freeze, err := NewSpectralFreeze(48000)
	if err != nil {
		t.Fatalf("NewSpectralFreeze() error = %v", err)
	}

	freeze.Freeze()
	in := make([]float64, 2048)
	for i := range in {
		if i < 512 {
			in[i] = math.Sin(2 * math.Pi * 330 * float64(i) / 48000)
		}
	}

	out1, err := freeze.ProcessWithError(in)
	if err != nil {
		t.Fatalf("ProcessWithError() error = %v", err)
	}

	freeze.Reset()
	out2, err := freeze.ProcessWithError(in)
	if err != nil {
		t.Fatalf("ProcessWithError() error = %v", err)
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g diff=%g", i, out2[i], out1[i], diff)
		}
	}
}

func TestSpectralFreezeSustainsEnergyOnSilenceTail(t *testing.T) {
	freeze, err := NewSpectralFreeze(48000)
	if err != nil {
		t.Fatalf("NewSpectralFreeze() error = %v", err)
	}

	if err := freeze.SetFrameSize(256); err != nil {
		t.Fatalf("SetFrameSize() error = %v", err)
	}
	if err := freeze.SetHopSize(64); err != nil {
		t.Fatalf("SetHopSize() error = %v", err)
	}
	if err := freeze.SetPhaseMode(SpectralFreezePhaseAdvance); err != nil {
		t.Fatalf("SetPhaseMode() error = %v", err)
	}
	freeze.Freeze()

	in := make([]float64, 4096)
	for i := 0; i < 256; i++ {
		in[i] = math.Sin(2 * math.Pi * 440 * float64(i) / 48000)
	}

	out, err := freeze.ProcessWithError(in)
	if err != nil {
		t.Fatalf("ProcessWithError() error = %v", err)
	}

	var tailEnergy float64
	for i := 1024; i < len(out); i++ {
		tailEnergy += out[i] * out[i]
	}

	if tailEnergy <= 1e-3 {
		t.Fatalf("expected sustained tail energy with freeze, got %g", tailEnergy)
	}
}
