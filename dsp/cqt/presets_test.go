package cqt

import (
	"errors"
	"math"
	"math/cmplx"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// measuredKernelFrequencies returns the centre frequency of every top-octave kernel,
// measured from Kernels(): adjacent kernel samples near the middle of the
// frame differ in phase by 2*pi*f/SampleRate().
func measuredKernelFrequencies(tr *Transform) []float64 {
	kernels := tr.Kernels()
	out := make([]float64, len(kernels))

	for f, row := range kernels {
		mid := len(row) / 2
		out[f] = cmplx.Phase(row[mid+1]/row[mid]) * tr.SampleRate() / (2 * math.Pi)
	}

	return out
}

// effectiveFrequencies returns the measured centre frequency of every output
// bin. Octave o (0 is the top octave) applies kernel f at SampleRate()/2^o
// and writes bin Bins()-(o+1)*nFilters+f, so that bin is centred on kernel
// f's frequency divided by 2^o. Bins no octave writes stay NaN.
func effectiveFrequencies(tr *Transform) []float64 {
	kf := measuredKernelFrequencies(tr)
	nFilters := len(kf)

	out := make([]float64, tr.Bins())
	for b := range out {
		out[b] = math.NaN()
	}

	for o := range tr.Octaves() {
		for f, freq := range kf {
			if b := tr.Bins() - (o+1)*nFilters + f; b >= 0 {
				out[b] = freq / float64(uint64(1)<<o)
			}
		}
	}

	return out
}

// peakBin returns the bin with the largest magnitude in the middle frame of
// the transform of a sine at freq Hz, and that frame.
func peakBin(t *testing.T, tr *Transform, sr, freq float64, n int) (int, []float64) {
	t.Helper()

	y, err := tr.Process(sine(n, freq, sr))
	if err != nil {
		t.Fatal(err)
	}

	mid := tr.FrameCount(n) / 2
	row := y[mid*tr.Bins() : (mid+1)*tr.Bins()]

	peak := 0
	for b, v := range row {
		if v > row[peak] {
			peak = b
		}
	}

	return peak, row
}

// Configurations with a partial octave (bins < binsPerOctave, or a remainder
// above whole octaves) and with whole octaves.
var placementConfigs = []struct {
	name string
	sr   float64
	opts []Option
}{
	// A single partial octave of 3 out of 12 bins.
	{"3 of 12", 8000, []Option{WithFMin(1000), WithBins(3), WithBinsPerOctave(12), WithEarlyDownsampling(false)}},
	// A single partial octave of 20 out of 36 bins, early downsampled by 8.
	{"20 of 36", 22050, []Option{WithFMin(220), WithBins(20), WithBinsPerOctave(36), WithHopLength(256)}},
	// Two octaves and 6 bins, early downsampled by 8.
	{"30 of 12", 22050, []Option{WithFMin(55), WithBins(30), WithBinsPerOctave(12), WithHopLength(256)}},
	// basic-pitch's numbers without the preset: 8 octaves and 21 bins.
	{"309 of 36", BasicPitchSampleRate, []Option{
		WithHopLength(BasicPitchHopLength), WithFMin(BasicPitchFMin),
		WithBins(BasicPitchBins), WithBinsPerOctave(BasicPitchBinsPerOctave),
	}},
	// Whole octaves: the generic defaults (7 octaves) and 2 octaves of 36.
	{"84 of 12", 22050, nil},
	{"72 of 36", 22050, []Option{WithFMin(110), WithBins(72), WithBinsPerOctave(36), WithHopLength(256)}},
}

// TestGenericKernelsCentred checks that every bin of the generic transform is
// analysed by a kernel centred on Frequencies()[b], whether or not the bin
// count is a whole number of octaves.
//
// The phase measurement itself is accurate to a few ulps: the measured
// maximum relative deviation over all configurations is 7.7e-16 (309 of 36).
// 1e-13 leaves room for other libm/FMA behaviour; nnAudio's shift of a
// partial octave is 0.44 and 0.75 octaves in the two single partial octaves
// here (see TestNNAudioPartialOctaveKernels).
func TestGenericKernelsCentred(t *testing.T) {
	t.Parallel()

	const relTol = 1e-13

	for _, tc := range placementConfigs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			want := tr.Frequencies()
			worst := 0.0

			for b, got := range effectiveFrequencies(tr) {
				e := math.Abs(got-want[b]) / want[b]
				worst = max(worst, e)

				if !(e <= relTol) {
					t.Errorf("bin %d centred on %.12g Hz, Frequencies() %.12g Hz (rel %.2e)", b, got, want[b], e)
				}
			}

			t.Logf("max relative deviation from Frequencies(): %.2e", worst)
		})
	}
}

// TestGenericTonePeaksAtBin checks the generic placement functionally: a sine
// at Frequencies()[b] peaks at bin b in every configuration with a partial
// octave, including both single partial octaves.
func TestGenericTonePeaksAtBin(t *testing.T) {
	t.Parallel()

	for _, tc := range placementConfigs[:3] {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			for b, f := range tr.Frequencies() {
				if peak, _ := peakBin(t, tr, tc.sr, f, 20000); peak != b {
					t.Errorf("sine at %.4g Hz (bin %d) peaks at bin %d", f, b, peak)
				}
			}
		})
	}
}

// TestNNAudioPartialOctaveKernels pins nnAudio's placement of a single partial
// octave under NNAudio(): with bins < binsPerOctave, kernel k is centred on
// fmin*2^((bins+k)/binsPerOctave-1), (binsPerOctave-bins)/binsPerOctave
// octaves below Frequencies()[k], while Frequencies() still reports the
// nominal centres.
func TestNNAudioPartialOctaveKernels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sr, fmin        float64
		bins, bpo       int
		opts            []Option
		wantShiftOctave float64
	}{
		// 9/12 = 0.75 octaves below Frequencies().
		{"3 of 12", 8000, 1000, 3, 12, []Option{WithEarlyDownsampling(false)}, 0.75},
		// 16/36 = 0.444 octaves below Frequencies().
		{"20 of 36", 22050, 220, 20, 36, []Option{WithHopLength(256)}, 16.0 / 36},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := slices.Concat(NNAudio(),
				[]Option{WithFMin(tc.fmin), WithBins(tc.bins), WithBinsPerOctave(tc.bpo)}, tc.opts)

			tr, err := New(tc.sr, opts...)
			if err != nil {
				t.Fatal(err)
			}

			freqs := tr.Frequencies()
			if freqs[0] != tc.fmin {
				t.Errorf("Frequencies()[0] = %g, want %g", freqs[0], tc.fmin)
			}

			for k, got := range measuredKernelFrequencies(tr) {
				want := tc.fmin * math.Pow(2, float64(tc.bins+k)/float64(tc.bpo)-1)
				if math.Abs(got-want) > 1e-13*want {
					t.Errorf("kernel %d centred on %.9g Hz, want %.9g Hz", k, got, want)
				}

				if shift := math.Log2(freqs[k] / got); math.Abs(shift-tc.wantShiftOctave) > 1e-12 {
					t.Errorf("kernel %d: %.6f octaves below Frequencies(), want %.6f", k, shift, tc.wantShiftOctave)
				}
			}
		})
	}
}

// TestNNAudioPartialOctaveTone pins the consequence of nnAudio's shifted
// partial octave: bin b responds to fmin*2^((bins+b)/binsPerOctave-1), not to
// Frequencies()[b].
//
// With 20 of 36 bins the kernels are shifted down by 16 bins, so a sine at
// Frequencies()[b] peaks at bin b+16 for b < 4. At Frequencies()[b] itself
// bin b responds with at most 2.2e-4 (3 of 12) and 4.9e-5 (20 of 36) of the
// generic transform's response (measured); the bound is 1e-3.
func TestNNAudioPartialOctaveTone(t *testing.T) {
	t.Parallel()

	const maxRatio = 1e-3

	// binsPerOctave-bins: how many bins nnAudio shifts the octave down.
	shifts := []int{12 - 3, 36 - 20}

	for i, tc := range placementConfigs[:2] {
		shift := shifts[i]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nn, err := New(tc.sr, slices.Concat(NNAudio(), tc.opts)...)
			if err != nil {
				t.Fatal(err)
			}

			gen, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			freqs, kernels := nn.Frequencies(), measuredKernelFrequencies(nn)
			worst := 0.0

			for b := range nn.Bins() {
				// A sine at the kernel's own frequency peaks at its bin.
				if peak, _ := peakBin(t, nn, tc.sr, kernels[b], 20000); peak != b {
					t.Errorf("sine at kernel %d's %.4g Hz peaks at bin %d", b, kernels[b], peak)
				}

				peakNN, rowNN := peakBin(t, nn, tc.sr, freqs[b], 20000)
				_, rowGen := peakBin(t, gen, tc.sr, freqs[b], 20000)

				ratio := rowNN[b] / rowGen[b]
				worst = max(worst, ratio)

				if !(ratio <= maxRatio) {
					t.Errorf("sine at Frequencies()[%d]: NNAudio bin %d = %.3g, generic %.3g (ratio %.2e > %.0e)",
						b, b, rowNN[b], rowGen[b], ratio, maxRatio)
				}

				if b+shift < nn.Bins() && peakNN != b+shift {
					t.Errorf("sine at Frequencies()[%d] peaks at bin %d, want %d", b, peakNN, b+shift)
				}
			}

			t.Logf("max NNAudio/generic response at Frequencies()[b]: %.2e", worst)
		})
	}
}

// TestPresetQuirks checks that the two nnAudio quirks are on in NNAudio()
// and BasicPitch() and off without a preset. Public options given after a
// preset override its parameters but leave the quirks on: that is the
// documented behaviour, there is no public option for them.
func TestPresetQuirks(t *testing.T) {
	t.Parallel()

	// Every public option at its generic default.
	genericDefaults := []Option{
		WithHopLength(512), WithFMin(32.70), WithBins(84), WithBinsPerOctave(12), WithFilterScale(1),
		WithWindow(window.TypeHann), WithBasisNorm(NormL1), WithCenter(PadZero), WithEarlyDownsampling(true),
		WithNormalization(NormalizationLibrosa), WithOutput(OutputMagnitude),
	}

	tests := []struct {
		name   string
		preset []Option
		quirks bool
	}{
		{"generic", nil, false},
		{"NNAudio", NNAudio(), true},
		{"BasicPitch", BasicPitch(), true},
		{"NNAudio overridden by every public option", slices.Concat(NNAudio(), genericDefaults), true},
		{"BasicPitch overridden by every public option", slices.Concat(BasicPitch(), genericDefaults), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Kernel placement: a single partial octave of 3 of 12 bins at
			// 1000 Hz. nnAudio starts it 9 bins lower, at 594.6 Hz.
			placed, err := New(8000, slices.Concat(tc.preset, []Option{
				WithFMin(1000), WithBins(3), WithBinsPerOctave(12), WithEarlyDownsampling(false),
			})...)
			if err != nil {
				t.Fatal(err)
			}

			want := 1000.0
			if tc.quirks {
				want = 1000 * math.Pow(2, 3.0/12-1)
			}

			if got := measuredKernelFrequencies(placed)[0]; math.Abs(got-want) > 1e-12*want {
				t.Errorf("kernel 0 centred on %.9g Hz, want %.9g Hz", got, want)
			}

			// Short octaves: basic-pitch's numbers with reflect padding on
			// 4000 samples, whose lowest octave has 15 samples, not more than
			// nfft/2 = 128 (the basic_pitch_short fixture). nnAudio zero pads
			// that octave and gives 4000/256+1 = 16 frames.
			short, err := New(BasicPitchSampleRate, slices.Concat(tc.preset, []Option{
				WithHopLength(BasicPitchHopLength), WithFMin(BasicPitchFMin), WithBins(BasicPitchBins),
				WithBinsPerOctave(BasicPitchBinsPerOctave), WithCenter(PadReflect),
			})...)
			if err != nil {
				t.Fatal(err)
			}

			if short.zeroPadShort != tc.quirks {
				t.Errorf("zeroPadShort = %v, want %v", short.zeroPadShort, tc.quirks)
			}

			wantFrames := 0
			if tc.quirks {
				wantFrames = 16
			}

			if got := short.FrameCount(4000); got != wantFrames {
				t.Errorf("FrameCount(4000) = %d, want %d", got, wantFrames)
			}

			_, err = short.Process(testSignal(t)[:4000])
			if tc.quirks && err != nil {
				t.Errorf("Process(4000 samples): %v", err)
			}

			if !tc.quirks && !errors.Is(err, ErrSignalTooShort) {
				t.Errorf("Process(4000 samples) error = %v, want ErrSignalTooShort", err)
			}
		})
	}
}

// TestPresetOverrides checks that options given after a preset override its
// parameters.
func TestPresetOverrides(t *testing.T) {
	t.Parallel()

	x := testSignal(t)

	// Hop 512 halves the frame rate: 43844/512+1 = 86 frames instead of 172.
	tr, err := New(BasicPitchSampleRate, append(BasicPitch(), WithHopLength(512))...)
	if err != nil {
		t.Fatal(err)
	}

	if tr.Hop() != 512 || tr.FrameCount(len(x)) != 86 || tr.Bins() != BasicPitchBins {
		t.Errorf("BasicPitch()+WithHopLength(512): hop %d, %d frames, %d bins, want 512, 86, %d",
			tr.Hop(), tr.FrameCount(len(x)), tr.Bins(), BasicPitchBins)
	}

	// Zero padding instead of reflect changes the edge frames only.
	zero, err := New(BasicPitchSampleRate, append(BasicPitch(), WithCenter(PadZero))...)
	if err != nil {
		t.Fatal(err)
	}

	if zero.Padding() != PadZero {
		t.Errorf("BasicPitch()+WithCenter(PadZero): Padding %v, want zero", zero.Padding())
	}

	refl := newBasicPitch(t)

	yz, err := zero.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	yr, err := refl.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(yz, yr) {
		t.Error("BasicPitch()+WithCenter(PadZero) gives the same output as BasicPitch()")
	}

	// The window is honoured after the preset (nnAudio itself ignores it).
	ham, err := New(BasicPitchSampleRate, append(BasicPitch(), WithWindow(window.TypeHamming))...)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(ham.Kernels(), refl.Kernels()) {
		t.Error("BasicPitch()+WithWindow(Hamming) has the Hann kernels")
	}
}

// TestBasicPitchPreset checks that BasicPitch() is NNAudio() plus basic-pitch's
// hop, fmin and bins, and that it turns basic-pitch's 43844-sample windows
// into 172 frames of 309 bins.
func TestBasicPitchPreset(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)

	if tr.Hop() != 256 || tr.Bins() != 309 || tr.Octaves() != 9 || tr.NFFT() != 256 ||
		tr.DownsampleFactor() != 1 || tr.SampleRate() != 22050 || tr.Padding() != PadReflect {
		t.Errorf("hop %d bins %d octaves %d nfft %d factor %d sr %g padding %v, "+
			"want 256 309 9 256 1 22050 reflect",
			tr.Hop(), tr.Bins(), tr.Octaves(), tr.NFFT(), tr.DownsampleFactor(), tr.SampleRate(), tr.Padding())
	}

	if f := tr.Frequencies()[0]; f != 27.5 {
		t.Errorf("Frequencies()[0] = %g, want 27.5", f)
	}

	if got := tr.FrameCount(43844); got != 172 {
		t.Errorf("FrameCount(43844) = %d, want 172", got)
	}

	// The remaining basic-pitch values (filter scale 1, L1, Hann, reflect,
	// librosa, magnitude) are nnAudio's defaults, so the transforms must be
	// identical.
	want, err := New(BasicPitchSampleRate, append(NNAudio(),
		WithHopLength(256), WithFMin(27.5), WithBins(309), WithBinsPerOctave(36))...)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(tr, want) {
		t.Error("BasicPitch() differs from NNAudio() plus hop 256, fmin 27.5, 309 bins, 36 per octave")
	}
}

// TestGenericMatchesNNAudioWholeOctaves checks that with bins >= binsPerOctave
// the generic placement and nnAudio's agree up to rounding: the cropped
// partial octave is then the lowest one, and nnAudio's kernel frequencies
// fminT*2^(k/bpo) equal the bin frequencies fmin*2^(b/bpo) up to the last
// bits of math.Pow.
//
// Measured maxima over these configurations: kernel frequencies 7.7e-16
// relative, kernel samples 3.5e-14 relative to the largest sample (the
// frequency rounding is multiplied by the phase index, up to half the kernel
// length), and generic+reflect against NNAudio() on the 43844-sample
// test signal 6.2e-14 in the per-bin metric of the golden tests (basic-pitch).
// The bounds are 1e-14, 1e-12 and tolF64: generic and nnAudio placement
// differ by less than the golden tolerance against nnAudio itself.
func TestGenericMatchesNNAudioWholeOctaves(t *testing.T) {
	t.Parallel()

	x := testSignal(t)

	for _, tc := range placementConfigs[2:] {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gen, err := New(tc.sr, slices.Concat(tc.opts, []Option{WithCenter(PadReflect)})...)
			if err != nil {
				t.Fatal(err)
			}

			nn, err := New(tc.sr, slices.Concat(NNAudio(), tc.opts)...)
			if err != nil {
				t.Fatal(err)
			}

			fg, fn := measuredKernelFrequencies(gen), measuredKernelFrequencies(nn)
			freqErr := 0.0

			for k := range fn {
				freqErr = max(freqErr, math.Abs(fg[k]-fn[k])/fn[k])
			}

			kg, kn := gen.Kernels(), nn.Kernels()
			peak, kernelErr := 0.0, 0.0

			for f := range kn {
				for i := range kn[f] {
					peak = max(peak, cmplx.Abs(kn[f][i]))
					kernelErr = max(kernelErr, cmplx.Abs(kg[f][i]-kn[f][i]))
				}
			}

			kernelErr /= peak

			yg, err := gen.Process(x)
			if err != nil {
				t.Fatal(err)
			}

			yn, err := nn.Process(x)
			if err != nil {
				t.Fatal(err)
			}

			outErr, bin := perBinError(yg, yn, nn.FrameCount(len(x)), nn.Bins(), false)
			t.Logf("kernel frequencies %.2e, kernels %.2e, output %.2e (bin %d)", freqErr, kernelErr, outErr, bin)

			if !(freqErr <= 1e-14) || !(kernelErr <= 1e-12) || !(outErr <= tolF64) {
				t.Errorf("generic vs NNAudio: kernel frequencies %.2e (> 1e-14?), kernels %.2e (> 1e-12?), "+
					"output %.2e at bin %d (> %.0e?)", freqErr, kernelErr, outErr, bin, tolF64)
			}
		})
	}
}

// TestReflectThreshold checks where generic reflect padding starts to work.
// Each 2:1 decimation maps m samples to floor(m/2) (m >= 2), and early
// downsampling by 8 maps n to floor((n+6)/8), so the lowest of O octaves has
// floor(top/2^(O-1)) samples. FrameCount is 0 while that is at most nfft/2
// and (top+2*(nfft/2)-nfft)/hop+1 from the first longer signal on.
// Zero padding and NNAudio()'s fallback process the shorter signals too.
func TestReflectThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sr         float64
		opts       []Option
		minLen     int // shortest signal whose lowest octave is longer than nfft/2
		wantFrames int // FrameCount(minLen)
	}{
		// One octave, nfft 256, hop 1: minLen = 128+1, 129/1+1 frames.
		{
			"single octave", 8000,
			[]Option{WithFMin(1000), WithBins(12), WithHopLength(1), WithEarlyDownsampling(false)},
			129, 130,
		},
		// basic-pitch's numbers: 9 octaves, nfft 256, hop 256:
		// minLen = (128+1)*2^8 = 33024, 33024/256+1 frames.
		{"basic-pitch", BasicPitchSampleRate, []Option{
			WithHopLength(BasicPitchHopLength), WithFMin(BasicPitchFMin),
			WithBins(BasicPitchBins), WithBinsPerOctave(BasicPitchBinsPerOctave),
		}, 33024, 130},
		// Early downsampling by 8, 4 octaves, nfft 256, effective hop 128:
		// the top octave needs (128+1)*2^3 = 1032 samples, so
		// minLen = 8*1032-6 = 8250, 1032/128+1 frames.
		{"early downsampling", 22050, []Option{WithHopLength(1024), WithFMin(30), WithBins(48)}, 8250, 9},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, slices.Concat(tc.opts, []Option{WithCenter(PadReflect)})...)
			if err != nil {
				t.Fatal(err)
			}

			half := tr.NFFT() / 2

			lens := octaveLengths(tr, tc.minLen)
			if lowest := lens[len(lens)-1]; lowest != half+1 {
				t.Fatalf("lowest octave of %d samples has %d samples, want %d", tc.minLen, lowest, half+1)
			}

			for _, n := range []int{0, 1, tc.minLen / 2, tc.minLen - 1} {
				if got := tr.FrameCount(n); got != 0 || tr.OutputLen(n) != 0 {
					t.Errorf("FrameCount(%d) = %d, want 0", n, got)
				}
			}

			if got := tr.FrameCount(tc.minLen); got != tc.wantFrames {
				t.Errorf("FrameCount(%d) = %d, want %d", tc.minLen, got, tc.wantFrames)
			}

			_, err = tr.Process(make([]float64, tc.minLen-1))
			if !errors.Is(err, ErrSignalTooShort) {
				t.Fatalf("Process(%d samples) error = %v, want ErrSignalTooShort", tc.minLen-1, err)
			}

			if want := "reflect padding needs the lowest octave longer than"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not mention %q", err, want)
			}

			err = tr.ProcessInto32(make([]float32, 1), make([]float32, tc.minLen-1))
			if !errors.Is(err, ErrSignalTooShort) {
				t.Errorf("ProcessInto32(%d samples) error = %v, want ErrSignalTooShort", tc.minLen-1, err)
			}

			if _, err := tr.Process(make([]float64, tc.minLen)); err != nil {
				t.Errorf("Process(%d samples): %v", tc.minLen, err)
			}

			// Zero padding and nnAudio's fallback have no such threshold.
			zero, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			nn, err := New(tc.sr, slices.Concat(NNAudio(), tc.opts)...)
			if err != nil {
				t.Fatal(err)
			}

			if zero.FrameCount(tc.minLen-1) == 0 || nn.FrameCount(tc.minLen-1) == 0 {
				t.Errorf("FrameCount(%d): zero padding %d, NNAudio %d, want > 0",
					tc.minLen-1, zero.FrameCount(tc.minLen-1), nn.FrameCount(tc.minLen-1))
			}
		})
	}
}
