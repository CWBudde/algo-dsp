package moog

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultCutoffHz       = 1000.0
	defaultResonance      = 0.8
	defaultDrive          = 1.0
	defaultInputGain      = 1.0
	defaultOutputGain     = 1.0
	defaultThermalVoltage = 5.0
	defaultOversampling   = 1

	minCutoffHz       = 1.0
	maxResonance      = 4.0
	minDrive          = 0.1
	maxDrive          = 24.0
	minInputGain      = 0.0
	maxInputGain      = 24.0
	minOutputGain     = 0.0
	maxOutputGain     = 24.0
	minThermalVoltage = 0.1
	maxThermalVoltage = 10.0

	stateLimit = 32.0
)

// Variant selects the nonlinear Moog ladder processing model.
type Variant int

const (
	// VariantClassic reproduces the classic four-stage nonlinear ladder from
	// DAV_DspFilterMoog.pas using exact tanh.
	VariantClassic Variant = iota
	// VariantClassicLightweight reproduces the same topology but replaces tanh
	// with a polynomial approximation for lower CPU use.
	VariantClassicLightweight
	// VariantImprovedClassic reproduces the legacy "improved classic" update
	// rule from DAV_DspFilterMoog.pas.
	VariantImprovedClassic
	// VariantImprovedClassicLightweight reproduces legacy "improved classic"
	// behavior with lightweight tanh approximation.
	VariantImprovedClassicLightweight
	// VariantHuovilainen applies Huovilainen-style tuning/resonance
	// compensation and a half-sample feedback estimate.
	VariantHuovilainen
)

func (v Variant) String() string {
	switch v {
	case VariantClassic:
		return "classic"
	case VariantClassicLightweight:
		return "classic_lightweight"
	case VariantImprovedClassic:
		return "improved_classic"
	case VariantImprovedClassicLightweight:
		return "improved_classic_lightweight"
	case VariantHuovilainen:
		return "huovilainen"
	default:
		return "unknown"
	}
}

// Option mutates constructor configuration.
type Option func(*config) error

type config struct {
	variant         Variant
	cutoffHz        float64
	resonance       float64
	drive           float64
	inputGain       float64
	outputGain      float64
	thermalVoltage  float64
	overSampling    int
	normalizeOutput bool
}

func defaultConfig() config {
	return config{
		variant:         VariantHuovilainen,
		cutoffHz:        defaultCutoffHz,
		resonance:       defaultResonance,
		drive:           defaultDrive,
		inputGain:       defaultInputGain,
		outputGain:      defaultOutputGain,
		thermalVoltage:  defaultThermalVoltage,
		overSampling:    defaultOversampling,
		normalizeOutput: true,
	}
}

// WithVariant selects the nonlinear ladder variant.
func WithVariant(variant Variant) Option {
	return func(cfg *config) error {
		if !validVariant(variant) {
			return fmt.Errorf("moog: invalid variant: %d", variant)
		}

		cfg.variant = variant

		return nil
	}
}

// WithCutoffHz sets cutoff in Hz. Must be finite and > 0.
func WithCutoffHz(cutoffHz float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(cutoffHz, minCutoffHz, math.Inf(1), "cutoff"); err != nil {
			return err
		}

		cfg.cutoffHz = cutoffHz

		return nil
	}
}

// WithResonance sets feedback resonance in [0, 4].
func WithResonance(resonance float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(resonance, 0, maxResonance, "resonance"); err != nil {
			return err
		}

		cfg.resonance = resonance

		return nil
	}
}

// WithDrive sets nonlinear drive in [0.1, 24].
func WithDrive(drive float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(drive, minDrive, maxDrive, "drive"); err != nil {
			return err
		}

		cfg.drive = drive

		return nil
	}
}

// WithInputGain sets linear pre-ladder gain in [0, 24].
func WithInputGain(gain float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(gain, minInputGain, maxInputGain, "input gain"); err != nil {
			return err
		}

		cfg.inputGain = gain

		return nil
	}
}

// WithOutputGain sets post-ladder output gain in [0, 24].
func WithOutputGain(gain float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(gain, minOutputGain, maxOutputGain, "output gain"); err != nil {
			return err
		}

		cfg.outputGain = gain

		return nil
	}
}

// WithThermalVoltage sets thermal-voltage-style shaping in [0.1, 10].
func WithThermalVoltage(thermalVoltage float64) Option {
	return func(cfg *config) error {
		if err := validateFiniteRange(thermalVoltage, minThermalVoltage, maxThermalVoltage, "thermal voltage"); err != nil {
			return err
		}

		cfg.thermalVoltage = thermalVoltage

		return nil
	}
}

// WithOversampling sets nonlinear oversampling mode. Allowed values: 1, 2, 4, 8.
func WithOversampling(factor int) Option {
	return func(cfg *config) error {
		if !validOversampling(factor) {
			return fmt.Errorf("moog: oversampling factor must be one of {1,2,4,8}: %d", factor)
		}

		cfg.overSampling = factor

		return nil
	}
}

// WithNormalizeOutput enables or disables output-level normalization.
func WithNormalizeOutput(enabled bool) Option {
	return func(cfg *config) error {
		cfg.normalizeOutput = enabled
		return nil
	}
}

// State contains explicit ladder runtime state for save/restore workflows.
type State struct {
	Stage      [4]float64
	TanhLast   [3]float64
	PrevInput  float64
	PrevOutput float64
}

// Filter is a nonlinear 4-stage Moog ladder low-pass processor.
//
// It supports legacy-faithful classic variants and a Huovilainen-style variant,
// with optional oversampled anti-alias processing for the nonlinear path.
type Filter struct {
	sampleRate float64

	variant         Variant
	cutoffHz        float64
	resonance       float64
	drive           float64
	inputGain       float64
	outputGain      float64
	thermalVoltage  float64
	overSampling    int
	normalizeOutput bool

	coefficient float64
	feedback    float64
	driveScale  float64
	outputScale float64

	state State

	antiAliasUp   *biquad.Section
	antiAliasDown *biquad.Section
}

// New constructs a nonlinear Moog ladder filter.
func New(sampleRate float64, opts ...Option) (*Filter, error) {
	if !isFinite(sampleRate) || sampleRate <= 0 {
		return nil, fmt.Errorf("moog: sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	f := &Filter{
		sampleRate:      sampleRate,
		variant:         cfg.variant,
		cutoffHz:        cfg.cutoffHz,
		resonance:       cfg.resonance,
		drive:           cfg.drive,
		inputGain:       cfg.inputGain,
		outputGain:      cfg.outputGain,
		thermalVoltage:  cfg.thermalVoltage,
		overSampling:    cfg.overSampling,
		normalizeOutput: cfg.normalizeOutput,
	}

	if err := f.rebuild(); err != nil {
		return nil, err
	}

	return f, nil
}

// SampleRate returns the sample rate in Hz.
func (f *Filter) SampleRate() float64 { return f.sampleRate }

// Variant returns the nonlinear ladder variant.
func (f *Filter) Variant() Variant { return f.variant }

// CutoffHz returns the cutoff frequency in Hz.
func (f *Filter) CutoffHz() float64 { return f.cutoffHz }

// Resonance returns the feedback resonance.
func (f *Filter) Resonance() float64 { return f.resonance }

// Drive returns nonlinear drive.
func (f *Filter) Drive() float64 { return f.drive }

// InputGain returns linear input gain.
func (f *Filter) InputGain() float64 { return f.inputGain }

// OutputGain returns post-ladder gain.
func (f *Filter) OutputGain() float64 { return f.outputGain }

// ThermalVoltage returns the thermal-voltage-style shaping parameter.
func (f *Filter) ThermalVoltage() float64 { return f.thermalVoltage }

// Oversampling returns the nonlinear oversampling factor.
func (f *Filter) Oversampling() int { return f.overSampling }

// NormalizeOutput reports whether resonance-gain normalization is enabled.
func (f *Filter) NormalizeOutput() bool { return f.normalizeOutput }

// SetSampleRate updates sample rate and rebuilds coefficients.
func (f *Filter) SetSampleRate(sampleRate float64) error {
	if !isFinite(sampleRate) || sampleRate <= 0 {
		return fmt.Errorf("moog: sample rate must be > 0 and finite: %f", sampleRate)
	}

	f.sampleRate = sampleRate

	return f.rebuild()
}

// SetVariant updates nonlinear ladder variant and rebuilds coefficients.
func (f *Filter) SetVariant(variant Variant) error {
	if !validVariant(variant) {
		return fmt.Errorf("moog: invalid variant: %d", variant)
	}

	f.variant = variant

	return f.rebuild()
}

// SetCutoffHz updates cutoff and rebuilds coefficients.
func (f *Filter) SetCutoffHz(cutoffHz float64) error {
	if err := validateFiniteRange(cutoffHz, minCutoffHz, math.Inf(1), "cutoff"); err != nil {
		return err
	}

	f.cutoffHz = cutoffHz

	return f.rebuild()
}

// SetResonance updates resonance and rebuilds coefficients.
func (f *Filter) SetResonance(resonance float64) error {
	if err := validateFiniteRange(resonance, 0, maxResonance, "resonance"); err != nil {
		return err
	}

	f.resonance = resonance

	return f.rebuild()
}

// SetDrive updates nonlinear drive.
func (f *Filter) SetDrive(drive float64) error {
	if err := validateFiniteRange(drive, minDrive, maxDrive, "drive"); err != nil {
		return err
	}

	f.drive = drive
	f.driveScale = 0.5 * f.drive / f.thermalVoltage

	return nil
}

// SetInputGain updates linear pre-ladder gain.
func (f *Filter) SetInputGain(gain float64) error {
	if err := validateFiniteRange(gain, minInputGain, maxInputGain, "input gain"); err != nil {
		return err
	}

	f.inputGain = gain

	return nil
}

// SetOutputGain updates post-ladder output gain.
func (f *Filter) SetOutputGain(gain float64) error {
	if err := validateFiniteRange(gain, minOutputGain, maxOutputGain, "output gain"); err != nil {
		return err
	}

	f.outputGain = gain
	f.updateOutputScale()

	return nil
}

// SetThermalVoltage updates shaping and rebuilds coefficients.
func (f *Filter) SetThermalVoltage(thermalVoltage float64) error {
	if err := validateFiniteRange(thermalVoltage, minThermalVoltage, maxThermalVoltage, "thermal voltage"); err != nil {
		return err
	}

	f.thermalVoltage = thermalVoltage

	return f.rebuild()
}

// SetOversampling updates oversampling mode and rebuilds anti-alias filters.
func (f *Filter) SetOversampling(factor int) error {
	if !validOversampling(factor) {
		return fmt.Errorf("moog: oversampling factor must be one of {1,2,4,8}: %d", factor)
	}

	f.overSampling = factor

	return f.rebuild()
}

// SetNormalizeOutput enables or disables resonance normalization.
func (f *Filter) SetNormalizeOutput(enabled bool) {
	f.normalizeOutput = enabled
	f.updateOutputScale()
}

// Reset clears ladder state.
func (f *Filter) Reset() {
	f.state = State{}

	if f.antiAliasUp != nil {
		f.antiAliasUp.Reset()
	}

	if f.antiAliasDown != nil {
		f.antiAliasDown.Reset()
	}
}

// State returns a copy of the current processor state.
func (f *Filter) State() State {
	return f.state
}

// SetState restores an externally saved processor state.
func (f *Filter) SetState(state State) error {
	if !stateIsFinite(state) {
		return fmt.Errorf("moog: state contains NaN or Inf")
	}

	f.state = state

	return nil
}

// ProcessSample processes one sample.
func (f *Filter) ProcessSample(input float64) float64 {
	if !isFinite(input) {
		input = 0
	}

	if f.overSampling <= 1 {
		out := f.processCore(input)
		f.state.PrevInput = input
		return sanitizeOutput(out)
	}

	prev := f.state.PrevInput
	delta := (input - prev) / float64(f.overSampling)

	var out float64
	for i := range f.overSampling {
		subInput := prev + delta*float64(i+1)

		if f.antiAliasUp != nil {
			subInput = f.antiAliasUp.ProcessSample(subInput)
		}

		subOutput := f.processCore(subInput)
		if f.antiAliasDown != nil {
			subOutput = f.antiAliasDown.ProcessSample(subOutput)
		}

		out = subOutput
	}

	f.state.PrevInput = input

	return sanitizeOutput(out)
}

// ProcessInPlace processes a mono buffer in place.
func (f *Filter) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = f.ProcessSample(buf[i])
	}
}

// ProcessTo processes src into dst. Both slices must have the same length.
func (f *Filter) ProcessTo(dst, src []float64) {
	n := len(src)
	if n == 0 {
		return
	}

	_ = dst[n-1]
	for i, x := range src {
		dst[i] = f.ProcessSample(x)
	}
}

func (f *Filter) processCore(input float64) float64 {
	switch f.variant {
	case VariantClassic:
		return f.processClassic(input, math.Tanh, false)
	case VariantClassicLightweight:
		return f.processClassic(input, fastTanhApprox, false)
	case VariantImprovedClassic:
		return f.processClassic(input, math.Tanh, true)
	case VariantImprovedClassicLightweight:
		return f.processClassic(input, fastTanhApprox, true)
	case VariantHuovilainen:
		return f.processHuovilainen(input)
	default:
		return 0
	}
}

func (f *Filter) processClassic(input float64, tanhFn func(float64) float64, improved bool) float64 {
	s := &f.state
	newInput := input*f.inputGain - f.feedback*s.Stage[3]

	stageCoefficient := f.coefficient
	if improved {
		stageCoefficient *= 2 * f.thermalVoltage
	}

	tanhInput := tanhFn(f.driveScale * newInput)
	s.Stage[0] = clipState(s.Stage[0] + stageCoefficient*(tanhInput-s.TanhLast[0]))
	s.TanhLast[0] = tanhFn(f.driveScale * s.Stage[0])

	s.Stage[1] = clipState(s.Stage[1] + stageCoefficient*(s.TanhLast[0]-s.TanhLast[1]))
	s.TanhLast[1] = tanhFn(f.driveScale * s.Stage[1])

	s.Stage[2] = clipState(s.Stage[2] + stageCoefficient*(s.TanhLast[1]-s.TanhLast[2]))
	s.TanhLast[2] = tanhFn(f.driveScale * s.Stage[2])

	s.Stage[3] = clipState(s.Stage[3] + stageCoefficient*(s.TanhLast[2]-tanhFn(f.driveScale*s.Stage[3])))
	s.PrevOutput = s.Stage[3]

	return f.outputScale * s.Stage[3]
}

func (f *Filter) processHuovilainen(input float64) float64 {
	s := &f.state

	feedbackSample := 0.5 * (s.Stage[3] + s.PrevOutput)
	driveInput := input*f.inputGain - f.feedback*feedbackSample

	shape := f.driveScale
	t0 := math.Tanh(shape * driveInput)
	tS0 := math.Tanh(shape * s.Stage[0])
	tS1 := math.Tanh(shape * s.Stage[1])
	tS2 := math.Tanh(shape * s.Stage[2])
	tS3 := math.Tanh(shape * s.Stage[3])

	g := f.coefficient
	s.Stage[0] = clipState(s.Stage[0] + g*(t0-tS0))
	s.TanhLast[0] = math.Tanh(shape * s.Stage[0])

	s.Stage[1] = clipState(s.Stage[1] + g*(s.TanhLast[0]-tS1))
	s.TanhLast[1] = math.Tanh(shape * s.Stage[1])

	s.Stage[2] = clipState(s.Stage[2] + g*(s.TanhLast[1]-tS2))
	s.TanhLast[2] = math.Tanh(shape * s.Stage[2])

	s.Stage[3] = clipState(s.Stage[3] + g*(s.TanhLast[2]-tS3))
	s.PrevOutput = s.Stage[3]

	return f.outputScale * s.Stage[3]
}

func (f *Filter) rebuild() error {
	if !validVariant(f.variant) {
		return fmt.Errorf("moog: invalid variant: %d", f.variant)
	}

	if err := validateFiniteRange(f.cutoffHz, minCutoffHz, math.Inf(1), "cutoff"); err != nil {
		return err
	}

	if err := validateFiniteRange(f.resonance, 0, maxResonance, "resonance"); err != nil {
		return err
	}

	if err := validateFiniteRange(f.drive, minDrive, maxDrive, "drive"); err != nil {
		return err
	}

	if err := validateFiniteRange(f.inputGain, minInputGain, maxInputGain, "input gain"); err != nil {
		return err
	}

	if err := validateFiniteRange(f.outputGain, minOutputGain, maxOutputGain, "output gain"); err != nil {
		return err
	}

	if err := validateFiniteRange(f.thermalVoltage, minThermalVoltage, maxThermalVoltage, "thermal voltage"); err != nil {
		return err
	}

	if !validOversampling(f.overSampling) {
		return fmt.Errorf("moog: oversampling factor must be one of {1,2,4,8}: %d", f.overSampling)
	}

	baseNyquist := f.sampleRate * 0.5
	if f.cutoffHz >= baseNyquist {
		return fmt.Errorf("moog: cutoff must be < Nyquist (%f Hz): %f", baseNyquist, f.cutoffHz)
	}

	effectiveSampleRate := f.sampleRate * float64(f.overSampling)
	fc := f.cutoffHz / effectiveSampleRate
	f.driveScale = 0.5 * f.drive / f.thermalVoltage

	f.feedback = f.resonance
	f.coefficient = 2 * f.thermalVoltage * (1 - math.Exp(-2*math.Pi*fc))

	if f.variant == VariantHuovilainen {
		fcr := 1.8730*fc*fc*fc + 0.4955*fc*fc - 0.6490*fc + 0.9988
		if fcr < 0 {
			fcr = 0
		}

		f.coefficient = 2 * f.thermalVoltage * (1 - math.Exp(-2*math.Pi*fcr*fc))

		resonanceComp := -3.9364*fc*fc + 1.8409*fc + 0.9968
		if resonanceComp < 0 {
			resonanceComp = 0
		}

		f.feedback = f.resonance * resonanceComp
	}

	f.updateOutputScale()
	f.buildAntiAliasFilters()

	return nil
}

func (f *Filter) updateOutputScale() {
	legacyResonanceScale := dbToAmp(f.resonance)
	legacyResonanceScale *= legacyResonanceScale

	norm := 1.0
	if f.normalizeOutput {
		norm = 1 / (1 + 0.5*f.resonance)
	}

	f.outputScale = f.outputGain * legacyResonanceScale * norm
}

func (f *Filter) buildAntiAliasFilters() {
	if f.overSampling <= 1 {
		f.antiAliasUp = nil
		f.antiAliasDown = nil
		return
	}

	osRate := f.sampleRate * float64(f.overSampling)
	antiAliasHz := f.sampleRate * 0.225
	if antiAliasHz >= osRate*0.5 {
		antiAliasHz = osRate * 0.225
	}

	coeff := design.Lowpass(antiAliasHz, 0.7071067811865476, osRate)
	f.antiAliasUp = biquad.NewSection(coeff)
	f.antiAliasDown = biquad.NewSection(coeff)
}

// Stereo is a helper that runs one Moog filter state per channel.
type Stereo struct {
	left  *Filter
	right *Filter
}

// NewStereo constructs a stereo helper with independent left/right state.
func NewStereo(sampleRate float64, opts ...Option) (*Stereo, error) {
	left, err := New(sampleRate, opts...)
	if err != nil {
		return nil, err
	}

	right, err := New(sampleRate, opts...)
	if err != nil {
		return nil, err
	}

	return &Stereo{left: left, right: right}, nil
}

// Left returns the left-channel filter.
func (s *Stereo) Left() *Filter { return s.left }

// Right returns the right-channel filter.
func (s *Stereo) Right() *Filter { return s.right }

// Reset clears both channel states.
func (s *Stereo) Reset() {
	s.left.Reset()
	s.right.Reset()
}

// ProcessSample processes one stereo sample frame.
func (s *Stereo) ProcessSample(leftIn, rightIn float64) (leftOut, rightOut float64) {
	return s.left.ProcessSample(leftIn), s.right.ProcessSample(rightIn)
}

// ProcessInPlace processes stereo planar buffers in place.
func (s *Stereo) ProcessInPlace(left, right []float64) {
	n := len(left)
	if n == 0 {
		return
	}

	_ = right[n-1]

	for i := range n {
		left[i], right[i] = s.ProcessSample(left[i], right[i])
	}
}

// ProcessFramesInPlace processes interleaved [left,right] frames in place.
func (s *Stereo) ProcessFramesInPlace(frames [][2]float64) {
	for i := range frames {
		frames[i][0], frames[i][1] = s.ProcessSample(frames[i][0], frames[i][1])
	}
}

func validVariant(variant Variant) bool {
	return variant >= VariantClassic && variant <= VariantHuovilainen
}

func validOversampling(factor int) bool {
	return factor == 1 || factor == 2 || factor == 4 || factor == 8
}

func validateFiniteRange(value, min, max float64, name string) error {
	if !isFinite(value) {
		return fmt.Errorf("moog: %s must be finite: %v", name, value)
	}

	if value < min || value > max {
		return fmt.Errorf("moog: %s must be in [%g, %g]: %f", name, min, max, value)
	}

	return nil
}

func sanitizeOutput(value float64) float64 {
	if !isFinite(value) {
		return 0
	}

	return value
}

func clipState(value float64) float64 {
	if value > stateLimit {
		return stateLimit
	}

	if value < -stateLimit {
		return -stateLimit
	}

	return value
}

func fastTanhApprox(x float64) float64 {
	if x > 3 {
		return 1
	}

	if x < -3 {
		return -1
	}

	x2 := x * x

	return clamp(x*(27+x2)/(27+9*x2), -1, 1)
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}

	if x > hi {
		return hi
	}

	return x
}

func dbToAmp(db float64) float64 {
	return math.Pow(10, db/20)
}

func stateIsFinite(state State) bool {
	for _, v := range state.Stage {
		if !isFinite(v) {
			return false
		}
	}

	for _, v := range state.TanhLast {
		if !isFinite(v) {
			return false
		}
	}

	return isFinite(state.PrevInput) && isFinite(state.PrevOutput)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
