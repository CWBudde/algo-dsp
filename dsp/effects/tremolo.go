package effects

import (
	"fmt"
	"math"
)

const (
	defaultTremoloRateHz      = 4.0
	defaultTremoloDepth       = 0.6
	defaultTremoloSmoothingMs = 5.0
	defaultTremoloMix         = 1.0
)

// TremoloOption mutates tremolo construction parameters.
type TremoloOption func(*tremoloConfig) error

type tremoloConfig struct {
	rateHz      float64
	depth       float64
	smoothingMs float64
	mix         float64
}

func defaultTremoloConfig() tremoloConfig {
	return tremoloConfig{
		rateHz:      defaultTremoloRateHz,
		depth:       defaultTremoloDepth,
		smoothingMs: defaultTremoloSmoothingMs,
		mix:         defaultTremoloMix,
	}
}

// WithTremoloRateHz sets modulation speed in Hz.
func WithTremoloRateHz(rateHz float64) TremoloOption {
	return func(cfg *tremoloConfig) error {
		if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
			return fmt.Errorf("tremolo rate must be > 0 and finite: %f", rateHz)
		}
		cfg.rateHz = rateHz
		return nil
	}
}

// WithTremoloDepth sets modulation depth in [0, 1].
func WithTremoloDepth(depth float64) TremoloOption {
	return func(cfg *tremoloConfig) error {
		if depth < 0 || depth > 1 || math.IsNaN(depth) || math.IsInf(depth, 0) {
			return fmt.Errorf("tremolo depth must be in [0, 1]: %f", depth)
		}
		cfg.depth = depth
		return nil
	}
}

// WithTremoloSmoothingMs sets smoothing time in milliseconds.
func WithTremoloSmoothingMs(smoothingMs float64) TremoloOption {
	return func(cfg *tremoloConfig) error {
		if smoothingMs < 0 || math.IsNaN(smoothingMs) || math.IsInf(smoothingMs, 0) {
			return fmt.Errorf("tremolo smoothing must be >= 0 and finite: %f", smoothingMs)
		}
		cfg.smoothingMs = smoothingMs
		return nil
	}
}

// WithTremoloMix sets wet amount in [0, 1].
func WithTremoloMix(mix float64) TremoloOption {
	return func(cfg *tremoloConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("tremolo mix must be in [0, 1]: %f", mix)
		}
		cfg.mix = mix
		return nil
	}
}

// Tremolo applies LFO amplitude modulation with optional smoothing.
type Tremolo struct {
	sampleRate float64
	rateHz     float64
	depth      float64
	smoothing  float64
	mix        float64

	lfoPhase      float64
	currentMod    float64
	smoothingCoef float64
}

// NewTremolo creates a tremolo with practical defaults and optional overrides.
func NewTremolo(sampleRate float64, opts ...TremoloOption) (*Tremolo, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("tremolo sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultTremoloConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	t := &Tremolo{
		sampleRate: sampleRate,
		rateHz:     cfg.rateHz,
		depth:      cfg.depth,
		smoothing:  cfg.smoothingMs,
		mix:        cfg.mix,
		currentMod: 1,
	}
	if err := t.validateParams(); err != nil {
		return nil, err
	}
	t.updateSmoothingCoefficient()
	return t, nil
}

// SetSampleRate updates sample rate.
func (t *Tremolo) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("tremolo sample rate must be > 0 and finite: %f", sampleRate)
	}
	t.sampleRate = sampleRate
	t.updateSmoothingCoefficient()
	return nil
}

// SetRateHz sets modulation speed in Hz.
func (t *Tremolo) SetRateHz(rateHz float64) error {
	if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
		return fmt.Errorf("tremolo rate must be > 0 and finite: %f", rateHz)
	}
	t.rateHz = rateHz
	return nil
}

// SetDepth sets modulation depth in [0, 1].
func (t *Tremolo) SetDepth(depth float64) error {
	if depth < 0 || depth > 1 || math.IsNaN(depth) || math.IsInf(depth, 0) {
		return fmt.Errorf("tremolo depth must be in [0, 1]: %f", depth)
	}
	t.depth = depth
	return nil
}

// SetSmoothingMs sets smoothing time in milliseconds.
func (t *Tremolo) SetSmoothingMs(smoothingMs float64) error {
	if smoothingMs < 0 || math.IsNaN(smoothingMs) || math.IsInf(smoothingMs, 0) {
		return fmt.Errorf("tremolo smoothing must be >= 0 and finite: %f", smoothingMs)
	}
	t.smoothing = smoothingMs
	t.updateSmoothingCoefficient()
	return nil
}

// SetMix sets wet amount in [0, 1].
func (t *Tremolo) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("tremolo mix must be in [0, 1]: %f", mix)
	}
	t.mix = mix
	return nil
}

// Reset clears modulation phase and smoothing state.
func (t *Tremolo) Reset() {
	t.lfoPhase = 0
	t.currentMod = 1
}

// Process processes one sample.
func (t *Tremolo) Process(sample float64) float64 {
	targetMod := t.targetModulation()
	if t.smoothingCoef >= 1 {
		t.currentMod = targetMod
	} else {
		t.currentMod += (targetMod - t.currentMod) * t.smoothingCoef
	}
	wet := sample * t.currentMod

	t.lfoPhase += 2 * math.Pi * t.rateHz / t.sampleRate
	if t.lfoPhase >= 2*math.Pi {
		t.lfoPhase -= 2 * math.Pi
	}

	return sample*(1-t.mix) + wet*t.mix
}

// ProcessSample is an alias for Process.
func (t *Tremolo) ProcessSample(sample float64) float64 {
	return t.Process(sample)
}

// ProcessInPlace applies tremolo to buf in place.
func (t *Tremolo) ProcessInPlace(buf []float64) error {
	for i := range buf {
		buf[i] = t.Process(buf[i])
	}
	return nil
}

// SampleRate returns sample rate in Hz.
func (t *Tremolo) SampleRate() float64 { return t.sampleRate }

// RateHz returns LFO speed in Hz.
func (t *Tremolo) RateHz() float64 { return t.rateHz }

// Depth returns modulation depth in [0, 1].
func (t *Tremolo) Depth() float64 { return t.depth }

// SmoothingMs returns smoothing time in milliseconds.
func (t *Tremolo) SmoothingMs() float64 { return t.smoothing }

// Mix returns wet amount in [0, 1].
func (t *Tremolo) Mix() float64 { return t.mix }

func (t *Tremolo) validateParams() error {
	if t.sampleRate <= 0 || math.IsNaN(t.sampleRate) || math.IsInf(t.sampleRate, 0) {
		return fmt.Errorf("tremolo sample rate must be > 0 and finite: %f", t.sampleRate)
	}
	if t.rateHz <= 0 || math.IsNaN(t.rateHz) || math.IsInf(t.rateHz, 0) {
		return fmt.Errorf("tremolo rate must be > 0 and finite: %f", t.rateHz)
	}
	if t.depth < 0 || t.depth > 1 || math.IsNaN(t.depth) || math.IsInf(t.depth, 0) {
		return fmt.Errorf("tremolo depth must be in [0, 1]: %f", t.depth)
	}
	if t.smoothing < 0 || math.IsNaN(t.smoothing) || math.IsInf(t.smoothing, 0) {
		return fmt.Errorf("tremolo smoothing must be >= 0 and finite: %f", t.smoothing)
	}
	if t.mix < 0 || t.mix > 1 || math.IsNaN(t.mix) || math.IsInf(t.mix, 0) {
		return fmt.Errorf("tremolo mix must be in [0, 1]: %f", t.mix)
	}
	return nil
}

func (t *Tremolo) updateSmoothingCoefficient() {
	if t.smoothing <= 0 {
		t.smoothingCoef = 1
		return
	}
	tauSeconds := t.smoothing / 1000
	t.smoothingCoef = 1 - math.Exp(-1/(tauSeconds*t.sampleRate))
	if t.smoothingCoef < 0 {
		t.smoothingCoef = 0
	}
	if t.smoothingCoef > 1 {
		t.smoothingCoef = 1
	}
}

func (t *Tremolo) targetModulation() float64 {
	lfo := 0.5 * (1 + math.Sin(t.lfoPhase))
	return (1 - t.depth) + t.depth*lfo
}
