package melody

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

const (
	// DefaultCQTBinsPerOctave is the recommended resolution for [WithCQT]:
	// 36 bins per octave, three per semitone. On the package's synthetic
	// glides and low notes it is at least as accurate as the STFT in every
	// case at the default filter scale. 24 bins are about as accurate but
	// put every other bin between two semitones for chroma, 48 or 60 bins
	// have longer kernels that smear fast glides, and 12 bins do not fit
	// the default band into the 5 octaves the default hop allows.
	DefaultCQTBinsPerOctave = 36
	// DefaultCQTVoicingThreshold is the voicing threshold [WithCQT] sets.
	// The CQT voicing of noise is much higher than the STFT voicing, because
	// ±2 constant-Q bins around the upper harmonics span many Hz: white
	// noise reaches about 0.6 (mean 0.4), so the STFT default
	// [DefaultVoicingThreshold] would voice it. Clean harmonic tones stay
	// close to 1.
	DefaultCQTVoicingThreshold = 0.6
)

// cqtConfig is the [WithCQT] front-end configuration.
type cqtConfig struct {
	binsPerOctave int
	opts          []cqt.Option
}

// WithCQT replaces the STFT front end of [Analyze] with a constant-Q
// transform (package dsp/cqt) of binsPerOctave bins per octave (>= 1;
// [DefaultCQTBinsPerOctave] is recommended). Salience, voicing and chroma
// are then taken from the CQT magnitudes instead of the FFT magnitudes; the
// frames (count ceil(n/hop), frame i centred on sample i*hop), the candidate
// grid, the RMS gate, the smoothing and the note segmentation are unchanged,
// and [WithFFTSize] has no effect. Without WithCQT, Analyze uses the STFT
// and its results are unchanged.
//
// WithCQT also sets the voicing threshold to [DefaultCQTVoicingThreshold].
// Give [WithVoicingThreshold] after it to change that, and give WithCQT
// after a preset such as [BassPreset], which sets the threshold too.
//
// The transform covers the candidates and the band: its bin 1 lies on the
// lowest candidate's fundamental f1 = MIDIToFrequency(minMIDI, referenceHz),
// bin 0 one bin below it (so that the interpolation below has support
// there), and the bins continue up to the band's upper edge maxHz, or the
// Nyquist frequency if that is lower. It is configured with these cqt
// options, in this order:
//
//	cqt.WithBinsPerOctave(binsPerOctave), cqt.WithFMin(f1*2^(-1/binsPerOctave)),
//	cqt.WithBins(bins), cqt.WithCenter(cqt.PadZero),
//	cqt.WithNormalization(cqt.NormalizationWrap), cqt.WithEarlyDownsampling(false),
//	opts...,
//	cqt.WithBinsPerOctave(binsPerOctave), cqt.WithHopLength(hop),
//	cqt.WithOutput(cqt.OutputMagnitude)
//
// so opts may change, for example, the filter scale or the window, but not
// the resolution, the hop or the output. The wrap normalization with the
// default L1 kernel norm reads a sine of amplitude A at a bin centre as
// about A at any frequency, which harmonic summation across frequencies
// needs; the librosa normalization would weight each bin by the square root
// of its kernel length. Early downsampling is off because it divides the hop
// by its factor, which then often fails the hop check below ([BassPreset]
// at the default hop, for example). When overriding the lowest frequency
// with cqt.WithFMin, also set cqt.WithBins so that the top bin stays below
// the Nyquist frequency. Harmonic positions follow the transform's actual
// bin frequencies.
//
// The hop must be divisible by 2^(octaves-1), octaves =
// ceil(bins/binsPerOctave). The default hop of 240 = 16*15 allows at most 5
// octaves, which at 36 bins per octave covers the default candidates and
// band (E3 165 Hz to 5 kHz, 4.95 octaves with the extra bin) and
// [BassPreset] (E1 41 Hz to 1.2 kHz, 4.89 octaves). At 12 bins per octave
// the extra bin is a semitone and the default band needs 6 octaves. For
// such spans [Analyze] returns an error wrapping cqt.ErrHop; narrow the band
// with [WithFrequencyRange], raise the lowest candidate, or use a hop with
// more factors of two. All errors of cqt.New are wrapped, so test for them
// with errors.Is.
//
// What differs from the STFT front end:
//
//   - Chroma adds the power of every CQT bin whose centre lies inside the
//     band to the pitch class of its nearest equal-tempered note. The CQT
//     starts one bin below the lowest candidate, so chroma below that (100
//     to 160 Hz with the defaults) is not seen. With binsPerOctave a
//     multiple of 12 every class gets the same number of bins; with an even
//     multiple (24, 48) every other bin lies exactly between two semitones,
//     and rounding decides its class.
//   - Salience interpolates the CQT magnitudes at the fractional bin
//     binsPerOctave*log2(h*f/f0) of harmonic h of candidate f (f0 the
//     centre of bin 0) with a 6-tap Lanczos-3 kernel instead of the linear
//     interpolation of the STFT front end. A linear interpolation peaks only
//     at bin centres, and since harmonic intervals are close to
//     equal-tempered ones, all harmonics then snap to the bin grid together
//     (on a slow glide at 36 bins per octave, a mean error of 0.06 semitone
//     against 0.024 with Lanczos-3).
//   - Voicing is the power within ±2 CQT bins of each harmonic (the main
//     lobe of the Hann kernels at filter scale 1) over the total power of
//     the band's CQT bins. ±2 constant-Q bins are many Hz wide at the upper
//     harmonics, so noise scores much higher than with the STFT; hence the
//     higher [DefaultCQTVoicingThreshold]. A filter scale s widens the main
//     lobe to ±2/s bins and lowers the voicing of tones.
//   - The whole signal is transformed once per call, so Analyze holds
//     frames*bins magnitudes in memory (8.6 MB per minute at the defaults
//     with 36 bins per octave). Signals too short for the transform's
//     decimation chain are zero padded at the end to the shortest length it
//     accepts (16 samples for 5 octaves).
//
// The low bins have long kernels (at 36 bins per octave about 0.31 s at
// 165 Hz and 1.25 s at 41 Hz), but the harmonic sum is dominated by the
// higher harmonics, whose kernels are short, so glides are followed at
// least as accurately as with the STFT; the package tests record the
// errors.
func WithCQT(binsPerOctave int, opts ...cqt.Option) Option {
	optsCopy := append([]cqt.Option(nil), opts...)

	return func(cfg *config) error {
		if binsPerOctave < 1 {
			return fmt.Errorf("%w: CQT bins per octave must be >= 1, got %d", ErrInvalidOption, binsPerOctave)
		}

		for i, o := range optsCopy {
			if o == nil {
				return fmt.Errorf("%w: CQT option %d", ErrNilOption, i)
			}
		}

		cfg.cqt = &cqtConfig{binsPerOctave: binsPerOctave, opts: optsCopy}
		cfg.voicing = DefaultCQTVoicingThreshold

		return nil
	}
}

// cqtFront is the CQT front end: the magnitudes of the whole signal,
// frame-major, computed once per [Analyze] call.
type cqtFront struct {
	tr            *cqt.Transform
	binsPerOctave float64
	fmin          float64 // centre of bin 0
	bins          int
	mags          []float64
	// buf holds one frame's band magnitudes with bufPad zero bins on each
	// side; analyzer.mag is buf[bufPad : bufPad+bins].
	buf       []float64
	harmonics []lanczosHarmonic
}

// interpTaps is the number of CQT bins each harmonic is interpolated from
// (Lanczos-3), and bufPad the number of zero bins on each side of
// cqtFront.buf, so that every tap of an inside harmonic is a valid index.
const (
	interpTaps = 6
	bufPad     = 3
)

// cqtBinsBelow is the number of CQT bins below the lowest candidate's
// fundamental, so that its interpolation taps are mostly real bins.
const cqtBinsBelow = 1

// lanczosHarmonic is a precomputed Lanczos-3 interpolation of the CQT
// magnitudes at a fractional bin p: the weighted sum of bins floor(p)-2 ..
// floor(p)+3, which are buf[idx .. idx+5], with the weights normalised to
// sum 1; bins outside the transform read as 0. It is 0 when p is negative
// or floor(p)+1 is past the last bin, as for the linear STFT interpolation.
type lanczosHarmonic struct {
	weight  float64
	idx     int
	c       [interpTaps]float64
	outside bool
}

// initCQT sets up the CQT front end; see [WithCQT].
func (a *analyzer) initCQT(sampleRate float64) error {
	cfg := a.cfg
	bpo := cfg.cqt.binsPerOctave
	fmin := pitch.MIDIToFrequency(cfg.minMIDI, cfg.referenceHz) * math.Pow(2, -cqtBinsBelow/float64(bpo))
	top := math.Min(cfg.maxHz, sampleRate/2)

	if !(fmin < top) {
		return fmt.Errorf("%w: CQT front end: lowest candidate %g Hz is not below the band edge %g Hz",
			ErrInvalidOption, fmin, top)
	}

	// Every bin centre fmin*2^(b/bpo) up to top, computed as dsp/cqt does so
	// that the top bin never exceeds the Nyquist frequency.
	bins := int(math.Floor(float64(bpo)*math.Log2(top/fmin))) + 1
	for bins > 1 && fmin*math.Pow(2, float64(bins-1)/float64(bpo)) > top {
		bins--
	}

	opts := make([]cqt.Option, 0, len(cfg.cqt.opts)+9)
	opts = append(opts,
		cqt.WithBinsPerOctave(bpo),
		cqt.WithFMin(fmin),
		cqt.WithBins(bins),
		cqt.WithCenter(cqt.PadZero),
		cqt.WithNormalization(cqt.NormalizationWrap),
		cqt.WithEarlyDownsampling(false),
	)
	opts = append(opts, cfg.cqt.opts...)
	opts = append(opts,
		cqt.WithBinsPerOctave(bpo),
		cqt.WithHopLength(cfg.hop),
		cqt.WithOutput(cqt.OutputMagnitude),
	)

	tr, err := cqt.New(sampleRate, opts...)
	if err != nil {
		return fmt.Errorf("melody: CQT front end (%d bins from %.4g Hz, %d per octave, hop %d): %w",
			bins, fmin, bpo, cfg.hop, err)
	}

	freqs := tr.Frequencies()
	n := len(freqs)
	a.cq = &cqtFront{
		tr:            tr,
		binsPerOctave: float64(bpo),
		fmin:          freqs[0],
		bins:          n,
		buf:           make([]float64, n+2*bufPad),
	}
	a.allocBins(n)
	a.mag = a.cq.buf[bufPad : bufPad+n : bufPad+n]

	a.lo, a.hi = n, -1
	for k, f := range freqs {
		a.pitchClass[k] = pitchClassOf(f, cfg.referenceHz)

		if f >= cfg.minHz && f <= cfg.maxHz {
			a.lo, a.hi = min(a.lo, k), max(a.hi, k)
		}
	}

	return nil
}

// addHarmonic appends the Lanczos-3 interpolation at fractional bin bin.
func (c *cqtFront) addHarmonic(bin, weight float64) {
	i := math.Floor(bin)
	t := bin - i
	h := lanczosHarmonic{
		weight:  weight,
		idx:     int(i) - interpTaps/2 + 1 + bufPad,
		outside: bin < 0 || int(i)+1 >= c.bins,
	}

	sum := 0.0

	for j := range h.c {
		h.c[j] = lanczos3(float64(j-interpTaps/2+1) - t)
		sum += h.c[j]
	}

	for j := range h.c {
		h.c[j] /= sum
	}

	c.harmonics = append(c.harmonics, h)
}

// lanczos3 is the Lanczos kernel sinc(x)*sinc(x/3) for |x| < 3, else 0.
func lanczos3(x float64) float64 {
	const a = 3

	switch {
	case x == 0:
		return 1
	case math.Abs(x) >= a:
		return 0
	}

	px := math.Pi * x

	return a * math.Sin(px) * math.Sin(px/a) / (px * px)
}

// salience is analyzer.salience for the CQT front end.
func (c *cqtFront) salience(cands []candidate) float64 {
	best, bestMIDI := 0.0, 0.0
	buf := c.buf

	for _, cand := range cands {
		s := 0.0

		for _, h := range c.harmonics[cand.first : cand.first+cand.count] {
			if h.outside {
				continue
			}

			m := buf[h.idx : h.idx+interpTaps : h.idx+interpTaps]
			v := m[0]*h.c[0] + m[1]*h.c[1] + m[2]*h.c[2] + m[3]*h.c[3] + m[4]*h.c[4] + m[5]*h.c[5]
			s += h.weight * v
		}

		if s > best {
			best, bestMIDI = s, cand.midi
		}
	}

	return bestMIDI
}

// process computes the CQT magnitudes of the whole signal. A signal too
// short for the transform is zero padded at the end to the shortest length
// the transform accepts (16 samples for 5 octaves); the transform has
// n/hop+1 >= ceil(n/hop) frames either way.
func (c *cqtFront) process(mono []float64) error {
	x := mono

	if c.tr.FrameCount(len(x)) == 0 {
		// FrameCount is monotonic in n and positive from at most
		// 2^(octaves-1) samples on (more under a user's PadReflect, which
		// needs the lowest octave longer than nfft/2), so this terminates.
		n := len(x) + 1
		for c.tr.FrameCount(n) == 0 {
			n++
		}

		x = make([]float64, n)
		copy(x, mono)
	}

	mags, err := c.tr.Process(x)
	if err != nil {
		return fmt.Errorf("melody: CQT front end: %w", err)
	}

	c.mags = mags

	return nil
}

// row returns the magnitudes of one frame.
func (c *cqtFront) row(frame int) []float64 {
	return c.mags[frame*c.bins : (frame+1)*c.bins]
}
