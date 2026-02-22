package effects

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultTransformerDrive        = 2.0
	defaultTransformerMix          = 1.0
	defaultTransformerOutputLevel  = 1.0
	defaultTransformerHighpassHz   = 25.0
	defaultTransformerDampingHz    = 9000.0
	defaultTransformerOversampling = 4

	minTransformerDrive       = 0.1
	maxTransformerDrive       = 30.0
	minTransformerOutputLevel = 0.0
	maxTransformerOutputLevel = 4.0
	minTransformerHighpassHz  = 5.0
	minTransformerDampingHz   = 200.0
)

// TransformerQuality selects transformer processing quality/performance tradeoff.
type TransformerQuality int

const (
	// TransformerQualityHigh uses oversampling + anti-alias filtering + exact tanh.
	TransformerQualityHigh TransformerQuality = iota
	// TransformerQualityLightweight uses base-rate polynomial shaping.
	TransformerQualityLightweight
)

// TransformerSimulationOption mutates construction-time parameters.
type TransformerSimulationOption func(*transformerSimulationConfig) error

type transformerSimulationConfig struct {
	quality      TransformerQuality
	drive        float64
	mix          float64
	outputLevel  float64
	highpassHz   float64
	dampingHz    float64
	overSampling int
}

func defaultTransformerSimulationConfig() transformerSimulationConfig {
	return transformerSimulationConfig{
		quality:      TransformerQualityHigh,
		drive:        defaultTransformerDrive,
		mix:          defaultTransformerMix,
		outputLevel:  defaultTransformerOutputLevel,
		highpassHz:   defaultTransformerHighpassHz,
		dampingHz:    defaultTransformerDampingHz,
		overSampling: defaultTransformerOversampling,
	}
}

// WithTransformerQuality sets high-quality or lightweight nonlinear mode.
func WithTransformerQuality(quality TransformerQuality) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if !validTransformerQuality(quality) {
			return fmt.Errorf("transformer simulation quality is invalid: %d", quality)
		}
		cfg.quality = quality
		return nil
	}
}

// WithTransformerDrive sets nonlinearity drive in [0.1, 30].
func WithTransformerDrive(drive float64) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if drive < minTransformerDrive || drive > maxTransformerDrive || math.IsNaN(drive) || math.IsInf(drive, 0) {
			return fmt.Errorf("transformer simulation drive must be in [%g, %g]: %f",
				minTransformerDrive, maxTransformerDrive, drive)
		}
		cfg.drive = drive
		return nil
	}
}

// WithTransformerMix sets dry/wet mix in [0, 1].
func WithTransformerMix(mix float64) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("transformer simulation mix must be in [0, 1]: %f", mix)
		}
		cfg.mix = mix
		return nil
	}
}

// WithTransformerOutputLevel sets post-shape output level in [0, 4].
func WithTransformerOutputLevel(level float64) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if level < minTransformerOutputLevel || level > maxTransformerOutputLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("transformer simulation output level must be in [%g, %g]: %f",
				minTransformerOutputLevel, maxTransformerOutputLevel, level)
		}
		cfg.outputLevel = level
		return nil
	}
}

// WithTransformerHighpassHz sets pre-emphasis high-pass frequency in Hz.
func WithTransformerHighpassHz(freq float64) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if freq < minTransformerHighpassHz || math.IsNaN(freq) || math.IsInf(freq, 0) {
			return fmt.Errorf("transformer simulation high-pass frequency must be >= %g: %f",
				minTransformerHighpassHz, freq)
		}
		cfg.highpassHz = freq
		return nil
	}
}

// WithTransformerDampingHz sets post-shape damping low-pass frequency in Hz.
func WithTransformerDampingHz(freq float64) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if freq < minTransformerDampingHz || math.IsNaN(freq) || math.IsInf(freq, 0) {
			return fmt.Errorf("transformer simulation damping frequency must be >= %g: %f",
				minTransformerDampingHz, freq)
		}
		cfg.dampingHz = freq
		return nil
	}
}

// WithTransformerOversampling sets oversampling factor for high-quality mode.
// Allowed values: 2, 4, 8.
func WithTransformerOversampling(factor int) TransformerSimulationOption {
	return func(cfg *transformerSimulationConfig) error {
		if !validOversamplingFactor(factor) {
			return fmt.Errorf("transformer simulation oversampling factor must be one of {2,4,8}: %d", factor)
		}
		cfg.overSampling = factor
		return nil
	}
}

// TransformerSimulation is a transformer-style saturation model with
// pre-emphasis high-pass, nonlinear saturation, damping low-pass, and an
// optional oversampled anti-alias path.
type TransformerSimulation struct {
	sampleRate float64

	quality      TransformerQuality
	drive        float64
	mix          float64
	outputLevel  float64
	highpassHz   float64
	dampingHz    float64
	overSampling int

	preHP *biquad.Section

	dampBase *biquad.Section
	upAA     *biquad.Section
	downAA   *biquad.Section
	dampOS   *biquad.Section
}

// NewTransformerSimulation creates a transformer-style saturation processor.
func NewTransformerSimulation(sampleRate float64, opts ...TransformerSimulationOption) (*TransformerSimulation, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("transformer simulation sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultTransformerSimulationConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	t := &TransformerSimulation{
		sampleRate:   sampleRate,
		quality:      cfg.quality,
		drive:        cfg.drive,
		mix:          cfg.mix,
		outputLevel:  cfg.outputLevel,
		highpassHz:   cfg.highpassHz,
		dampingHz:    cfg.dampingHz,
		overSampling: cfg.overSampling,
	}

	if err := t.rebuildFilters(); err != nil {
		return nil, err
	}

	return t, nil
}

// SetSampleRate updates sample rate and rebuilds filters.
func (t *TransformerSimulation) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("transformer simulation sample rate must be > 0 and finite: %f", sampleRate)
	}
	t.sampleRate = sampleRate
	return t.rebuildFilters()
}

// SetQuality sets processing quality mode.
func (t *TransformerSimulation) SetQuality(quality TransformerQuality) error {
	if !validTransformerQuality(quality) {
		return fmt.Errorf("transformer simulation quality is invalid: %d", quality)
	}
	t.quality = quality
	return nil
}

// SetDrive updates nonlinearity drive in [0.1, 30].
func (t *TransformerSimulation) SetDrive(drive float64) error {
	if drive < minTransformerDrive || drive > maxTransformerDrive || math.IsNaN(drive) || math.IsInf(drive, 0) {
		return fmt.Errorf("transformer simulation drive must be in [%g, %g]: %f",
			minTransformerDrive, maxTransformerDrive, drive)
	}
	t.drive = drive
	return nil
}

// SetMix updates dry/wet mix in [0, 1].
func (t *TransformerSimulation) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("transformer simulation mix must be in [0, 1]: %f", mix)
	}
	t.mix = mix
	return nil
}

// SetOutputLevel updates post-shape output level in [0, 4].
func (t *TransformerSimulation) SetOutputLevel(level float64) error {
	if level < minTransformerOutputLevel || level > maxTransformerOutputLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("transformer simulation output level must be in [%g, %g]: %f",
			minTransformerOutputLevel, maxTransformerOutputLevel, level)
	}
	t.outputLevel = level
	return nil
}

// SetHighpassHz updates pre-emphasis high-pass frequency and rebuilds filters.
func (t *TransformerSimulation) SetHighpassHz(freq float64) error {
	if freq < minTransformerHighpassHz || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return fmt.Errorf("transformer simulation high-pass frequency must be >= %g: %f",
			minTransformerHighpassHz, freq)
	}
	t.highpassHz = freq
	return t.rebuildFilters()
}

// SetDampingHz updates damping low-pass frequency and rebuilds filters.
func (t *TransformerSimulation) SetDampingHz(freq float64) error {
	if freq < minTransformerDampingHz || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return fmt.Errorf("transformer simulation damping frequency must be >= %g: %f",
			minTransformerDampingHz, freq)
	}
	t.dampingHz = freq
	return t.rebuildFilters()
}

// SetOversampling updates the oversampling factor and rebuilds filters.
func (t *TransformerSimulation) SetOversampling(factor int) error {
	if !validOversamplingFactor(factor) {
		return fmt.Errorf("transformer simulation oversampling factor must be one of {2,4,8}: %d", factor)
	}
	t.overSampling = factor
	return t.rebuildFilters()
}

// Reset clears all filter states.
func (t *TransformerSimulation) Reset() {
	if t.preHP != nil {
		t.preHP.Reset()
	}
	if t.dampBase != nil {
		t.dampBase.Reset()
	}
	if t.upAA != nil {
		t.upAA.Reset()
	}
	if t.downAA != nil {
		t.downAA.Reset()
	}
	if t.dampOS != nil {
		t.dampOS.Reset()
	}
}

// ProcessSample processes one sample.
func (t *TransformerSimulation) ProcessSample(input float64) float64 {
	if t.preHP == nil {
		return input
	}

	x := t.preHP.ProcessSample(input)

	var wet float64
	if t.quality == TransformerQualityLightweight {
		wet = t.processLightweight(x)
	} else {
		wet = t.processHighQuality(x)
	}

	wet *= t.outputLevel
	if math.IsNaN(wet) || math.IsInf(wet, 0) {
		wet = 0
	}

	return input*(1-t.mix) + wet*t.mix
}

// ProcessInPlace applies the effect to a buffer in place.
func (t *TransformerSimulation) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = t.ProcessSample(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (t *TransformerSimulation) SampleRate() float64 { return t.sampleRate }

// Quality returns processing quality mode.
func (t *TransformerSimulation) Quality() TransformerQuality { return t.quality }

// Drive returns drive factor.
func (t *TransformerSimulation) Drive() float64 { return t.drive }

// Mix returns dry/wet mix in [0, 1].
func (t *TransformerSimulation) Mix() float64 { return t.mix }

// OutputLevel returns post-shape output gain.
func (t *TransformerSimulation) OutputLevel() float64 { return t.outputLevel }

// HighpassHz returns pre-emphasis high-pass frequency in Hz.
func (t *TransformerSimulation) HighpassHz() float64 { return t.highpassHz }

// DampingHz returns damping low-pass frequency in Hz.
func (t *TransformerSimulation) DampingHz() float64 { return t.dampingHz }

// Oversampling returns oversampling factor for high-quality mode.
func (t *TransformerSimulation) Oversampling() int { return t.overSampling }

func (t *TransformerSimulation) processLightweight(x float64) float64 {
	y := transformerPolySaturate(x * t.drive)
	if t.dampBase != nil {
		y = t.dampBase.ProcessSample(y)
	}
	return y
}

func (t *TransformerSimulation) processHighQuality(x float64) float64 {
	if t.overSampling <= 1 || t.upAA == nil || t.downAA == nil || t.dampOS == nil {
		return t.processLightweight(x)
	}

	os := float64(t.overSampling)
	var out float64

	for i := 0; i < t.overSampling; i++ {
		inOS := 0.0
		if i == 0 {
			inOS = x * os
		}

		u := t.upAA.ProcessSample(inOS)
		u = math.Tanh(u * t.drive)
		u = t.dampOS.ProcessSample(u)
		u = t.downAA.ProcessSample(u)

		if i == t.overSampling-1 {
			out = u
		}
	}

	return out
}

func (t *TransformerSimulation) rebuildFilters() error {
	if t.sampleRate <= 0 || math.IsNaN(t.sampleRate) || math.IsInf(t.sampleRate, 0) {
		return fmt.Errorf("transformer simulation sample rate must be > 0 and finite: %f", t.sampleRate)
	}
	if !validTransformerQuality(t.quality) {
		return fmt.Errorf("transformer simulation quality is invalid: %d", t.quality)
	}
	if !validOversamplingFactor(t.overSampling) {
		return fmt.Errorf("transformer simulation oversampling factor must be one of {2,4,8}: %d", t.overSampling)
	}

	nyquist := t.sampleRate / 2
	hpHz := t.highpassHz
	if hpHz >= nyquist {
		hpHz = nyquist * 0.9
	}
	if hpHz < minTransformerHighpassHz {
		hpHz = minTransformerHighpassHz
	}

	dampHz := t.dampingHz
	if dampHz >= nyquist {
		dampHz = nyquist * 0.95
	}
	if dampHz < minTransformerDampingHz {
		dampHz = minTransformerDampingHz
	}

	pre := design.Highpass(hpHz, 0.7071067811865476, t.sampleRate)
	dampBase := design.Lowpass(dampHz, 0.7071067811865476, t.sampleRate)
	t.preHP = biquad.NewSection(pre)
	t.dampBase = biquad.NewSection(dampBase)

	osRate := t.sampleRate * float64(t.overSampling)
	antiAliasHz := t.sampleRate * 0.45 / 2 // keep content below base Nyquist before decimation
	if antiAliasHz >= osRate/2 {
		antiAliasHz = osRate * 0.45 / 2
	}

	upAA := design.Lowpass(antiAliasHz, 0.7071067811865476, osRate)
	downAA := design.Lowpass(antiAliasHz, 0.7071067811865476, osRate)
	dampOS := design.Lowpass(dampHz, 0.7071067811865476, osRate)

	t.upAA = biquad.NewSection(upAA)
	t.downAA = biquad.NewSection(downAA)
	t.dampOS = biquad.NewSection(dampOS)

	return nil
}

func validTransformerQuality(quality TransformerQuality) bool {
	return quality == TransformerQualityHigh || quality == TransformerQualityLightweight
}

func validOversamplingFactor(factor int) bool {
	return factor == 2 || factor == 4 || factor == 8
}

func transformerPolySaturate(x float64) float64 {
	if x > 3 {
		return 1
	}
	if x < -3 {
		return -1
	}
	x2 := x * x
	return clamp(x*(27+x2)/(27+9*x2), -1, 1)
}
