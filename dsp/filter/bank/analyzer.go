package bank

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
	"github.com/cwbudde/algo-dsp/dsp/resample"
)

const (
	defaultAnalyzerOrder         = 10
	defaultAnalyzerEnvelopeHz    = 100.0
	defaultAnalyzerEnvelopeOrder = 4
	defaultAnalyzerMaxDownsample = 64
)

// Analyzer estimates per-band envelope levels using an octave or fractional-octave
// filter bank with Butterworth bandpass sections.
type Analyzer struct {
	bands      []analyzerBand
	peaks      []float64
	sampleRate float64
	fraction   int
}

type analyzerBand struct {
	spec       bandSpec
	lp         *biquad.Chain
	hp         *biquad.Chain
	env        *biquad.Chain
	resampler  *resample.Resampler
	downsample int
	downPow    int
	sampleRate float64
	scratch    []float64
}

type analyzerConfig struct {
	order         int
	envelopeHz    float64
	envelopeOrder int
	lowerHz       float64
	upperHz       float64
	resample      bool
	resampleQual  resample.Quality
	maxDownsample int
}

func defaultAnalyzerConfig() analyzerConfig {
	return analyzerConfig{
		order:         defaultAnalyzerOrder,
		envelopeHz:    defaultAnalyzerEnvelopeHz,
		envelopeOrder: defaultAnalyzerEnvelopeOrder,
		lowerHz:       defaultLowerFreq,
		upperHz:       defaultUpperFreq,
		resample:      true,
		resampleQual:  resample.QualityBalanced,
		maxDownsample: defaultAnalyzerMaxDownsample,
	}
}

// AnalyzerOption configures a fractional-octave analyzer.
type AnalyzerOption func(*analyzerConfig)

// WithAnalyzerOrder sets the Butterworth filter order per band.
// Must be a positive even integer; defaults to 10.
func WithAnalyzerOrder(n int) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		if n > 0 && n%2 == 0 {
			cfg.order = n
		}
	}
}

// WithAnalyzerFrequencyRange sets custom lower and upper frequency limits
// for the analyzer. Bands outside this range are excluded.
func WithAnalyzerFrequencyRange(lower, upper float64) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		if lower > 0 && upper > lower {
			cfg.lowerHz = lower
			cfg.upperHz = upper
		}
	}
}

// WithAnalyzerEnvelopeHz sets the envelope follower cutoff frequency in Hz.
func WithAnalyzerEnvelopeHz(freqHz float64) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		if freqHz > 0 {
			cfg.envelopeHz = freqHz
		}
	}
}

// WithAnalyzerEnvelopeOrder sets the Butterworth order for the envelope smoother.
// Must be a positive even integer; defaults to 4.
func WithAnalyzerEnvelopeOrder(n int) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		if n > 0 && n%2 == 0 {
			cfg.envelopeOrder = n
		}
	}
}

// WithAnalyzerResampleQuality sets the quality used for polyphase resampling.
func WithAnalyzerResampleQuality(q resample.Quality) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		cfg.resampleQual = q
		cfg.resample = true
	}
}

// WithAnalyzerMaxDownsample caps the maximum downsample factor (power of two).
func WithAnalyzerMaxDownsample(maxFactor int) AnalyzerOption {
	return func(cfg *analyzerConfig) {
		if maxFactor > 0 {
			cfg.maxDownsample = maxFactor
		}
	}
}

// WithoutAnalyzerResampling disables per-band downsampling.
func WithoutAnalyzerResampling() AnalyzerOption {
	return func(cfg *analyzerConfig) {
		cfg.resample = false
	}
}

// NewOctaveAnalyzer builds an octave or fractional-octave analyzer.
// The fraction parameter controls the bandwidth: fraction=1 gives full octave
// bands, fraction=3 gives 1/3-octave bands, etc.
func NewOctaveAnalyzer(fraction int, sampleRate float64, opts ...AnalyzerOption) (*Analyzer, error) {
	if fraction <= 0 {
		fraction = 1
	}

	if sampleRate <= 0 || math.IsNaN(sampleRate) {
		return nil, fmt.Errorf("bank: invalid sample rate %.3f", sampleRate)
	}

	cfg := defaultAnalyzerConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	if cfg.order <= 0 || cfg.order%2 != 0 {
		return nil, fmt.Errorf("bank: analyzer order must be positive even, got %d", cfg.order)
	}

	if cfg.envelopeOrder <= 0 || cfg.envelopeOrder%2 != 0 {
		return nil, fmt.Errorf("bank: envelope order must be positive even, got %d", cfg.envelopeOrder)
	}

	specs := octaveBandSpecs(fraction, sampleRate, cfg.lowerHz, cfg.upperHz)
	if len(specs) == 0 {
		return nil, fmt.Errorf("bank: no bands in frequency range %.2f-%.2f Hz", cfg.lowerHz, cfg.upperHz)
	}

	an := &Analyzer{
		bands:      make([]analyzerBand, 0, len(specs)),
		peaks:      make([]float64, len(specs)),
		sampleRate: sampleRate,
		fraction:   fraction,
	}

	for i, spec := range specs {
		downsample := 1

		downPow := 0
		if cfg.resample {
			downsample, downPow = chooseDownsample(sampleRate, spec.high, cfg.maxDownsample)
		}

		bandRate := sampleRate / float64(downsample)

		lp := biquad.NewChain(design.ButterworthLP(spec.high, cfg.order, bandRate))
		hp := biquad.NewChain(design.ButterworthHP(spec.low, cfg.order, bandRate))

		envRate := sampleRate
		if downPow > 0 {
			// Match legacy behavior: envelope LP used SampleRate/DownsampleAmount,
			// where DownsampleAmount is the exponent (not the factor).
			envRate = sampleRate / float64(downPow)
		}

		envHz := clampEnvelopeHz(cfg.envelopeHz, envRate)
		env := biquad.NewChain(design.ButterworthLP(envHz, cfg.envelopeOrder, envRate))

		var rs *resample.Resampler

		if downsample > 1 && cfg.resample {
			var err error

			rs, err = resample.NewRational(1, downsample, resample.WithQuality(cfg.resampleQual))
			if err != nil {
				return nil, fmt.Errorf("bank: resampler init (down=%d): %w", downsample, err)
			}
		}

		an.bands = append(an.bands, analyzerBand{
			spec:       spec,
			lp:         lp,
			hp:         hp,
			env:        env,
			resampler:  rs,
			downsample: downsample,
			downPow:    downPow,
			sampleRate: bandRate,
		})
		an.peaks[i] = 0
	}

	return an, nil
}

// BandInfo describes the configured analyzer bands.
type BandInfo struct {
	CenterHz   float64
	LowHz      float64
	HighHz     float64
	SampleRate float64
	Downsample int
}

// Bands returns metadata for each analyzer band.
func (a *Analyzer) Bands() []BandInfo {
	if a == nil {
		return nil
	}

	out := make([]BandInfo, len(a.bands))
	for i := range a.bands {
		b := a.bands[i]
		out[i] = BandInfo{
			CenterHz:   b.spec.center,
			LowHz:      b.spec.low,
			HighHz:     b.spec.high,
			SampleRate: b.sampleRate,
			Downsample: b.downsample,
		}
	}

	return out
}

// Peaks returns the current per-band envelope values (linear).
// The returned slice is owned by the analyzer and is updated on each call
// to ProcessBlock.
func (a *Analyzer) Peaks() []float64 {
	if a == nil {
		return nil
	}

	return a.peaks
}

// SampleRate returns the input sample rate for the analyzer.
func (a *Analyzer) SampleRate() float64 {
	if a == nil {
		return 0
	}

	return a.sampleRate
}

// Fraction returns the configured fractional-octave bandwidth (1, 3, 6, ...).
func (a *Analyzer) Fraction() int {
	if a == nil {
		return 0
	}

	return a.fraction
}

// Reset clears all filter and resampler states.
func (a *Analyzer) Reset() {
	if a == nil {
		return
	}

	for i := range a.bands {
		b := &a.bands[i]
		b.lp.Reset()
		b.hp.Reset()
		b.env.Reset()

		if b.resampler != nil {
			b.resampler.Reset()
		}
	}

	for i := range a.peaks {
		a.peaks[i] = 0
	}
}

// ProcessBlock runs the analyzer on an input block and returns per-band
// envelope values (linear). The returned slice is owned by the analyzer.
func (a *Analyzer) ProcessBlock(input []float64) []float64 {
	if a == nil {
		return nil
	}

	if len(input) == 0 {
		return a.peaks
	}

	for i := range a.bands {
		b := &a.bands[i]

		var data []float64
		if b.resampler != nil {
			data = b.resampler.Process(input)
		} else {
			if cap(b.scratch) < len(input) {
				b.scratch = make([]float64, len(input))
			} else {
				b.scratch = b.scratch[:len(input)]
			}

			copy(b.scratch, input)
			data = b.scratch
		}

		b.lp.ProcessBlock(data)
		b.hp.ProcessBlock(data)

		var last float64

		for _, v := range data {
			x := math.Abs(v)
			last = b.env.ProcessSample(x)
		}

		a.peaks[i] = last
	}

	return a.peaks
}

func chooseDownsample(sampleRate, highHz float64, maxDownsample int) (int, int) {
	if maxDownsample < 1 || highHz <= 0 || sampleRate <= 0 {
		return 1, 0
	}

	ds := 1
	pow := 0

	limit := sampleRate / 8
	for ds*2 <= maxDownsample && float64(ds)*highHz < limit {
		ds *= 2
		pow++
	}

	return ds, pow
}

func clampEnvelopeHz(freqHz, sampleRate float64) float64 {
	if sampleRate <= 0 {
		return 1
	}

	nyquist := sampleRate / 2
	if freqHz <= 0 {
		return math.Min(1, nyquist*0.1)
	}

	maxHz := nyquist * 0.45
	if freqHz > maxHz {
		return maxHz
	}

	return freqHz
}
