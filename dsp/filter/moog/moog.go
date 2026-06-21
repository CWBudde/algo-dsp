package moog

import (
	"fmt"
	"math"
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
	coeff       float64 // per-stage update gain k
	scaleFactor float64 // output scaling
	vtInv       float64 // 1 / thermalVoltage

	s [4]float64 // ladder integrator states
	t [3]float64 // cached tanh outputs of stages 0..2
}

type config struct {
	variant   Variant
	fastTanh  bool
	resonance float64
	thermalV  float64
	gain      float64
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

// New creates a Moog ladder filter at the given cutoff (Hz) and sample rate.
//
// The cutoff must lie in (0, sampleRate/2). Optional parameters are applied via
// [Option] values. Returns an error for invalid parameters.
func New(cutoffHz, sampleRate float64, opts ...Option) (*Filter, error) {
	cfg := config{
		variant:   SimpleClassic,
		fastTanh:  false,
		resonance: 0,
		thermalV:  defaultThermalVoltage,
		gain:      defaultGain,
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

	f := &Filter{
		variant:    cfg.variant,
		fastTanh:   cfg.fastTanh,
		cutoff:     cutoffHz,
		resonance:  cfg.resonance,
		thermalV:   cfg.thermalV,
		gain:       cfg.gain,
		sampleRate: sampleRate,
	}
	f.recalc()

	return f, nil
}

// recalc recomputes the derived coefficients from the current parameters.
func (f *Filter) recalc() {
	g := 2 * f.thermalV * (1 - math.Exp(-2*math.Pi*f.cutoff/f.sampleRate))
	if f.variant == ImprovedClassic {
		g *= 2 * f.thermalV
	}

	f.coeff = g
	f.vtInv = 1 / f.thermalV

	amp := math.Pow(10, f.resonance/20) // dB_to_Amp(resonance)
	f.scaleFactor = f.gain * amp * amp
}

// ProcessSample filters one input sample and returns the output.
func (f *Filter) ProcessSample(x float64) float64 {
	tanhFn := math.Tanh
	if f.fastTanh {
		tanhFn = fastTanh
	}

	in := x - f.resonance*f.s[3]

	f.s[0] += f.coeff * (tanhFn(0.5*in*f.vtInv) - f.t[0])
	f.t[0] = tanhFn(0.5 * f.s[0] * f.vtInv)

	f.s[1] += f.coeff * (f.t[0] - f.t[1])
	f.t[1] = tanhFn(0.5 * f.s[1] * f.vtInv)

	f.s[2] += f.coeff * (f.t[1] - f.t[2])
	f.t[2] = tanhFn(0.5 * f.s[2] * f.vtInv)

	f.s[3] += f.coeff * (f.t[2] - tanhFn(0.5*f.s[3]*f.vtInv))

	return f.scaleFactor * f.s[3]
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

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
