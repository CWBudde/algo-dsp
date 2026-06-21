package moog

import (
	"math"
	"math/rand"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func rms(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	var sum float64
	for _, v := range x {
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(x)))
}

func TestNewValidation(t *testing.T) {
	const sr = 48000.0

	cases := []struct {
		name    string
		cutoff  float64
		sr      float64
		opts    []Option
		wantErr bool
	}{
		{"ok", 1000, sr, nil, false},
		{"zero cutoff", 0, sr, nil, true},
		{"negative cutoff", -100, sr, nil, true},
		{"cutoff at nyquist", sr / 2, sr, nil, true},
		{"cutoff above nyquist", sr, sr, nil, true},
		{"nan cutoff", math.NaN(), sr, nil, true},
		{"zero sample rate", 1000, 0, nil, true},
		{"negative sample rate", 1000, -48000, nil, true},
		{"inf sample rate", 1000, math.Inf(1), nil, true},
		{"bad variant", 1000, sr, []Option{WithVariant(Variant(99))}, true},
		{"zero thermal voltage", 1000, sr, []Option{WithThermalVoltage(0)}, true},
		{"negative thermal voltage", 1000, sr, []Option{WithThermalVoltage(-1)}, true},
		{"resonance too high", 1000, sr, []Option{WithResonance(maxResonance + 1)}, true},
		{"resonance negative", 1000, sr, []Option{WithResonance(-1)}, true},
		{"nan gain", 1000, sr, []Option{WithGain(math.NaN())}, true},
		{"ok improved fast", 1000, sr, []Option{WithVariant(ImprovedClassic), WithFastTanh(true), WithResonance(3)}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := New(tc.cutoff, tc.sr, tc.opts...)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if f == nil {
				t.Fatal("expected non-nil filter")
			}
		})
	}
}

func TestSetterValidation(t *testing.T) {
	f, err := New(1000, 48000)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := f.SetCutoff(2000); err != nil {
		t.Errorf("SetCutoff valid: %v", err)
	}

	if f.Cutoff() != 2000 {
		t.Errorf("Cutoff() = %v, want 2000", f.Cutoff())
	}

	if err := f.SetCutoff(40000); err == nil {
		t.Error("SetCutoff above nyquist: expected error")
	}

	if err := f.SetResonance(3); err != nil {
		t.Errorf("SetResonance valid: %v", err)
	}

	if err := f.SetResonance(maxResonance + 1); err == nil {
		t.Error("SetResonance out of range: expected error")
	}

	if err := f.SetResonanceNormalized(0.5); err != nil {
		t.Errorf("SetResonanceNormalized valid: %v", err)
	}

	if want := 0.5 * resonanceSelfOscillation; f.Resonance() != want {
		t.Errorf("normalized resonance: got %v, want %v", f.Resonance(), want)
	}

	if err := f.SetResonanceNormalized(1.5); err == nil {
		t.Error("SetResonanceNormalized out of range: expected error")
	}

	if err := f.SetSampleRate(96000); err != nil {
		t.Errorf("SetSampleRate valid: %v", err)
	}

	if f.SampleRate() != 96000 {
		t.Errorf("SampleRate() = %v, want 96000", f.SampleRate())
	}

	// Cutoff 2000 is below the new Nyquist of 96000 but lowering the rate so it
	// exceeds Nyquist must fail.
	if err := f.SetSampleRate(3000); err == nil {
		t.Error("SetSampleRate below 2*cutoff: expected error")
	}
}

func TestDCSteadyState(t *testing.T) {
	// At steady state the ladder converges to scaleFactor/(1+resonance) for a
	// unit DC input. With resonance 0 and gain 1 the output settles to 1.0.
	for _, sr := range []float64{44100, 48000, 96000} {
		f, err := New(1000, sr)
		if err != nil {
			t.Fatalf("New(sr=%g): %v", sr, err)
		}

		var y float64
		for range 5000 {
			y = f.ProcessSample(1.0)
		}

		if math.Abs(y-1.0) > 1e-3 {
			t.Errorf("sr=%g: DC steady state = %.6f, want 1.0", sr, y)
		}
	}
}

func TestLowpassAttenuation(t *testing.T) {
	const (
		sr     = 48000.0
		cutoff = 1000.0
		n      = 8192
	)

	low := testutil.DeterministicSine(100, sr, 0.05, n)    // well below cutoff
	high := testutil.DeterministicSine(12000, sr, 0.05, n) // well above cutoff

	fLow, _ := New(cutoff, sr)
	fHigh, _ := New(cutoff, sr)

	fLow.ProcessInPlace(low)
	fHigh.ProcessInPlace(high)

	// Discard the transient (first quarter) before measuring.
	lowRMS := rms(low[n/4:])
	highRMS := rms(high[n/4:])

	if highRMS >= lowRMS {
		t.Errorf("expected high-freq RMS (%.6g) below low-freq RMS (%.6g)", highRMS, lowRMS)
	}

	// A 4-pole lowpass should attenuate a tone an octave-plus above cutoff hard.
	if highRMS > 0.1*lowRMS {
		t.Errorf("high-freq attenuation too weak: highRMS=%.6g lowRMS=%.6g", highRMS, lowRMS)
	}
}

func TestResonancePeak(t *testing.T) {
	const (
		sr     = 48000.0
		cutoff = 1000.0
		n      = 8192
	)

	ratio := func(resonance float64) float64 {
		f, _ := New(cutoff, sr, WithResonance(resonance))

		// Steady DC response.
		var dc float64
		for range 4000 {
			dc = f.ProcessSample(1.0)
		}

		f.Reset()

		tone := testutil.DeterministicSine(cutoff, sr, 0.05, n)
		f.ProcessInPlace(tone)

		return rms(tone[n/4:]) / math.Abs(dc)
	}

	low := ratio(0.0)
	high := ratio(3.0)

	if high <= low {
		t.Errorf("resonance did not raise the peak: ratio(0)=%.6g ratio(3)=%.6g", low, high)
	}
}

func TestStabilityUnderModulation(t *testing.T) {
	const (
		sr = 48000.0
		n  = 20000
	)

	f, err := New(1000, sr, WithResonance(3.5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rng := rand.New(rand.NewSource(1))

	out := make([]float64, n)
	for i := range out {
		// Modulate cutoff and resonance every sample.
		_ = f.SetCutoff(100 + rng.Float64()*18000)
		_ = f.SetResonance(rng.Float64() * 4)

		x := math.Sin(2*math.Pi*440*float64(i)/sr) + 0.5*(rng.Float64()*2-1)
		out[i] = f.ProcessSample(x)
	}

	testutil.RequireFinite(t, out)

	for i, v := range out {
		if math.Abs(v) > 100 {
			t.Fatalf("output blew up at %d: %v", i, v)
		}
	}
}

func TestEdgeCases(t *testing.T) {
	const sr = 48000.0

	t.Run("near dc cutoff", func(t *testing.T) {
		f, err := New(1, sr)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		out := make([]float64, 256)
		for i := range out {
			out[i] = f.ProcessSample(1.0)
		}

		testutil.RequireFinite(t, out)
	})

	t.Run("near nyquist cutoff", func(t *testing.T) {
		f, err := New(sr/2-1, sr)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		in := testutil.DeterministicSine(5000, sr, 0.5, 512)
		f.ProcessInPlace(in)
		testutil.RequireFinite(t, in)
	})

	t.Run("silence", func(t *testing.T) {
		f, _ := New(1000, sr)

		for i := range 100 {
			if y := f.ProcessSample(0); y != 0 {
				t.Fatalf("silence produced nonzero output %v at %d", y, i)
			}
		}
	})

	t.Run("large amplitude", func(t *testing.T) {
		f, _ := New(1000, sr, WithResonance(4))

		out := make([]float64, 1024)
		for i := range out {
			out[i] = f.ProcessSample(1000.0)
		}

		testutil.RequireFinite(t, out)
	})

	t.Run("short block", func(t *testing.T) {
		f, _ := New(1000, sr)
		buf := []float64{0.5}
		f.ProcessInPlace(buf)
		testutil.RequireFinite(t, buf)
	})

	t.Run("empty block", func(t *testing.T) {
		f, _ := New(1000, sr)
		f.ProcessInPlace(nil)
	})
}

func TestResetClearsState(t *testing.T) {
	f, _ := New(1000, 48000, WithResonance(2))

	in := testutil.DeterministicSine(500, 48000, 0.5, 1024)

	first := make([]float64, len(in))
	copy(first, in)
	f.ProcessInPlace(first)

	f.Reset()

	second := make([]float64, len(in))
	copy(second, in)
	f.ProcessInPlace(second)

	assertVectorClose(t, second, first, 0)
}

func TestVariantsDiffer(t *testing.T) {
	in := testutil.DeterministicSine(500, 48000, 0.5, 1024)

	simple, _ := New(1000, 48000, WithVariant(SimpleClassic), WithResonance(2))
	improved, _ := New(1000, 48000, WithVariant(ImprovedClassic), WithResonance(2))

	a := make([]float64, len(in))
	b := make([]float64, len(in))
	copy(a, in)
	copy(b, in)

	simple.ProcessInPlace(a)
	improved.ProcessInPlace(b)

	if diff, _ := testutil.MaxAbsDiff(a, b); diff < 1e-6 {
		t.Errorf("expected variants to differ, max abs diff = %g", diff)
	}
}

func TestFastTanhApproximation(t *testing.T) {
	// fastPow2 should track 2^x closely.
	for x := -10.0; x <= 10.0; x += 0.25 {
		got := fastPow2(x)
		want := math.Pow(2, x)

		if rel := math.Abs(got-want) / want; rel > 1e-3 {
			t.Errorf("fastPow2(%g) = %g, want %g (rel %g)", x, got, want, rel)
		}
	}

	// fastTanh should track math.Tanh within a small absolute tolerance.
	var maxDiff float64

	for x := -8.0; x <= 8.0; x += 0.01 {
		diff := math.Abs(fastTanh(x) - math.Tanh(x))
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	if maxDiff > 1e-3 {
		t.Errorf("fastTanh max abs error %g exceeds 1e-3", maxDiff)
	}

	// Saturation bounds.
	if fastTanh(50) != 1 || fastTanh(-50) != -1 {
		t.Errorf("fastTanh saturation: got %g and %g", fastTanh(50), fastTanh(-50))
	}
}

func TestZeroAlloc(t *testing.T) {
	buf := testutil.DeterministicSine(500, 48000, 0.5, 1024)

	for _, fast := range []bool{false, true} {
		f, _ := New(1000, 48000, WithFastTanh(fast), WithResonance(2))

		work := make([]float64, len(buf))

		allocs := testing.AllocsPerRun(50, func() {
			copy(work, buf)
			f.ProcessInPlace(work)
		})

		// copy() itself does not allocate; ProcessInPlace must not either.
		if allocs != 0 {
			t.Errorf("fast=%v: ProcessInPlace allocated %g times, want 0", fast, allocs)
		}
	}
}
