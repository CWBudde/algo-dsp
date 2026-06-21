package moog

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

// Variant selects the ladder topology used by a [Filter].
type Variant int

const (
	// SimpleClassic scales each stage update by the base coefficient g.
	SimpleClassic Variant = iota

	// ImprovedClassic additionally scales each stage update by
	// 2*thermalVoltage, driving the saturating stages harder.
	ImprovedClassic
)

const (
	defaultThermalVoltage = 5.0
	defaultGain           = 1.0

	minResonance = 0.0
	maxResonance = 10.0

	// resonanceSelfOscillation is the feedback amount mapped to a normalized
	// resonance of 1.0 (classic self-oscillation onset for a 4-pole ladder).
	resonanceSelfOscillation = 4.0

	// antiAliasOrder is the Butterworth order of the oversampling anti-alias
	// filters used by the high-quality path.
	antiAliasOrder = 4

	// antiAliasCutoffScale places the anti-alias cutoff just below the base
	// Nyquist frequency (relative to the base sample rate).
	antiAliasCutoffScale = 0.45
)

// Filter is a stateful nonlinear Moog ladder lowpass filter.
//
// Use [New] to construct one, [Filter.ProcessSample] or
// [Filter.ProcessInPlace] to filter audio, and [Filter.Reset] to clear state.
type Filter struct {
	variant  Variant
	fastTanh bool

	cutoff     float64
	resonance  float64
	thermalV   float64
	gain       float64
	sampleRate float64

	// Derived coefficients (recomputed by recalc).
	coeff       float64 // per-stage update gain k at the base rate
	coeffOS     float64 // per-stage update gain k at the oversampled rate
	scaleFactor float64 // output scaling
	vtInv       float64 // 1 / thermalVoltage

	// High-quality oversampling path (nil/1 when disabled).
	oversampling int
	upAA         *biquad.Chain // interpolation anti-alias filter (at OS rate)
	downAA       *biquad.Chain // decimation anti-alias filter (at OS rate)

	s      [4]float64 // ladder integrator states
	t      [3]float64 // cached tanh outputs of stages 0..2
	prevS3 float64    // previous fourth-stage state (half-sample feedback comp.)
}

type config struct {
	variant      Variant
	fastTanh     bool
	resonance    float64
	thermalV     float64
	gain         float64
	oversampling int
}

// Option configures a [Filter] at construction time.
type Option func(*config)

// WithVariant selects the ladder topology (default [SimpleClassic]).
func WithVariant(v Variant) Option { return func(c *config) { c.variant = v } }

// WithFastTanh enables the faster polynomial tanh approximation (default off).
func WithFastTanh(enabled bool) Option { return func(c *config) { c.fastTanh = enabled } }

// WithResonance sets the raw feedback amount (default 0). See
// [Filter.SetResonanceNormalized] for a musical [0, 1] control.
func WithResonance(r float64) Option { return func(c *config) { c.resonance = r } }

// WithThermalVoltage sets the thermal-voltage shaping control (default 5).
func WithThermalVoltage(vt float64) Option { return func(c *config) { c.thermalV = vt } }

// WithGain sets the output gain applied before resonance scaling (default 1).
func WithGain(g float64) Option { return func(c *config) { c.gain = g } }

// WithOversampling enables the high-quality oversampled path at the given
// factor (1, 2, 4, or 8; default 1 = disabled). Factors above 1 run the ladder
// at factor×sampleRate with anti-alias filtering and Huovilainen half-sample
// feedback compensation, reducing aliasing at high drive and resonance.
func WithOversampling(factor int) Option { return func(c *config) { c.oversampling = factor } }

// New creates a Moog ladder filter at the given cutoff (Hz) and sample rate.
//
// The cutoff must lie in (0, sampleRate/2). Optional parameters are applied via
// [Option] values. Returns an error for invalid parameters.
func New(cutoffHz, sampleRate float64, opts ...Option) (*Filter, error) {
	cfg := config{
		variant:      SimpleClassic,
		fastTanh:     false,
		resonance:    0,
		thermalV:     defaultThermalVoltage,
		gain:         defaultGain,
		oversampling: 1,
	}

	for _, o := range opts {
		o(&cfg)
	}

	if sampleRate <= 0 || !isFinite(sampleRate) {
		return nil, fmt.Errorf("moog: sample rate must be positive and finite, got %v", sampleRate)
	}

	if cutoffHz <= 0 || cutoffHz >= sampleRate/2 || !isFinite(cutoffHz) {
		return nil, fmt.Errorf("moog: cutoff must be in (0, %v), got %v", sampleRate/2, cutoffHz)
	}

	if cfg.variant != SimpleClassic && cfg.variant != ImprovedClassic {
		return nil, fmt.Errorf("moog: unknown variant %d", cfg.variant)
	}

	if cfg.thermalV <= 0 || !isFinite(cfg.thermalV) {
		return nil, fmt.Errorf("moog: thermal voltage must be positive and finite, got %v", cfg.thermalV)
	}

	if cfg.resonance < minResonance || cfg.resonance > maxResonance || !isFinite(cfg.resonance) {
		return nil, fmt.Errorf("moog: resonance must be in [%v, %v], got %v", minResonance, maxResonance, cfg.resonance)
	}

	if !isFinite(cfg.gain) {
		return nil, fmt.Errorf("moog: gain must be finite, got %v", cfg.gain)
	}

	if !validOversampling(cfg.oversampling) {
		return nil, fmt.Errorf("moog: oversampling factor must be one of {1, 2, 4, 8}, got %d", cfg.oversampling)
	}

	f := &Filter{
		variant:      cfg.variant,
		fastTanh:     cfg.fastTanh,
		cutoff:       cutoffHz,
		resonance:    cfg.resonance,
		thermalV:     cfg.thermalV,
		gain:         cfg.gain,
		sampleRate:   sampleRate,
		oversampling: cfg.oversampling,
	}
	f.recalc()

	return f, nil
}

func validOversampling(factor int) bool {
	return factor == 1 || factor == 2 || factor == 4 || factor == 8
}

// recalc recomputes the derived coefficients from the current parameters.
func (f *Filter) recalc() {
	f.vtInv = 1 / f.thermalV
	f.coeff = f.stageGain(f.sampleRate)

	amp := math.Pow(10, f.resonance/20) // dB_to_Amp(resonance)
	f.scaleFactor = f.gain * amp * amp

	if f.oversampling > 1 {
		osRate := f.sampleRate * float64(f.oversampling)
		f.coeffOS = f.stageGain(osRate)

		// Anti-alias cutoff just below the base Nyquist, designed at OS rate.
		aaHz := antiAliasCutoffScale * f.sampleRate
		coeffs := design.ButterworthLP(aaHz, antiAliasOrder, osRate)
		f.upAA = biquad.NewChain(coeffs)
		f.downAA = biquad.NewChain(coeffs)
	} else {
		f.coeffOS = 0
		f.upAA = nil
		f.downAA = nil
	}
}

// stageGain returns the per-stage update coefficient at the given rate.
func (f *Filter) stageGain(rate float64) float64 {
	g := 2 * f.thermalV * (1 - math.Exp(-2*math.Pi*f.cutoff/rate))
	if f.variant == ImprovedClassic {
		g *= 2 * f.thermalV
	}

	return g
}

// ProcessSample filters one input sample and returns the output.
func (f *Filter) ProcessSample(x float64) float64 {
	if f.oversampling > 1 {
		return f.processOversampled(x)
	}

	return f.ladderStep(x, f.coeff, f.s[3])
}

// ladderStep runs one ladder iteration at the given stage coefficient, using
// feedback as the resonance feedback signal, and returns the scaled output.
func (f *Filter) ladderStep(x, coeff, feedback float64) float64 {
	tanhFn := math.Tanh
	if f.fastTanh {
		tanhFn = fastTanh
	}

	in := x - f.resonance*feedback

	f.s[0] += coeff * (tanhFn(0.5*in*f.vtInv) - f.t[0])
	f.t[0] = tanhFn(0.5 * f.s[0] * f.vtInv)

	f.s[1] += coeff * (f.t[0] - f.t[1])
	f.t[1] = tanhFn(0.5 * f.s[1] * f.vtInv)

	f.s[2] += coeff * (f.t[1] - f.t[2])
	f.t[2] = tanhFn(0.5 * f.s[2] * f.vtInv)

	f.s[3] += coeff * (f.t[2] - tanhFn(0.5*f.s[3]*f.vtInv))

	return f.scaleFactor * f.s[3]
}

// processOversampled runs the high-quality path: the input is zero-stuffed to
// the oversampled rate, interpolation/decimation anti-alias filtered, and the
// ladder is run per oversampled tick with Huovilainen half-sample feedback
// compensation (the feedback averages the current and previous fourth-stage
// state).
func (f *Filter) processOversampled(x float64) float64 {
	os := float64(f.oversampling)

	var out float64

	for i := range f.oversampling {
		in := 0.0
		if i == 0 {
			in = x * os
		}

		in = f.upAA.ProcessSample(in)

		fb := 0.5 * (f.s[3] + f.prevS3)
		f.prevS3 = f.s[3]

		y := f.downAA.ProcessSample(f.ladderStep(in, f.coeffOS, fb))

		if i == f.oversampling-1 {
			out = y
		}
	}

	return out
}

// ProcessInPlace filters a block of samples in place.
func (f *Filter) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = f.ProcessSample(buf[i])
	}
}

// Reset clears the internal ladder state.
func (f *Filter) Reset() {
	f.s = [4]float64{}
	f.t = [3]float64{}
	f.prevS3 = 0

	if f.upAA != nil {
		f.upAA.Reset()
	}

	if f.downAA != nil {
		f.downAA.Reset()
	}
}

// SetCutoff updates the cutoff frequency (Hz) and recomputes coefficients.
func (f *Filter) SetCutoff(hz float64) error {
	if hz <= 0 || hz >= f.sampleRate/2 || !isFinite(hz) {
		return fmt.Errorf("moog: cutoff must be in (0, %v), got %v", f.sampleRate/2, hz)
	}

	f.cutoff = hz
	f.recalc()

	return nil
}

// SetResonance updates the raw feedback amount and recomputes coefficients.
func (f *Filter) SetResonance(r float64) error {
	if r < minResonance || r > maxResonance || !isFinite(r) {
		return fmt.Errorf("moog: resonance must be in [%v, %v], got %v", minResonance, maxResonance, r)
	}

	f.resonance = r
	f.recalc()

	return nil
}

// SetResonanceNormalized sets resonance from a musical [0, 1] control, where
// 1.0 maps to the classic self-oscillation onset.
func (f *Filter) SetResonanceNormalized(r float64) error {
	if r < 0 || r > 1 || !isFinite(r) {
		return fmt.Errorf("moog: normalized resonance must be in [0, 1], got %v", r)
	}

	return f.SetResonance(r * resonanceSelfOscillation)
}

// SetOversampling updates the oversampling factor (1, 2, 4, or 8) and rebuilds
// the anti-alias filters. The internal state is reset.
func (f *Filter) SetOversampling(factor int) error {
	if !validOversampling(factor) {
		return fmt.Errorf("moog: oversampling factor must be one of {1, 2, 4, 8}, got %d", factor)
	}

	f.oversampling = factor
	f.recalc()
	f.Reset()

	return nil
}

// SetSampleRate updates the sample rate (Hz) and recomputes coefficients.
// The current cutoff must remain below the new Nyquist frequency.
func (f *Filter) SetSampleRate(sr float64) error {
	if sr <= 0 || !isFinite(sr) {
		return fmt.Errorf("moog: sample rate must be positive and finite, got %v", sr)
	}

	if f.cutoff >= sr/2 {
		return fmt.Errorf("moog: cutoff %v exceeds Nyquist for sample rate %v", f.cutoff, sr)
	}

	f.sampleRate = sr
	f.recalc()

	return nil
}

// Cutoff returns the cutoff frequency in Hz.
func (f *Filter) Cutoff() float64 { return f.cutoff }

// Resonance returns the raw feedback amount.
func (f *Filter) Resonance() float64 { return f.resonance }

// SampleRate returns the sample rate in Hz.
func (f *Filter) SampleRate() float64 { return f.sampleRate }

// ThermalVoltage returns the thermal-voltage shaping control.
func (f *Filter) ThermalVoltage() float64 { return f.thermalV }

// Variant returns the ladder topology in use.
func (f *Filter) Variant() Variant { return f.variant }

// Oversampling returns the oversampling factor (1 when the high-quality path is
// disabled).
func (f *Filter) Oversampling() int { return f.oversampling }

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
