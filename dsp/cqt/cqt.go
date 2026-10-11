package cqt

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrNilOption reports a nil [Option] passed to [New].
	ErrNilOption = errors.New("cqt: nil option")
	// ErrInvalidOption reports an invalid sample rate or option value, or a
	// configuration whose kernels cannot be built.
	ErrInvalidOption = errors.New("cqt: invalid option")
	// ErrNyquist reports a top bin above the Nyquist frequency.
	ErrNyquist = errors.New("cqt: top bin exceeds the Nyquist frequency")
	// ErrHop reports an effective hop size (after early downsampling) that
	// is zero or not divisible by 2^(octaves-1).
	ErrHop = errors.New("cqt: hop length not divisible by 2^(octaves-1)")
	// ErrSignalTooShort reports an empty signal, or one too short to be
	// downsampled down to the lowest octave.
	ErrSignalTooShort = errors.New("cqt: signal too short")
	// ErrShortDst reports a destination slice shorter than required.
	ErrShortDst = errors.New("cqt: destination too short")
)

const (
	// lowpassTaps is the length of nnAudio's anti-alias filters.
	lowpassTaps = 256
	// maxNFFT bounds the kernel frame length so that absurd configurations
	// (a tiny fmin with many octaves) fail instead of exhausting memory.
	maxNFFT = 1 << 24
)

// kernel is the support of one top-octave kernel inside its nfft frame:
// re/im hold the non-zero samples at frame offsets [start, start+len(re)).
type kernel struct {
	start int
	re    []float64
	im    []float64
}

// Transform computes a multi-rate constant-Q transform. Create it with
// [New]; the [NNAudio] and [BasicPitch] presets make it reproduce nnAudio's
// CQT2010v2.
//
// A Transform holds scratch buffers and is not safe for concurrent use; see
// [Transform.Clone].
type Transform struct {
	sampleRate    float64 // effective sample rate of the top octave
	hop           int     // effective hop of the top octave
	nBins         int
	nFilters      int
	nOctaves      int
	nFFT          int
	factor        int // early downsampling factor
	padding       Padding
	zeroPadShort  bool // nnAudio quirk: zero pad octaves too short to reflect
	output        Output
	normalization Normalization

	kernels      []kernel  // nFilters top-octave kernels
	lowpass      []float64 // octave anti-alias filter
	earlyLowpass []float64 // nil without early downsampling
	freqs        []float64 // nBins centre frequencies
	lengths      []float64 // nBins kernel lengths
	scale        []float64 // nBins output scale factors

	// Scratch, grown lazily to the input length and reused.
	octave [2][]float64 // ping-pong octave signals
	padded []float64    // padded octave signal
	in64   []float64    // float32 input converted to float64
}

// New returns a constant-Q transform for signals sampled at sampleRate Hz.
// Without options it uses the generic defaults: hop 512, fmin 32.70 Hz, 84
// bins, 12 bins per octave, filter scale 1, periodic Hann kernels with L1
// normalization, zero padding ([PadZero]), early downsampling, librosa
// normalization and magnitude output. Every kernel is centred on the
// frequency [Transform.Frequencies] reports. [NNAudio] and [BasicPitch]
// return presets that reproduce nnAudio's CQT2010v2 and basic-pitch's front
// end exactly.
//
// New follows nnAudio's setup: the top octave's kernels are built at the
// (possibly early-downsampled) sample rate, the top bin must not exceed the
// Nyquist frequency ([ErrNyquist]), and early downsampling is applied when
// nnAudio would apply it. In addition, New requires the effective hop size to
// be divisible by 2^(octaves-1) ([ErrHop]). nnAudio halves the hop with
// integer division for each lower octave; for other hops the octaves yield
// different frame counts and nnAudio fails when it concatenates them.
func New(sampleRate float64, opts ...Option) (*Transform, error) {
	if !finitePositive(sampleRate) {
		return nil, fmt.Errorf("%w: sample rate %g", ErrInvalidOption, sampleRate)
	}

	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	return build(sampleRate, cfg)
}

// build follows nnAudio CQT2010v2.__init__; the nnAudio quirks are applied
// only when the [NNAudio] options set them.
func build(sr float64, cfg config) (*Transform, error) {
	bpo := cfg.binsPerOctave
	q := cfg.filterScale / (math.Pow(2, 1/float64(bpo)) - 1)

	// The 2:1 anti-alias filter: band centre 0.5, transition 0.001. The
	// variables keep Go from folding the band edges into exact constants;
	// nnAudio computes them in float64.
	bandCenter, transition := 0.5, 0.001

	lowpass, err := lowpassFilter(bandCenter, transition)
	if err != nil {
		return nil, err
	}

	nFilters := min(bpo, cfg.nBins)
	nOctaves := (cfg.nBins + bpo - 1) / bpo

	// The top octave's kernels analyse the highest nFilters bins; every
	// lower octave reuses them at half the sample rate, and the surplus
	// kernels of the lowest octave are cropped. With fewer bins than one
	// octave the single octave is cropped at the top instead: its kernels
	// start at fmin, as if the full octave had been built and its upper
	// kernels dropped (without building them, since they may lie above the
	// Nyquist frequency).
	fminT, fmaxT := topOctaveRange(cfg, nFilters, nOctaves)

	if fmaxT > sr/2 {
		return nil, fmt.Errorf("%w: top bin %g Hz > %g Hz", ErrNyquist, fmaxT, sr/2)
	}

	hop, srEff, factor := cfg.hop, sr, 1

	var earlyLowpass []float64

	if cfg.earlyDownsample {
		factor = earlyDownsampleFactor(sr, cfg.hop, fmaxT, q, nOctaves)
		if factor != 1 {
			hop /= factor
			srEff = sr / float64(factor)

			earlyLowpass, err = lowpassFilter(1/float64(factor), 0.03)
			if err != nil {
				return nil, err
			}
		}
	}

	if nOctaves-1 >= 62 || hop < 1 || hop%(1<<(nOctaves-1)) != 0 {
		return nil, fmt.Errorf("%w: effective hop %d (hop %d / early downsampling factor %d), %d octaves",
			ErrHop, hop, cfg.hop, factor, nOctaves)
	}

	kernels, nFFT, err := buildKernels(cfg, q, srEff, kernelFrequencies(cfg, fminT, nFilters))
	if err != nil {
		return nil, err
	}

	t := &Transform{
		sampleRate:    srEff,
		hop:           hop,
		nBins:         cfg.nBins,
		nFilters:      nFilters,
		nOctaves:      nOctaves,
		nFFT:          nFFT,
		factor:        factor,
		padding:       cfg.padding,
		zeroPadShort:  cfg.nnAudioShortOctaves,
		output:        cfg.output,
		normalization: cfg.normalization,
		kernels:       kernels,
		lowpass:       lowpass,
		earlyLowpass:  earlyLowpass,
		freqs:         make([]float64, cfg.nBins),
		lengths:       make([]float64, cfg.nBins),
		scale:         make([]float64, cfg.nBins),
	}

	for b := range cfg.nBins {
		f := binFrequency(cfg, b)
		t.freqs[b] = f
		t.lengths[b] = math.Ceil(q * srEff / f)

		s := float64(factor)

		switch cfg.normalization {
		case NormalizationLibrosa:
			s *= math.Sqrt(t.lengths[b])
		case NormalizationWrap:
			s *= 2
		case NormalizationConvolutional:
		}

		t.scale[b] = s
	}

	return t, nil
}

// lowpassFilter is nnAudio's create_lowpass_filter: a 256-tap firwin2
// low-pass with the pass band up to bandCenter/(1+transition) and the stop
// band from bandCenter*(1+transition), frequencies relative to Nyquist.
func lowpassFilter(bandCenter, transition float64) ([]float64, error) {
	passMax := bandCenter / (1 + transition)
	stopMin := bandCenter * (1 + transition)

	h, err := design.Firwin2(lowpassTaps, []float64{0, passMax, stopMin, 1}, []float64{1, 1, 0, 0})
	if err != nil {
		return nil, fmt.Errorf("cqt: design low-pass filter: %w", err)
	}

	return h, nil
}

// earlyDownsampleFactor is nnAudio's get_early_downsample_params /
// early_downsample_count (taken from librosa): the largest power of two the
// input can be decimated by before the top octave, limited by the top
// octave's bandwidth and by the hop size.
func earlyDownsampleFactor(sr float64, hop int, fmaxT, q float64, nOctaves int) int {
	const windowBandwidth = 1.5 // for the Hann window

	cutoff := fmaxT * (1 + 0.5*windowBandwidth/q)
	nyquist := math.Floor(sr / 2) // python sr // 2

	count1 := max(0, int(math.Ceil(math.Log2(0.85*nyquist/cutoff))-1)-1)
	numTwos := int(math.Ceil(math.Log2(float64(hop))))
	count2 := max(0, numTwos-nOctaves+1)

	return 1 << min(count1, count2)
}

// topOctaveRange returns the frequencies of the lowest and highest kernel of
// the top octave, which covers the highest nFilters bins.
//
// The generic placement takes them from the bin frequencies. nnAudio instead
// starts the top octave a full octave minus one bin below the top bin, which
// is the same for bins >= binsPerOctave but shifts the kernels of a single
// partial octave (bins < binsPerOctave) below Frequencies(); see [NNAudio].
func topOctaveRange(cfg config, nFilters, nOctaves int) (float64, float64) {
	bpo := cfg.binsPerOctave

	if !cfg.nnAudioKernelPlacement {
		return binFrequency(cfg, cfg.nBins-nFilters), binFrequency(cfg, cfg.nBins-1)
	}

	fminT := cfg.fmin * math.Pow(2, float64(nOctaves-1))

	var fmaxT float64
	if rem := cfg.nBins % bpo; rem == 0 {
		fmaxT = fminT * math.Pow(2, float64(bpo-1)/float64(bpo))
	} else {
		fmaxT = fminT * math.Pow(2, float64(rem-1)/float64(bpo))
	}

	return fmaxT / math.Pow(2, 1-1/float64(bpo)), fmaxT
}

// binFrequency returns the centre frequency of bin b, fmin*2^(b/bpo).
func binFrequency(cfg config, b int) float64 {
	return cfg.fmin * math.Pow(2, float64(b)/float64(cfg.binsPerOctave))
}

// kernelFrequencies returns the frequencies of the nFilters top-octave
// kernels. The generic placement uses the bin frequencies themselves, so the
// top octave's kernels sit exactly at Frequencies(); nnAudio steps up from
// fminT.
func kernelFrequencies(cfg config, fminT float64, nFilters int) []float64 {
	freqs := make([]float64, nFilters)

	for k := range freqs {
		if cfg.nnAudioKernelPlacement {
			freqs[k] = fminT * math.Pow(2, float64(k)/float64(cfg.binsPerOctave))
		} else {
			freqs[k] = binFrequency(cfg, cfg.nBins-nFilters+k)
		}
	}

	return freqs
}

// buildKernels is nnAudio's create_cqt_kernels for the top octave: one
// windowed complex exponential per frequency, each centred in an nfft frame.
func buildKernels(cfg config, q, fs float64, freqs []float64) ([]kernel, int, error) {
	lengths := make([]float64, len(freqs))
	maxLen := 0.0

	for k, f := range freqs {
		lengths[k] = math.Ceil(q * fs / f)
		maxLen = max(maxLen, lengths[k])
	}

	if !(maxLen <= maxNFFT) {
		return nil, 0, fmt.Errorf("%w: kernel length %g exceeds %d", ErrInvalidOption, maxLen, maxNFFT)
	}

	nFFT := int(math.Pow(2, math.Ceil(math.Log2(maxLen))))
	if nFFT < 2 {
		// A one-sample frame has no padding, and the octaves would then
		// yield different frame counts.
		return nil, 0, fmt.Errorf("%w: single-sample kernels (filter scale %g too small)",
			ErrInvalidOption, cfg.filterScale)
	}

	winOpts := make([]window.Option, 0, len(cfg.windowOpts)+1)
	winOpts = append(winOpts, cfg.windowOpts...)
	winOpts = append(winOpts, window.WithPeriodic())

	kernels := make([]kernel, len(freqs))

	for k := range kernels {
		freq, l := freqs[k], lengths[k]
		n := int(l)

		start := int(math.Ceil(float64(nFFT)/2 - l/2))
		if n%2 == 1 {
			start-- // the extra zero goes to the right-hand side
		}

		win := []float64{1} // scipy's get_window returns [1] for length 1
		if n > 1 {
			win = window.Generate(cfg.windowType, n, winOpts...)
		}

		err := checkWindow(win)
		if err != nil {
			return nil, 0, err
		}

		re, im := make([]float64, n), make([]float64, n)
		m0 := math.Floor(-l / 2)
		// numpy divides a complex array by a real scalar by multiplying with
		// its reciprocal; mirror that.
		invFs, invL := 1/fs, 1/l

		for i := range n {
			m := m0 + float64(i)
			s, c := math.Sincos(m * 2 * math.Pi * freq * invFs)
			re[i] = win[i] * c * invL
			im[i] = win[i] * s * invL
		}

		normalizeKernel(re, im, cfg.basisNorm)

		kernels[k] = kernel{start: start, re: re, im: im}
	}

	return kernels, nFFT, nil
}

func checkWindow(w []float64) error {
	nonZero := false

	for _, v := range w {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%w: window has non-finite coefficients", ErrInvalidOption)
		}

		if v != 0 {
			nonZero = true
		}
	}

	if !nonZero {
		return fmt.Errorf("%w: window of length %d is all zero", ErrInvalidOption, len(w))
	}

	return nil
}

func normalizeKernel(re, im []float64, norm Norm) {
	var d float64

	switch norm {
	case NormL1:
		for i := range re {
			d += math.Hypot(re[i], im[i])
		}
	case NormL2:
		var sr, si float64
		for i := range re {
			sr += re[i] * re[i]
			si += im[i] * im[i]
		}

		d = math.Sqrt(sr + si)
	case NormNone:
		return
	}

	inv := 1 / d
	for i := range re {
		re[i] *= inv
		im[i] *= inv
	}
}

// Clone returns an independent Transform with the same configuration. The
// kernels and filters are shared (they are immutable); scratch buffers are
// not, so the clone can be used concurrently with the original. Clone reads
// t, so it must not run while t is processing on another goroutine.
func (t *Transform) Clone() *Transform {
	c := *t
	c.octave = [2][]float64{}
	c.padded = nil
	c.in64 = nil

	return &c
}

// Bins returns the number of frequency bins per frame.
func (t *Transform) Bins() int { return t.nBins }

// Padding returns the padding mode of the octaves.
func (t *Transform) Padding() Padding { return t.padding }

// NFFT returns the frame length of the top-octave kernels, a power of two.
func (t *Transform) NFFT() int { return t.nFFT }

// Octaves returns the number of octaves, ceil(bins / binsPerOctave).
func (t *Transform) Octaves() int { return t.nOctaves }

// DownsampleFactor returns the early downsampling factor, 1 if the input is
// not decimated before the top octave.
func (t *Transform) DownsampleFactor() int { return t.factor }

// Hop returns the hop size of the top octave after early downsampling, in
// samples at [Transform.SampleRate].
func (t *Transform) Hop() int { return t.hop }

// SampleRate returns the sample rate of the top octave after early
// downsampling.
func (t *Transform) SampleRate() float64 { return t.sampleRate }

// Frequencies returns a copy of the bin centre frequencies in Hz,
// fmin*2^(b/binsPerOctave).
func (t *Transform) Frequencies() []float64 {
	return append([]float64(nil), t.freqs...)
}

// Lengths returns a copy of the nominal kernel length of every bin,
// ceil(Q*SampleRate()/Frequencies()[b]), as nnAudio computes it for the
// librosa normalization.
func (t *Transform) Lengths() []float64 {
	return append([]float64(nil), t.lengths...)
}

// Kernels returns a copy of the top-octave kernels, one row of NFFT()
// samples per filter, lowest frequency first. Lower octaves reuse these
// kernels on the downsampled signal.
func (t *Transform) Kernels() [][]complex128 {
	out := make([][]complex128, len(t.kernels))
	data := make([]complex128, len(t.kernels)*t.nFFT)

	for f, k := range t.kernels {
		row := data[f*t.nFFT : (f+1)*t.nFFT : (f+1)*t.nFFT]
		for i := range k.re {
			row[k.start+i] = complex(k.re[i], k.im[i])
		}

		out[f] = row
	}

	return out
}

// Lowpass returns a copy of the 256-tap anti-alias filter applied before
// each 2:1 decimation between octaves.
func (t *Transform) Lowpass() []float64 {
	return append([]float64(nil), t.lowpass...)
}

// EarlyLowpass returns a copy of the 256-tap anti-alias filter of the early
// downsampling stage, or nil if no early downsampling happens.
func (t *Transform) EarlyLowpass() []float64 {
	if t.earlyLowpass == nil {
		return nil
	}

	return append([]float64(nil), t.earlyLowpass...)
}
