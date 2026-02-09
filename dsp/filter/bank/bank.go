package bank

import (
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

// octaveRatio is G = 10^(3/10) per IEC 61260.
var octaveRatio = math.Pow(10, 0.3)

const (
	defaultOrder     = 4
	defaultLowerFreq = 20.0
	defaultUpperFreq = 20000.0
)

// Band represents one frequency band in a filter bank.
type Band struct {
	CenterFreq float64       // center frequency in Hz
	LowCutoff  float64       // lower -3 dB frequency in Hz
	HighCutoff float64       // upper -3 dB frequency in Hz
	LP         *biquad.Chain // lowpass filter at HighCutoff
	HP         *biquad.Chain // highpass filter at LowCutoff
}

// MagnitudeDB returns the combined bandpass magnitude response in dB
// at the given frequency.
func (b *Band) MagnitudeDB(freqHz, sampleRate float64) float64 {
	return b.LP.MagnitudeDB(freqHz, sampleRate) + b.HP.MagnitudeDB(freqHz, sampleRate)
}

// Bank is a collection of frequency bands with matched filter pairs.
type Bank struct {
	bands      []Band
	sampleRate float64
	order      int
}

type bankConfig struct {
	order   int
	lowerHz float64
	upperHz float64
}

func defaultBankConfig() bankConfig {
	return bankConfig{
		order:   defaultOrder,
		lowerHz: defaultLowerFreq,
		upperHz: defaultUpperFreq,
	}
}

// Option configures a Bank.
type Option func(*bankConfig)

// WithOrder sets the Butterworth filter order per LP/HP pair.
// Must be a positive even integer; defaults to 4.
func WithOrder(n int) Option {
	return func(cfg *bankConfig) {
		if n > 0 && n%2 == 0 {
			cfg.order = n
		}
	}
}

// WithFrequencyRange sets custom lower and upper frequency limits
// for the filter bank. Bands outside this range are excluded.
func WithFrequencyRange(lower, upper float64) Option {
	return func(cfg *bankConfig) {
		if lower > 0 && upper > lower {
			cfg.lowerHz = lower
			cfg.upperHz = upper
		}
	}
}

// Octave builds an octave or fractional-octave filter bank.
//
// The fraction parameter controls the bandwidth: fraction=1 gives full octave
// bands, fraction=3 gives 1/3-octave bands, etc. Center frequencies follow
// the IEC 61260 base-10 system: f_m = 1000 * G^(k/N) where G = 10^(3/10).
//
// Band edges are:
//
//	f_upper = f_center * G^(1/(2*N))
//	f_lower = f_center * G^(-1/(2*N))
func Octave(fraction int, sampleRate float64, opts ...Option) *Bank {
	if fraction <= 0 {
		fraction = 1
	}
	cfg := defaultBankConfig()
	for _, o := range opts {
		o(&cfg)
	}
	specs := octaveBandSpecs(fraction, sampleRate, cfg.lowerHz, cfg.upperHz)
	bands := make([]Band, 0, len(specs))
	for _, spec := range specs {
		lp := biquad.NewChain(design.ButterworthLP(spec.high, cfg.order, sampleRate))
		hp := biquad.NewChain(design.ButterworthHP(spec.low, cfg.order, sampleRate))
		bands = append(bands, Band{
			CenterFreq: spec.center,
			LowCutoff:  spec.low,
			HighCutoff: spec.high,
			LP:         lp,
			HP:         hp,
		})
	}

	sort.Slice(bands, func(i, j int) bool {
		return bands[i].CenterFreq < bands[j].CenterFreq
	})

	return &Bank{
		bands:      bands,
		sampleRate: sampleRate,
		order:      cfg.order,
	}
}

// Custom builds a filter bank from arbitrary center frequencies and a
// specified bandwidth in octaves.
func Custom(centers []float64, bandwidth float64, sampleRate float64, opts ...Option) *Bank {
	if bandwidth <= 0 {
		bandwidth = 1
	}
	cfg := defaultBankConfig()
	for _, o := range opts {
		o(&cfg)
	}

	halfBW := math.Pow(2, bandwidth/2)
	nyquist := sampleRate / 2

	var bands []Band
	for _, fc := range centers {
		fLo := fc / halfBW
		fHi := fc * halfBW
		if fHi >= nyquist || fLo <= 0 || fc <= 0 {
			continue
		}
		lp := biquad.NewChain(design.ButterworthLP(fHi, cfg.order, sampleRate))
		hp := biquad.NewChain(design.ButterworthHP(fLo, cfg.order, sampleRate))
		bands = append(bands, Band{
			CenterFreq: fc,
			LowCutoff:  fLo,
			HighCutoff: fHi,
			LP:         lp,
			HP:         hp,
		})
	}

	sort.Slice(bands, func(i, j int) bool {
		return bands[i].CenterFreq < bands[j].CenterFreq
	})

	return &Bank{
		bands:      bands,
		sampleRate: sampleRate,
		order:      cfg.order,
	}
}

// Bands returns all bands in the bank, ordered low to high frequency.
func (b *Bank) Bands() []Band { return b.bands }

// NumBands returns the number of bands.
func (b *Bank) NumBands() int { return len(b.bands) }

// SampleRate returns the sample rate the bank was built for.
func (b *Bank) SampleRate() float64 { return b.sampleRate }

// Order returns the Butterworth filter order used per LP/HP pair.
func (b *Bank) Order() int { return b.order }

// ProcessSample processes one input sample through all bands in parallel,
// returning per-band output values.
func (b *Bank) ProcessSample(x float64) []float64 {
	out := make([]float64, len(b.bands))
	for i := range b.bands {
		lp := b.bands[i].LP.ProcessSample(x)
		out[i] = b.bands[i].HP.ProcessSample(lp)
	}
	return out
}

// ProcessBlock processes a block of input samples through all bands.
// Returns a slice of per-band output blocks: result[band][sample].
func (b *Bank) ProcessBlock(input []float64) [][]float64 {
	n := len(input)
	result := make([][]float64, len(b.bands))
	for i := range b.bands {
		buf := make([]float64, n)
		copy(buf, input)
		b.bands[i].LP.ProcessBlock(buf)
		b.bands[i].HP.ProcessBlock(buf)
		result[i] = buf
	}
	return result
}

// Reset clears all filter states across all bands.
func (b *Bank) Reset() {
	for i := range b.bands {
		b.bands[i].LP.Reset()
		b.bands[i].HP.Reset()
	}
}

type bandSpec struct {
	center float64
	low    float64
	high   float64
}

func octaveBandSpecs(fraction int, sampleRate, lowerHz, upperHz float64) []bandSpec {
	if fraction <= 0 || sampleRate <= 0 || lowerHz <= 0 || upperHz <= lowerHz {
		return nil
	}

	n := float64(fraction)
	halfBW := math.Pow(octaveRatio, 1/(2*n))
	nyquist := sampleRate / 2

	// Determine the range of band indices k such that
	// 1000 * G^(k/N) falls within [lowerHz, upperHz].
	kMin := int(math.Ceil(n * math.Log(lowerHz/1000) / math.Log(octaveRatio)))
	kMax := int(math.Floor(n * math.Log(upperHz/1000) / math.Log(octaveRatio)))
	if kMax < kMin {
		return nil
	}

	specs := make([]bandSpec, 0, kMax-kMin+1)
	for k := kMin; k <= kMax; k++ {
		fc := 1000 * math.Pow(octaveRatio, float64(k)/n)
		fLo := fc / halfBW
		fHi := fc * halfBW

		// Skip bands whose edges exceed Nyquist.
		if fHi >= nyquist || fLo <= 0 {
			continue
		}
		specs = append(specs, bandSpec{center: fc, low: fLo, high: fHi})
	}
	return specs
}
