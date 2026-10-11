package cqt

import (
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

// This file holds the exact oracle of the transform: a naive multirate CQT
// written with plain loops. It reproduces every stage of nnAudio's
// CQT2010v2.forward (early downsampling, per-octave padding, kernel
// correlation, 2:1 decimation, stacking, normalization) without any of the
// production primitives (no dsp/conv, vecmath or dsp/core, and none of the
// package's unexported processing helpers). It takes the kernels from
// Kernels(), which are checked separately (pinned nnAudio values and the
// centred-frequency tests in presets_test.go), and designs the anti-alias
// low-passes itself with design.Firwin2.

// oracleTaps is the length of nnAudio's anti-alias filters.
const oracleTaps = 256

// oracleSpec is what the oracle needs to know about a configuration beyond
// the transform's public accessors.
type oracleSpec struct {
	hop           int // hop in input samples, as given to WithHopLength
	padding       Padding
	normalization Normalization
	complexOut    bool
	// zeroPadShort is the NNAudio() quirk: an octave not longer than
	// nfft/2 samples is zero padded instead of reflected. Without it such a
	// signal cannot be processed under PadReflect.
	zeroPadShort bool
}

// oracleSpecFor reads hop, padding, normalization and output from opts
// (applied to the generic defaults, as New does); the quirk is passed in
// explicitly by the test case.
func oracleSpecFor(tb testing.TB, opts []Option, zeroPadShort bool) oracleSpec {
	tb.Helper()

	cfg := defaultConfig()

	for _, opt := range opts {
		err := opt(&cfg)
		if err != nil {
			tb.Fatal(err)
		}
	}

	return oracleSpec{
		hop:           cfg.hop,
		padding:       cfg.padding,
		normalization: cfg.normalization,
		complexOut:    cfg.output == OutputComplex,
		zeroPadShort:  zeroPadShort,
	}
}

// oracleLowpass is nnAudio's create_lowpass_filter: firwin2(256,
// [0, c/(1+t), c*(1+t), 1], [1, 1, 0, 0]). bandCenter and transition are
// variables, so 1+t, c/(1+t) and c*(1+t) are rounded in float64 as in
// nnAudio. Folded as an exact Go constant, 0.5/1.001 rounds to a different
// float64 than 0.5/(1+0.001) computed in float64 (0.5*1.001 and the early
// low-pass edges for factors 2-32 happen to agree).
func oracleLowpass(tb testing.TB, bandCenter, transition float64) []float64 {
	tb.Helper()

	h, err := design.Firwin2(oracleTaps,
		[]float64{0, bandCenter / (1 + transition), bandCenter * (1 + transition), 1},
		[]float64{1, 1, 0, 0})
	if err != nil {
		tb.Fatal(err)
	}

	return h
}

// oracleFilters returns the octave low-pass (band centre 0.5, transition
// 0.001) and, for factor > 1, the early-downsampling low-pass (band centre
// 1/factor, transition 0.03).
func oracleFilters(tb testing.TB, factor int) ([]float64, []float64) {
	tb.Helper()

	center, transition := 0.5, 0.001
	lowpass := oracleLowpass(tb, center, transition)

	if factor == 1 {
		return lowpass, nil
	}

	return lowpass, oracleLowpass(tb, 1/float64(factor), 0.03)
}

// oracleDecimatedLen is torch conv1d's output length with padding
// (taps-1)/2 on both sides and the given stride, or -1 if the padded input
// is shorter than the filter (torch raises).
func oracleDecimatedLen(n, taps, stride int) int {
	padded := n + 2*((taps-1)/2)
	if padded < taps {
		return -1
	}

	return (padded-taps)/stride + 1
}

// naiveDecimate is nnAudio's downsampling_by_n: torch conv1d (correlation,
// no kernel flip) of x with h, zero padding (len(h)-1)/2 on both sides,
// stride factor.
func naiveDecimate(x, h []float64, factor int) []float64 {
	p := (len(h) - 1) / 2

	m := oracleDecimatedLen(len(x), len(h), factor)
	if m < 1 {
		return nil
	}

	y := make([]float64, m)

	for i := range y {
		var s float64

		for k, hk := range h {
			if j := i*factor + k - p; j >= 0 && j < len(x) {
				s += hk * x[j]
			}
		}

		y[i] = s
	}

	return y
}

// oracleOctaveLens returns the signal length of every octave, top first, for
// n input samples, and the frame count every octave gives, or ok = false if
// the signal cannot be processed. It fails the test if two octaves disagree
// on the frame count.
func oracleOctaveLens(tb testing.TB, tr *Transform, spec oracleSpec, n int) ([]int, int, bool) {
	tb.Helper()

	nFFT, octaves, factor := tr.NFFT(), oracleOctaves(tr), tr.DownsampleFactor()
	pad := nFFT / 2

	if n < 1 {
		return nil, 0, false
	}

	if factor > 1 {
		n = oracleDecimatedLen(n, oracleTaps, factor)
	}

	lens := make([]int, octaves)
	frames := -1

	for o := range octaves {
		if o > 0 {
			n = oracleDecimatedLen(n, oracleTaps, 2)
		}

		if n < 1 {
			return nil, 0, false
		}

		if spec.padding == PadReflect && n <= pad && !spec.zeroPadShort {
			return nil, 0, false // torch's reflection pad needs pad < n
		}

		hop := spec.hop / factor >> o

		// torch conv1d of the padded octave with an nfft-sample kernel at
		// stride hop.
		f := (n+2*pad-nFFT)/hop + 1
		if frames >= 0 && f != frames {
			tb.Fatalf("n=%d: octave %d gives %d frames, octave 0 gives %d", n, o, f, frames)
		}

		lens[o], frames = n, f
	}

	return lens, frames, true
}

// oracleOctaves is ceil(bins/filters): the number of octaves computed from
// the number of kernels, which is min(bins, binsPerOctave).
func oracleOctaves(tr *Transform) int {
	filters := len(tr.Kernels())

	return (tr.Bins() + filters - 1) / filters
}

// naivePad extends x by pad samples on both sides: numpy/torch "reflect"
// (edge sample not repeated) if reflect, zeros otherwise.
func naivePad(x []float64, pad int, reflect bool) []float64 {
	n := len(x)
	xp := make([]float64, n+2*pad)

	if !reflect {
		copy(xp[pad:], x)

		return xp
	}

	for i := range xp {
		j := i - pad
		if j < 0 {
			j = -j
		} else if j >= n {
			j = 2*(n-1) - j
		}

		xp[i] = x[j]
	}

	return xp
}

// oracleCQT computes the transform of x naively, in Process's layout, and
// returns it with the frame count. It returns nil, 0 if the signal cannot be
// processed.
func oracleCQT(tb testing.TB, tr *Transform, spec oracleSpec, x []float64) ([]float64, int) {
	tb.Helper()

	lens, frames, ok := oracleOctaveLens(tb, tr, spec, len(x))
	if !ok {
		return nil, 0
	}

	factor := tr.DownsampleFactor()
	lowpass, early := oracleFilters(tb, factor)
	kernels := tr.Kernels()
	nFFT, bins, filters := tr.NFFT(), tr.Bins(), len(kernels)
	octaves := len(lens)
	pad := nFFT / 2

	// rows[o][frame*filters+f] is kernel f's response in octave o (0 is the
	// top octave).
	rows := make([][]complex128, octaves)
	cur := x

	if factor > 1 {
		cur = naiveDecimate(cur, early, factor)
	}

	for o := range octaves {
		if o > 0 {
			cur = naiveDecimate(cur, lowpass, 2)
		}

		if len(cur) != lens[o] {
			tb.Fatalf("octave %d: %d samples, want %d", o, len(cur), lens[o])
		}

		// Under the quirk an octave not longer than nfft/2 is zero padded.
		xp := naivePad(cur, pad, spec.padding == PadReflect && len(cur) > pad)
		hop := spec.hop / factor >> o
		rows[o] = make([]complex128, frames*filters)

		for fr := range frames {
			seg := xp[fr*hop : fr*hop+nFFT]

			for f, kernel := range kernels {
				var re, im float64

				for i, k := range kernel {
					re += seg[i] * real(k)
					im += seg[i] * imag(k)
				}

				// torch: CQT_imag = -conv1d(x, kernel_imag).
				rows[o][fr*filters+f] = complex(re, -im)
			}
		}
	}

	// Stack the octaves lowest first and drop the surplus lowest rows.
	drop := octaves*filters - bins
	lengths := tr.Lengths()

	width := 1
	if spec.complexOut {
		width = 2
	}

	out := make([]float64, frames*bins*width)

	for o := range octaves {
		for f := range filters {
			b := (octaves-1-o)*filters + f - drop
			if b < 0 {
				continue
			}

			scale := float64(factor)

			switch spec.normalization {
			case NormalizationLibrosa:
				scale *= math.Sqrt(lengths[b])
			case NormalizationWrap:
				scale *= 2
			case NormalizationConvolutional:
			}

			for fr := range frames {
				v := rows[o][fr*filters+f]
				i := fr*bins + b

				if spec.complexOut {
					out[2*i] = real(v) * scale
					out[2*i+1] = imag(v) * scale
				} else {
					out[i] = math.Hypot(real(v)*scale, imag(v)*scale)
				}
			}
		}
	}

	return out, frames
}

// oracleCase is one configuration of TestOracle.
type oracleCase struct {
	name         string
	sr           float64
	length       int
	opts         []Option
	zeroPadShort bool
}

func oracleCases() []oracleCase {
	var cases []oracleCase

	// Every reference configuration of 46.1 is built on NNAudio(), so the
	// short-octave quirk is on.
	for _, rc := range refConfigs() {
		cases = append(cases, oracleCase{rc.name, rc.sr, rc.length, rc.opts, true})
	}

	return append(cases,
		// The generic defaults: 7 octaves, zero padding.
		oracleCase{"generic_defaults", 22050, testSignalLen, nil, false},
		// Two octaves and 6 bins (a partial lowest octave), complex output,
		// early downsampling by 8.
		oracleCase{"generic_30_of_12_complex", 22050, testSignalLen, []Option{
			WithFMin(55), WithBins(30), WithHopLength(256), WithOutput(OutputComplex),
		}, false},
		// Fewer bins than one octave: the single octave is cropped at the top.
		oracleCase{"generic_20_of_36", 22050, 30000, []Option{
			WithFMin(220), WithBins(20), WithBinsPerOctave(36), WithHopLength(256),
			WithNormalization(NormalizationWrap),
		}, false},
		// Early downsampling by 8 without the preset.
		oracleCase{"generic_early_ds8", 22050, testSignalLen, []Option{
			WithHopLength(1024), WithFMin(30), WithBins(48),
		}, false},
		// basic-pitch's numbers with generic reflect padding on a signal long
		// enough to reflect every octave (33024 samples are needed), L2
		// Kaiser kernels.
		oracleCase{"generic_reflect_309_of_36", BasicPitchSampleRate, testSignalLen, []Option{
			WithHopLength(BasicPitchHopLength), WithFMin(BasicPitchFMin), WithBins(BasicPitchBins),
			WithBinsPerOctave(BasicPitchBinsPerOctave), WithCenter(PadReflect),
			WithWindow(window.TypeKaiser, window.WithAlpha(6)), WithBasisNorm(NormL2),
		}, false},
	)
}

// TestOracle checks Process, ProcessInto and ProcessInto32 against the naive
// multirate oracle for the eight 46.1 reference configurations and for
// generic ones (default, partial octaves, early downsampling, reflect
// padding). float64 must match to tolExact (1e-12) in the per-bin metric.
// Measured maximum 3.1e-14 (basic_pitch_short, whose short octaves give
// small per-bin scales); 1.0e-14 (hamming_nonorm_wrap), 8.8e-15
// (const_l2_conv_complex) and 1.1e-15 to 3.3e-15 for the others, so the two
// computations differ only in summation order and libm rounding. float32
// output must equal the oracle rounded to float32 to within one float32
// rounding step (2^-23, for a value whose float64 results straddle a
// float32 rounding boundary) plus tolExact; measured maximum 1.5e-16: the
// float32 results are identical except for a few values that are tiny
// against their bin's scale.
func TestOracle(t *testing.T) {
	t.Parallel()

	for _, tc := range oracleCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			spec := oracleSpecFor(t, tc.opts, tc.zeroPadShort)
			x := testSignal(t)[:tc.length]

			// The filters the oracle designs are the transform's.
			lowpass, early := oracleFilters(t, tr.DownsampleFactor())
			if !slices.Equal(lowpass, tr.Lowpass()) || !slices.Equal(early, tr.EarlyLowpass()) {
				t.Fatal("oracle low-pass filters differ from Lowpass()/EarlyLowpass()")
			}

			if oracleOctaves(tr) != tr.Octaves() || spec.hop/tr.DownsampleFactor() != tr.Hop() {
				t.Fatalf("octaves %d, hop %d; transform: %d, %d",
					oracleOctaves(tr), spec.hop/tr.DownsampleFactor(), tr.Octaves(), tr.Hop())
			}

			want, frames := oracleCQT(t, tr, spec, x)
			if frames == 0 || frames != tr.FrameCount(len(x)) || len(want) != tr.OutputLen(len(x)) {
				t.Fatalf("oracle: %d frames, %d values; FrameCount %d, OutputLen %d",
					frames, len(want), tr.FrameCount(len(x)), tr.OutputLen(len(x)))
			}

			got, err := tr.Process(x)
			if err != nil {
				t.Fatal(err)
			}

			into := make([]float64, len(want))
			if err := tr.ProcessInto(into, x); err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(got, into) {
				t.Error("Process and ProcessInto differ")
			}

			e, bin := perBinError(got, want, frames, tr.Bins(), spec.complexOut)
			t.Logf("float64: max per-bin error %.2e (bin %d), %d frames x %d bins", e, bin, frames, tr.Bins())

			if !(e <= tolExact) {
				t.Errorf("float64: per-bin error %.3e at bin %d > %.0e", e, bin, tolExact)
			}

			x32 := make([]float32, len(x))
			for i, v := range x {
				x32[i] = float32(v) // exact: testSignal is float32-rounded
			}

			got32 := make([]float32, len(want))
			if err := tr.ProcessInto32(got32, x32); err != nil {
				t.Fatal(err)
			}

			g32, w32 := make([]float64, len(want)), make([]float64, len(want))
			for i := range want {
				g32[i], w32[i] = float64(got32[i]), float64(float32(want[i]))
			}

			tol32 := 0x1p-23 + tolExact
			e32, bin32 := perBinError(g32, w32, frames, tr.Bins(), spec.complexOut)
			t.Logf("float32: max per-bin error %.2e (bin %d)", e32, bin32)

			if !(e32 <= tol32) {
				t.Errorf("float32: per-bin error %.3e at bin %d > %.3e", e32, bin32, tol32)
			}
		})
	}
}

// TestOracleFrameCount compares FrameCount with the oracle's own length
// arithmetic around the thresholds: signals too short to be decimated, too
// short to be reflected (generic PadReflect) and the zero-pad fallback of
// NNAudio().
func TestOracleFrameCount(t *testing.T) {
	t.Parallel()

	lengths := []int{0, 1, 2, 3, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1000, 1031, 1032,
		4000, 8249, 8250, 33023, 33024, 43844, 44100}

	for _, tc := range oracleCases() {
		tr, err := New(tc.sr, tc.opts...)
		if err != nil {
			t.Fatal(err)
		}

		spec := oracleSpecFor(t, tc.opts, tc.zeroPadShort)

		for _, n := range lengths {
			_, frames, ok := oracleOctaveLens(t, tr, spec, n)
			if !ok {
				frames = 0
			}

			if got := tr.FrameCount(n); got != frames {
				t.Errorf("%s: FrameCount(%d) = %d, oracle %d", tc.name, n, got, frames)
			}
		}
	}
}
