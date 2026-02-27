package hilbert

import (
	"math"
	"testing"
)

func TestPresetConfig(t *testing.T) {
	tests := []struct {
		preset         Preset
		wantCoeffs     int
		wantTransition float64
	}{
		{PresetFast, 8, 0.1},
		{PresetBalanced, 12, 0.06},
		{PresetLowFrequency, 20, 0.02},
	}

	for _, tc := range tests {
		n, tr, err := PresetConfig(tc.preset)
		if err != nil {
			t.Fatalf("PresetConfig(%v) error = %v", tc.preset, err)
		}

		if n != tc.wantCoeffs || math.Abs(tr-tc.wantTransition) > 1e-12 {
			t.Fatalf("PresetConfig(%v) = (%d,%g), want (%d,%g)", tc.preset, n, tr, tc.wantCoeffs, tc.wantTransition)
		}
	}

	if _, _, err := PresetConfig(Preset(999)); err == nil {
		t.Fatal("expected error for invalid preset")
	}
}

func TestNewPresetConstructors(t *testing.T) {
	p64, err := New64Preset(PresetBalanced)
	if err != nil {
		t.Fatalf("New64Preset() error = %v", err)
	}

	if p64.NumberOfCoefficients() != 12 {
		t.Fatalf("64-bit preset coeffs = %d, want 12", p64.NumberOfCoefficients())
	}

	p32, err := New32Preset(PresetLowFrequency)
	if err != nil {
		t.Fatalf("New32Preset() error = %v", err)
	}

	if p32.NumberOfCoefficients() != 20 {
		t.Fatalf("32-bit preset coeffs = %d, want 20", p32.NumberOfCoefficients())
	}
}

func TestLowFrequencyPresetImproves100HzQuadrature(t *testing.T) {
	fastErr := quadratureErrorAt(t, PresetFast, 100)

	lowErr := quadratureErrorAt(t, PresetLowFrequency, 100)
	if lowErr >= fastErr {
		t.Fatalf("expected low-frequency preset to improve 100 Hz quadrature: fast=%.3f low=%.3f", fastErr, lowErr)
	}
}

func quadratureErrorAt(t *testing.T, preset Preset, freqHz float64) float64 {
	t.Helper()

	p, err := New64Preset(preset)
	if err != nil {
		t.Fatalf("New64Preset() error = %v", err)
	}

	const (
		sampleRate = 44100.0
		n          = 26000
		warmup     = 3000
	)

	w := 2 * math.Pi * freqHz / sampleRate

	var (
		aSin, aCos float64
		bSin, bCos float64
	)

	for i := range n {
		x := math.Sin(w * float64(i))
		a, b := p.ProcessSample(x)

		if i < warmup {
			continue
		}

		s := math.Sin(w * float64(i))
		c := math.Cos(w * float64(i))
		aSin += a * s
		aCos += a * c
		bSin += b * s
		bCos += b * c
	}

	phaseA := math.Atan2(aCos, aSin)
	phaseB := math.Atan2(bCos, bSin)

	delta := math.Abs((phaseB - phaseA) * 180 / math.Pi)
	for delta > 180 {
		delta -= 360
	}

	if delta < 0 {
		delta = -delta
	}

	return math.Abs(delta - 90)
}
