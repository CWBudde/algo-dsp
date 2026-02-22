package spatial

import (
	"fmt"
	"math"
)

// HRTFMode controls output routing behavior.
type HRTFMode int

const (
	// HRTFModeCrossfeedOnly keeps dry direct channels and adds crossfeed convolution.
	HRTFModeCrossfeedOnly HRTFMode = iota
	// HRTFModeComplete computes output from direct and crossfeed convolutions.
	HRTFModeComplete
)

// HRTFImpulseResponseSet holds channel impulse responses for stereo routing.
type HRTFImpulseResponseSet struct {
	LeftDirect  []float64
	LeftCross   []float64
	RightDirect []float64
	RightCross  []float64
}

// HRTFProvider supplies deterministic impulse responses for a sample rate.
type HRTFProvider interface {
	ImpulseResponses(sampleRate float64) (HRTFImpulseResponseSet, error)
}

// HRTFCrosstalkSimulatorOption mutates construction-time parameters.
type HRTFCrosstalkSimulatorOption func(*hrtfCrosstalkConfig) error

type hrtfCrosstalkConfig struct {
	mode     HRTFMode
	provider HRTFProvider
}

func defaultHRTFCrosstalkConfig() hrtfCrosstalkConfig {
	return hrtfCrosstalkConfig{
		mode: HRTFModeCrossfeedOnly,
	}
}

// WithHRTFMode sets routing mode for the HRTF simulator.
func WithHRTFMode(mode HRTFMode) HRTFCrosstalkSimulatorOption {
	return func(cfg *hrtfCrosstalkConfig) error {
		if !validHRTFMode(mode) {
			return fmt.Errorf("hrtf crosstalk simulator mode is invalid: %d", mode)
		}
		cfg.mode = mode
		return nil
	}
}

// WithHRTFProvider sets the impulse-response provider.
func WithHRTFProvider(provider HRTFProvider) HRTFCrosstalkSimulatorOption {
	return func(cfg *hrtfCrosstalkConfig) error {
		if provider == nil {
			return fmt.Errorf("hrtf crosstalk simulator provider must not be nil")
		}
		cfg.provider = provider
		return nil
	}
}

// HRTFCrosstalkSimulator applies FIR convolution on direct and/or crossfeed paths.
type HRTFCrosstalkSimulator struct {
	sampleRate float64
	mode       HRTFMode
	provider   HRTFProvider

	leftDirect  firPath
	leftCross   firPath
	rightDirect firPath
	rightCross  firPath
}

// NewHRTFCrosstalkSimulator creates a new HRTF-based crosstalk simulator.
func NewHRTFCrosstalkSimulator(sampleRate float64, opts ...HRTFCrosstalkSimulatorOption) (*HRTFCrosstalkSimulator, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("hrtf crosstalk simulator sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultHRTFCrosstalkConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.provider == nil {
		return nil, fmt.Errorf("hrtf crosstalk simulator provider must not be nil")
	}

	s := &HRTFCrosstalkSimulator{
		sampleRate: sampleRate,
		mode:       cfg.mode,
		provider:   cfg.provider,
	}
	if err := s.reloadIR(); err != nil {
		return nil, err
	}
	return s, nil
}

// ProcessStereo processes one stereo sample pair.
func (s *HRTFCrosstalkSimulator) ProcessStereo(left, right float64) (float64, float64) {
	crossL := s.leftCross.process(right)
	crossR := s.rightCross.process(left)

	switch s.mode {
	case HRTFModeComplete:
		outL := s.leftDirect.process(left) + crossL
		outR := s.rightDirect.process(right) + crossR
		return outL, outR
	case HRTFModeCrossfeedOnly:
		fallthrough
	default:
		return left + crossL, right + crossR
	}
}

// ProcessInPlace processes paired left/right buffers in place.
func (s *HRTFCrosstalkSimulator) ProcessInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("hrtf crosstalk simulator: left and right lengths must match: %d != %d", len(left), len(right))
	}
	for i := range left {
		left[i], right[i] = s.ProcessStereo(left[i], right[i])
	}
	return nil
}

// Reset clears FIR histories.
func (s *HRTFCrosstalkSimulator) Reset() {
	s.leftDirect.reset()
	s.leftCross.reset()
	s.rightDirect.reset()
	s.rightCross.reset()
}

// SetMode updates routing mode.
func (s *HRTFCrosstalkSimulator) SetMode(mode HRTFMode) error {
	if !validHRTFMode(mode) {
		return fmt.Errorf("hrtf crosstalk simulator mode is invalid: %d", mode)
	}
	s.mode = mode
	return nil
}

// SetSampleRate updates sample rate and reloads IR state.
func (s *HRTFCrosstalkSimulator) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("hrtf crosstalk simulator sample rate must be > 0 and finite: %f", sampleRate)
	}
	s.sampleRate = sampleRate
	return s.reloadIR()
}

// SetProvider updates HRTF provider and reloads IR state.
func (s *HRTFCrosstalkSimulator) SetProvider(provider HRTFProvider) error {
	if provider == nil {
		return fmt.Errorf("hrtf crosstalk simulator provider must not be nil")
	}
	s.provider = provider
	return s.reloadIR()
}

func (s *HRTFCrosstalkSimulator) reloadIR() error {
	irSet, err := s.provider.ImpulseResponses(s.sampleRate)
	if err != nil {
		return fmt.Errorf("hrtf crosstalk simulator impulse response load failed: %w", err)
	}
	if err := validateIRSet(irSet, s.mode); err != nil {
		return err
	}

	s.leftCross.init(copyIR(irSet.LeftCross))
	s.rightCross.init(copyIR(irSet.RightCross))

	if len(irSet.LeftDirect) == 0 {
		s.leftDirect.init([]float64{1})
	} else {
		s.leftDirect.init(copyIR(irSet.LeftDirect))
	}
	if len(irSet.RightDirect) == 0 {
		s.rightDirect.init([]float64{1})
	} else {
		s.rightDirect.init(copyIR(irSet.RightDirect))
	}

	return nil
}

func validateIRSet(irSet HRTFImpulseResponseSet, mode HRTFMode) error {
	if len(irSet.LeftCross) == 0 {
		return fmt.Errorf("hrtf crosstalk simulator left crossfeed IR must not be empty")
	}
	if len(irSet.RightCross) == 0 {
		return fmt.Errorf("hrtf crosstalk simulator right crossfeed IR must not be empty")
	}
	if mode == HRTFModeComplete {
		if len(irSet.LeftDirect) == 0 {
			return fmt.Errorf("hrtf crosstalk simulator left direct IR must not be empty in complete mode")
		}
		if len(irSet.RightDirect) == 0 {
			return fmt.Errorf("hrtf crosstalk simulator right direct IR must not be empty in complete mode")
		}
	}
	return nil
}

func validHRTFMode(mode HRTFMode) bool {
	switch mode {
	case HRTFModeCrossfeedOnly, HRTFModeComplete:
		return true
	default:
		return false
	}
}

func copyIR(ir []float64) []float64 {
	cp := make([]float64, len(ir))
	copy(cp, ir)
	return cp
}

type firPath struct {
	ir    []float64
	hist  []float64
	write int
}

func (f *firPath) init(ir []float64) {
	if len(ir) == 0 {
		ir = []float64{1}
	}
	f.ir = ir
	f.hist = make([]float64, len(ir))
	f.write = 0
}

func (f *firPath) process(x float64) float64 {
	if len(f.ir) == 0 {
		return x
	}

	f.hist[f.write] = x

	sum := 0.0
	idx := f.write
	for i := 0; i < len(f.ir); i++ {
		sum += f.ir[i] * f.hist[idx]
		idx--
		if idx < 0 {
			idx = len(f.hist) - 1
		}
	}

	f.write++
	if f.write >= len(f.hist) {
		f.write = 0
	}

	return sum
}

func (f *firPath) reset() {
	for i := range f.hist {
		f.hist[i] = 0
	}
	f.write = 0
}
