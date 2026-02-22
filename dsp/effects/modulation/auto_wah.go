package modulation

import (
	"fmt"
	"math"
)

const (
	defaultAutoWahMinFreqHz   = 300.0
	defaultAutoWahMaxFreqHz   = 2200.0
	defaultAutoWahQ           = 0.8
	defaultAutoWahSensitivity = 2.0
	defaultAutoWahAttackMs    = 2.0
	defaultAutoWahReleaseMs   = 80.0
	defaultAutoWahMix         = 1.0

	autoWahNyquistSafetyRatio = 0.49
)

// AutoWahOption mutates auto-wah construction parameters.
type AutoWahOption func(*autoWahConfig) error

type autoWahConfig struct {
	minFreqHz   float64
	maxFreqHz   float64
	q           float64
	sensitivity float64
	attackMs    float64
	releaseMs   float64
	mix         float64
}

func defaultAutoWahConfig() autoWahConfig {
	return autoWahConfig{
		minFreqHz:   defaultAutoWahMinFreqHz,
		maxFreqHz:   defaultAutoWahMaxFreqHz,
		q:           defaultAutoWahQ,
		sensitivity: defaultAutoWahSensitivity,
		attackMs:    defaultAutoWahAttackMs,
		releaseMs:   defaultAutoWahReleaseMs,
		mix:         defaultAutoWahMix,
	}
}

// WithAutoWahFrequencyRangeHz sets envelope-mapped band-pass center range in Hz.
func WithAutoWahFrequencyRangeHz(minFreqHz, maxFreqHz float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if minFreqHz <= 0 || math.IsNaN(minFreqHz) || math.IsInf(minFreqHz, 0) {
			return fmt.Errorf("auto-wah min frequency must be > 0 and finite: %f", minFreqHz)
		}

		if maxFreqHz <= minFreqHz || math.IsNaN(maxFreqHz) || math.IsInf(maxFreqHz, 0) {
			return fmt.Errorf("auto-wah max frequency must be > min frequency and finite: min=%f max=%f", minFreqHz, maxFreqHz)
		}

		cfg.minFreqHz = minFreqHz
		cfg.maxFreqHz = maxFreqHz

		return nil
	}
}

// WithAutoWahQ sets filter Q (> 0).
func WithAutoWahQ(q float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if q <= 0 || math.IsNaN(q) || math.IsInf(q, 0) {
			return fmt.Errorf("auto-wah Q must be > 0 and finite: %f", q)
		}

		cfg.q = q

		return nil
	}
}

// WithAutoWahSensitivity sets envelope sensitivity (> 0).
func WithAutoWahSensitivity(sensitivity float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if sensitivity <= 0 || math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
			return fmt.Errorf("auto-wah sensitivity must be > 0 and finite: %f", sensitivity)
		}

		cfg.sensitivity = sensitivity

		return nil
	}
}

// WithAutoWahAttackMs sets attack time in milliseconds (>= 0).
func WithAutoWahAttackMs(attackMs float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if attackMs < 0 || math.IsNaN(attackMs) || math.IsInf(attackMs, 0) {
			return fmt.Errorf("auto-wah attack must be >= 0 and finite: %f", attackMs)
		}

		cfg.attackMs = attackMs

		return nil
	}
}

// WithAutoWahReleaseMs sets release time in milliseconds (>= 0).
func WithAutoWahReleaseMs(releaseMs float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if releaseMs < 0 || math.IsNaN(releaseMs) || math.IsInf(releaseMs, 0) {
			return fmt.Errorf("auto-wah release must be >= 0 and finite: %f", releaseMs)
		}

		cfg.releaseMs = releaseMs

		return nil
	}
}

// WithAutoWahMix sets wet amount in [0, 1].
func WithAutoWahMix(mix float64) AutoWahOption {
	return func(cfg *autoWahConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("auto-wah mix must be in [0, 1]: %f", mix)
		}

		cfg.mix = mix

		return nil
	}
}

// AutoWah is an envelope-following band-pass modulation effect.
type AutoWah struct {
	sampleRate float64
	minFreqHz  float64
	maxFreqHz  float64
	q          float64

	sensitivity float64
	attackMs    float64
	releaseMs   float64
	mix         float64

	envelope      float64
	currentFreqHz float64

	attackCoef  float64
	releaseCoef float64

	b0 float64
	b1 float64
	b2 float64
	a1 float64
	a2 float64

	z1 float64
	z2 float64
}

// NewAutoWah creates an auto-wah with practical defaults and optional overrides.
func NewAutoWah(sampleRate float64, opts ...AutoWahOption) (*AutoWah, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("auto-wah sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultAutoWahConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	a := &AutoWah{
		sampleRate:  sampleRate,
		minFreqHz:   cfg.minFreqHz,
		maxFreqHz:   cfg.maxFreqHz,
		q:           cfg.q,
		sensitivity: cfg.sensitivity,
		attackMs:    cfg.attackMs,
		releaseMs:   cfg.releaseMs,
		mix:         cfg.mix,
	}
	if err := a.validateParams(); err != nil {
		return nil, err
	}

	a.updateEnvelopeCoefficients()
	a.currentFreqHz = a.minFreqHz
	a.updateFilterCoefficients(a.currentFreqHz)

	return a, nil
}

// SetSampleRate updates sample rate.
func (a *AutoWah) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("auto-wah sample rate must be > 0 and finite: %f", sampleRate)
	}

	a.sampleRate = sampleRate
	if err := a.validateParams(); err != nil {
		return err
	}

	a.updateEnvelopeCoefficients()
	a.updateFilterCoefficients(a.currentFreqHz)

	return nil
}

// SetFrequencyRangeHz sets envelope-mapped center frequency range in Hz.
func (a *AutoWah) SetFrequencyRangeHz(minFreqHz, maxFreqHz float64) error {
	if minFreqHz <= 0 || math.IsNaN(minFreqHz) || math.IsInf(minFreqHz, 0) {
		return fmt.Errorf("auto-wah min frequency must be > 0 and finite: %f", minFreqHz)
	}

	if maxFreqHz <= minFreqHz || math.IsNaN(maxFreqHz) || math.IsInf(maxFreqHz, 0) {
		return fmt.Errorf("auto-wah max frequency must be > min frequency and finite: min=%f max=%f", minFreqHz, maxFreqHz)
	}

	prevMin := a.minFreqHz
	prevMax := a.maxFreqHz

	a.minFreqHz = minFreqHz
	a.maxFreqHz = maxFreqHz

	if err := a.validateParams(); err != nil {
		a.minFreqHz = prevMin
		a.maxFreqHz = prevMax
		return err
	}

	a.currentFreqHz = a.clampFrequency(a.currentFreqHz)
	a.updateFilterCoefficients(a.currentFreqHz)

	return nil
}

// SetQ sets filter Q (> 0).
func (a *AutoWah) SetQ(q float64) error {
	if q <= 0 || math.IsNaN(q) || math.IsInf(q, 0) {
		return fmt.Errorf("auto-wah Q must be > 0 and finite: %f", q)
	}

	a.q = q
	a.updateFilterCoefficients(a.currentFreqHz)

	return nil
}

// SetSensitivity sets envelope sensitivity (> 0).
func (a *AutoWah) SetSensitivity(sensitivity float64) error {
	if sensitivity <= 0 || math.IsNaN(sensitivity) || math.IsInf(sensitivity, 0) {
		return fmt.Errorf("auto-wah sensitivity must be > 0 and finite: %f", sensitivity)
	}

	a.sensitivity = sensitivity

	return nil
}

// SetAttackMs sets attack time in milliseconds (>= 0).
func (a *AutoWah) SetAttackMs(attackMs float64) error {
	if attackMs < 0 || math.IsNaN(attackMs) || math.IsInf(attackMs, 0) {
		return fmt.Errorf("auto-wah attack must be >= 0 and finite: %f", attackMs)
	}

	a.attackMs = attackMs
	a.updateEnvelopeCoefficients()

	return nil
}

// SetReleaseMs sets release time in milliseconds (>= 0).
func (a *AutoWah) SetReleaseMs(releaseMs float64) error {
	if releaseMs < 0 || math.IsNaN(releaseMs) || math.IsInf(releaseMs, 0) {
		return fmt.Errorf("auto-wah release must be >= 0 and finite: %f", releaseMs)
	}

	a.releaseMs = releaseMs
	a.updateEnvelopeCoefficients()

	return nil
}

// SetMix sets wet amount in [0, 1].
func (a *AutoWah) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("auto-wah mix must be in [0, 1]: %f", mix)
	}

	a.mix = mix

	return nil
}

// Reset clears detector and filter state.
func (a *AutoWah) Reset() {
	a.envelope = 0
	a.currentFreqHz = a.minFreqHz
	a.z1 = 0
	a.z2 = 0
	a.updateFilterCoefficients(a.currentFreqHz)
}

// Process processes one sample.
func (a *AutoWah) Process(sample float64) float64 {
	absSample := math.Abs(sample)
	if absSample > a.envelope {
		a.envelope += (absSample - a.envelope) * a.attackCoef
	} else {
		a.envelope += (absSample - a.envelope) * a.releaseCoef
	}

	envNorm := a.envelope * a.sensitivity
	if envNorm > 1 {
		envNorm = 1
	}

	a.currentFreqHz = a.minFreqHz + envNorm*(a.maxFreqHz-a.minFreqHz)
	a.updateFilterCoefficients(a.currentFreqHz)

	wet := a.processBandPass(sample)

	return sample*(1-a.mix) + wet*a.mix
}

// ProcessSample is an alias for Process.
func (a *AutoWah) ProcessSample(sample float64) float64 {
	return a.Process(sample)
}

// ProcessInPlace applies auto-wah to buf in place.
func (a *AutoWah) ProcessInPlace(buf []float64) error {
	for i := range buf {
		buf[i] = a.Process(buf[i])
	}

	return nil
}

// SampleRate returns sample rate in Hz.
func (a *AutoWah) SampleRate() float64 { return a.sampleRate }

// MinFreqHz returns the minimum center frequency in Hz.
func (a *AutoWah) MinFreqHz() float64 { return a.minFreqHz }

// MaxFreqHz returns the maximum center frequency in Hz.
func (a *AutoWah) MaxFreqHz() float64 { return a.maxFreqHz }

// Q returns the filter Q.
func (a *AutoWah) Q() float64 { return a.q }

// Sensitivity returns envelope sensitivity.
func (a *AutoWah) Sensitivity() float64 { return a.sensitivity }

// AttackMs returns attack time in milliseconds.
func (a *AutoWah) AttackMs() float64 { return a.attackMs }

// ReleaseMs returns release time in milliseconds.
func (a *AutoWah) ReleaseMs() float64 { return a.releaseMs }

// Mix returns wet amount in [0, 1].
func (a *AutoWah) Mix() float64 { return a.mix }

// CurrentCenterHz returns the instantaneous modulated center frequency in Hz.
func (a *AutoWah) CurrentCenterHz() float64 { return a.currentFreqHz }

func (a *AutoWah) validateParams() error {
	if a.sampleRate <= 0 || math.IsNaN(a.sampleRate) || math.IsInf(a.sampleRate, 0) {
		return fmt.Errorf("auto-wah sample rate must be > 0 and finite: %f", a.sampleRate)
	}

	if a.minFreqHz <= 0 || math.IsNaN(a.minFreqHz) || math.IsInf(a.minFreqHz, 0) {
		return fmt.Errorf("auto-wah min frequency must be > 0 and finite: %f", a.minFreqHz)
	}

	if a.maxFreqHz <= a.minFreqHz || math.IsNaN(a.maxFreqHz) || math.IsInf(a.maxFreqHz, 0) {
		return fmt.Errorf("auto-wah max frequency must be > min frequency and finite: min=%f max=%f", a.minFreqHz, a.maxFreqHz)
	}

	maxAllowed := a.sampleRate * autoWahNyquistSafetyRatio
	if a.maxFreqHz >= maxAllowed {
		return fmt.Errorf("auto-wah max frequency must be below %0.2f * sampleRate (%f): %f", autoWahNyquistSafetyRatio, maxAllowed, a.maxFreqHz)
	}

	if a.q <= 0 || math.IsNaN(a.q) || math.IsInf(a.q, 0) {
		return fmt.Errorf("auto-wah Q must be > 0 and finite: %f", a.q)
	}

	if a.sensitivity <= 0 || math.IsNaN(a.sensitivity) || math.IsInf(a.sensitivity, 0) {
		return fmt.Errorf("auto-wah sensitivity must be > 0 and finite: %f", a.sensitivity)
	}

	if a.attackMs < 0 || math.IsNaN(a.attackMs) || math.IsInf(a.attackMs, 0) {
		return fmt.Errorf("auto-wah attack must be >= 0 and finite: %f", a.attackMs)
	}

	if a.releaseMs < 0 || math.IsNaN(a.releaseMs) || math.IsInf(a.releaseMs, 0) {
		return fmt.Errorf("auto-wah release must be >= 0 and finite: %f", a.releaseMs)
	}

	if a.mix < 0 || a.mix > 1 || math.IsNaN(a.mix) || math.IsInf(a.mix, 0) {
		return fmt.Errorf("auto-wah mix must be in [0, 1]: %f", a.mix)
	}

	return nil
}

func (a *AutoWah) updateEnvelopeCoefficients() {
	a.attackCoef = envelopeSmoothingCoefficient(a.attackMs, a.sampleRate)
	a.releaseCoef = envelopeSmoothingCoefficient(a.releaseMs, a.sampleRate)
}

func envelopeSmoothingCoefficient(timeMs, sampleRate float64) float64 {
	if timeMs <= 0 {
		return 1
	}

	tauSeconds := timeMs / 1000
	coef := 1 - math.Exp(-1/(tauSeconds*sampleRate))
	if coef < 0 {
		return 0
	}

	if coef > 1 {
		return 1
	}

	return coef
}

func (a *AutoWah) clampFrequency(freqHz float64) float64 {
	if freqHz < a.minFreqHz {
		return a.minFreqHz
	}

	if freqHz > a.maxFreqHz {
		return a.maxFreqHz
	}

	return freqHz
}

func (a *AutoWah) updateFilterCoefficients(centerHz float64) {
	freqHz := a.clampFrequency(centerHz)
	w0 := 2 * math.Pi * freqHz / a.sampleRate
	sinW0 := math.Sin(w0)
	cosW0 := math.Cos(w0)
	alpha := sinW0 / (2 * a.q)

	b0 := alpha
	b1 := 0.0
	b2 := -alpha
	a0 := 1 + alpha
	a1 := -2 * cosW0
	a2 := 1 - alpha

	invA0 := 1 / a0
	a.b0 = b0 * invA0
	a.b1 = b1 * invA0
	a.b2 = b2 * invA0
	a.a1 = a1 * invA0
	a.a2 = a2 * invA0
}

func (a *AutoWah) processBandPass(input float64) float64 {
	output := a.b0*input + a.z1
	a.z1 = a.b1*input - a.a1*output + a.z2
	a.z2 = a.b2*input - a.a2*output
	return output
}
