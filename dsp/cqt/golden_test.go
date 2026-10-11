package cqt

import (
	"math"
	"slices"
	"testing"
)

// Tolerances for the golden tests, in the per-bin metric of perBinError.
//
// tolF64 bounds the difference to nnAudio run in float64 (fixture field
// "f64"). The remaining difference comes from libm (sin/cos/pow), summation
// order and FMA contraction; the measured maximum over all fixtures is
// 1.3e-13 (basic_pitch_short; see the log output of go test -v -run TestGolden).
//
// tolF32 bounds the difference to nnAudio as shipped in float32 (fixture field
// "f32", what basic-pitch computes). That is dominated by nnAudio's own
// float32 rounding, about 1.3e-6 in this metric; 1e-5 is the exit criterion
// of PLAN.md task 46.1.
const (
	tolF64 = 1e-12
	tolF32 = 1e-5
)

// perBinError returns max |got-ref| / scale(bin) over all elements, where
// scale(bin) is the largest reference magnitude of that bin over all frames.
// For complex output the real and imaginary parts are compared separately
// against the scale of the complex magnitudes. Normalizing per bin keeps the
// quiet low bins from being judged against the loud ones.
func perBinError(got, ref []float64, frames, bins int, complexOut bool) (float64, int) {
	width := 1
	if complexOut {
		width = 2
	}

	worst, worstBin := 0.0, -1

	for b := range bins {
		scale := 0.0

		for fr := range frames {
			i := (fr*bins + b) * width
			m := math.Abs(ref[i])

			if complexOut {
				m = math.Hypot(ref[i], ref[i+1])
			}

			scale = max(scale, m)
		}

		if scale == 0 {
			scale = math.SmallestNonzeroFloat64
		}

		for fr := range frames {
			for c := range width {
				i := (fr*bins+b)*width + c

				e := math.Abs(got[i]-ref[i]) / scale
				if e > worst || math.IsNaN(e) {
					worst, worstBin = e, b
				}
			}
		}
	}

	return worst, worstBin
}

// TestGolden runs every fixture through NNAudio() followed by the options
// that describe the fixture's configuration.
func TestGolden(t *testing.T) {
	t.Parallel()

	signal := testSignal(t)

	for _, fx := range loadFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			t.Parallel()

			checkGolden(t, fx, signal[:fx.Length], fx.Config.SR, fx.Config.options(t))
		})
	}
}

// TestGoldenPresets runs the fixtures of the preset configurations through
// the preset alone, without any option derived from the fixture: BasicPitch()
// must reproduce basic-pitch's front end (including the short signal, which
// needs the zero-pad fallback of short octaves) and NNAudio() nnAudio's
// defaults, at the same tolerances as TestGolden.
func TestGoldenPresets(t *testing.T) {
	t.Parallel()

	signal := testSignal(t)

	tests := []struct {
		fixture string
		sr      float64
		opts    []Option
	}{
		{"basic_pitch", BasicPitchSampleRate, BasicPitch()},
		{"basic_pitch_short", BasicPitchSampleRate, BasicPitch()},
		// A preset cannot set the sample rate; nnAudio's default is 22050 Hz.
		{"nnaudio_defaults", 22050, NNAudio()},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			fx := loadFixture(t, tc.fixture)
			if fx.Config.SR != tc.sr {
				t.Fatalf("fixture sample rate %g, want %g", fx.Config.SR, tc.sr)
			}

			checkGolden(t, fx, signal[:fx.Length], tc.sr, tc.opts)
		})
	}
}

func checkGolden(t *testing.T, fx fixture, x []float64, sr float64, opts []Option) {
	t.Helper()

	tr, err := New(sr, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	checkSetup(t, tr, fx)

	frames := tr.FrameCount(len(x))
	if frames != fx.Frames {
		t.Fatalf("FrameCount = %d, want %d", frames, fx.Frames)
	}

	complexOut := fx.Config.Output == "complex"
	if complexOut != (tr.output == OutputComplex) {
		t.Fatalf("output %v, fixture output %q", tr.output, fx.Config.Output)
	}

	got, err := tr.Process(x)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(got) != len(fx.F64) || len(got) != tr.OutputLen(len(x)) {
		t.Fatalf("output length %d, OutputLen %d, want %d", len(got), tr.OutputLen(len(x)), len(fx.F64))
	}

	e64, bin := perBinError(got, fx.F64, frames, tr.Bins(), complexOut)
	t.Logf("vs nnAudio float64: max per-bin error %.3e (bin %d)", e64, bin)

	if !(e64 <= tolF64) {
		t.Errorf("vs nnAudio float64: max per-bin error %.3e at bin %d > %.0e", e64, bin, tolF64)
	}

	if fx.F32 != nil {
		checkF32(t, tr, fx, x, got)
	}
}

func checkSetup(t *testing.T, tr *Transform, fx fixture) {
	t.Helper()

	if tr.NFFT() != fx.NFFT || tr.Octaves() != fx.NOctaves ||
		tr.DownsampleFactor() != fx.DownsampleFactor || tr.Hop() != fx.HopEffective {
		t.Fatalf("NFFT/Octaves/DownsampleFactor/Hop = %d/%d/%d/%d, want %d/%d/%d/%d",
			tr.NFFT(), tr.Octaves(), tr.DownsampleFactor(), tr.Hop(),
			fx.NFFT, fx.NOctaves, fx.DownsampleFactor, fx.HopEffective)
	}

	if want := fx.Config.SR / float64(fx.DownsampleFactor); tr.SampleRate() != want {
		t.Errorf("SampleRate = %g, want %g", tr.SampleRate(), want)
	}

	if got := tr.Bins(); got != fx.Config.NBins {
		t.Errorf("Bins = %d, want %d", got, fx.Config.NBins)
	}

	if want := map[string]Padding{"reflect": PadReflect, "constant": PadZero}[fx.Config.PadMode]; tr.Padding() != want {
		t.Errorf("Padding = %v, want %v (%q)", tr.Padding(), want, fx.Config.PadMode)
	}

	// Frequencies go through math.Pow, which may differ from numpy's pow in
	// the last bit.
	assertClose(t, "Frequencies", tr.Frequencies(), fx.Frequencies, 1e-15, true)

	if !slices.Equal(tr.Lengths(), fx.Lengths) {
		t.Errorf("Lengths = %v, want %v", tr.Lengths(), fx.Lengths)
	}

	assertClose(t, "Lowpass", tr.Lowpass(), fx.Lowpass, 1e-15, false)

	if fx.EarlyLowpass == nil {
		if tr.EarlyLowpass() != nil {
			t.Errorf("EarlyLowpass = %d taps, want nil", len(tr.EarlyLowpass()))
		}
	} else {
		assertClose(t, "EarlyLowpass", tr.EarlyLowpass(), fx.EarlyLowpass, 1e-15, false)
	}

	kernels := tr.Kernels()
	if len(kernels)*tr.NFFT() != len(fx.KernelsReal) {
		t.Fatalf("Kernels: %d x %d, want %d values", len(kernels), tr.NFFT(), len(fx.KernelsReal))
	}

	re := make([]float64, 0, len(fx.KernelsReal))
	im := make([]float64, 0, len(fx.KernelsImag))

	for _, row := range kernels {
		for _, v := range row {
			re = append(re, real(v))
			im = append(im, imag(v))
		}
	}

	// Kernel samples are O(1/length); compare relative to the largest one.
	assertClose(t, "Kernels real", re, fx.KernelsReal, 1e-13, false)
	assertClose(t, "Kernels imag", im, fx.KernelsImag, 1e-13, false)
}

// checkF32 compares ProcessInto and ProcessInto32 against nnAudio as shipped
// in float32.
func checkF32(t *testing.T, tr *Transform, fx fixture, x, got64 []float64) {
	t.Helper()

	frames, bins := tr.FrameCount(len(x)), tr.Bins()
	complexOut := fx.Config.Output == "complex"

	e, bin := perBinError(got64, fx.F32, frames, bins, complexOut)
	t.Logf("vs nnAudio float32: ProcessInto max per-bin error %.3e (bin %d)", e, bin)

	if !(e <= tolF32) {
		t.Errorf("ProcessInto vs nnAudio float32: max per-bin error %.3e at bin %d > %.0e", e, bin, tolF32)
	}

	x32 := make([]float32, len(x))
	for i, v := range x {
		x32[i] = float32(v)
	}

	dst32 := make([]float32, tr.OutputLen(len(x)))

	err := tr.ProcessInto32(dst32, x32)
	if err != nil {
		t.Fatalf("ProcessInto32: %v", err)
	}

	got32 := make([]float64, len(dst32))
	for i, v := range dst32 {
		got32[i] = float64(v)
	}

	e, bin = perBinError(got32, fx.F32, frames, bins, complexOut)
	t.Logf("vs nnAudio float32: ProcessInto32 max per-bin error %.3e (bin %d)", e, bin)

	if !(e <= tolF32) {
		t.Errorf("ProcessInto32 vs nnAudio float32: max per-bin error %.3e at bin %d > %.0e", e, bin, tolF32)
	}

	e, _ = perBinError(fx.F32, fx.F64, frames, bins, complexOut)
	t.Logf("nnAudio float32 vs float64 (reference): %.3e", e)
}

// assertClose checks |got-want| <= tol*scale element-wise, where scale is
// |want[i]| (relative) or max|want| (scaled by the largest value).
func assertClose(t *testing.T, name string, got, want []float64, tol float64, relative bool) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: length %d, want %d", name, len(got), len(want))

		return
	}

	peak := 0.0
	for _, v := range want {
		peak = max(peak, math.Abs(v))
	}

	for i := range want {
		scale := peak
		if relative {
			scale = math.Abs(want[i])
		}

		if d := math.Abs(got[i] - want[i]); !(d <= tol*scale) {
			t.Errorf("%s[%d] = %.17g, want %.17g (diff %.3e, tol %.0e*%.3e)", name, i, got[i], want[i], d, tol, scale)

			return
		}
	}
}
