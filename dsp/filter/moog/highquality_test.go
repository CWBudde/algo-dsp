package moog

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

// goertzelPower returns the Goertzel power of x at the given frequency.
func goertzelPower(x []float64, freq, sr float64) float64 {
	g, err := spectrum.NewGoertzel(freq, sr)
	if err != nil {
		panic(err)
	}

	g.ProcessBlock(x)

	return g.Power()
}

func TestOversamplingValidation(t *testing.T) {
	for _, factor := range []int{1, 2, 4, 8} {
		if _, err := New(1000, 48000, WithOversampling(factor)); err != nil {
			t.Errorf("New oversampling %d: unexpected error %v", factor, err)
		}
	}

	for _, factor := range []int{0, 3, 5, -2, 16} {
		if _, err := New(1000, 48000, WithOversampling(factor)); err == nil {
			t.Errorf("New oversampling %d: expected error", factor)
		}
	}

	f, _ := New(1000, 48000)
	if f.Oversampling() != 1 {
		t.Errorf("default oversampling = %d, want 1", f.Oversampling())
	}

	if err := f.SetOversampling(4); err != nil {
		t.Fatalf("SetOversampling(4): %v", err)
	}

	if f.Oversampling() != 4 {
		t.Errorf("Oversampling() = %d, want 4", f.Oversampling())
	}

	if err := f.SetOversampling(3); err == nil {
		t.Error("SetOversampling(3): expected error")
	}
}

func TestOversampledStability(t *testing.T) {
	for _, factor := range []int{2, 4, 8} {
		f, err := New(2000, 48000, WithOversampling(factor), WithResonance(3.5))
		if err != nil {
			t.Fatalf("New os=%d: %v", factor, err)
		}

		in := testutil.DeterministicSine(500, 48000, 5.0, 4096) // hard drive
		f.ProcessInPlace(in)
		testutil.RequireFinite(t, in)
	}
}

func TestSaturationSymmetry(t *testing.T) {
	// The ladder is an odd system (tanh is odd, feedback is odd), so negating
	// the input must negate the output exactly.
	for _, os := range []int{1, 4} {
		in := testutil.DeterministicSine(300, 48000, 3.0, 2048)

		neg := make([]float64, len(in))
		for i, v := range in {
			neg[i] = -v
		}

		fp, _ := New(1500, 48000, WithOversampling(os), WithResonance(2))
		fn, _ := New(1500, 48000, WithOversampling(os), WithResonance(2))

		fp.ProcessInPlace(in)
		fn.ProcessInPlace(neg)

		for i := range in {
			if diff := math.Abs(in[i] + neg[i]); diff > 1e-9 {
				t.Fatalf("os=%d sample %d: not antisymmetric, out(+)=%g out(-)=%g", os, i, in[i], neg[i])
			}
		}
	}
}

func TestHarmonicGrowthWithDrive(t *testing.T) {
	const (
		sr     = 48000.0
		f0     = 300.0
		cutoff = 3000.0
		n      = 9600
	)

	// Relative third-harmonic content for a given input amplitude.
	relThird := func(amp float64) float64 {
		f, _ := New(cutoff, sr, WithResonance(1))

		in := testutil.DeterministicSine(f0, sr, amp, n)
		f.ProcessInPlace(in)

		block := in[n/2:] // skip transient
		fund := goertzelPower(block, f0, sr)
		third := goertzelPower(block, 3*f0, sr)

		return third / fund
	}

	low := relThird(0.5)
	high := relThird(30.0)

	if high <= low {
		t.Errorf("harmonics did not grow with drive: rel3(0.5)=%g rel3(30)=%g", low, high)
	}
}

func TestSelfOscillationRings(t *testing.T) {
	const (
		sr     = 48000.0
		cutoff = 1000.0
		n      = 8192
	)

	// Late-window energy at the cutoff frequency after an impulse: a highly
	// resonant ladder rings far longer than a low-resonance one.
	ring := func(resonance float64) float64 {
		f, _ := New(cutoff, sr, WithResonance(resonance))

		out := make([]float64, n)
		out[0] = 1.0
		f.ProcessInPlace(out)

		return goertzelPower(out[n/2:], cutoff, sr)
	}

	low := ring(0.5)
	high := ring(3.9)

	if high <= low {
		t.Errorf("high resonance did not ring longer: ring(0.5)=%g ring(3.9)=%g", low, high)
	}
}

func TestOversamplingReducesAliasing(t *testing.T) {
	const (
		sr     = 48000.0
		f0     = 9000.0
		cutoff = 10000.0
		amp    = 40.0
		n      = 19200
		block  = 9600 // bin width 5 Hz: f0 and the alias frequency are on-bin
		// tanh is odd, so it generates odd harmonics. The 5th harmonic (45 kHz)
		// folds to 3 kHz at the base rate; with oversampling it sits in the
		// anti-alias stopband and is removed before decimation.
		aliasHz = 3000.0
	)

	aliasPower := func(os int) float64 {
		f, _ := New(cutoff, sr, WithOversampling(os), WithResonance(2))

		in := testutil.DeterministicSine(f0, sr, amp, n)
		f.ProcessInPlace(in)

		return goertzelPower(in[n-block:], aliasHz, sr)
	}

	base := aliasPower(1)
	oversampled := aliasPower(4)

	if base == 0 {
		t.Fatal("no aliasing measured at base rate; test signal is ineffective")
	}

	if oversampled >= 0.5*base {
		t.Errorf("oversampling did not reduce aliasing enough: base=%g os4=%g (ratio %.3f)",
			base, oversampled, oversampled/base)
	}
}
