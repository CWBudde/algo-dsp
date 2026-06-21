package moog

import (
	"math"
	"testing"
)

// referenceMoog is an independent transcription of the legacy Pascal recurrence
// (DAV_DspFilterMoog.pas) used to validate the Filter port. It mirrors the
// documented stage math exactly so any structural drift in ProcessSample is
// caught.
func referenceMoog(variant Variant, fast bool, cutoff, resonance, thermalV, gain, sr float64, in []float64) []float64 {
	g := 2 * thermalV * (1 - math.Exp(-2*math.Pi*cutoff/sr))
	if variant == ImprovedClassic {
		g *= 2 * thermalV
	}

	vtInv := 1 / thermalV
	amp := math.Pow(10, resonance/20)
	scale := gain * amp * amp

	th := math.Tanh
	if fast {
		th = fastTanh
	}

	var (
		s [4]float64
		t [3]float64
	)

	out := make([]float64, len(in))
	for i, x := range in {
		u := x - resonance*s[3]

		s[0] += g * (th(0.5*u*vtInv) - t[0])
		t[0] = th(0.5 * s[0] * vtInv)

		s[1] += g * (t[0] - t[1])
		t[1] = th(0.5 * s[1] * vtInv)

		s[2] += g * (t[1] - t[2])
		t[2] = th(0.5 * s[2] * vtInv)

		s[3] += g * (t[2] - th(0.5*s[3]*vtInv))

		out[i] = scale * s[3]
	}

	return out
}

// makeParitySignal builds a deterministic multi-segment excitation: quiet tone,
// loud tone, impulse-ish step, and silence tail.
func makeParitySignal() []float64 {
	const sr = 48000.0

	out := make([]float64, 4096)
	for i := range out {
		x := float64(i)

		switch {
		case i < 1024:
			out[i] = 0.05 * math.Sin(2*math.Pi*220*x/sr)
		case i < 2048:
			out[i] = 0.8 * math.Sin(2*math.Pi*440*x/sr)
		case i < 2304:
			out[i] = 1.0
		default:
			out[i] = 0
		}
	}

	return out
}

func assertVectorClose(t *testing.T, got, want []float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("length mismatch: got=%d want=%d", len(got), len(want))
	}

	for i := range got {
		if diff := math.Abs(got[i] - want[i]); diff > tol {
			t.Fatalf("sample %d: got=%.15g want=%.15g diff=%g", i, got[i], want[i], diff)
		}
	}
}

func TestLegacyParityAllVariants(t *testing.T) {
	const (
		sr        = 48000.0
		cutoff    = 1200.0
		resonance = 2.5
		thermalV  = defaultThermalVoltage
		gain      = defaultGain
	)

	in := makeParitySignal()

	cases := []struct {
		name    string
		variant Variant
		fast    bool
	}{
		{"simple_full", SimpleClassic, false},
		{"simple_fast", SimpleClassic, true},
		{"improved_full", ImprovedClassic, false},
		{"improved_fast", ImprovedClassic, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := New(cutoff, sr,
				WithVariant(tc.variant),
				WithFastTanh(tc.fast),
				WithResonance(resonance),
				WithThermalVoltage(thermalV),
				WithGain(gain),
			)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			got := make([]float64, len(in))
			copy(got, in)
			f.ProcessInPlace(got)

			want := referenceMoog(tc.variant, tc.fast, cutoff, resonance, thermalV, gain, sr, in)

			assertVectorClose(t, got, want, 1e-12)
		})
	}
}
