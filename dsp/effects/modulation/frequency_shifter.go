package modulation

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

const (
	defaultFreqShiftHz = 100.0
	sqrtHalf           = 0.70710678118654752440084436210485
)

// FrequencyShifterOption mutates frequency shifter construction parameters.
type FrequencyShifterOption func(*frequencyShifterConfig) error

type frequencyShifterConfig struct {
	shiftHz float64

	useCustomDesign bool
	preset          hilbert.Preset
	coeffCount      int
	transition      float64
}

func defaultFrequencyShifterConfig() frequencyShifterConfig {
	return frequencyShifterConfig{
		shiftHz: defaultFreqShiftHz,
		preset:  hilbert.PresetFast,
	}
}

// WithFrequencyShiftHz sets frequency shift in Hz (> 0).
func WithFrequencyShiftHz(shiftHz float64) FrequencyShifterOption {
	return func(cfg *frequencyShifterConfig) error {
		if shiftHz <= 0 || math.IsNaN(shiftHz) || math.IsInf(shiftHz, 0) {
			return fmt.Errorf("frequency shifter shift Hz must be > 0 and finite: %f", shiftHz)
		}
		cfg.shiftHz = shiftHz
		return nil
	}
}

// WithFrequencyShifterHilbertPreset selects a Hilbert design preset.
func WithFrequencyShifterHilbertPreset(preset hilbert.Preset) FrequencyShifterOption {
	return func(cfg *frequencyShifterConfig) error {
		if _, _, err := hilbert.PresetConfig(preset); err != nil {
			return err
		}
		cfg.useCustomDesign = false
		cfg.preset = preset
		return nil
	}
}

// WithFrequencyShifterHilbertDesign selects an explicit Hilbert design.
func WithFrequencyShifterHilbertDesign(numberOfCoeffs int, transition float64) FrequencyShifterOption {
	return func(cfg *frequencyShifterConfig) error {
		if _, err := hilbert.DesignCoefficients(numberOfCoeffs, transition); err != nil {
			return err
		}
		cfg.useCustomDesign = true
		cfg.coeffCount = numberOfCoeffs
		cfg.transition = transition
		return nil
	}
}

// FrequencyShifter is a Bode-style single-sideband frequency shifter that
// produces independent upshift and downshift outputs.
type FrequencyShifter struct {
	sampleRate float64
	shiftHz    float64
	phase      float64
	phaseInc   float64

	hilbert *hilbert.Processor64

	useCustomDesign bool
	preset          hilbert.Preset
	coeffCount      int
	transition      float64
}

// NewFrequencyShifter creates a frequency shifter with optional settings.
func NewFrequencyShifter(sampleRate float64, opts ...FrequencyShifterOption) (*FrequencyShifter, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("frequency shifter sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultFrequencyShifterConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	var (
		h   *hilbert.Processor64
		err error
	)
	if cfg.useCustomDesign {
		h, err = hilbert.New64(cfg.coeffCount, cfg.transition)
	} else {
		h, err = hilbert.New64Preset(cfg.preset)
	}
	if err != nil {
		return nil, err
	}

	f := &FrequencyShifter{
		sampleRate:      sampleRate,
		shiftHz:         cfg.shiftHz,
		hilbert:         h,
		useCustomDesign: cfg.useCustomDesign,
		preset:          cfg.preset,
		coeffCount:      cfg.coeffCount,
		transition:      cfg.transition,
	}
	f.updatePhaseIncrement()

	if !cfg.useCustomDesign {
		f.coeffCount = h.NumberOfCoefficients()
		f.transition = h.Transition()
	}

	return f, nil
}

// SetSampleRate updates the sample rate.
func (f *FrequencyShifter) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("frequency shifter sample rate must be > 0 and finite: %f", sampleRate)
	}
	f.sampleRate = sampleRate
	f.updatePhaseIncrement()
	return nil
}

// SetShiftHz updates shift frequency in Hz.
func (f *FrequencyShifter) SetShiftHz(shiftHz float64) error {
	if shiftHz <= 0 || math.IsNaN(shiftHz) || math.IsInf(shiftHz, 0) {
		return fmt.Errorf("frequency shifter shift Hz must be > 0 and finite: %f", shiftHz)
	}
	f.shiftHz = shiftHz
	f.updatePhaseIncrement()
	return nil
}

// SetHilbertPreset replaces the internal Hilbert design with a preset.
func (f *FrequencyShifter) SetHilbertPreset(preset hilbert.Preset) error {
	h, err := hilbert.New64Preset(preset)
	if err != nil {
		return err
	}
	f.hilbert = h
	f.useCustomDesign = false
	f.preset = preset
	f.coeffCount = h.NumberOfCoefficients()
	f.transition = h.Transition()
	return nil
}

// SetHilbertDesign replaces the internal Hilbert design with explicit params.
func (f *FrequencyShifter) SetHilbertDesign(numberOfCoeffs int, transition float64) error {
	h, err := hilbert.New64(numberOfCoeffs, transition)
	if err != nil {
		return err
	}
	f.hilbert = h
	f.useCustomDesign = true
	f.coeffCount = numberOfCoeffs
	f.transition = transition
	return nil
}

// Reset clears Hilbert and oscillator phase state.
func (f *FrequencyShifter) Reset() {
	f.phase = 0
	f.hilbert.Reset()
}

// ProcessSample processes one sample and returns upshift/downshift outputs.
func (f *FrequencyShifter) ProcessSample(input float64) (upshift, downshift float64) {
	re, im := f.hilbert.ProcessSample(input)
	sinOsc, cosOsc := math.Sincos(f.phase)

	re *= cosOsc
	im *= sinOsc

	upshift = (re - im) * sqrtHalf
	downshift = (re + im) * sqrtHalf

	f.phase += f.phaseInc
	if f.phase >= 2*math.Pi {
		f.phase -= 2 * math.Pi
	}

	return upshift, downshift
}

// ProcessUpshiftSample returns only the upshift output.
func (f *FrequencyShifter) ProcessUpshiftSample(input float64) float64 {
	up, _ := f.ProcessSample(input)
	return up
}

// ProcessDownshiftSample returns only the downshift output.
func (f *FrequencyShifter) ProcessDownshiftSample(input float64) float64 {
	_, down := f.ProcessSample(input)
	return down
}

// ProcessBlock processes input into upshift/downshift outputs.
func (f *FrequencyShifter) ProcessBlock(input, upshift, downshift []float64) error {
	if len(input) != len(upshift) || len(input) != len(downshift) {
		return fmt.Errorf("frequency shifter block length mismatch: in=%d up=%d down=%d",
			len(input), len(upshift), len(downshift))
	}
	for i, x := range input {
		upshift[i], downshift[i] = f.ProcessSample(x)
	}
	return nil
}

// SampleRate returns sample rate in Hz.
func (f *FrequencyShifter) SampleRate() float64 { return f.sampleRate }

// ShiftHz returns configured frequency shift in Hz.
func (f *FrequencyShifter) ShiftHz() float64 { return f.shiftHz }

// HilbertPreset returns the active preset and true when preset mode is active.
func (f *FrequencyShifter) HilbertPreset() (hilbert.Preset, bool) {
	if f.useCustomDesign {
		return hilbert.PresetFast, false
	}
	return f.preset, true
}

// HilbertDesign returns active Hilbert order/transition.
func (f *FrequencyShifter) HilbertDesign() (numberOfCoeffs int, transition float64) {
	return f.coeffCount, f.transition
}

func (f *FrequencyShifter) updatePhaseIncrement() {
	f.phaseInc = 2 * math.Pi * f.shiftHz / f.sampleRate
}
