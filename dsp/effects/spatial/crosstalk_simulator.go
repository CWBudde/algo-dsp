package spatial

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultSimulatorDiameter  = 0.175
	defaultSimulatorSpeed     = 343.0
	defaultSimulatorCrossfeed = 0.2
	defaultSimulatorPreset    = CrosstalkPresetHandcrafted
	minSimulatorDiameter      = 0.08
	maxSimulatorDiameter      = 0.35
	minSimulatorDelaySamples  = 1
	maxSimulatorCrossfeed     = 1.0
	defaultSimulatorFilterQ   = 0.7071067811865476
	minSimulatorSpeedOfSound  = 300.0
	maxSimulatorSpeedOfSound  = 370.0
)

// CrosstalkPreset names the cascaded IIR shaping profile used for crossfeed.
type CrosstalkPreset int

const (
	CrosstalkPresetHandcrafted CrosstalkPreset = iota
	CrosstalkPresetIRCAM
	CrosstalkPresetHDPHX
)

// CrosstalkSimulatorOption mutates simulator construction parameters.
type CrosstalkSimulatorOption func(*crosstalkSimulatorConfig) error

type crosstalkSimulatorConfig struct {
	diameter       float64
	speedOfSound   float64
	crossfeedMix   float64
	invertPolarity bool
	preset         CrosstalkPreset
}

func defaultCrosstalkSimulatorConfig() crosstalkSimulatorConfig {
	return crosstalkSimulatorConfig{
		diameter:       defaultSimulatorDiameter,
		speedOfSound:   defaultSimulatorSpeed,
		crossfeedMix:   defaultSimulatorCrossfeed,
		invertPolarity: false,
		preset:         defaultSimulatorPreset,
	}
}

// WithSimulatorDiameter sets head diameter model in meters.
func WithSimulatorDiameter(diameter float64) CrosstalkSimulatorOption {
	return func(cfg *crosstalkSimulatorConfig) error {
		if diameter < minSimulatorDiameter || diameter > maxSimulatorDiameter ||
			math.IsNaN(diameter) || math.IsInf(diameter, 0) {
			return fmt.Errorf("crosstalk simulator diameter must be in [%g, %g]: %f",
				minSimulatorDiameter, maxSimulatorDiameter, diameter)
		}

		cfg.diameter = diameter

		return nil
	}
}

// WithSimulatorCrossfeedMix sets opposite-channel mix amount in [0,1].
func WithSimulatorCrossfeedMix(mix float64) CrosstalkSimulatorOption {
	return func(cfg *crosstalkSimulatorConfig) error {
		if mix < 0 || mix > maxSimulatorCrossfeed || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("crosstalk simulator crossfeed mix must be in [0, 1]: %f", mix)
		}

		cfg.crossfeedMix = mix

		return nil
	}
}

// WithSimulatorSpeedOfSound sets speed-of-sound model (m/s).
func WithSimulatorSpeedOfSound(speed float64) CrosstalkSimulatorOption {
	return func(cfg *crosstalkSimulatorConfig) error {
		if speed < minSimulatorSpeedOfSound || speed > maxSimulatorSpeedOfSound ||
			math.IsNaN(speed) || math.IsInf(speed, 0) {
			return fmt.Errorf("crosstalk simulator speed of sound must be in [%g, %g]: %f",
				minSimulatorSpeedOfSound, maxSimulatorSpeedOfSound, speed)
		}

		cfg.speedOfSound = speed

		return nil
	}
}

// WithSimulatorPolarityInvert toggles crossfeed polarity inversion.
func WithSimulatorPolarityInvert(invert bool) CrosstalkSimulatorOption {
	return func(cfg *crosstalkSimulatorConfig) error {
		cfg.invertPolarity = invert
		return nil
	}
}

// WithSimulatorPreset selects an IIR crossfeed shaping preset.
func WithSimulatorPreset(preset CrosstalkPreset) CrosstalkSimulatorOption {
	return func(cfg *crosstalkSimulatorConfig) error {
		if !validCrosstalkPreset(preset) {
			return fmt.Errorf("crosstalk simulator preset is invalid: %d", preset)
		}

		cfg.preset = preset

		return nil
	}
}

// CrosstalkSimulator emulates acoustic crosstalk via delayed IIR-shaped crossfeed.
type CrosstalkSimulator struct {
	sampleRate     float64
	diameter       float64
	speedOfSound   float64
	crossfeedMix   float64
	invertPolarity bool
	preset         CrosstalkPreset
	delaySamples   int
	lineLFromR     monoDelay
	lineRFromL     monoDelay
	shapeLFromR    *biquad.Chain
	shapeRFromL    *biquad.Chain
}

// NewCrosstalkSimulator creates an IIR crosstalk simulator.
func NewCrosstalkSimulator(sampleRate float64, opts ...CrosstalkSimulatorOption) (*CrosstalkSimulator, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("crosstalk simulator sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultCrosstalkSimulatorConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	s := &CrosstalkSimulator{
		sampleRate:     sampleRate,
		diameter:       cfg.diameter,
		speedOfSound:   cfg.speedOfSound,
		crossfeedMix:   cfg.crossfeedMix,
		invertPolarity: cfg.invertPolarity,
		preset:         cfg.preset,
	}

	err := s.rebuild()
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ProcessStereo processes one stereo sample pair.
func (s *CrosstalkSimulator) ProcessStereo(left, right float64) (float64, float64) {
	xR := s.shapeLFromR.ProcessSample(s.lineLFromR.tick(right))
	xL := s.shapeRFromL.ProcessSample(s.lineRFromL.tick(left))

	if s.invertPolarity {
		xR = -xR
		xL = -xL
	}

	dry := 1 - s.crossfeedMix
	outL := left*dry + xR*s.crossfeedMix
	outR := right*dry + xL*s.crossfeedMix

	return outL, outR
}

// ProcessInPlace processes paired left/right buffers in place.
func (s *CrosstalkSimulator) ProcessInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("crosstalk simulator: left and right lengths must match: %d != %d", len(left), len(right))
	}

	for i := range left {
		left[i], right[i] = s.ProcessStereo(left[i], right[i])
	}

	return nil
}

// Reset clears internal delay/filter state.
func (s *CrosstalkSimulator) Reset() {
	s.lineLFromR.reset()
	s.lineRFromL.reset()

	if s.shapeLFromR != nil {
		s.shapeLFromR.Reset()
	}

	if s.shapeRFromL != nil {
		s.shapeRFromL.Reset()
	}
}

// SetSampleRate updates sample rate and rebuilds delay/filter internals.
func (s *CrosstalkSimulator) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("crosstalk simulator sample rate must be > 0 and finite: %f", sampleRate)
	}

	s.sampleRate = sampleRate

	return s.rebuild()
}

// SetDiameter updates physical diameter and rebuilds delay/filter internals.
func (s *CrosstalkSimulator) SetDiameter(diameter float64) error {
	if diameter < minSimulatorDiameter || diameter > maxSimulatorDiameter || math.IsNaN(diameter) || math.IsInf(diameter, 0) {
		return fmt.Errorf("crosstalk simulator diameter must be in [%g, %g]: %f",
			minSimulatorDiameter, maxSimulatorDiameter, diameter)
	}

	s.diameter = diameter

	return s.rebuild()
}

// SetSpeedOfSound updates speed-of-sound model and rebuilds delay internals.
func (s *CrosstalkSimulator) SetSpeedOfSound(speed float64) error {
	if speed < minSimulatorSpeedOfSound || speed > maxSimulatorSpeedOfSound ||
		math.IsNaN(speed) || math.IsInf(speed, 0) {
		return fmt.Errorf("crosstalk simulator speed of sound must be in [%g, %g]: %f",
			minSimulatorSpeedOfSound, maxSimulatorSpeedOfSound, speed)
	}

	s.speedOfSound = speed

	return s.rebuild()
}

// SetCrossfeedMix updates crossfeed amount in [0,1].
func (s *CrosstalkSimulator) SetCrossfeedMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("crosstalk simulator crossfeed mix must be in [0, 1]: %f", mix)
	}

	s.crossfeedMix = mix

	return nil
}

// SetPreset updates shaping preset and rebuilds filter internals.
func (s *CrosstalkSimulator) SetPreset(preset CrosstalkPreset) error {
	if !validCrosstalkPreset(preset) {
		return fmt.Errorf("crosstalk simulator preset is invalid: %d", preset)
	}

	s.preset = preset

	return s.rebuild()
}

// SetPolarityInvert toggles polarity inversion for crossfeed path.
func (s *CrosstalkSimulator) SetPolarityInvert(invert bool) {
	s.invertPolarity = invert
}

// DelaySamples returns current crossfeed delay in samples.
func (s *CrosstalkSimulator) DelaySamples() int { return s.delaySamples }

func (s *CrosstalkSimulator) rebuild() error {
	if s.sampleRate <= 0 || math.IsNaN(s.sampleRate) || math.IsInf(s.sampleRate, 0) {
		return fmt.Errorf("crosstalk simulator sample rate must be > 0 and finite: %f", s.sampleRate)
	}

	if !validCrosstalkPreset(s.preset) {
		return fmt.Errorf("crosstalk simulator preset is invalid: %d", s.preset)
	}

	if s.speedOfSound < minSimulatorSpeedOfSound || s.speedOfSound > maxSimulatorSpeedOfSound ||
		math.IsNaN(s.speedOfSound) || math.IsInf(s.speedOfSound, 0) {
		return fmt.Errorf("crosstalk simulator speed of sound must be in [%g, %g]: %f",
			minSimulatorSpeedOfSound, maxSimulatorSpeedOfSound, s.speedOfSound)
	}

	delaySeconds := s.diameter / s.speedOfSound

	delaySamples := max(int(math.Round(delaySeconds*s.sampleRate)), minSimulatorDelaySamples)

	s.delaySamples = delaySamples
	s.lineLFromR.init(delaySamples)
	s.lineRFromL.init(delaySamples)

	coeffs := simulatorPresetCoefficients(s.preset, s.sampleRate)
	s.shapeLFromR = biquad.NewChain(coeffs)
	s.shapeRFromL = biquad.NewChain(coeffs)

	return nil
}

func validCrosstalkPreset(preset CrosstalkPreset) bool {
	switch preset {
	case CrosstalkPresetHandcrafted, CrosstalkPresetIRCAM, CrosstalkPresetHDPHX:
		return true
	default:
		return false
	}
}

func simulatorPresetCoefficients(preset CrosstalkPreset, sampleRate float64) []biquad.Coefficients {
	switch preset {
	case CrosstalkPresetIRCAM:
		return []biquad.Coefficients{
			design.Lowpass(1400, defaultSimulatorFilterQ, sampleRate),
			design.Peak(2500, -3, 0.9, sampleRate),
			design.HighShelf(5200, -9, defaultSimulatorFilterQ, sampleRate),
		}
	case CrosstalkPresetHDPHX:
		return []biquad.Coefficients{
			design.Lowpass(1800, defaultSimulatorFilterQ, sampleRate),
			design.Peak(3600, -4, 1.1, sampleRate),
			design.HighShelf(6800, -6, defaultSimulatorFilterQ, sampleRate),
		}
	case CrosstalkPresetHandcrafted:
		fallthrough
	default:
		return []biquad.Coefficients{
			design.Lowpass(1600, defaultSimulatorFilterQ, sampleRate),
			design.HighShelf(4200, -6, defaultSimulatorFilterQ, sampleRate),
		}
	}
}
