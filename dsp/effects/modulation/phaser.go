package modulation

import (
	"fmt"
	"math"
)

const (
	defaultPhaserRateHz      = 0.4
	defaultPhaserMinFreqHz   = 300.0
	defaultPhaserMaxFreqHz   = 1600.0
	defaultPhaserStages      = 6
	defaultPhaserFeedback    = 0.2
	defaultPhaserMix         = 0.5
	maxPhaserStages          = 12
	phaserNyquistSafetyRatio = 0.49
)

// PhaserOption mutates phaser construction parameters.
type PhaserOption func(*phaserConfig) error

type phaserConfig struct {
	rateHz    float64
	minFreqHz float64
	maxFreqHz float64
	stages    int
	feedback  float64
	mix       float64
}

func defaultPhaserConfig() phaserConfig {
	return phaserConfig{
		rateHz:    defaultPhaserRateHz,
		minFreqHz: defaultPhaserMinFreqHz,
		maxFreqHz: defaultPhaserMaxFreqHz,
		stages:    defaultPhaserStages,
		feedback:  defaultPhaserFeedback,
		mix:       defaultPhaserMix,
	}
}

// WithPhaserRateHz sets modulation speed in Hz.
func WithPhaserRateHz(rateHz float64) PhaserOption {
	return func(cfg *phaserConfig) error {
		if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
			return fmt.Errorf("phaser rate must be > 0 and finite: %f", rateHz)
		}

		cfg.rateHz = rateHz

		return nil
	}
}

// WithPhaserFrequencyRangeHz sets the modulation center frequency range in Hz.
func WithPhaserFrequencyRangeHz(minFreqHz, maxFreqHz float64) PhaserOption {
	return func(cfg *phaserConfig) error {
		if minFreqHz <= 0 || math.IsNaN(minFreqHz) || math.IsInf(minFreqHz, 0) {
			return fmt.Errorf("phaser min frequency must be > 0 and finite: %f", minFreqHz)
		}

		if maxFreqHz <= minFreqHz || math.IsNaN(maxFreqHz) || math.IsInf(maxFreqHz, 0) {
			return fmt.Errorf("phaser max frequency must be > min frequency and finite: min=%f max=%f", minFreqHz, maxFreqHz)
		}

		cfg.minFreqHz = minFreqHz
		cfg.maxFreqHz = maxFreqHz

		return nil
	}
}

// WithPhaserStages sets the number of allpass stages in [1, 12].
func WithPhaserStages(stages int) PhaserOption {
	return func(cfg *phaserConfig) error {
		if stages < 1 || stages > maxPhaserStages {
			return fmt.Errorf("phaser stages must be in [1, %d]: %d", maxPhaserStages, stages)
		}

		cfg.stages = stages

		return nil
	}
}

// WithPhaserFeedback sets feedback amount in [-0.99, 0.99].
func WithPhaserFeedback(feedback float64) PhaserOption {
	return func(cfg *phaserConfig) error {
		if feedback < -0.99 || feedback > 0.99 || math.IsNaN(feedback) || math.IsInf(feedback, 0) {
			return fmt.Errorf("phaser feedback must be in [-0.99, 0.99]: %f", feedback)
		}

		cfg.feedback = feedback

		return nil
	}
}

// WithPhaserMix sets wet amount in [0, 1].
func WithPhaserMix(mix float64) PhaserOption {
	return func(cfg *phaserConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("phaser mix must be in [0, 1]: %f", mix)
		}

		cfg.mix = mix

		return nil
	}
}

type phaserAllpassStage struct {
	x1 float64
	y1 float64
}

func (s *phaserAllpassStage) reset() {
	s.x1 = 0
	s.y1 = 0
}

func (s *phaserAllpassStage) process(x, a float64) float64 {
	y := a*x + s.x1 - a*s.y1
	s.x1 = x
	s.y1 = y

	return y
}

// Phaser is a mono allpass-cascade phaser with LFO modulation.
type Phaser struct {
	sampleRate float64
	rateHz     float64
	minFreqHz  float64
	maxFreqHz  float64
	feedback   float64
	mix        float64

	lfoPhase       float64
	feedbackSample float64

	stages []phaserAllpassStage
}

// NewPhaser creates a phaser with practical defaults and optional overrides.
func NewPhaser(sampleRate float64, opts ...PhaserOption) (*Phaser, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("phaser sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultPhaserConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	p := &Phaser{
		sampleRate: sampleRate,
		rateHz:     cfg.rateHz,
		minFreqHz:  cfg.minFreqHz,
		maxFreqHz:  cfg.maxFreqHz,
		feedback:   cfg.feedback,
		mix:        cfg.mix,
		stages:     make([]phaserAllpassStage, cfg.stages),
	}

	err := p.validateParams()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// SetSampleRate updates sample rate.
func (p *Phaser) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("phaser sample rate must be > 0 and finite: %f", sampleRate)
	}

	p.sampleRate = sampleRate

	return p.validateParams()
}

// SetRateHz sets modulation speed in Hz.
func (p *Phaser) SetRateHz(rateHz float64) error {
	if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
		return fmt.Errorf("phaser rate must be > 0 and finite: %f", rateHz)
	}

	p.rateHz = rateHz

	return nil
}

// SetFrequencyRangeHz sets the modulation center frequency range in Hz.
func (p *Phaser) SetFrequencyRangeHz(minFreqHz, maxFreqHz float64) error {
	if minFreqHz <= 0 || math.IsNaN(minFreqHz) || math.IsInf(minFreqHz, 0) {
		return fmt.Errorf("phaser min frequency must be > 0 and finite: %f", minFreqHz)
	}

	if maxFreqHz <= minFreqHz || math.IsNaN(maxFreqHz) || math.IsInf(maxFreqHz, 0) {
		return fmt.Errorf("phaser max frequency must be > min frequency and finite: min=%f max=%f", minFreqHz, maxFreqHz)
	}

	p.minFreqHz = minFreqHz
	p.maxFreqHz = maxFreqHz

	return p.validateParams()
}

// SetStages sets the number of allpass stages in [1, 12].
func (p *Phaser) SetStages(stages int) error {
	if stages < 1 || stages > maxPhaserStages {
		return fmt.Errorf("phaser stages must be in [1, %d]: %d", maxPhaserStages, stages)
	}

	if stages == len(p.stages) {
		return nil
	}

	p.stages = make([]phaserAllpassStage, stages)

	return nil
}

// SetFeedback sets feedback amount in [-0.99, 0.99].
func (p *Phaser) SetFeedback(feedback float64) error {
	if feedback < -0.99 || feedback > 0.99 || math.IsNaN(feedback) || math.IsInf(feedback, 0) {
		return fmt.Errorf("phaser feedback must be in [-0.99, 0.99]: %f", feedback)
	}

	p.feedback = feedback

	return nil
}

// SetMix sets wet amount in [0, 1].
func (p *Phaser) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("phaser mix must be in [0, 1]: %f", mix)
	}

	p.mix = mix

	return nil
}

// Reset clears allpass and modulation state.
func (p *Phaser) Reset() {
	for i := range p.stages {
		p.stages[i].reset()
	}

	p.feedbackSample = 0
	p.lfoPhase = 0
}

// Process processes one sample.
func (p *Phaser) Process(sample float64) float64 {
	x := sample + p.feedbackSample*p.feedback
	coef := phaserAllpassCoefficient(p.modulatedFrequency(), p.sampleRate)

	y := x
	for i := range p.stages {
		y = p.stages[i].process(y, coef)
	}

	p.feedbackSample = y

	p.lfoPhase += 2 * math.Pi * p.rateHz / p.sampleRate
	if p.lfoPhase >= 2*math.Pi {
		p.lfoPhase -= 2 * math.Pi
	}

	return sample*(1-p.mix) + y*p.mix
}

// ProcessSample is an alias for Process.
func (p *Phaser) ProcessSample(sample float64) float64 {
	return p.Process(sample)
}

// ProcessInPlace applies phasing to buf in place.
func (p *Phaser) ProcessInPlace(buf []float64) error {
	for i := range buf {
		buf[i] = p.Process(buf[i])
	}

	return nil
}

// SampleRate returns sample rate in Hz.
func (p *Phaser) SampleRate() float64 { return p.sampleRate }

// RateHz returns LFO speed in Hz.
func (p *Phaser) RateHz() float64 { return p.rateHz }

// MinFrequencyHz returns the modulation minimum frequency in Hz.
func (p *Phaser) MinFrequencyHz() float64 { return p.minFreqHz }

// MaxFrequencyHz returns the modulation maximum frequency in Hz.
func (p *Phaser) MaxFrequencyHz() float64 { return p.maxFreqHz }

// Stages returns number of allpass stages.
func (p *Phaser) Stages() int { return len(p.stages) }

// Feedback returns feedback amount in [-0.99, 0.99].
func (p *Phaser) Feedback() float64 { return p.feedback }

// Mix returns wet amount in [0, 1].
func (p *Phaser) Mix() float64 { return p.mix }

//nolint:cyclop
func (p *Phaser) validateParams() error {
	if p.sampleRate <= 0 || math.IsNaN(p.sampleRate) || math.IsInf(p.sampleRate, 0) {
		return fmt.Errorf("phaser sample rate must be > 0 and finite: %f", p.sampleRate)
	}

	if p.rateHz <= 0 || math.IsNaN(p.rateHz) || math.IsInf(p.rateHz, 0) {
		return fmt.Errorf("phaser rate must be > 0 and finite: %f", p.rateHz)
	}

	if p.minFreqHz <= 0 || math.IsNaN(p.minFreqHz) || math.IsInf(p.minFreqHz, 0) {
		return fmt.Errorf("phaser min frequency must be > 0 and finite: %f", p.minFreqHz)
	}

	if p.maxFreqHz <= p.minFreqHz || math.IsNaN(p.maxFreqHz) || math.IsInf(p.maxFreqHz, 0) {
		return fmt.Errorf("phaser max frequency must be > min frequency and finite: min=%f max=%f", p.minFreqHz, p.maxFreqHz)
	}

	maxAllowed := phaserNyquistSafetyRatio * p.sampleRate
	if p.maxFreqHz >= maxAllowed {
		return fmt.Errorf("phaser max frequency must be < %.2f Hz for sample rate %.2f", maxAllowed, p.sampleRate)
	}

	if len(p.stages) < 1 || len(p.stages) > maxPhaserStages {
		return fmt.Errorf("phaser stages must be in [1, %d]: %d", maxPhaserStages, len(p.stages))
	}

	if p.feedback < -0.99 || p.feedback > 0.99 || math.IsNaN(p.feedback) || math.IsInf(p.feedback, 0) {
		return fmt.Errorf("phaser feedback must be in [-0.99, 0.99]: %f", p.feedback)
	}

	if p.mix < 0 || p.mix > 1 || math.IsNaN(p.mix) || math.IsInf(p.mix, 0) {
		return fmt.Errorf("phaser mix must be in [0, 1]: %f", p.mix)
	}

	return nil
}

func (p *Phaser) modulatedFrequency() float64 {
	lfo := 0.5 * (1 + math.Sin(p.lfoPhase))
	return p.minFreqHz + (p.maxFreqHz-p.minFreqHz)*lfo
}

func phaserAllpassCoefficient(freqHz, sampleRate float64) float64 {
	maxFreq := phaserNyquistSafetyRatio * sampleRate
	if freqHz < 1 {
		freqHz = 1
	} else if freqHz > maxFreq {
		freqHz = maxFreq
	}

	g := math.Tan(math.Pi * freqHz / sampleRate)
	if math.IsInf(g, 0) || math.IsNaN(g) {
		return 0
	}

	return (1 - g) / (1 + g)
}
