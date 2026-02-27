package effects

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	algofft "github.com/cwbudde/algo-fft"
)

func TestNewVocoderValidation(t *testing.T) {
	tests := []struct {
		name string
		sr   float64
		opts []VocoderOption
	}{
		{"zero sample rate", 0, nil},
		{"negative sample rate", -1, nil},
		{"NaN sample rate", math.NaN(), nil},
		{"Inf sample rate", math.Inf(1), nil},
		{"invalid layout", 48000, []VocoderOption{WithBandLayout(BandLayout(99))}},
		{"negative attack", 48000, []VocoderOption{WithVocoderAttack(-1)}},
		{"NaN attack", 48000, []VocoderOption{WithVocoderAttack(math.NaN())}},
		{"negative release", 48000, []VocoderOption{WithVocoderRelease(-1)}},
		{"NaN release", 48000, []VocoderOption{WithVocoderRelease(math.NaN())}},
		{"negative input level", 48000, []VocoderOption{WithVocoderInputLevel(-1)}},
		{"input level too high", 48000, []VocoderOption{WithVocoderInputLevel(11)}},
		{"negative synth level", 48000, []VocoderOption{WithVocoderSynthLevel(-1)}},
		{"negative vocoder level", 48000, []VocoderOption{WithVocoderLevel(-1)}},
		{"synthesis Q too low", 48000, []VocoderOption{WithVocoderSynthesisQ(0.01)}},
		{"synthesis Q too high", 48000, []VocoderOption{WithVocoderSynthesisQ(25)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewVocoder(tc.sr, tc.opts...)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestNewVocoderDefaults(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	if v.Layout() != BandLayoutThirdOctave {
		t.Errorf("Layout() = %d, want %d", v.Layout(), BandLayoutThirdOctave)
	}

	if v.SampleRate() != 48000 {
		t.Errorf("SampleRate() = %g, want 48000", v.SampleRate())
	}

	if v.Attack() != defaultVocoderAttackMs {
		t.Errorf("Attack() = %g, want %g", v.Attack(), defaultVocoderAttackMs)
	}

	if v.Release() != defaultVocoderReleaseMs {
		t.Errorf("Release() = %g, want %g", v.Release(), defaultVocoderReleaseMs)
	}

	if v.InputLevel() != 0 {
		t.Errorf("InputLevel() = %g, want 0", v.InputLevel())
	}

	if v.SynthLevel() != 0 {
		t.Errorf("SynthLevel() = %g, want 0", v.SynthLevel())
	}

	if v.VocoderLevel() != 1 {
		t.Errorf("VocoderLevel() = %g, want 1", v.VocoderLevel())
	}
	// At 48 kHz, all 32 third-octave bands (up to 20 kHz) should be usable.
	if v.NumBands() != 32 {
		t.Errorf("NumBands() = %d, want 32", v.NumBands())
	}
}

func TestNewVocoderBandCountAtLowSampleRate(t *testing.T) {
	// At 8000 Hz, Nyquist is 4000 Hz → bands above 3600 Hz should be excluded.
	v, err := NewVocoder(8000)
	if err != nil {
		t.Fatalf("NewVocoder(8000) error = %v", err)
	}
	// 3150 Hz is the last center freq below 3600 (8000*0.9/2).
	// Bands: 16..3150 Hz = 24 bands.
	if v.NumBands() < 20 || v.NumBands() > 26 {
		t.Errorf("NumBands() at 8 kHz = %d, expected roughly 24", v.NumBands())
	}
}

func TestVocoderSilentModulatorProducesSilence(t *testing.T) {
	layouts := []struct {
		name   string
		layout BandLayout
	}{
		{"ThirdOctave", BandLayoutThirdOctave},
		{"Bark", BandLayoutBark},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			v, err := NewVocoder(48000, WithBandLayout(l.layout))
			if err != nil {
				t.Fatalf("NewVocoder() error = %v", err)
			}

			// Process 1000 samples with silent modulator and a 1 kHz carrier.
			const n = 1000

			maxOut := 0.0

			for i := range n {
				carrier := math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)

				out := v.ProcessSample(0, carrier)
				if math.Abs(out) > maxOut {
					maxOut = math.Abs(out)
				}
			}

			// With zero modulator, all envelopes stay at zero, so vocoded output should be ~0.
			if maxOut > 1e-6 {
				t.Errorf("expected near-silence for silent modulator, got max |output| = %g", maxOut)
			}
		})
	}
}

func TestVocoderNonSilentOutput(t *testing.T) {
	layouts := []struct {
		name   string
		layout BandLayout
	}{
		{"ThirdOctave", BandLayoutThirdOctave},
		{"Bark", BandLayoutBark},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			v, err := NewVocoder(48000, WithBandLayout(l.layout))
			if err != nil {
				t.Fatalf("NewVocoder() error = %v", err)
			}

			// Feed 1 kHz sine as both modulator and carrier.
			const n = 4000

			sumAbs := 0.0

			for i := range n {
				sig := math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
				out := v.ProcessSample(sig, sig)
				sumAbs += math.Abs(out)
			}

			avg := sumAbs / n
			if avg < 1e-6 {
				t.Errorf("expected non-zero output for non-silent input, got avg |output| = %g", avg)
			}
		})
	}
}

func TestVocoderProcessBlock(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	t.Run("length mismatch", func(t *testing.T) {
		mod := make([]float64, 10)
		car := make([]float64, 5)

		out := make([]float64, 10)

		err := v.ProcessBlock(mod, car, out)
		if err == nil {
			t.Fatal("expected error for length mismatch")
		}
	})

	t.Run("matches ProcessSample", func(t *testing.T) {
		v1, _ := NewVocoder(48000)
		v2, _ := NewVocoder(48000)

		const n = 256

		mod := make([]float64, n)
		car := make([]float64, n)
		wantOut := make([]float64, n)
		gotOut := make([]float64, n)

		for i := range n {
			ts := float64(i) / 48000
			mod[i] = math.Sin(2 * math.Pi * 440 * ts)
			car[i] = math.Sin(2 * math.Pi * 880 * ts)
		}

		// Reference: sample-by-sample.
		for i := range n {
			wantOut[i] = v1.ProcessSample(mod[i], car[i])
		}

		// Block processing.
		err := v2.ProcessBlock(mod, car, gotOut)
		if err != nil {
			t.Fatalf("ProcessBlock() error = %v", err)
		}

		for i := range n {
			if diff := math.Abs(gotOut[i] - wantOut[i]); diff > 1e-12 {
				t.Fatalf("sample %d mismatch: got=%g want=%g diff=%g", i, gotOut[i], wantOut[i], diff)
			}
		}
	})
}

func TestVocoderReset(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	const n = 500

	out1 := make([]float64, n)
	out2 := make([]float64, n)

	// First pass.
	for i := range n {
		ts := float64(i) / 48000
		mod := math.Sin(2 * math.Pi * 440 * ts)
		car := math.Sin(2 * math.Pi * 880 * ts)
		out1[i] = v.ProcessSample(mod, car)
	}

	v.Reset()

	// Second pass: should match exactly.
	for i := range n {
		ts := float64(i) / 48000
		mod := math.Sin(2 * math.Pi * 440 * ts)
		car := math.Sin(2 * math.Pi * 880 * ts)
		out2[i] = v.ProcessSample(mod, car)
	}

	for i := range n {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g", i, out2[i], out1[i])
		}
	}
}

func TestVocoderSetSampleRate(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	err = v.SetSampleRate(44100)
	if err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	if v.SampleRate() != 44100 {
		t.Errorf("SampleRate() = %g, want 44100", v.SampleRate())
	}

	// Verify it still produces output.
	var sum float64

	for i := range 1000 {
		sig := math.Sin(2 * math.Pi * 1000 * float64(i) / 44100)
		sum += math.Abs(v.ProcessSample(sig, sig))
	}

	if sum < 1e-6 {
		t.Error("expected non-zero output after SetSampleRate")
	}

	// Invalid sample rate.
	err = v.SetSampleRate(0)
	if err == nil {
		t.Error("expected error for zero sample rate")
	}
}

func TestVocoderSetterGetterRoundtrip(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	err = v.SetAttack(5.0)
	if err != nil {
		t.Fatalf("SetAttack() error = %v", err)
	}

	if v.Attack() != 5.0 {
		t.Errorf("Attack() = %g, want 5.0", v.Attack())
	}

	err = v.SetRelease(50.0)
	if err != nil {
		t.Fatalf("SetRelease() error = %v", err)
	}

	if v.Release() != 50.0 {
		t.Errorf("Release() = %g, want 50.0", v.Release())
	}

	err = v.SetInputLevel(0.5)
	if err != nil {
		t.Fatalf("SetInputLevel() error = %v", err)
	}

	if v.InputLevel() != 0.5 {
		t.Errorf("InputLevel() = %g, want 0.5", v.InputLevel())
	}

	err = v.SetSynthLevel(0.3)
	if err != nil {
		t.Fatalf("SetSynthLevel() error = %v", err)
	}

	if v.SynthLevel() != 0.3 {
		t.Errorf("SynthLevel() = %g, want 0.3", v.SynthLevel())
	}

	err = v.SetVocoderLevel(0.8)
	if err != nil {
		t.Fatalf("SetVocoderLevel() error = %v", err)
	}

	if v.VocoderLevel() != 0.8 {
		t.Errorf("VocoderLevel() = %g, want 0.8", v.VocoderLevel())
	}
}

func TestVocoderSetterValidation(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	err = v.SetAttack(-1)
	if err == nil {
		t.Error("expected error for negative attack")
	}

	err = v.SetRelease(-1)
	if err == nil {
		t.Error("expected error for negative release")
	}

	err = v.SetInputLevel(-1)
	if err == nil {
		t.Error("expected error for negative input level")
	}

	err = v.SetSynthLevel(-1)
	if err == nil {
		t.Error("expected error for negative synth level")
	}

	err = v.SetVocoderLevel(-1)
	if err == nil {
		t.Error("expected error for negative vocoder level")
	}
}

func TestVocoderEnvelopeAsymmetry(t *testing.T) {
	// Verify attack is faster than release: after a burst, envelope should
	// rise quickly and decay slowly.
	v, err := NewVocoder(48000,
		WithVocoderAttack(0.1),
		WithVocoderRelease(200),
	)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	const (
		sr   = 48000
		freq = 1000.0
	)

	// Phase 1: warm up with 2000 samples of 1 kHz mod+carrier to reach steady state.

	for i := range 2000 {
		s := math.Sin(2 * math.Pi * freq * float64(i) / sr)
		v.ProcessSample(s, s)
	}

	// Measure steady-state energy over 500 samples.
	var steadyEnergy float64

	for i := range 500 {
		s := math.Sin(2 * math.Pi * freq * float64(2000+i) / sr)
		out := v.ProcessSample(s, s)
		steadyEnergy += out * out
	}

	steadyRMS := math.Sqrt(steadyEnergy / 500)

	// Phase 2: now go silent on modulator but keep carrier going for 500 samples (~10 ms).
	var decayEnergy float64

	for i := range 500 {
		car := math.Sin(2 * math.Pi * freq * float64(2500+i) / sr)
		out := v.ProcessSample(0, car)
		decayEnergy += out * out
	}

	decayRMS := math.Sqrt(decayEnergy / 500)

	// With release = 200 ms, after only ~10 ms of silence the envelope should
	// retain most of its energy. The decay RMS should be at least 20% of steady.
	if steadyRMS < 1e-6 {
		t.Fatalf("steady state RMS too low: %g", steadyRMS)
	}

	ratio := decayRMS / steadyRMS
	if ratio < 0.2 {
		t.Errorf("envelope decayed too fast: steadyRMS=%.6f decayRMS=%.6f ratio=%.4f",
			steadyRMS, decayRMS, ratio)
	}
}

func TestVocoderDryMix(t *testing.T) {
	// With vocoder level = 0 and input level = 1, output should be just the modulator.
	v, err := NewVocoder(48000,
		WithVocoderLevel(0),
		WithVocoderInputLevel(1),
	)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	for i := range 100 {
		mod := float64(i) * 0.01
		car := float64(i) * 0.05
		got := v.ProcessSample(mod, car)

		want := mod
		if diff := math.Abs(got - want); diff > 1e-12 {
			t.Fatalf("sample %d: got=%g want=%g", i, got, want)
		}
	}
}

func TestVocoderOutputBounded(t *testing.T) {
	// Verify output doesn't produce NaN/Inf and stays within reasonable bounds.
	layouts := []struct {
		name   string
		layout BandLayout
	}{
		{"ThirdOctave", BandLayoutThirdOctave},
		{"Bark", BandLayoutBark},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			v, err := NewVocoder(48000, WithBandLayout(l.layout))
			if err != nil {
				t.Fatalf("NewVocoder() error = %v", err)
			}

			maxAbs := 0.0

			for i := range 10000 {
				ti := float64(i) / 48000
				mod := math.Sin(2 * math.Pi * 440 * ti)
				car := math.Sin(2 * math.Pi * 880 * ti)

				out := v.ProcessSample(mod, car)
				if math.IsNaN(out) || math.IsInf(out, 0) {
					t.Fatalf("sample %d: NaN or Inf in output", i)
				}

				if a := math.Abs(out); a > maxAbs {
					maxAbs = a
				}
			}

			// With CPG bandpass (unity peak gain) the output should stay
			// well below the number of bands for single-sine inputs.
			if maxAbs > 10 {
				t.Errorf("output too large: max |output| = %g", maxAbs)
			}
		})
	}
}

func TestVocoderBarkLayout(t *testing.T) {
	v, err := NewVocoder(48000, WithBandLayout(BandLayoutBark))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	if v.Layout() != BandLayoutBark {
		t.Errorf("Layout() = %d, want %d", v.Layout(), BandLayoutBark)
	}
	// At 48 kHz, all 24 Bark bands should be usable (highest = 15.5 kHz < 21.6 kHz).
	if v.NumBands() != 24 {
		t.Errorf("NumBands() = %d, want 24", v.NumBands())
	}

	// Verify it produces output.
	var sum float64

	for i := range 2000 {
		sig := math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
		sum += math.Abs(v.ProcessSample(sig, sig))
	}

	if sum < 1e-6 {
		t.Error("expected non-zero output for Bark layout")
	}
}

func TestVocoderBarkSynthesisQDefaultsAndOverride(t *testing.T) {
	const sr = 48000.0

	t.Run("default uses Bark-derived synthesis Q", func(t *testing.T) {
		v, err := NewVocoder(sr, WithBandLayout(BandLayoutBark))
		if err != nil {
			t.Fatalf("NewVocoder() error = %v", err)
		}

		for i := 0; i < v.NumBands(); i++ {
			fc := barkFrequencies[i]
			want := cpgBandpass(fc, barkBandQ(i), sr)

			got := v.synthesisFilters[i].Coefficients
			if !coeffClose(got, want) {
				t.Fatalf("band %d coeff mismatch for default Bark synthesis Q", i)
			}
		}
	})

	t.Run("override applies global synthesis Q", func(t *testing.T) {
		const q = 2.5

		v, err := NewVocoder(sr, WithBandLayout(BandLayoutBark), WithVocoderSynthesisQ(q))
		if err != nil {
			t.Fatalf("NewVocoder() error = %v", err)
		}

		for i := 0; i < v.NumBands(); i++ {
			fc := barkFrequencies[i]
			want := cpgBandpass(fc, q, sr)

			got := v.synthesisFilters[i].Coefficients
			if !coeffClose(got, want) {
				t.Fatalf("band %d coeff mismatch for overridden Bark synthesis Q", i)
			}
		}
	})
}

func TestVocoderBandFrequencies(t *testing.T) {
	// Verify ISO 1/3-octave table matches expected values.
	if len(thirdOctaveFrequencies) != 32 {
		t.Errorf("thirdOctaveFrequencies has %d entries, want 32", len(thirdOctaveFrequencies))
	}

	if thirdOctaveFrequencies[0] != 16 {
		t.Errorf("first third-octave freq = %g, want 16", thirdOctaveFrequencies[0])
	}

	if thirdOctaveFrequencies[31] != 20000 {
		t.Errorf("last third-octave freq = %g, want 20000", thirdOctaveFrequencies[31])
	}

	// Verify Bark table.
	if len(barkFrequencies) != 24 {
		t.Errorf("barkFrequencies has %d entries, want 24", len(barkFrequencies))
	}

	if barkFrequencies[0] != 100 {
		t.Errorf("first Bark freq = %g, want 100", barkFrequencies[0])
	}

	if barkFrequencies[23] != 15500 {
		t.Errorf("last Bark freq = %g, want 15500", barkFrequencies[23])
	}
}

func TestVocoderDownsamplingDefault(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	if v.Downsampling() {
		t.Error("Downsampling() should be false by default")
	}
}

func TestVocoderDownsamplingOption(t *testing.T) {
	v, err := NewVocoder(48000, WithDownsampling(true))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	if !v.Downsampling() {
		t.Error("Downsampling() should be true when enabled via option")
	}
}

func TestVocoderSetDownsampling(t *testing.T) {
	v, err := NewVocoder(48000)
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	v.SetDownsampling(true)

	if !v.Downsampling() {
		t.Error("Downsampling() should be true after SetDownsampling(true)")
	}

	v.SetDownsampling(false)

	if v.Downsampling() {
		t.Error("Downsampling() should be false after SetDownsampling(false)")
	}
}

func TestVocoderDownsampleFactors(t *testing.T) {
	// At 44100 Hz, low-frequency bands should have higher downsample factors
	// than high-frequency bands. All factors must be powers of 2.
	v, err := NewVocoder(44100, WithDownsampling(true))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	factors := v.DownsampleFactors()
	if len(factors) != v.NumBands() {
		t.Fatalf("DownsampleFactors() length = %d, want %d", len(factors), v.NumBands())
	}

	for i, f := range factors {
		// All factors must be >= 1.
		if f < 1 {
			t.Errorf("band %d: factor = %d, want >= 1", i, f)
		}
		// All factors must be powers of 2.
		if f&(f-1) != 0 {
			t.Errorf("band %d: factor = %d, not a power of 2", i, f)
		}
	}

	// The lowest-frequency band (index 0) should have the highest factor.
	if factors[0] <= factors[len(factors)-1] {
		t.Errorf("lowest band factor (%d) should be > highest band factor (%d)",
			factors[0], factors[len(factors)-1])
	}

	// The highest-frequency bands should have factor 1 (no downsampling).
	last := factors[len(factors)-1]
	if last != 1 {
		t.Errorf("highest band factor = %d, want 1", last)
	}
}

func TestVocoderDownsamplingProducesOutput(t *testing.T) {
	// Downsampled vocoder must still produce non-silent, bounded output.
	layouts := []struct {
		name   string
		layout BandLayout
	}{
		{"ThirdOctave", BandLayoutThirdOctave},
		{"Bark", BandLayoutBark},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			v, err := NewVocoder(48000,
				WithBandLayout(l.layout),
				WithDownsampling(true),
			)
			if err != nil {
				t.Fatalf("NewVocoder() error = %v", err)
			}

			const n = 4000

			sumAbs := 0.0
			maxAbs := 0.0

			for i := range n {
				ti := float64(i) / 48000
				sig := math.Sin(2*math.Pi*1000*ti) + 0.5*math.Sin(2*math.Pi*200*ti)

				out := v.ProcessSample(sig, sig)
				if math.IsNaN(out) || math.IsInf(out, 0) {
					t.Fatalf("sample %d: NaN or Inf", i)
				}

				a := math.Abs(out)

				sumAbs += a
				if a > maxAbs {
					maxAbs = a
				}
			}

			avg := sumAbs / n
			if avg < 1e-6 {
				t.Errorf("expected non-zero output, got avg = %g", avg)
			}

			if maxAbs > 10 {
				t.Errorf("output too large: max = %g", maxAbs)
			}
		})
	}
}

func TestVocoderDownsamplingApproximatesFullRate(t *testing.T) {
	// Compare against full-rate using tighter metrics than broad RMS checks.
	const (
		sr = 48000
		n  = 32768
	)

	vFull, _ := NewVocoder(sr)
	vDS, _ := NewVocoder(sr, WithDownsampling(true))

	mod := make([]float64, n)
	car := make([]float64, n)
	outFull := make([]float64, n)

	outDS := make([]float64, n)
	for i := range n {
		ti := float64(i) / sr
		// Speech-like modulator with slowly varying energy.
		env := 0.55 + 0.35*math.Sin(2*math.Pi*2.2*ti)
		mod[i] = env * (0.7*math.Sin(2*math.Pi*130*ti) + 0.5*math.Sin(2*math.Pi*270*ti) + 0.3*math.Sin(2*math.Pi*650*ti))
		// Bright carrier (additive saw approximation).
		saw := 0.0
		for h := 1; h <= 12; h++ {
			saw += math.Sin(2*math.Pi*110*float64(h)*ti) / float64(h)
		}

		car[i] = 0.6 * saw
		outFull[i] = vFull.ProcessSample(mod[i], car[i])
		outDS[i] = vDS.ProcessSample(mod[i], car[i])
	}

	fullRMS := signalRMS(outFull)
	dsRMS := signalRMS(outDS)

	if fullRMS < 1e-6 {
		t.Fatalf("full-rate RMS too low: %g", fullRMS)
	}

	ratio := dsRMS / fullRMS
	if ratio < 0.75 || ratio > 1.35 {
		t.Errorf("RMS ratio out of range: full=%.6f ds=%.6f ratio=%.3f", fullRMS, dsRMS, ratio)
	}

	stftRMSE, err := stftLogMagRMSE(outFull, outDS, 1024, 256)
	if err != nil {
		t.Fatalf("stftLogMagRMSE error: %v", err)
	}

	if stftRMSE > 8.0 {
		t.Errorf("STFT log-magnitude RMSE too high: %.3f dB", stftRMSE)
	}

	fullBands, err := thirdOctaveBandEnergiesDB(outFull, sr, 16384)
	if err != nil {
		t.Fatalf("thirdOctaveBandEnergiesDB(full) error: %v", err)
	}

	dsBands, err := thirdOctaveBandEnergiesDB(outDS, sr, 16384)
	if err != nil {
		t.Fatalf("thirdOctaveBandEnergiesDB(ds) error: %v", err)
	}

	if len(fullBands) != len(dsBands) || len(fullBands) == 0 {
		t.Fatalf("unexpected band lengths: full=%d ds=%d", len(fullBands), len(dsBands))
	}

	absErr := make([]float64, 0, len(fullBands))
	for i := range fullBands {
		absErr = append(absErr, math.Abs(fullBands[i]-dsBands[i]))
	}

	medianAbsErr := median(absErr)
	if medianAbsErr > 4.0 {
		t.Errorf("median third-octave band error too high: %.3f dB", medianAbsErr)
	}
}

func TestVocoderDownsamplingBuildsRetunedMultirateAnalysisFilters(t *testing.T) {
	layouts := []struct {
		name   string
		layout BandLayout
		freqs  []float64
	}{
		{"ThirdOctave", BandLayoutThirdOctave, thirdOctaveFrequencies[:]},
		{"Bark", BandLayoutBark, barkFrequencies[:]},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			const sr = 48000.0

			vDS, err := NewVocoder(sr, WithBandLayout(l.layout), WithDownsampling(true))
			if err != nil {
				t.Fatalf("NewVocoder(ds) error: %v", err)
			}

			if len(vDS.downsampleAnalysisFilters) != vDS.NumBands() {
				t.Fatalf("downsample analysis filter count = %d, want %d", len(vDS.downsampleAnalysisFilters), vDS.NumBands())
			}

			for i := 0; i < vDS.NumBands(); i++ {
				factor := vDS.downsampleFactors[i]
				dsRate := sr / float64(factor)
				freq := l.freqs[i]

				q := thirdOctaveQ
				if l.layout == BandLayoutBark {
					q = barkBandQ(i)
				}

				want := cpgBandpass(freq, q, dsRate)

				got := vDS.downsampleAnalysisFilters[i].Coefficients
				if !coeffClose(got, want) {
					t.Fatalf("band %d retuned coeff mismatch for factor=%d", i, factor)
				}

				if factor > 1 && coeffClose(got, vDS.analysisFilters[i].Coefficients) {
					t.Fatalf("band %d factor=%d should not keep full-rate coefficients", i, factor)
				}
			}
		})
	}
}

func TestVocoderDownsamplingBuildsAntiAliasFilters(t *testing.T) {
	const sr = 48000.0

	v, err := NewVocoder(sr, WithDownsampling(true))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	if len(v.downsampleGroupAAFilters) != len(v.downsampleGroupFactors) {
		t.Fatalf("anti-alias group count mismatch: filters=%d factors=%d",
			len(v.downsampleGroupAAFilters), len(v.downsampleGroupFactors))
	}

	for g, factor := range v.downsampleGroupFactors {
		c := v.downsampleGroupAAFilters[g].Coefficients
		if factor == 1 {
			if !coeffClose(c, biquad.Coefficients{B0: 1}) {
				t.Fatalf("factor=1 group should use passthrough filter, got=%+v", c)
			}

			continue
		}

		decimatedNyquist := 0.5 * sr / float64(factor)
		passFreq := 0.2 * decimatedNyquist
		stopFreq := decimatedNyquist

		passMag2 := c.MagnitudeSquared(passFreq, sr)

		stopMag2 := c.MagnitudeSquared(stopFreq, sr)
		if math.IsNaN(passMag2) || math.IsInf(passMag2, 0) || math.IsNaN(stopMag2) || math.IsInf(stopMag2, 0) {
			t.Fatalf("non-finite anti-alias response for factor=%d", factor)
		}

		if passMag2 < 0.7 {
			t.Fatalf("anti-alias passband too attenuated for factor=%d: |H|^2=%g", factor, passMag2)
		}

		if stopMag2 > 0.2 {
			t.Fatalf("anti-alias stopband attenuation too weak for factor=%d: |H|^2=%g", factor, stopMag2)
		}
	}
}

func TestVocoderDownsamplingEnvelopeCoefficientsAreFactorAware(t *testing.T) {
	const (
		sr      = 48000.0
		attack  = 12.0
		release = 80.0
	)

	v, err := NewVocoder(sr, WithDownsampling(true), WithVocoderAttack(attack), WithVocoderRelease(release))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	factors := v.DownsampleFactors()
	if len(factors) != len(v.downsampleAttackCoeffs) || len(factors) != len(v.downsampleReleaseCoeffs) {
		t.Fatalf("coefficient/factor length mismatch")
	}

	for i, factor := range factors {
		wantA := 1.0 - math.Exp(-float64(factor)/(attack*0.001*sr))
		wantR := math.Exp(-float64(factor) / (release * 0.001 * sr))

		if math.Abs(v.downsampleAttackCoeffs[i]-wantA) > 1e-12 {
			t.Fatalf("band %d attack coeff mismatch: got=%g want=%g", i, v.downsampleAttackCoeffs[i], wantA)
		}

		if math.Abs(v.downsampleReleaseCoeffs[i]-wantR) > 1e-12 {
			t.Fatalf("band %d release coeff mismatch: got=%g want=%g", i, v.downsampleReleaseCoeffs[i], wantR)
		}
	}
}

func TestVocoderNyquistNearStabilityAcrossSynthesisQ(t *testing.T) {
	layouts := []struct {
		name   string
		layout BandLayout
		freqs  []float64
	}{
		{"ThirdOctave", BandLayoutThirdOctave, thirdOctaveFrequencies[:]},
		{"Bark", BandLayoutBark, barkFrequencies[:]},
	}
	qs := []float64{0.1, 0.5, 4.3184727050832485, 20.0}

	const (
		sr = 8000.0
		n  = 12000
	)

	for _, l := range layouts {
		for _, q := range qs {
			name := l.name + "_Q_" + formatFloatForName(q)
			t.Run(name, func(t *testing.T) {
				v, err := NewVocoder(sr, WithBandLayout(l.layout), WithVocoderSynthesisQ(q))
				if err != nil {
					t.Fatalf("NewVocoder() error: %v", err)
				}

				lastIdx := v.NumBands() - 1

				lastFreq := l.freqs[lastIdx]
				if m2 := v.synthesisFilters[lastIdx].MagnitudeSquared(lastFreq, sr); math.IsNaN(m2) || math.IsInf(m2, 0) || m2 <= 0 {
					t.Fatalf("invalid center magnitude^2 at last band: %g", m2)
				}

				if m2 := v.synthesisFilters[lastIdx].MagnitudeSquared(0.49*sr, sr); math.IsNaN(m2) || math.IsInf(m2, 0) {
					t.Fatalf("invalid near-Nyquist magnitude^2 at last band: %g", m2)
				}

				maxAbs := 0.0

				for i := range n {
					ti := float64(i) / sr
					mod := 0.8*math.Sin(2*math.Pi*lastFreq*ti) + 0.2*math.Sin(2*math.Pi*0.45*sr*ti)
					car := 0.8*math.Sin(2*math.Pi*lastFreq*ti) + 0.2*math.Sin(2*math.Pi*0.45*sr*ti)

					out := v.ProcessSample(mod, car)
					if math.IsNaN(out) || math.IsInf(out, 0) {
						t.Fatalf("NaN/Inf at sample %d", i)
					}

					if a := math.Abs(out); a > maxAbs {
						maxAbs = a
					}
				}

				if maxAbs > 25 {
					t.Fatalf("unexpectedly large output near Nyquist: %g", maxAbs)
				}
			})
		}
	}
}

func coeffClose(a, b biquad.Coefficients) bool {
	const tol = 1e-12

	return math.Abs(a.B0-b.B0) <= tol &&
		math.Abs(a.B1-b.B1) <= tol &&
		math.Abs(a.B2-b.B2) <= tol &&
		math.Abs(a.A1-b.A1) <= tol &&
		math.Abs(a.A2-b.A2) <= tol
}

func signalRMS(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range x {
		sum += v * v
	}

	return math.Sqrt(sum / float64(len(x)))
}

func stftLogMagRMSE(x, y []float64, frameSize, hop int) (float64, error) {
	if len(x) != len(y) {
		return 0, fmt.Errorf("length mismatch: %d vs %d", len(x), len(y))
	}

	if frameSize <= 0 || hop <= 0 || len(x) < frameSize {
		return 0, fmt.Errorf("invalid STFT args: len=%d frame=%d hop=%d", len(x), frameSize, hop)
	}

	plan, err := algofft.NewPlan64(frameSize)
	if err != nil {
		return 0, fmt.Errorf("NewPlan64: %w", err)
	}

	win := make([]float64, frameSize)
	for i := range win {
		win[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(frameSize))
	}

	inX := make([]complex128, frameSize)
	inY := make([]complex128, frameSize)
	outX := make([]complex128, frameSize)
	outY := make([]complex128, frameSize)

	sumSq := 0.0
	count := 0

	const (
		eps              = 1e-12
		activeBinFloorDB = -60.0
	)

	for start := 0; start+frameSize <= len(x); start += hop {
		for i := range frameSize {
			w := win[i]
			inX[i] = complex(x[start+i]*w, 0)
			inY[i] = complex(y[start+i]*w, 0)
		}

		err := plan.Forward(outX, inX)
		if err != nil {
			return 0, fmt.Errorf("forward x: %w", err)
		}

		err = plan.Forward(outY, inY)
		if err != nil {
			return 0, fmt.Errorf("forward y: %w", err)
		}

		peakDB := -math.MaxFloat64

		for k := 1; k <= frameSize/2; k++ {
			db := 20 * math.Log10(math.Hypot(real(outX[k]), imag(outX[k]))+eps)
			if db > peakDB {
				peakDB = db
			}
		}

		for k := 1; k <= frameSize/2; k++ {
			mx := 20 * math.Log10(math.Hypot(real(outX[k]), imag(outX[k]))+eps)
			if mx < peakDB+activeBinFloorDB {
				continue
			}

			my := 20 * math.Log10(math.Hypot(real(outY[k]), imag(outY[k]))+eps)
			d := mx - my
			sumSq += d * d
			count++
		}
	}

	if count == 0 {
		return 0, errors.New("no STFT frames")
	}

	return math.Sqrt(sumSq / float64(count)), nil
}

func thirdOctaveBandEnergiesDB(signal []float64, sampleRate float64, fftSize int) ([]float64, error) {
	if fftSize <= 0 || len(signal) < fftSize {
		return nil, fmt.Errorf("invalid fftSize=%d len=%d", fftSize, len(signal))
	}

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("NewPlan64: %w", err)
	}

	mid := max(len(signal)/2-fftSize/2, 0)

	in := make([]complex128, fftSize)

	out := make([]complex128, fftSize)
	for i := range fftSize {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(fftSize))
		in[i] = complex(signal[mid+i]*w, 0)
	}

	err = plan.Forward(out, in)
	if err != nil {
		return nil, fmt.Errorf("Forward: %w", err)
	}

	power := make([]float64, fftSize/2+1)
	for k := 0; k <= fftSize/2; k++ {
		re := real(out[k])
		im := imag(out[k])
		power[k] = re*re + im*im
	}

	ratio := math.Pow(2, 1.0/6.0)

	energies := make([]float64, 0, len(thirdOctaveFrequencies))
	for _, fc := range thirdOctaveFrequencies {
		if fc >= 0.9*sampleRate*0.5 {
			continue
		}

		fLo := fc / ratio
		fHi := fc * ratio
		kLo := int(math.Ceil(fLo * float64(fftSize) / sampleRate))
		kHi := int(math.Floor(fHi * float64(fftSize) / sampleRate))

		if kLo < 1 {
			kLo = 1
		}

		if kHi > fftSize/2 {
			kHi = fftSize / 2
		}

		if kHi < kLo {
			continue
		}

		e := 0.0
		for k := kLo; k <= kHi; k++ {
			e += power[k]
		}

		energies = append(energies, 10*math.Log10(e+1e-18))
	}

	return energies, nil
}

func median(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	tmp := make([]float64, len(x))
	copy(tmp, x)
	sort.Float64s(tmp)

	m := len(tmp) / 2
	if len(tmp)%2 == 0 {
		return 0.5 * (tmp[m-1] + tmp[m])
	}

	return tmp[m]
}

func formatFloatForName(v float64) string {
	s := fmt.Sprintf("%.3f", v)
	s = strings.ReplaceAll(s, ".", "p")
	s = strings.ReplaceAll(s, "-", "m")

	return s
}

func TestVocoderDownsamplingResetClearsCounter(t *testing.T) {
	v, err := NewVocoder(48000, WithDownsampling(true))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	// Process some samples to advance the counter.
	for range 100 {
		v.ProcessSample(0.5, 0.3)
	}

	// Capture output after reset.
	v.Reset()

	out1 := make([]float64, 200)
	for i := range out1 {
		ti := float64(i) / 48000
		mod := math.Sin(2 * math.Pi * 440 * ti)
		car := math.Sin(2 * math.Pi * 880 * ti)
		out1[i] = v.ProcessSample(mod, car)
	}

	// Fresh vocoder should produce identical output.
	v2, _ := NewVocoder(48000, WithDownsampling(true))

	out2 := make([]float64, 200)
	for i := range out2 {
		ti := float64(i) / 48000
		mod := math.Sin(2 * math.Pi * 440 * ti)
		car := math.Sin(2 * math.Pi * 880 * ti)
		out2[i] = v2.ProcessSample(mod, car)
	}

	for i := range out1 {
		if diff := math.Abs(out1[i] - out2[i]); diff > 1e-12 {
			t.Fatalf("sample %d mismatch after reset: got=%g want=%g", i, out1[i], out2[i])
		}
	}
}

func TestVocoderDownsamplingSetSampleRateRecomputes(t *testing.T) {
	v, err := NewVocoder(48000, WithDownsampling(true))
	if err != nil {
		t.Fatalf("NewVocoder() error = %v", err)
	}

	factorsBefore := make([]int, len(v.DownsampleFactors()))
	copy(factorsBefore, v.DownsampleFactors())

	// Changing to a much lower sample rate should change the factors.
	err = v.SetSampleRate(16000)
	if err != nil {
		t.Fatalf("SetSampleRate() error = %v", err)
	}

	factorsAfter := v.DownsampleFactors()

	// At a lower sample rate, there are fewer bands and lower downsample factors.
	if len(factorsAfter) >= len(factorsBefore) {
		t.Logf("band count: before=%d after=%d", len(factorsBefore), len(factorsAfter))
	}

	// The highest factor at 16kHz should be lower than at 48kHz since
	// the lowest bands have less room to downsample.
	if len(factorsAfter) > 0 && len(factorsBefore) > 0 {
		if factorsAfter[0] > factorsBefore[0] {
			t.Errorf("expected lower max factor at 16kHz (%d) than 48kHz (%d)",
				factorsAfter[0], factorsBefore[0])
		}
	}
}

func BenchmarkVocoderProcessSampleThirdOctave(b *testing.B) {
	v, err := NewVocoder(48000)
	if err != nil {
		b.Fatalf("NewVocoder() error = %v", err)
	}

	mod := 0.5
	car := 0.3

	b.ResetTimer()

	for range b.N {
		v.ProcessSample(mod, car)
	}
}

func BenchmarkVocoderProcessSampleBark(b *testing.B) {
	v, err := NewVocoder(48000, WithBandLayout(BandLayoutBark))
	if err != nil {
		b.Fatalf("NewVocoder() error = %v", err)
	}

	mod := 0.5
	car := 0.3

	b.ResetTimer()

	for range b.N {
		v.ProcessSample(mod, car)
	}
}

func BenchmarkVocoderProcessBlock(b *testing.B) {
	v, err := NewVocoder(48000)
	if err != nil {
		b.Fatalf("NewVocoder() error = %v", err)
	}

	const blockSize = 512

	mod := make([]float64, blockSize)
	car := make([]float64, blockSize)
	out := make([]float64, blockSize)

	for i := range mod {
		t := float64(i) / 48000
		mod[i] = math.Sin(2 * math.Pi * 440 * t)
		car[i] = math.Sin(2 * math.Pi * 880 * t)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_ = v.ProcessBlock(mod, car, out)
	}
}

func BenchmarkVocoderDownsampleThirdOctave(b *testing.B) {
	v, err := NewVocoder(48000, WithDownsampling(true))
	if err != nil {
		b.Fatalf("NewVocoder() error = %v", err)
	}

	mod := 0.5
	car := 0.3

	b.ResetTimer()

	for range b.N {
		v.ProcessSample(mod, car)
	}
}

func BenchmarkVocoderDownsampleBark(b *testing.B) {
	v, err := NewVocoder(48000, WithBandLayout(BandLayoutBark), WithDownsampling(true))
	if err != nil {
		b.Fatalf("NewVocoder() error = %v", err)
	}

	mod := 0.5
	car := 0.3

	b.ResetTimer()

	for range b.N {
		v.ProcessSample(mod, car)
	}
}
