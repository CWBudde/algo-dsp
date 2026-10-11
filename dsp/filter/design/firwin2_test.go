package design_test

import (
	"errors"
	"math"
	"math/bits"
	"math/cmplx"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

// Firwin2 is checked two ways, without any test data files:
//
//   - TestFirwin2Scipy pins scipy.signal.firwin2's taps for the cases that
//     exercise its corner rules (type II, one and two taps, a non-default and
//     odd mesh, a repeated frequency, no window, a multiband response) and for
//     nnAudio's 256-tap anti-aliasing low-pass. The values (scipyXxx below)
//     were extracted once from testdata/firwin2.json.gz, the scipy 1.18.1
//     fixture added in 46.1, which 46.4 deleted.
//   - TestFirwin2Reference compares Firwin2 on every case with
//     firwin2Reference, an independent implementation of the same scipy
//     algorithm that computes the inverse real DFT with algo-fft rather than
//     with Firwin2's direct twiddle-table sum.

// firwin2ScipyTol is the absolute tolerance against the pinned scipy taps.
// Measured against the full fixture before it was deleted, Firwin2 matched all
// 17 cases to <= 2.2e-16 (window_kaiser excepted, see firwin2Reference).
//
// The pinned filters are symmetric, so only their first ceil(numtaps/2) taps
// are stored and the rest are mirrored. scipy's own taps are not bitwise
// symmetric (they come from an FFT): its mirrored taps differ by up to
// 5.2e-15 (type1_multiband; 2.4e-15 for the 256-tap low-pass), so the
// mirrored half is compared at this tolerance rather than at the rounding
// level. Measured against the mirrored taps: Firwin2 <= 5.1e-15 and
// firwin2Reference <= 5.3e-15 (both type1_multiband), and <= 2.2e-16 on the
// stored half.
const firwin2ScipyTol = 1e-14

// firwin2StoredTol is the absolute tolerance on the stored, unmirrored half of
// the pinned scipy taps, where both Firwin2 and firwin2Reference measured
// <= 2.2e-16. It catches regressions that firwin2ScipyTol would let through.
const firwin2StoredTol = 1e-15

// firwin2RefTol is the absolute tolerance between Firwin2 and
// firwin2Reference. Measured on all 17 cases: <= 2.2e-16.
const firwin2RefTol = 1e-14

// firwin2Case is one scipy.signal.firwin2 call. The zero values of nfreqs, fs
// and win select scipy's defaults: nfreqs = 1 + 2^ceil(log2(numtaps)), fs = 2
// (frequencies normalized to Nyquist) and a Hamming window.
type firwin2Case struct {
	name    string
	numtaps int
	freq    []float64
	gain    []float64
	nfreqs  int
	fs      float64
	win     string  // "", "none", "hann", "blackman" or "kaiser"
	beta    float64 // Kaiser beta
	// scipyHalf holds scipy's first ceil(numtaps/2) taps for the pinned
	// cases; it is nil for the cases checked against firwin2Reference only.
	scipyHalf []float64
}

// firwin2Cases are the 17 cases of the deleted 46.1 fixture, with the same
// names. All are checked against firwin2Reference; the ones with scipyHalf
// are also pinned to scipy 1.18.1.
var firwin2Cases = []firwin2Case{
	{
		name: "nnaudio_lowpass", numtaps: 256,
		freq: []float64{0, 0.5 / 1.001, 0.5 * 1.001, 1}, gain: []float64{1, 1, 0, 0},
		scipyHalf: scipyNNAudioLowpass,
	},
	{name: "nnaudio_early_ds2", numtaps: 256, freq: []float64{0, 0.5 / 1.03, 0.5 * 1.03, 1}, gain: []float64{1, 1, 0, 0}},
	{name: "nnaudio_early_ds4", numtaps: 256, freq: []float64{0, 0.25 / 1.03, 0.25 * 1.03, 1}, gain: []float64{1, 1, 0, 0}},
	{name: "type1_lowpass", numtaps: 101, freq: []float64{0, 0.2, 0.3, 1}, gain: []float64{1, 1, 0, 0}},
	{name: "type1_highpass", numtaps: 51, freq: []float64{0, 0.4, 0.5, 1}, gain: []float64{0, 0, 1, 1}},
	{
		name: "type1_multiband", numtaps: 63,
		freq:      []float64{0, 0.1, 0.2, 0.35, 0.6, 0.8, 1},
		gain:      []float64{0.5, 1, 0.25, 0.25, 2, 1, 0.75},
		scipyHalf: scipyType1Multiband,
	},
	{
		name: "step_repeated_freq", numtaps: 65,
		freq: []float64{0, 0.5, 0.5, 1}, gain: []float64{1, 1, 0, 0},
		scipyHalf: scipyStepRepeatedFreq,
	},
	{
		name: "type2_ramp", numtaps: 6,
		freq: []float64{0, 0.5, 1}, gain: []float64{1, 1, 0},
		scipyHalf: scipyType2Ramp,
	},
	{name: "nfreqs_custom", numtaps: 31, freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, nfreqs: 100},
	{
		name: "nfreqs_odd_len", numtaps: 20,
		freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, nfreqs: 33,
		scipyHalf: scipyNFreqsOddLen,
	},
	{
		name: "window_none", numtaps: 41,
		freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, win: "none",
		scipyHalf: scipyWindowNone,
	},
	{name: "window_hann", numtaps: 41, freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, win: "hann"},
	{name: "window_blackman", numtaps: 40, freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, win: "blackman"},
	{
		name: "window_kaiser", numtaps: 45,
		freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}, win: "kaiser", beta: 8.6,
	},
	{name: "fs_hz", numtaps: 51, freq: []float64{0, 1000, 1500, 4000}, gain: []float64{1, 1, 0, 0}, fs: 8000},
	{name: "one_tap", numtaps: 1, freq: []float64{0, 1}, gain: []float64{1, 1}, scipyHalf: scipyOneTap},
	{name: "two_taps", numtaps: 2, freq: []float64{0, 1}, gain: []float64{1, 0}, scipyHalf: scipyTwoTaps},
}

// windowType maps the case's window name to a dsp/window type.
func (c firwin2Case) windowType(t *testing.T) (window.Type, []window.Option) {
	t.Helper()

	switch c.win {
	case "":
		return window.TypeHamming, nil
	case "hann":
		return window.TypeHann, nil
	case "blackman":
		return window.TypeBlackman, nil
	case "kaiser":
		return window.TypeKaiser, []window.Option{window.WithAlpha(c.beta)}
	default:
		t.Fatalf("case %s: unknown window %q", c.name, c.win)

		return 0, nil
	}
}

// options translates the scipy keyword arguments of a case.
func (c firwin2Case) options(t *testing.T) []design.Option {
	t.Helper()

	var opts []design.Option

	if c.nfreqs != 0 {
		opts = append(opts, design.WithNFreqs(c.nfreqs))
	}

	if c.fs != 0 {
		opts = append(opts, design.WithSampleRate(c.fs))
	}

	switch c.win {
	case "":
	case "none":
		opts = append(opts, design.WithoutWindow())
	default:
		wt, wopts := c.windowType(t)
		opts = append(opts, design.WithWindow(wt, wopts...))
	}

	return opts
}

// scipyTaps returns the full pinned scipy taps, mirroring the stored half.
func (c firwin2Case) scipyTaps() []float64 {
	taps := make([]float64, c.numtaps)
	copy(taps, c.scipyHalf)

	for i := len(c.scipyHalf); i < c.numtaps; i++ {
		taps[i] = taps[c.numtaps-1-i]
	}

	return taps
}

// firwin2Reference is an independent implementation of
// scipy.signal.firwin2 (scipy/signal/_fir_filter_design.py) for the
// symmetric types I and II. It follows scipy step by step:
//
//  1. nfreqs defaults to 1 + 2^ceil(log2(numtaps)); nyq = fs/2.
//  2. Each repeated frequency pair is pulled apart by eps*nyq, with eps the
//     float64 machine epsilon: freq[k] -= eps*nyq, freq[k+1] += eps*nyq.
//  3. The gain is interpolated linearly (numpy.interp) on the mesh
//     x = numpy.linspace(0, nyq, nfreqs).
//  4. Each sample is multiplied by exp(-j*pi*(numtaps-1)/2 * x/nyq), which
//     delays the impulse response to the centre of the filter.
//  5. The taps are the first numtaps samples of numpy.fft.irfft of that
//     half spectrum, a real sequence of length 2*(nfreqs-1). irfft ignores
//     the imaginary parts of the DC and Nyquist bins.
//  6. The taps are multiplied by get_window(window, numtaps, fftbins=False),
//     the symmetric window. For a single tap scipy's window is [1].
//
// Step 5 is where Firwin2 and this reference differ in method: Firwin2 sums
// the inverse DFT directly with a twiddle table, this reference runs an
// inverse real FFT from algo-fft (which handles the non-power-of-two size
// 198 of nfreqs_custom).
//
// The window comes from dsp/window, as in Firwin2; the window is not what is
// under test. dsp/window evaluates the Kaiser window with the
// Abramowitz-Stegun polynomial approximation of I0 (about 1e-7 relative
// accuracy), whereas scipy uses a full-precision Bessel function, so for
// window_kaiser both Firwin2 and this reference differ from scipy by about
// 1.5e-10. That is a limitation of dsp/window, not of Firwin2 (46.1
// discovery 4), and is why window_kaiser is not pinned.
//
// Measured against the scipy 1.18.1 fixture before it was deleted, the
// reference matched 16 cases to <= 2.2e-16 and window_kaiser to 1.5e-10.
func firwin2Reference(t *testing.T, c firwin2Case) []float64 {
	t.Helper()

	fs := c.fs
	if fs == 0 {
		fs = 2
	}

	nyq := 0.5 * fs

	nfreqs := c.nfreqs
	if nfreqs == 0 {
		// 2^ceil(log2(n)) is 2^bitlen(n-1) for n >= 1.
		nfreqs = 1 + 1<<bits.Len(uint(c.numtaps-1))
	}

	freq := slices.Clone(c.freq)
	eps := (math.Nextafter(1, 2) - 1) * nyq

	for k := range len(freq) - 1 {
		if freq[k] == freq[k+1] {
			freq[k] -= eps
			freq[k+1] += eps
		}
	}

	// numpy.linspace(0, nyq, nfreqs): i*step, with the end point set exactly.
	step := nyq / float64(nfreqs-1)
	spec := make([]complex128, nfreqs)

	for i := range spec {
		x := float64(i) * step
		if i == nfreqs-1 {
			x = nyq
		}

		shift := cmplx.Exp(complex(0, -float64(c.numtaps-1)/2*math.Pi*x/nyq))
		spec[i] = complex(npInterp(x, freq, c.gain), 0) * shift
	}

	spec[0] = complex(real(spec[0]), 0)
	spec[nfreqs-1] = complex(real(spec[nfreqs-1]), 0)

	size := 2 * (nfreqs - 1)

	plan, err := algofft.NewPlanReal64(size)
	if err != nil {
		t.Fatalf("NewPlanReal64(%d): %v", size, err)
	}

	full := make([]float64, size)

	err = plan.Inverse(full, spec)
	if err != nil {
		t.Fatalf("inverse FFT: %v", err)
	}

	// algo-fft's Inverse is normalized by 1/size, as numpy's irfft is.
	taps := full[:c.numtaps:c.numtaps]

	if c.win == "none" || c.numtaps == 1 {
		return taps
	}

	wt, wopts := c.windowType(t)
	w := window.Generate(wt, c.numtaps, wopts...)

	for i := range taps {
		taps[i] *= w[i]
	}

	return taps
}

// npInterp is numpy.interp for a strictly increasing xp: fp[0] left of the
// grid, fp[last] right of it, linear in between.
func npInterp(x float64, xp, fp []float64) float64 {
	last := len(xp) - 1
	if x <= xp[0] {
		return fp[0]
	}

	for j := range last {
		if x < xp[j+1] {
			if x == xp[j] {
				return fp[j]
			}

			slope := (fp[j+1] - fp[j]) / (xp[j+1] - xp[j])

			return slope*(x-xp[j]) + fp[j]
		}
	}

	return fp[last]
}

func maxAbsDiff(a, b []float64) float64 {
	m := 0.0

	for i := range a {
		m = max(m, math.Abs(a[i]-b[i]))
	}

	return m
}

// checkTaps reports every tap of got that differs from want by more than tol.
func checkTaps(t *testing.T, got, want []float64, tol float64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}

	for i := range got {
		if !(math.Abs(got[i]-want[i]) <= tol) {
			t.Errorf("tap %d = %.17g, want %.17g", i, got[i], want[i])
		}
	}
}

func TestFirwin2Scipy(t *testing.T) {
	pinned := 0

	for _, tc := range firwin2Cases {
		if tc.scipyHalf == nil {
			continue
		}

		pinned++

		t.Run(tc.name, func(t *testing.T) {
			if want := (tc.numtaps + 1) / 2; len(tc.scipyHalf) != want {
				t.Fatalf("%d pinned taps, want %d", len(tc.scipyHalf), want)
			}

			want := tc.scipyTaps()

			got, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.options(t)...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			stored := len(tc.scipyHalf)

			checkTaps(t, got, want, firwin2ScipyTol)
			checkTaps(t, got[:stored], want[:stored], firwin2StoredTol)
			t.Logf("Firwin2 max abs error vs scipy %.3g (tol %.0e)", maxAbsDiff(got, want), firwin2ScipyTol)

			// The reference must reproduce scipy too, or agreeing with it
			// would prove nothing.
			ref := firwin2Reference(t, tc)
			checkTaps(t, ref, want, firwin2ScipyTol)
			checkTaps(t, ref[:stored], want[:stored], firwin2StoredTol)
			t.Logf("reference max abs error vs scipy %.3g", maxAbsDiff(ref, want))
		})
	}

	if pinned != 8 {
		t.Errorf("%d pinned cases, want 8", pinned)
	}
}

func TestFirwin2Reference(t *testing.T) {
	for _, tc := range firwin2Cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.options(t)...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			ref := firwin2Reference(t, tc)
			checkTaps(t, got, ref, firwin2RefTol)
			t.Logf("max abs error vs reference %.3g (tol %.0e)", maxAbsDiff(got, ref), firwin2RefTol)
		})
	}
}

func TestFirwin2Validation(t *testing.T) {
	lpFreq := []float64{0, 0.3, 0.4, 1}
	lpGain := []float64{1, 1, 0, 0}

	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		opts    []design.Option
	}{
		{name: "numtaps zero", numtaps: 0, freq: lpFreq, gain: lpGain},
		{name: "numtaps negative", numtaps: -3, freq: lpFreq, gain: lpGain},
		{name: "length mismatch", numtaps: 11, freq: lpFreq, gain: []float64{1, 1, 0}},
		{name: "single frequency", numtaps: 11, freq: []float64{0}, gain: []float64{1}},
		{name: "empty", numtaps: 11, freq: nil, gain: nil},
		{
			name: "nfreqs equal numtaps", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(11)},
		},
		{
			name: "nfreqs below numtaps", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(5)},
		},
		{
			name: "nfreqs one", numtaps: 1, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(1)},
		},
		{
			name: "nfreqs zero", numtaps: 1, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithNFreqs(0)},
		},
		{name: "NaN frequency", numtaps: 11, freq: []float64{0, math.NaN(), 1}, gain: []float64{1, 1, 0}},
		{name: "Inf frequency", numtaps: 11, freq: []float64{0, math.Inf(1), 1}, gain: []float64{1, 1, 0}},
		{name: "NaN gain", numtaps: 11, freq: []float64{0, 0.5, 1}, gain: []float64{1, math.NaN(), 0}},
		{name: "Inf gain", numtaps: 11, freq: []float64{0, 0.5, 1}, gain: []float64{math.Inf(-1), 1, 0}},
		{name: "does not start at 0", numtaps: 11, freq: []float64{0.1, 0.5, 1}, gain: []float64{1, 1, 0}},
		{name: "does not end at Nyquist", numtaps: 11, freq: []float64{0, 0.5, 0.9}, gain: []float64{1, 1, 0}},
		{
			name: "normalized Nyquist with sample rate", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(8000)},
		},
		{name: "0 repeated", numtaps: 11, freq: []float64{0, 0, 1}, gain: []float64{1, 0, 0}},
		{name: "Nyquist repeated", numtaps: 11, freq: []float64{0, 1, 1}, gain: []float64{1, 1, 0}},
		{name: "decreasing", numtaps: 11, freq: []float64{0, 0.6, 0.4, 1}, gain: lpGain},
		{
			name: "value three times", numtaps: 11,
			freq: []float64{0, 0.5, 0.5, 0.5, 1}, gain: []float64{1, 1, 0.5, 0, 0},
		},
		{
			// The repeated 0.5 is separated by eps*nyq = 2^-52 on each side,
			// which pushes it past its neighbour one ulp above 0.5.
			name: "too close to a repeated value", numtaps: 11,
			freq: []float64{0, 0.5, 0.5, math.Nextafter(0.5, 1), 1},
			gain: []float64{1, 1, 0, 0, 0},
		},
		{name: "type II nonzero Nyquist gain", numtaps: 10, freq: []float64{0, 0.5, 1}, gain: []float64{1, 1, 0.5}},
		{name: "nil option", numtaps: 11, freq: lpFreq, gain: lpGain, opts: []design.Option{nil}},
		{
			name: "unknown window type", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithWindow(window.TypeFreeCosine + 1)},
		},
		{
			name: "negative window type", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithWindow(window.Type(-1))},
		},
		{
			name: "sample rate zero", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(0)},
		},
		{
			name: "sample rate negative", numtaps: 11, freq: []float64{0, 0.5, -1}, gain: []float64{1, 1, 0},
			opts: []design.Option{design.WithSampleRate(-2)},
		},
		{
			name: "sample rate NaN", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.NaN())},
		},
		{
			name: "sample rate +Inf", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.Inf(1))},
		},
		{
			name: "sample rate -Inf", numtaps: 11, freq: lpFreq, gain: lpGain,
			opts: []design.Option{design.WithSampleRate(math.Inf(-1))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.opts...)
			if err == nil {
				t.Fatalf("Firwin2 succeeded with %d taps, want error", len(taps))
			}

			if !errors.Is(err, design.ErrInvalidFirwin2) {
				t.Fatalf("error %q does not wrap ErrInvalidFirwin2", err)
			}

			if taps != nil {
				t.Errorf("taps = %v, want nil on error", taps)
			}
		})
	}
}

func TestFirwin2ValidBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		opts    []design.Option
	}{
		{
			name: "nfreqs just above numtaps", numtaps: 11, freq: []float64{0, 1}, gain: []float64{1, 1},
			opts: []design.Option{design.WithNFreqs(12)},
		},
		{
			name: "minimal mesh", numtaps: 1, freq: []float64{0, 1}, gain: []float64{1, 1},
			opts: []design.Option{design.WithNFreqs(2)},
		},
		{name: "type I nonzero Nyquist gain", numtaps: 11, freq: []float64{0, 1}, gain: []float64{0, 1}},
		{name: "repeated interior frequency", numtaps: 11, freq: []float64{0, 0.5, 0.5, 1}, gain: []float64{1, 1, 0, 0}},
		{
			name: "two adjacent steps", numtaps: 11,
			freq: []float64{0, 0.3, 0.3, 0.6, 0.6, 1}, gain: []float64{1, 1, 0, 0, 1, 1},
		},
		{
			name: "sample rate in Hz", numtaps: 11, freq: []float64{0, 100, 24000}, gain: []float64{1, 1, 0},
			opts: []design.Option{design.WithSampleRate(48000)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.opts...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			if len(taps) != tc.numtaps {
				t.Fatalf("len = %d, want %d", len(taps), tc.numtaps)
			}

			for i, v := range taps {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("tap %d = %v", i, v)
				}
			}
		})
	}
}

func TestFirwin2Symmetric(t *testing.T) {
	for _, tc := range firwin2Cases {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain, tc.options(t)...)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			// The taps come from a DFT, so mirrored taps agree only to
			// rounding (a few 1e-15 for 256 taps; scipy's agree no better).
			n := len(taps)
			for i := range n / 2 {
				if d := math.Abs(taps[i] - taps[n-1-i]); d > 1e-14 {
					t.Errorf("taps[%d] = %.17g, taps[%d] = %.17g, |diff| = %.3g", i, taps[i], n-1-i, taps[n-1-i], d)
				}
			}
		})
	}
}

func TestFirwin2DoesNotModifyInputs(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
	}{
		{name: "plain", numtaps: 31, freq: []float64{0, 0.3, 0.4, 1}, gain: []float64{1, 1, 0, 0}},
		{name: "repeated frequency", numtaps: 31, freq: []float64{0, 0.5, 0.5, 1}, gain: []float64{1, 1, 0, 0}},
		{
			name: "two repeated frequencies", numtaps: 32,
			freq: []float64{0, 0.25, 0.25, 0.75, 0.75, 1}, gain: []float64{0, 0, 1, 1, 0, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			freq := slices.Clone(tc.freq)
			gain := slices.Clone(tc.gain)

			_, err := design.Firwin2(tc.numtaps, freq, gain)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			if !slices.Equal(freq, tc.freq) {
				t.Errorf("freq modified: %v, want %v", freq, tc.freq)
			}

			if !slices.Equal(gain, tc.gain) {
				t.Errorf("gain modified: %v, want %v", gain, tc.gain)
			}
		})
	}
}

// responseAt evaluates |H(e^{j*pi*w})| for w normalized to Nyquist.
func responseAt(taps []float64, w float64) float64 {
	var re, im float64

	for n, h := range taps {
		re += h * math.Cos(math.Pi*w*float64(n))
		im -= h * math.Sin(math.Pi*w*float64(n))
	}

	return math.Hypot(re, im)
}

func TestFirwin2Response(t *testing.T) {
	tests := []struct {
		name    string
		numtaps int
		freq    []float64
		gain    []float64
		check   []float64 // normalized frequencies to probe
		want    []float64 // desired magnitude there
		tol     float64
	}{
		{
			name: "lowpass", numtaps: 101,
			freq: []float64{0, 0.2, 0.3, 1}, gain: []float64{1, 1, 0, 0},
			check: []float64{0, 0.1, 0.5, 1}, want: []float64{1, 1, 0, 0}, tol: 5e-3,
		},
		{
			name: "highpass", numtaps: 101,
			freq: []float64{0, 0.4, 0.5, 1}, gain: []float64{0, 0, 1, 1},
			check: []float64{0, 0.2, 0.7, 1}, want: []float64{0, 0, 1, 1}, tol: 5e-3,
		},
		{
			name: "scaled lowpass", numtaps: 101,
			freq: []float64{0, 0.2, 0.3, 1}, gain: []float64{0.5, 0.5, 0, 0},
			check: []float64{0, 0.1, 0.5}, want: []float64{0.5, 0.5, 0}, tol: 5e-3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			taps, err := design.Firwin2(tc.numtaps, tc.freq, tc.gain)
			if err != nil {
				t.Fatalf("Firwin2: %v", err)
			}

			// The DC gain is the plain sum of the taps.
			var sum float64
			for _, h := range taps {
				sum += h
			}

			if !(math.Abs(sum-tc.gain[0]) <= tc.tol) {
				t.Errorf("DC gain (sum of taps) = %.6g, want %.6g", sum, tc.gain[0])
			}

			for i, w := range tc.check {
				got := responseAt(taps, w)
				if !(math.Abs(got-tc.want[i]) <= tc.tol) {
					t.Errorf("|H(%g)| = %.6g, want %.6g", w, got, tc.want[i])
				}
			}
		})
	}
}

func TestFirwin2Options(t *testing.T) {
	freq := []float64{0, 0.3, 0.4, 1}
	gain := []float64{1, 1, 0, 0}

	design41 := func(t *testing.T, opts ...design.Option) []float64 {
		t.Helper()

		taps, err := design.Firwin2(41, freq, gain, opts...)
		if err != nil {
			t.Fatalf("Firwin2: %v", err)
		}

		return taps
	}

	hann := design41(t, design.WithWindow(window.TypeHann))
	none := design41(t, design.WithoutWindow())
	hamming := design41(t)

	tests := []struct {
		name string
		got  []float64
		want []float64
	}{
		{"default is Hamming", design41(t, design.WithWindow(window.TypeHamming)), hamming},
		{"WithWindow after WithoutWindow", design41(t, design.WithoutWindow(), design.WithWindow(window.TypeHann)), hann},
		{"WithoutWindow after WithWindow", design41(t, design.WithWindow(window.TypeHann), design.WithoutWindow()), none},
		{"later WithWindow wins", design41(t, design.WithWindow(window.TypeBlackman), design.WithWindow(window.TypeHann)), hann},
		{"default nfreqs", design41(t, design.WithNFreqs(65)), hamming},
		{"later WithNFreqs wins", design41(t, design.WithNFreqs(100), design.WithNFreqs(65)), hamming},
		{"default sample rate is 2", design41(t, design.WithSampleRate(2)), hamming},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !slices.Equal(tc.got, tc.want) {
				t.Errorf("taps differ, max abs diff %.3g", maxAbsDiff(tc.got, tc.want))
			}
		})
	}

	if slices.Equal(hann, none) || slices.Equal(hann, hamming) {
		t.Fatal("window options had no effect")
	}

	t.Run("window options are copied", func(t *testing.T) {
		wopts := []window.Option{window.WithAlpha(8.6)}
		opt := design.WithWindow(window.TypeKaiser, wopts...)
		wopts[0] = window.WithAlpha(1)

		got := design41(t, opt)
		want := design41(t, design.WithWindow(window.TypeKaiser, window.WithAlpha(8.6)))

		if !slices.Equal(got, want) {
			t.Errorf("WithWindow aliases its options slice, max abs diff %.3g", maxAbsDiff(got, want))
		}
	})
}

func TestFirwin2SampleRateEquivalence(t *testing.T) {
	norm, err := design.Firwin2(51, []float64{0, 0.25, 0.375, 1}, []float64{1, 1, 0, 0})
	if err != nil {
		t.Fatalf("Firwin2: %v", err)
	}

	hz, err := design.Firwin2(51, []float64{0, 1000, 1500, 4000}, []float64{1, 1, 0, 0}, design.WithSampleRate(8000))
	if err != nil {
		t.Fatalf("Firwin2 with sample rate: %v", err)
	}

	if d := maxAbsDiff(norm, hz); d > 1e-15 {
		t.Errorf("Hz design differs from normalized design by %.3g", d)
	}
}

func BenchmarkFirwin2(b *testing.B) {
	freq := []float64{0, 0.5 / 1.001, 0.5 * 1.001, 1}
	gain := []float64{1, 1, 0, 0}

	b.ReportAllocs()

	for b.Loop() {
		_, err := design.Firwin2(256, freq, gain)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Pinned scipy.signal.firwin2 taps, extracted from the 46.1 fixture
// testdata/firwin2.json.gz (scipy 1.18.1, numpy 2.5.3) before 46.4 deleted
// it, and printed with strconv.FormatFloat(v, 'g', -1, 64) so that every
// value round-trips exactly. Each block holds the first ceil(numtaps/2) taps
// of a symmetric filter; firwin2Case.scipyTaps mirrors the rest.

// nnaudio_lowpass, nnAudio CQT2010v2's anti-aliasing low-pass (first 128 of 256 taps).
// scipy 1.18.1: scipy.signal.firwin2(256, [0, 0.5/1.001, 0.5*1.001, 1], [1, 1, 0, 0]), float64.
var scipyNNAudioLowpass = []float64{
	-0.00011111023799078378, -0.00011278991187029311, 0.00011466784625627201, 0.00011720297252868967,
	-0.00011996320516061139, -0.00012341544310819237, 0.00012711709134259722, 0.00013154916584157708,
	-0.00013625244284831922, -0.0001417281744029534, 0.00014749443325959093, 0.00015407878175092168,
	-0.0001609705740760647, -0.00016872969804359607, 0.00017681084907762736, 0.00018581218257671868,
	-0.00019514788486069032, -0.00020546023419965628, 0.00021611716228527676, 0.00022781082525085626,
	-0.0002398572742041225, -0.00025300418474027357, 0.00026651023558768523, 0.00028118413731088756,
	-0.0002962218530307539, -0.000312498505457252, 0.0003291421616537397, 0.00034709958359519844,
	-0.00036542593862562347, -0.0003851446938981732, 0.00040523330397414296, 0.00042679683538368406,
	-0.00044873042106072247, -0.00047222543960281026, 0.0004960903111395706, 0.000521607248519232,
	-0.0005474937988665493, -0.0005751273328449531, 0.0006031306085632009, 0.0006329802723271502,
	-0.0006632006345884799, -0.0006953715233826579, 0.0007279154134668876, 0.0007625190042120038,
	-0.0007974998306472616, -0.0008346549332968308, 0.0008721941011490265, 0.0009120279642506993,
	-0.0009522560711833099, -0.0009949056686819422, 0.0010379638974907593, 0.0010835774294661298,
	-0.0011296191730964485, -0.0011783578201616963, 0.0012275505830750394, 0.0012795905629544738,
	-0.001332118192565739, -0.001387653178426391, 0.001443718492756258, 0.0015029624668507565,
	-0.0015627903603024247, -0.0016259809942784852, 0.0016898221235773013, 0.0017572247996129463,
	-0.00182535997782757, -0.0018972725941747111, 0.001970018054254913, 0.002046776797022505,
	-0.0021244905300265276, -0.0022064768431490616, 0.002289566273877373, 0.0023772153254305456,
	-0.0024661466645490068, -0.002559957695849092, 0.0026552674098718042, 0.0027558164726072367,
	-0.0028581254514694465, -0.0029660811996647337, 0.003076112390764104, 0.0031922558164583915,
	-0.0033108563555650753, -0.003436105665990035, 0.003564274901449907, 0.0036997171706318298,
	-0.003838642496367859, -0.003985574345492536, 0.004136677504999131, 0.00429665796657563,
	-0.004461655580959909, -0.0046365756285649505, 0.004817559322456838, 0.005009734536934371,
	-0.0052092784890311146, -0.005421574372352691, 0.005642881905847625, 0.005878886101071256,
	-0.006125992912990169, -0.006390256178180939, 0.006668317495975328, 0.006966697736385588,
	-0.00728240282218511, -0.007622567541109441, 0.007984752676535329, 0.008376932002868586,
	-0.008797512413003823, -0.00925566165604641, 0.00975109426520133, 0.010294751588889707,
	-0.010888418141029898, -0.011545795566980486, 0.012272060283969894, 0.013085441221631942,
	-0.013996936256216804, -0.015032670084943014, 0.0162142575351559, 0.017582666294362666,
	-0.019180486116402932, -0.021079392186940073, 0.02336812628486709, 0.026190623301015317,
	-0.029753782927282417, -0.034406124578612404, 0.04073471030181519, 0.04986429593935469,
	-0.06418781308226466, -0.08994671569277171, 0.1500006542192437, 0.4501417212098745,
}

// type1_multiband (first 32 of 63 taps).
// scipy 1.18.1: scipy.signal.firwin2(63, [0, 0.1, 0.2, 0.35, 0.6, 0.8, 1],
// [0.5, 1, 0.25, 0.25, 2, 1, 0.75]), float64.
var scipyType1Multiband = []float64{
	-0.00013289309318975024, -0.0001813359210223413, -0.00029975623960582084, -0.0004484549657623712,
	6.012153046827762e-06, 0.0001625836723459292, -0.0001834995572979679, 0.0002571966298512259,
	0.00028700816934812174, -0.0007374837427099664, 0.00027042390636867656, 0.001044459457527464,
	0.0003962438293480083, -0.0018480081979899959, 0.001155972442732911, 0.0015749907580323423,
	-0.0017996822314292947, 0.002845816427682843, 0.00025424082279201446, -0.017262524920663748,
	-0.017693306143688997, -0.014385694550951294, -0.016591845733193358, -0.03241188551262248,
	-0.002512048991164131, -0.010289370287810403, -0.06464173634389214, 0.07654372776597163,
	0.23531498798702924, -0.13082972566921844, -0.18022439766638376, 0.9309570312500002,
}

// step_repeated_freq (first 33 of 65 taps).
// scipy 1.18.1: scipy.signal.firwin2(65, [0, 0.5, 0.5, 1], [1, 1, 0, 0]), float64.
var scipyStepRepeatedFreq = []float64{
	-3.9429924734491486e-18, -0.0008030654171477866, -8.006759157552802e-19, 0.0010488617227646744,
	5.093179835045131e-18, -0.0015251112156451881, 6.076263565705999e-18, 0.002273911588362921,
	-1.3156479937418604e-17, -0.0033429914909595894, 4.4001172246496195e-18, 0.0047893751845152115,
	-1.511485454384537e-17, -0.00668578396806141, 3.5755140128092116e-17, 0.009131958528345063,
	-6.695225232740851e-17, -0.012275396217043743, 8.460541681689257e-17, 0.016351510262706072,
	-6.689953151915384e-17, -0.021767758913570297, 3.097831500310006e-17, 0.029299924459545025,
	-3.5906939315270284e-17, -0.04062456254331347, 8.624555394606796e-17, 0.060128525158941314,
	-1.6631912108843744e-16, -0.10395466850862385, 3.4926426345882967e-16, 0.31758887786273704,
	0.5,
}

// type2_ramp (first 3 of 6 taps).
// scipy 1.18.1: scipy.signal.firwin2(6, [0, 0.5, 1], [1, 1, 0]), float64.
var scipyType2Ramp = []float64{
	-0.001988803500918899, -0.026082460192815005, 0.5244887779006642,
}

// nfreqs_odd_len (first 10 of 20 taps).
// scipy 1.18.1: scipy.signal.firwin2(20, [0, 0.3, 0.4, 1], [1, 1, 0, 0], nfreqs=33), float64.
var scipyNFreqsOddLen = []float64{
	-0.0015120257432036305, 0.00021151212516869964, 0.005394915402527752, 0.008896383745516741,
	-0.005124093841698259, -0.036452032022967255, -0.040452351596344645, 0.04066663132758124,
	0.1979462334840294, 0.32986577557350705,
}

// window_none (first 21 of 41 taps).
// scipy 1.18.1: scipy.signal.firwin2(41, [0, 0.3, 0.4, 1], [1, 1, 0, 0], window=None), float64.
var scipyWindowNone = []float64{
	0.00010748833524547828, 0.0007691872732988455, 0.0014384911650975464, -0.000584515187252992,
	-0.004369993881008736, -0.004379987535202613, 0.002624032498117246, 0.010470640377861256,
	0.007779579156762058, -0.007493477527121234, -0.020181628938806342, -0.011189054992935554,
	0.01763951091214746, 0.03633745205037167, 0.01410570542835522, -0.04044812389612729,
	-0.07078740812614892, -0.01607093399998558, 0.12658125487368604, 0.2824862168566804,
	0.35009765625,
}

// one_tap (the single tap; scipy never windows one tap).
// scipy 1.18.1: scipy.signal.firwin2(1, [0, 1], [1, 1]), float64.
var scipyOneTap = []float64{
	1,
}

// two_taps (first 1 of 2 taps).
// scipy 1.18.1: scipy.signal.firwin2(2, [0, 1], [1, 0]), float64.
var scipyTwoTaps = []float64{
	0.03414213562373098,
}
