package modulation

import (
	"fmt"
	"math"
)

const (
	defaultRingModCarrierHz = 440.0
	defaultRingModMix       = 1.0
)

// RingModulatorOption mutates ring modulator construction parameters.
type RingModulatorOption func(*ringModConfig) error

type ringModConfig struct {
	carrierHz float64
	mix       float64
}

func defaultRingModConfig() ringModConfig {
	return ringModConfig{
		carrierHz: defaultRingModCarrierHz,
		mix:       defaultRingModMix,
	}
}

// WithRingModCarrierHz sets the carrier oscillator frequency in Hz.
func WithRingModCarrierHz(carrierHz float64) RingModulatorOption {
	return func(cfg *ringModConfig) error {
		if carrierHz <= 0 || math.IsNaN(carrierHz) || math.IsInf(carrierHz, 0) {
			return fmt.Errorf("ring modulator carrier frequency must be > 0 and finite: %f", carrierHz)
		}

		cfg.carrierHz = carrierHz

		return nil
	}
}

// WithRingModMix sets the dry/wet mix in [0, 1], where 0 is fully dry and 1 is fully wet.
func WithRingModMix(mix float64) RingModulatorOption {
	return func(cfg *ringModConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("ring modulator mix must be in [0, 1]: %f", mix)
		}

		cfg.mix = mix

		return nil
	}
}

// RingModulator multiplies the input signal by a sine-wave carrier oscillator,
// producing sum and difference frequencies of the input and carrier. Unlike
// tremolo (which modulates amplitude unipolar), ring modulation uses a bipolar
// carrier, creating inharmonic, metallic tones characteristic of the effect.
//
// The output for a single sample is:
//
//	wet = input * sin(2π * carrierHz * t)
//	output = input * (1 - mix) + wet * mix
type RingModulator struct {
	sampleRate float64
	carrierHz  float64
	mix        float64

	phase    float64
	phaseInc float64
}

// NewRingModulator creates a ring modulator with the given sample rate and
// optional configuration overrides.
func NewRingModulator(sampleRate float64, opts ...RingModulatorOption) (*RingModulator, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("ring modulator sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultRingModConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	r := &RingModulator{
		sampleRate: sampleRate,
		carrierHz:  cfg.carrierHz,
		mix:        cfg.mix,
	}
	r.updatePhaseIncrement()

	return r, nil
}

// SetSampleRate updates the sample rate and recalculates internal coefficients.
func (r *RingModulator) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("ring modulator sample rate must be > 0 and finite: %f", sampleRate)
	}

	r.sampleRate = sampleRate
	r.updatePhaseIncrement()

	return nil
}

// SetCarrierHz sets the carrier oscillator frequency in Hz.
func (r *RingModulator) SetCarrierHz(carrierHz float64) error {
	if carrierHz <= 0 || math.IsNaN(carrierHz) || math.IsInf(carrierHz, 0) {
		return fmt.Errorf("ring modulator carrier frequency must be > 0 and finite: %f", carrierHz)
	}

	r.carrierHz = carrierHz
	r.updatePhaseIncrement()

	return nil
}

// SetMix sets the dry/wet mix in [0, 1].
func (r *RingModulator) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("ring modulator mix must be in [0, 1]: %f", mix)
	}

	r.mix = mix

	return nil
}

// Reset clears the oscillator phase.
func (r *RingModulator) Reset() {
	r.phase = 0
}

// Process processes one sample through the ring modulator.
func (r *RingModulator) Process(sample float64) float64 {
	carrier := math.Sin(r.phase)
	wet := sample * carrier

	r.phase += r.phaseInc
	if r.phase >= 2*math.Pi {
		r.phase -= 2 * math.Pi
	}

	return sample*(1-r.mix) + wet*r.mix
}

// ProcessSample is an alias for Process.
func (r *RingModulator) ProcessSample(sample float64) float64 {
	return r.Process(sample)
}

// ProcessInPlace applies ring modulation to buf in place.
func (r *RingModulator) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = r.Process(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (r *RingModulator) SampleRate() float64 { return r.sampleRate }

// CarrierHz returns the carrier oscillator frequency in Hz.
func (r *RingModulator) CarrierHz() float64 { return r.carrierHz }

// Mix returns the dry/wet mix in [0, 1].
func (r *RingModulator) Mix() float64 { return r.mix }

func (r *RingModulator) updatePhaseIncrement() {
	r.phaseInc = 2 * math.Pi * r.carrierHz / r.sampleRate
}
