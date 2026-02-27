package effects

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultHarmonicBassSampleRate = 44100.0
	defaultHarmonicBassFrequency  = 80.0
	defaultHarmonicBassRatio      = 1.0
	defaultHarmonicBassResponseMs = 20.0
	defaultHarmonicBassDecay      = 0.0

	minHarmonicBassFrequency = 10.0
	maxHarmonicBassFrequency = 500.0
)

// HighpassSelect controls the highpass topology used before harmonic generation.
type HighpassSelect int

const (
	// HighpassDC uses a fixed 2nd-order highpass at 16 Hz to remove DC/rumble.
	HighpassDC HighpassSelect = iota
	// Highpass1stOrder uses a first-order highpass at half the crossover.
	Highpass1stOrder
	// Highpass2ndOrder uses a second-order highpass at half the crossover.
	Highpass2ndOrder
)

// HarmonicBass is a psychoacoustic bass enhancer inspired by the legacy
// DSP/VST implementation. It splits the signal into low/high bands, applies
// a non-linear harmonic generator to the bass band, and mixes the results.
//
// This processor is mono, real-time safe, and not thread-safe.
type HarmonicBass struct {
	sampleRate float64
	frequency  float64
	ratio      float64
	responseMs float64
	decay      float64

	inputLevel        float64
	highFrequencyGain float64
	originalBassGain  float64
	harmonicBassGain  float64

	highpassSelect HighpassSelect

	crossoverLP *biquad.Chain
	crossoverHP *biquad.Chain
	highpass    *biquad.Chain
	limiter     *dynamics.Limiter
}

// NewHarmonicBass creates a harmonic bass enhancer with tuned defaults.
func NewHarmonicBass(sampleRate float64) (*HarmonicBass, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("harmonic bass sample rate must be positive and finite: %f", sampleRate)
	}

	l, err := dynamics.NewLimiter(sampleRate)
	if err != nil {
		return nil, err
	}

	b := &HarmonicBass{
		sampleRate:        sampleRate,
		frequency:         defaultHarmonicBassFrequency,
		ratio:             defaultHarmonicBassRatio,
		responseMs:        defaultHarmonicBassResponseMs,
		decay:             defaultHarmonicBassDecay,
		inputLevel:        1.0,
		highFrequencyGain: 1.0,
		originalBassGain:  1.0,
		harmonicBassGain:  0.0,
		highpassSelect:    HighpassDC,
		limiter:           l,
	}
	if err := b.rebuildFilters(); err != nil {
		return nil, err
	}

	if err := b.applyResponse(); err != nil {
		return nil, err
	}

	return b, nil
}

// SampleRate returns the current sample rate in Hz.
func (b *HarmonicBass) SampleRate() float64 { return b.sampleRate }

// Frequency returns the crossover frequency in Hz.
func (b *HarmonicBass) Frequency() float64 { return b.frequency }

// Ratio returns the drive ratio used for harmonic generation.
func (b *HarmonicBass) Ratio() float64 { return b.ratio }

// Response returns the response time in milliseconds.
func (b *HarmonicBass) Response() float64 { return b.responseMs }

// Decay returns the decay parameter that shapes the nonlinearity.
func (b *HarmonicBass) Decay() float64 { return b.decay }

// InputLevel returns the input gain applied before the crossover.
func (b *HarmonicBass) InputLevel() float64 { return b.inputLevel }

// HighFrequencyLevel returns the high-band gain.
func (b *HarmonicBass) HighFrequencyLevel() float64 { return b.highFrequencyGain }

// OriginalBassLevel returns the original (clean) bass gain.
func (b *HarmonicBass) OriginalBassLevel() float64 { return b.originalBassGain }

// HarmonicBassLevel returns the synthesized harmonic bass gain.
func (b *HarmonicBass) HarmonicBassLevel() float64 { return b.harmonicBassGain }

// HighpassMode returns the current highpass mode.
func (b *HarmonicBass) HighpassMode() HighpassSelect { return b.highpassSelect }

// SetSampleRate updates the sample rate and recomputes internal filters.
func (b *HarmonicBass) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("harmonic bass sample rate must be positive and finite: %f", sampleRate)
	}

	b.sampleRate = sampleRate

	err := b.rebuildFilters()
	if err != nil {
		return err
	}

	return b.applyResponse()
}

// SetFrequency sets the crossover frequency in Hz.
func (b *HarmonicBass) SetFrequency(freq float64) error {
	if freq < minHarmonicBassFrequency || freq > maxHarmonicBassFrequency ||
		math.IsNaN(freq) || math.IsInf(freq, 0) {
		return fmt.Errorf("harmonic bass frequency must be in [%f, %f]: %f",
			minHarmonicBassFrequency, maxHarmonicBassFrequency, freq)
	}

	b.frequency = freq

	return b.rebuildFilters()
}

// SetRatio sets the drive ratio for harmonic generation.
func (b *HarmonicBass) SetRatio(ratio float64) error {
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return fmt.Errorf("harmonic bass ratio must be positive and finite: %f", ratio)
	}

	b.ratio = ratio

	return nil
}

// SetResponse sets the attack/release response time in milliseconds.
func (b *HarmonicBass) SetResponse(ms float64) error {
	if ms <= 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("harmonic bass response must be positive and finite: %f", ms)
	}

	b.responseMs = ms

	return b.applyResponse()
}

// SetDecay updates the decay parameter used by the harmonic shaper.
func (b *HarmonicBass) SetDecay(decay float64) error {
	if math.IsNaN(decay) || math.IsInf(decay, 0) {
		return fmt.Errorf("harmonic bass decay must be finite: %f", decay)
	}

	b.decay = decay

	return nil
}

// SetInputLevel sets the input gain applied before the crossover.
func (b *HarmonicBass) SetInputLevel(gain float64) error {
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("harmonic bass input gain must be finite: %f", gain)
	}

	b.inputLevel = gain

	return nil
}

// SetHighFrequencyLevel sets the gain for the high band.
func (b *HarmonicBass) SetHighFrequencyLevel(gain float64) error {
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("harmonic bass high frequency gain must be finite: %f", gain)
	}

	b.highFrequencyGain = gain

	return nil
}

// SetOriginalBassLevel sets the gain for the original bass band.
func (b *HarmonicBass) SetOriginalBassLevel(gain float64) error {
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("harmonic bass original bass gain must be finite: %f", gain)
	}

	b.originalBassGain = gain

	return nil
}

// SetHarmonicBassLevel sets the gain for the synthesized harmonic bass band.
func (b *HarmonicBass) SetHarmonicBassLevel(gain float64) error {
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("harmonic bass harmonic gain must be finite: %f", gain)
	}

	b.harmonicBassGain = gain

	return nil
}

// SetHighpassMode changes the highpass mode used before the nonlinearity.
func (b *HarmonicBass) SetHighpassMode(mode HighpassSelect) error {
	b.highpassSelect = mode
	return b.rebuildFilters()
}

// Reset clears internal filter and limiter state.
func (b *HarmonicBass) Reset() {
	if b.crossoverLP != nil {
		b.crossoverLP.Reset()
	}

	if b.crossoverHP != nil {
		b.crossoverHP.Reset()
	}

	if b.highpass != nil {
		b.highpass.Reset()
	}

	if b.limiter != nil {
		b.limiter.Reset()
	}
}

// ProcessSample processes a single sample.
func (b *HarmonicBass) ProcessSample(input float64) float64 {
	x := input * b.inputLevel
	low := b.crossoverLP.ProcessSample(x)
	high := b.crossoverHP.ProcessSample(x)

	shaped := b.decay + low*(1+low*-2*b.decay)
	shaped = b.highpass.ProcessSample(shaped)
	shaped = 4 * shaped
	shaped = b.limiter.ProcessSample(shaped)
	shaped = 0.5 * shaped
	shaped = clampUnit(shaped)

	return b.originalBassGain*low + b.harmonicBassGain*shaped + b.highFrequencyGain*high
}

// ProcessInPlace applies the effect to a buffer in place.
func (b *HarmonicBass) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = b.ProcessSample(buf[i])
	}
}

func (b *HarmonicBass) applyResponse() error {
	if b.limiter == nil {
		return nil
	}

	err := b.limiter.SetRelease(b.responseMs)
	if err != nil {
		return err
	}

	return b.limiter.SetThreshold(0)
}

func (b *HarmonicBass) rebuildFilters() error {
	if b.sampleRate <= 0 || math.IsNaN(b.sampleRate) || math.IsInf(b.sampleRate, 0) {
		return fmt.Errorf("harmonic bass sample rate must be positive and finite: %f", b.sampleRate)
	}

	freq := b.frequency
	if freq < minHarmonicBassFrequency {
		freq = minHarmonicBassFrequency
	}

	lp := design.ButterworthLP(freq, 3, b.sampleRate)

	hp := design.ButterworthHP(freq, 3, b.sampleRate)
	if len(lp) == 0 || len(hp) == 0 {
		return fmt.Errorf("harmonic bass crossover design failed for freq=%f sr=%f", freq, b.sampleRate)
	}

	b.crossoverLP = biquad.NewChain(lp)
	b.crossoverHP = biquad.NewChain(hp)

	var (
		hpFreq  float64
		hpOrder int
	)

	switch b.highpassSelect {
	case HighpassDC:
		hpFreq = 16.0
		hpOrder = 2
	case Highpass1stOrder:
		hpFreq = 0.5 * b.frequency
		hpOrder = 1
	case Highpass2ndOrder:
		hpFreq = 0.5 * b.frequency
		hpOrder = 2
	default:
		return fmt.Errorf("harmonic bass highpass mode invalid: %d", b.highpassSelect)
	}

	if hpFreq <= 0 {
		hpFreq = 16.0
	}

	hpCoeffs := design.ButterworthHP(hpFreq, hpOrder, b.sampleRate)
	if len(hpCoeffs) == 0 {
		return fmt.Errorf("harmonic bass highpass design failed for freq=%f sr=%f", hpFreq, b.sampleRate)
	}

	b.highpass = biquad.NewChain(hpCoeffs)

	if b.limiter != nil {
		return b.limiter.SetSampleRate(b.sampleRate)
	}

	return nil
}

func clampUnit(x float64) float64 {
	return core.Clamp(x, -1, 1)
}

// NewDefaultHarmonicBass is a convenience constructor at 44.1 kHz.
func NewDefaultHarmonicBass() (*HarmonicBass, error) {
	return NewHarmonicBass(defaultHarmonicBassSampleRate)
}
