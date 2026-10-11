package cqt

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// tolExact bounds the per-bin error (see perBinError) between two
// computations of the same transform that differ only in rounding: summation
// order, libm and FMA contraction.
const tolExact = 1e-12

const (
	// testSignalLen is basic-pitch's analysis window, 43844 samples.
	testSignalLen = 43844
	// testSignalRate is the sample rate the test signal is generated for.
	testSignalRate = 22050.0
	// testSignalSeed seeds the noise of the test signal.
	testSignalSeed = 20261010
)

var (
	signalOnce sync.Once
	signalData []float64
)

// testSignal returns the shared test input: testSignalLen samples at
// testSignalRate Hz of an exponential chirp from 20 Hz to 10 kHz (amplitude
// 0.5), stationary tones at 440 Hz (0.3) and 55 Hz (0.2), and Gaussian noise
// (standard deviation 0.05) from a PCG generator with a fixed seed. Every
// sample is rounded to float32, so ProcessInto32 sees exactly the same input
// as ProcessInto. The signal is deterministic; it is not shared with any
// external reference.
//
// The callers must not modify the returned slice.
func testSignal(tb testing.TB) []float64 {
	tb.Helper()

	signalOnce.Do(func() {
		const (
			f0, f1 = 20.0, 10000.0
			dur    = testSignalLen / testSignalRate
		)

		k := math.Log(f1 / f0)
		rng := rand.New(rand.NewPCG(testSignalSeed, 0)) //nolint:gosec // deterministic test noise

		signalData = make([]float64, testSignalLen)
		for i := range signalData {
			t := float64(i) / testSignalRate
			chirp := math.Sin(2 * math.Pi * f0 * dur / k * (math.Exp(k*t/dur) - 1))
			tone := 0.3*math.Sin(2*math.Pi*440*t) + 0.2*math.Sin(2*math.Pi*55*t)
			x := 0.5*chirp + tone + 0.05*rng.NormFloat64()
			signalData[i] = float64(float32(x))
		}
	})

	return signalData
}

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

// refConfig is one of the eight configurations that 46.1 compared against
// nnAudio CQT2010v2 (the deleted scripts/fixtures/cqt/gen.py), expressed as
// options on top of [NNAudio]. length is the signal length that comparison
// used, a prefix of testSignal.
type refConfig struct {
	name   string
	sr     float64
	length int
	opts   []Option
	// fallback reports whether an octave is not longer than nfft/2 at
	// length samples, so that NNAudio() zero pads it instead of reflecting.
	fallback bool
}

// refConfigs returns the eight reference configurations:
//
//   - basic_pitch: basic-pitch's front end ([BasicPitch]);
//   - basic_pitch_short: the same on 4000 samples, whose lower octaves are
//     too short to reflect;
//   - nnaudio_defaults: nnAudio's defaults ([NNAudio]; early downsampling
//     evaluates to a factor of 1);
//   - early_ds8_complex: early downsampling by 8 with complex output;
//   - no_early_ds: the same with early downsampling disabled;
//   - const_l2_conv_complex: zero padding, L2 kernels, convolutional
//     normalization, a partial octave, filter scale 0.8 and complex output;
//   - hamming_nonorm_wrap: unnormalized Hamming kernels, wrap normalization;
//   - blackman: Blackman kernels on part of basic-pitch's range.
func refConfigs() []refConfig {
	earlyDS8 := func(early bool) []Option {
		return append(NNAudio(), WithHopLength(1024), WithFMin(30), WithBins(48),
			WithOutput(OutputComplex), WithEarlyDownsampling(early))
	}

	return []refConfig{
		{name: "basic_pitch", sr: BasicPitchSampleRate, length: testSignalLen, opts: BasicPitch()},
		{name: "basic_pitch_short", sr: BasicPitchSampleRate, length: 4000, opts: BasicPitch(), fallback: true},
		{name: "nnaudio_defaults", sr: 22050, length: testSignalLen, opts: NNAudio()},
		{name: "early_ds8_complex", sr: 22050, length: testSignalLen, opts: earlyDS8(true)},
		{name: "no_early_ds", sr: 22050, length: testSignalLen, opts: earlyDS8(false)},
		{name: "const_l2_conv_complex", sr: 16000, length: 20000, opts: append(NNAudio(),
			WithHopLength(128), WithFMin(50), WithBins(100), WithBinsPerOctave(24),
			WithFilterScale(0.8), WithBasisNorm(NormL2), WithCenter(PadZero),
			WithNormalization(NormalizationConvolutional), WithOutput(OutputComplex))},
		{name: "hamming_nonorm_wrap", sr: 22050, length: 10000, opts: append(NNAudio(),
			WithHopLength(64), WithFMin(100), WithBins(60), WithBinsPerOctave(12),
			WithFilterScale(1.5), WithBasisNorm(NormNone), WithWindow(window.TypeHamming),
			WithNormalization(NormalizationWrap))},
		{name: "blackman", sr: BasicPitchSampleRate, length: testSignalLen, opts: append(BasicPitch(),
			WithWindow(window.TypeBlackman), WithBins(72), WithFMin(110))},
	}
}
