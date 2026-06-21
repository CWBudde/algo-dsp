package modulation

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

func TestFrequencyShifterProcessBlockMatchesSample(t *testing.T) {
	fBlock, err := NewFrequencyShifter(48000,
		WithFrequencyShiftHz(120),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		t.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	fSample, err := NewFrequencyShifter(48000,
		WithFrequencyShiftHz(120),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		t.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	input := make([]float64, 1024)
	for i := range input {
		input[i] = 0.7*math.Sin(2*math.Pi*float64(i)/47) + 0.15*math.Sin(2*math.Pi*float64(i)/13)
	}

	gotUp := make([]float64, len(input))

	gotDown := make([]float64, len(input))

	err = fBlock.ProcessBlock(input, gotUp, gotDown)
	if err != nil {
		t.Fatalf("ProcessBlock() error = %v", err)
	}

	for i, x := range input {
		wantUp, wantDown := fSample.ProcessSample(x)
		if d := math.Abs(gotUp[i] - wantUp); d > 1e-12 {
			t.Fatalf("up[%d] mismatch: got=%g want=%g", i, gotUp[i], wantUp)
		}

		if d := math.Abs(gotDown[i] - wantDown); d > 1e-12 {
			t.Fatalf("down[%d] mismatch: got=%g want=%g", i, gotDown[i], wantDown)
		}
	}
}

func TestFrequencyShifterResetRestoresState(t *testing.T) {
	f, err := NewFrequencyShifter(48000,
		WithFrequencyShiftHz(150),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		t.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	input := make([]float64, 256)
	input[0] = 1

	up1 := make([]float64, len(input))

	down1 := make([]float64, len(input))
	for i, x := range input {
		up1[i], down1[i] = f.ProcessSample(x)
	}

	f.Reset()

	for i, x := range input {
		up2, down2 := f.ProcessSample(x)
		if math.Abs(up1[i]-up2) > 1e-12 {
			t.Fatalf("up[%d] mismatch after reset", i)
		}

		if math.Abs(down1[i]-down2) > 1e-12 {
			t.Fatalf("down[%d] mismatch after reset", i)
		}
	}
}

func TestFrequencyShifterValidation(t *testing.T) {
	_, err := NewFrequencyShifter(0)
	if err == nil {
		t.Fatal("expected error for invalid sample rate")
	}

	_, err = NewFrequencyShifter(48000, WithFrequencyShiftHz(0))
	if err == nil {
		t.Fatal("expected error for invalid shift Hz")
	}

	_, err = NewFrequencyShifter(48000, WithFrequencyShifterHilbertPreset(hilbert.Preset(999)))
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}

	_, err = NewFrequencyShifter(48000, WithFrequencyShifterHilbertDesign(0, 0.1))
	if err == nil {
		t.Fatal("expected error for invalid custom design")
	}
}

func TestFrequencyShifterSetters(t *testing.T) {
	f, err := NewFrequencyShifter(48000)
	if err != nil {
		t.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	err = f.SetSampleRate(96000)
	if err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	err = f.SetShiftHz(80)
	if err != nil {
		t.Fatalf("SetShiftHz() error = %v", err)
	}

	err = f.SetHilbertPreset(hilbert.PresetLowFrequency)
	if err != nil {
		t.Fatalf("SetHilbertPreset() error = %v", err)
	}

	if n, tr := f.HilbertDesign(); n != 20 || math.Abs(tr-0.02) > 1e-12 {
		t.Fatalf("preset design mismatch: (%d,%g)", n, tr)
	}

	err = f.SetHilbertDesign(10, 0.08)
	if err != nil {
		t.Fatalf("SetHilbertDesign() error = %v", err)
	}

	if n, tr := f.HilbertDesign(); n != 10 || math.Abs(tr-0.08) > 1e-12 {
		t.Fatalf("custom design mismatch: (%d,%g)", n, tr)
	}
}

func TestFrequencyShifterSpectralShift(t *testing.T) {
	const (
		sampleRate = 48000.0
		inputHz    = 1000.0
		shiftHz    = 120.0
		nSamples   = 48000
	)

	f, err := NewFrequencyShifter(sampleRate,
		WithFrequencyShiftHz(shiftHz),
		WithFrequencyShifterHilbertPreset(hilbert.PresetBalanced),
	)
	if err != nil {
		t.Fatalf("NewFrequencyShifter() error = %v", err)
	}

	up := make([]float64, nSamples)

	down := make([]float64, nSamples)
	for i := range nSamples {
		x := math.Sin(2 * math.Pi * inputHz * float64(i) / sampleRate)
		up[i], down[i] = f.ProcessSample(x)
	}

	mag := func(sig []float64, freq float64) float64 {
		var re, im float64

		for i, v := range sig {
			a := 2 * math.Pi * freq * float64(i) / sampleRate
			re += v * math.Cos(a)
			im += v * math.Sin(a)
		}

		return math.Hypot(re, im) / float64(len(sig))
	}

	upTarget := mag(up, inputHz+shiftHz)
	upImage := mag(up, inputHz-shiftHz)
	downTarget := mag(down, inputHz-shiftHz)
	downImage := mag(down, inputHz+shiftHz)

	if upTarget < 0.15 {
		t.Fatalf("upshift target too small: %g", upTarget)
	}

	if downTarget < 0.15 {
		t.Fatalf("downshift target too small: %g", downTarget)
	}

	upRej := 20 * math.Log10(upTarget/upImage)
	downRej := 20 * math.Log10(downTarget/downImage)

	if upRej < 35 {
		t.Fatalf("upshift image rejection too small: %.2f dB", upRej)
	}

	if downRej < 35 {
		t.Fatalf("downshift image rejection too small: %.2f dB", downRej)
	}
}
