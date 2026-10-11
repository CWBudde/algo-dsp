package cqt

import (
	"math"
	"math/cmplx"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// This file compares the multirate transform with a full-rate direct CQT
// (every bin's kernel built at the input sample rate, no decimation) and
// checks the output level of the normalizations. The full-rate CQT is not an
// oracle: the multirate path also carries the low-pass passband ripple and
// residual aliasing in the lower octaves, so the two are compared only on
// pure tones well inside the passband, through bounds derived below, and
// never elementwise.

// fullRateRippleStep is the frequency grid of fullRateRipple, relative to
// the Nyquist frequency. The passband ripple of a 256-tap filter oscillates
// with a period of about 2/256 = 0.0078, so the grid has some 300 points
// per period.
const fullRateRippleStep = 2.5e-5

// fullRateRipple returns max ||H(nu)|-1| of the FIR filter h over the band
// [0, nuMax], nu relative to the Nyquist frequency, evaluated on a grid of
// step fullRateRippleStep.
func fullRateRipple(h []float64, nuMax float64) float64 {
	worst := 0.0

	for i := 0; ; i++ {
		nu := min(float64(i)*fullRateRippleStep, nuMax)

		var re, im float64

		for k, hk := range h {
			s, c := math.Sincos(math.Pi * nu * float64(k))
			re += hk * c
			im -= hk * s
		}

		worst = max(worst, math.Abs(math.Hypot(re, im)-1))

		if nu == nuMax {
			return worst
		}
	}
}

// fullRatePlace returns the octave (0 is the top octave) and the kernel that
// produce bin b: the octaves are stacked lowest first and the lowest
// octaves*filters-bins rows dropped.
func fullRatePlace(tr *Transform, b int) (int, int) {
	filters := len(tr.Kernels())
	octaves := (tr.Bins() + filters - 1) / filters
	r := b + octaves*filters - tr.Bins()

	return octaves - 1 - r/filters, r % filters
}

// fullRateInterior reports whether frame fr of octave o depends only on
// samples inside an input of n samples, so that neither the octave padding
// nor the decimation filters' zero padding reaches it. Frame fr of octave o
// reads octave samples [fr*hop_o-nfft/2, fr*hop_o+nfft/2); each 2:1
// decimation (256 taps, padding 127) maps sample i to the samples
// [2i-127, 2i+128] of the octave above, and early downsampling by F maps i
// to [F*i-127, F*i+128] of the input.
func fullRateInterior(tr *Transform, n, fr, o int) bool {
	pad := tr.NFFT() / 2
	hop := tr.Hop() >> o
	lo, hi := fr*hop-pad, fr*hop+pad-1
	lo = lo<<o - 127*(1<<o-1)
	hi = hi<<o + 128*(1<<o-1)

	if f := tr.DownsampleFactor(); f > 1 {
		lo, hi = f*lo-127, f*hi+128
	}

	return lo >= 0 && hi <= n-1
}

// fullRateTerms returns the gains of a correlation kernel for the two
// complex exponentials of a real tone at omega radians per sample. kernel[i]
// multiplies the input sample at offset m0+i from the frame centre, and the
// output is sum x*conj(kernel) (the imaginary part is negated, as in
// nnAudio). For x = A*sin(omega*n+phi) the output is
// A/2 * (main*e^(i*a) - image*e^(i*b)) for some phases a and b, so its
// magnitude lies in A/2*[main-image, main+image]. For an L1-normalized
// kernel with a non-negative window centred on omega, main is 1.
func fullRateTerms(kernel []complex128, m0 int, omega float64) (float64, float64) {
	var main, image complex128

	for i, k := range kernel {
		m := float64(m0 + i)
		main += k * cmplx.Rect(1, -omega*m)
		image += k * cmplx.Rect(1, omega*m)
	}

	return cmplx.Abs(main), cmplx.Abs(image)
}

// fullRateKernel builds bin b's kernel directly at the transform's sample
// rate, with the design of buildKernels: a periodic window of length
// l = ceil(q*fs/f), times exp(i*2*pi*f*m/fs) for m = floor(-l/2)..., divided
// by l and L1-normalized. It returns the kernel and the offset of its first
// sample from the frame centre.
func fullRateKernel(tr *Transform, q float64, wt window.Type, b int) ([]complex128, int) {
	fs, f := tr.SampleRate(), tr.Frequencies()[b]
	l := math.Ceil(q * fs / f)
	n := int(l)
	m0 := int(math.Floor(-l / 2))

	w := window.Generate(wt, n, window.WithPeriodic())
	kernel := make([]complex128, n)
	norm := 0.0

	for i := range kernel {
		kernel[i] = complex(w[i]/l, 0) * cmplx.Rect(1, 2*math.Pi*f*float64(m0+i)/fs)
		norm += cmplx.Abs(kernel[i])
	}

	for i := range kernel {
		kernel[i] /= complex(norm, 0)
	}

	return kernel, m0
}

// fullRateTone is the input of the full-rate and level tests: n samples at
// fs Hz of a sine of amplitude fullRateAmp at freq Hz.
func fullRateTone(n int, freq, fs float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = fullRateAmp * math.Sin(2*math.Pi*freq*float64(i)/fs+0.3)
	}

	return x
}

const fullRateAmp = 0.8

// fullRateNuLimit is the highest frequency, relative to the Nyquist
// frequency of a decimation stage's input, at which the tones of these
// tests may meet the octave low-pass. Its pass band ends at 0.5/1.001 =
// 0.4995, but its Hamming-windowed transition is about 4*2/256 = 0.03 wide
// and the ripple grows towards it; 0.36 keeps the tones well inside.
const fullRateNuLimit = 0.36

// fullRateConfig is a configuration of TestFullRateTones and
// TestNormalizationLevel, with the bins at whose centre frequencies tones
// are placed.
type fullRateConfig struct {
	name  string
	sr    float64
	n     int
	opts  []Option
	tones []int // TestFullRateTones
	// levelTones are the bins TestNormalizationLevel places tones at.
	levelTones []int
	window     window.Type
}

func fullRateConfigs() []fullRateConfig {
	return []fullRateConfig{
		// basic-pitch: 9 octaves of 36 bins. Bins 0-4 are in the lowest
		// octave (8 decimations), whose frames reach about 53000 samples to
		// either side through the kernel and the decimation filters; kernels
		// 15-19 meet the last low-pass at nu = 0.32-0.345.
		{
			name: "basic_pitch", sr: BasicPitchSampleRate, n: 163840, opts: BasicPitch(),
			tones:      []int{0, 4, 108, 112, 252, 256, 288, 292},
			levelTones: []int{2, 110, 254, 290}, window: window.TypeHann,
		},
		// The generic defaults: 7 octaves of 12 bins, every kernel below
		// nu = 0.18.
		{
			name: "generic_defaults", sr: 22050, n: 65536,
			tones:      []int{0, 5, 11, 36, 41, 72, 77, 83},
			levelTones: []int{5, 41, 77}, window: window.TypeHann,
		},
	}
}

// fullRateQ returns the quality factor of a configuration.
func fullRateQ(tb testing.TB, opts []Option) float64 {
	tb.Helper()

	cfg := defaultConfig()

	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			tb.Fatal(err)
		}
	}

	return cfg.filterScale / (math.Pow(2, 1/float64(cfg.binsPerOctave)) - 1)
}

// fullRateNu returns the frequency of a tone at bin b, relative to the
// Nyquist frequency, at the input of the last decimation stage before its
// octave o: f*2^o/SampleRate(). The earlier stages see it lower.
func fullRateNu(tr *Transform, b, o int) float64 {
	return tr.Frequencies()[b] * float64(int(1)<<o) / tr.SampleRate()
}

// TestFullRateTones compares the multirate transform with a full-rate direct
// CQT on pure tones at bin centre frequencies, in the top, a middle and the
// lowest octave, on two interior frames each.
//
// Both peak at the tone's bin. The magnitudes are related as follows. A tone
// below the low-pass's pass-band edge stays a pure tone through filtering
// and 2:1 decimation (it maps to itself; there is nothing to alias), scaled
// by |H(nu_j)| at each of the o stages before its octave. So with tones
// whose highest stage frequency is at most nuMax and the measured ripple
// delta = max ||H(nu)|-1| over [0, nuMax], the multirate magnitude is
// G*A/2*|main_mr ± image_mr| and the full-rate one A/2*|main_fr ± image_fr|,
// with G in [(1-delta)^o, (1+delta)^o] and main/image the kernels' gains
// for the tone's positive- and negative-frequency exponential
// (fullRateTerms). With L1-normalized Hann kernels main = 1; the kernel
// lengths differ (the multirate kernel has 2^o*ceil(q*fs/2^o/f) samples at
// the input rate, the full-rate one ceil(q*fs/f)), which only changes the
// image terms. Hence
//
//	(1-delta)^o*(1-image_mr)/(1+image_fr) <= mr/fr <= (1+delta)^o*(1+image_mr)/(1-image_fr),
//
// widened by 1e-12 for rounding. Measured (fullRateRipple on a 2.5e-5 grid):
//
//   - basic-pitch: nuMax = 0.3448, delta = 3.97e-4, image terms <= 2.9e-6
//     (multirate) and 1.9e-6 (full rate); worst |mr/fr-1| = 4.7e-4, widest
//     bound (8 decimations) 3.2e-3.
//   - generic defaults: nuMax = 0.1792, delta = 2.48e-4, image terms
//     <= 7.6e-6; worst |mr/fr-1| = 6.1e-4, widest bound (6 decimations)
//     1.5e-3.
//
// The ripple dominates; the bound does not try to predict G per tone, so it
// is a few times wider than the observed deviation.
func TestFullRateTones(t *testing.T) {
	t.Parallel()

	for _, cfg := range fullRateConfigs() {
		t.Run(cfg.name, func(t *testing.T) {
			t.Parallel()

			opts := slices.Concat(cfg.opts, []Option{WithNormalization(NormalizationConvolutional)})

			tr, err := New(cfg.sr, opts...)
			if err != nil {
				t.Fatal(err)
			}

			if tr.DownsampleFactor() != 1 {
				t.Fatalf("early downsampling factor %d, want 1", tr.DownsampleFactor())
			}

			q := fullRateQ(t, opts)
			bins, kernels, pad := tr.Bins(), tr.Kernels(), tr.NFFT()/2
			freqs, lengths := tr.Frequencies(), tr.Lengths()
			mid := tr.FrameCount(cfg.n) / 2
			frames := []int{mid - 5, mid + 3}
			lowest, _ := fullRatePlace(tr, 0)

			for _, fr := range frames {
				if !fullRateInterior(tr, cfg.n, fr, lowest) {
					t.Fatalf("frame %d of the lowest octave is not interior", fr)
				}
			}

			nuMax := 0.0

			for _, b := range cfg.tones {
				if o, _ := fullRatePlace(tr, b); o > 0 {
					nuMax = max(nuMax, fullRateNu(tr, b, o))
				}
			}

			if nuMax > fullRateNuLimit {
				t.Fatalf("tones reach nu = %.4f > %.2f", nuMax, fullRateNuLimit)
			}

			delta := fullRateRipple(tr.Lowpass(), nuMax)

			// Multirate magnitudes of every tone, and full-rate ones at the
			// two frames: full[tone][frame][bin].
			tones := make([][]float64, len(cfg.tones))
			mr := make([][]float64, len(cfg.tones))
			full := make([][][]float64, len(cfg.tones))

			for i, b := range cfg.tones {
				tones[i] = fullRateTone(cfg.n, freqs[b], cfg.sr)

				mr[i], err = tr.Process(tones[i])
				if err != nil {
					t.Fatal(err)
				}

				full[i] = make([][]float64, len(frames))
				for j := range frames {
					full[i][j] = make([]float64, bins)
				}
			}

			imageFull := make([]float64, len(cfg.tones))

			for b := range bins {
				kernel, m0 := fullRateKernel(tr, q, cfg.window, b)
				if float64(len(kernel)) != lengths[b] {
					t.Fatalf("bin %d: full-rate kernel of %d samples, Lengths() %g", b, len(kernel), lengths[b])
				}

				for i, x := range tones {
					if cfg.tones[i] == b {
						main, image := fullRateTerms(kernel, m0, 2*math.Pi*freqs[b]/cfg.sr)
						if math.Abs(main-1) > 1e-12 {
							t.Fatalf("bin %d: full-rate main gain %.15g, want 1", b, main)
						}

						imageFull[i] = image
					}

					for j, fr := range frames {
						c := fr * tr.Hop()
						if c+m0 < 0 || c+m0+len(kernel) > cfg.n {
							t.Fatalf("bin %d: full-rate kernel at frame %d leaves the signal", b, fr)
						}

						// sum x*conj(kernel); only the magnitude is used.
						var re, im float64

						seg := x[c+m0 : c+m0+len(kernel)]
						for k, v := range kernel {
							re += seg[k] * real(v)
							im += seg[k] * imag(v)
						}

						full[i][j][b] = math.Hypot(re, im)
					}
				}
			}

			worstDev, worstBound := 0.0, 0.0

			for i, b := range cfg.tones {
				o, k := fullRatePlace(tr, b)
				main, image := fullRateTerms(kernels[k], -pad, 2*math.Pi*freqs[b]*float64(int(1)<<o)/tr.SampleRate())

				if math.Abs(main-1) > 1e-12 {
					t.Fatalf("bin %d: multirate main gain %.15g, want 1", b, main)
				}

				g := float64(o)
				lo := math.Pow(1-delta, g)*(1-image)/(1+imageFull[i]) - 1e-12
				hi := math.Pow(1+delta, g)*(1+image)/(1-imageFull[i]) + 1e-12
				worstBound = max(worstBound, hi-1, 1-lo)

				for j, fr := range frames {
					row := mr[i][fr*bins : (fr+1)*bins]
					ref := full[i][j]

					if p, pf := argmax(row), argmax(ref); p != b || pf != b {
						t.Errorf("tone at bin %d, frame %d: multirate peaks at %d, full rate at %d", b, fr, p, pf)
					}

					ratio := row[b] / ref[b]
					worstDev = max(worstDev, math.Abs(ratio-1))

					if !(ratio >= lo && ratio <= hi) {
						t.Errorf("tone at bin %d (octave %d), frame %d: multirate/full-rate %.9f outside [%.9f, %.9f]",
							b, o, fr, ratio, lo, hi)
					}
				}

				t.Logf("bin %3d octave %d: image terms %.1e (multirate) %.1e (full rate)", b, o, image, imageFull[i])
			}

			t.Logf("nuMax %.4f, ripple %.2e, worst |mr/fr-1| %.2e, widest bound %.2e", nuMax, delta, worstDev, worstBound)
		})
	}
}

func argmax(v []float64) int {
	best := 0

	for i, x := range v {
		if x > v[best] {
			best = i
		}
	}

	return best
}

// TestNormalizationLevel replaces 46.1's TestLibrosaSanity. 46.1 checked
// NormalizationLibrosa against librosa 1.0.0's cqt in basic-pitch's
// configuration on the fixture signal (median ratio librosa/ours 1.0013 on
// the interior frames); that Python check was removed in 46.4 together with
// the fixtures. This test checks the same level in Go: a stationary sine of
// amplitude A at bin b's centre frequency, analysed by L1-normalized Hann
// kernels, gives in interior frames
//
//   - F*A/2 with NormalizationConvolutional, F being DownsampleFactor()
//     (every normalization multiplies by it, as nnAudio does),
//   - sqrt(Lengths()[b]) times that with NormalizationLibrosa, which for
//     F = 1 is librosa's scale=True level A*sqrt(l_b)/2,
//   - 2 times that with NormalizationWrap.
//
// The convolutional level is F*A/2*G*|main ± image| (see TestFullRateTones):
// main = 1 for L1 Hann kernels (checked to 1e-12), image is the kernel's gain
// at the tone's negative frequency (computed from Kernels(), at most 7.6e-6
// here), and G is the product of the low-pass gains, within (1±delta)^o for
// the o octave decimations and a further factor 1±delta_early for early
// downsampling, the ripples measured over the band the tones occupy. The
// tolerance is therefore F*A/2*[(1-delta_early)(1-delta)^o(1-image),
// (1+delta_early)(1+delta)^o(1+image)], widened by 1e-12. Measured worst
// deviations from F*A/2: basic-pitch 5.8e-4 (delta 3.71e-4, bound up to
// 3.0e-3 at 8 decimations), generic defaults 6.2e-4 (delta 2.34e-4, bound
// 1.4e-3 at 6 decimations), early downsampling by 8 8.1e-4 (delta 2.36e-4,
// delta_early 8.55e-4, bound 1.6e-3 at 3 decimations).
//
// The librosa and wrap levels are checked over the whole output, not only
// the tone's bin: wrap is exactly 2 times convolutional (a power-of-two
// scale is exact), librosa is sqrt(l_b) times it to within 4e-15 relative
// (measured 4.5e-16).
func TestNormalizationLevel(t *testing.T) {
	t.Parallel()

	configs := slices.Concat(fullRateConfigs(), []fullRateConfig{
		// Early downsampling by 8 (the generic early_ds8 configuration),
		// 4 octaves; the tones are at most at 450 Hz, nu = 0.041 for the
		// early low-pass (pass band up to 0.121).
		{
			name: "generic_early_ds8", sr: 22050, n: 65536,
			opts:       []Option{WithHopLength(1024), WithFMin(30), WithBins(48)},
			levelTones: []int{0, 6, 20, 47}, window: window.TypeHann,
		},
	})

	for _, cfg := range configs {
		t.Run(cfg.name, func(t *testing.T) {
			t.Parallel()

			var out [3]*Transform

			for i, norm := range []Normalization{
				NormalizationConvolutional, NormalizationLibrosa, NormalizationWrap,
			} {
				tr, err := New(cfg.sr, slices.Concat(cfg.opts, []Option{WithNormalization(norm)})...)
				if err != nil {
					t.Fatal(err)
				}

				out[i] = tr
			}

			conv := out[0]
			factor := conv.DownsampleFactor()
			bins, kernels, pad := conv.Bins(), conv.Kernels(), conv.NFFT()/2
			freqs, lengths := conv.Frequencies(), conv.Lengths()
			mid := conv.FrameCount(cfg.n) / 2
			lowest, _ := fullRatePlace(conv, 0)

			if !fullRateInterior(conv, cfg.n, mid, lowest) {
				t.Fatalf("frame %d of the lowest octave is not interior", mid)
			}

			nuMax, nuEarly := 0.0, 0.0

			for _, b := range cfg.levelTones {
				if o, _ := fullRatePlace(conv, b); o > 0 {
					nuMax = max(nuMax, fullRateNu(conv, b, o))
				}

				nuEarly = max(nuEarly, freqs[b]/(cfg.sr/2))
			}

			if nuMax > fullRateNuLimit {
				t.Fatalf("tones reach nu = %.4f > %.2f", nuMax, fullRateNuLimit)
			}

			delta, deltaEarly := fullRateRipple(conv.Lowpass(), nuMax), 0.0
			if factor > 1 {
				deltaEarly = fullRateRipple(conv.EarlyLowpass(), nuEarly)
			}

			worstDev, worstLib := 0.0, 0.0

			for ti, b := range cfg.levelTones {
				x := fullRateTone(cfg.n, freqs[b], cfg.sr)

				// The librosa and wrap outputs are compared with the
				// convolutional one over the whole output of the first tone.
				var y [3][]float64

				for i, tr := range out {
					if i > 0 && ti > 0 {
						break
					}

					var err error

					y[i], err = tr.Process(x)
					if err != nil {
						t.Fatal(err)
					}
				}

				o, k := fullRatePlace(conv, b)
				main, image := fullRateTerms(kernels[k], -pad, 2*math.Pi*freqs[b]*float64(int(1)<<o)/conv.SampleRate())

				if math.Abs(main-1) > 1e-12 {
					t.Fatalf("bin %d: main gain %.15g, want 1", b, main)
				}

				level := float64(factor) * fullRateAmp / 2
				lo := (1-deltaEarly)*math.Pow(1-delta, float64(o))*(1-image) - 1e-12
				hi := (1+deltaEarly)*math.Pow(1+delta, float64(o))*(1+image) + 1e-12

				got := y[0][mid*bins+b] / level
				worstDev = max(worstDev, math.Abs(got-1))

				if !(got >= lo && got <= hi) {
					t.Errorf("bin %d (octave %d): convolutional level %.9f*F*A/2, want within [%.9f, %.9f]",
						b, o, got, lo, hi)
				}

				if ti > 0 {
					continue
				}

				for i, v := range y[0] {
					want := math.Sqrt(lengths[i%bins]) * v

					d := math.Abs(y[1][i] - want)
					if v != 0 {
						worstLib = max(worstLib, d/want)
					}

					if !(d <= 4e-15*want) {
						t.Fatalf("bin %d frame %d: librosa %.17g, sqrt(l_b)*convolutional %.17g",
							i%bins, i/bins, y[1][i], want)
					}

					if y[2][i] != 2*v {
						t.Fatalf("bin %d frame %d: wrap %.17g, 2*convolutional %.17g", i%bins, i/bins, y[2][i], 2*v)
					}
				}
			}

			if factor > 1 {
				t.Logf("early downsampling by %d: nu up to %.4f, ripple %.2e", factor, nuEarly, deltaEarly)
			}

			t.Logf("nuMax %.4f, ripple %.2e; worst level deviation %.2e, librosa vs sqrt(l)*conv %.2e",
				nuMax, delta, worstDev, worstLib)
		})
	}
}
