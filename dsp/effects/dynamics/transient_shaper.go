package dynamics

import (
	"fmt"
	"math"
)

const (
	defaultTransientShaperAttackAmount  = 0.0
	defaultTransientShaperSustainAmount = 0.0
	defaultTransientShaperAttackMs      = 10.0
	defaultTransientShaperReleaseMs     = 120.0

	minTransientShaperAmount    = -1.0
	maxTransientShaperAmount    = 1.0
	minTransientShaperAttackMs  = 0.1
	maxTransientShaperAttackMs  = 200.0
	minTransientShaperReleaseMs = 1.0
	maxTransientShaperReleaseMs = 2000.0
)

// TransientShaper emphasizes or attenuates attack and sustain regions by
// splitting envelope movement into rising (attack) and falling (release)
// components.
type TransientShaper struct {
	attackAmount  float64
	sustainAmount float64
	attackMs      float64
	releaseMs     float64
	sampleRate    float64

	envelope     float64
	attackCoeff  float64
	releaseCoeff float64
}

// NewTransientShaper creates a transient shaper with production defaults.
func NewTransientShaper(sampleRate float64) (*TransientShaper, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("transient shaper %w", err)
	}

	t := &TransientShaper{
		attackAmount:  defaultTransientShaperAttackAmount,
		sustainAmount: defaultTransientShaperSustainAmount,
		attackMs:      defaultTransientShaperAttackMs,
		releaseMs:     defaultTransientShaperReleaseMs,
		sampleRate:    sampleRate,
	}
	t.updateCoefficients()

	return t, nil
}

// SetAttackAmount sets transient attack shaping amount in [-1, 1].
func (t *TransientShaper) SetAttackAmount(amount float64) error {
	if amount < minTransientShaperAmount || amount > maxTransientShaperAmount || !isFinite(amount) {
		return fmt.Errorf("transient shaper attack amount must be in [%f, %f]: %f",
			minTransientShaperAmount, maxTransientShaperAmount, amount)
	}

	t.attackAmount = amount

	return nil
}

// SetSustainAmount sets sustain shaping amount in [-1, 1].
func (t *TransientShaper) SetSustainAmount(amount float64) error {
	if amount < minTransientShaperAmount || amount > maxTransientShaperAmount || !isFinite(amount) {
		return fmt.Errorf("transient shaper sustain amount must be in [%f, %f]: %f",
			minTransientShaperAmount, maxTransientShaperAmount, amount)
	}

	t.sustainAmount = amount

	return nil
}

// SetAttack sets attack detector time in milliseconds.
func (t *TransientShaper) SetAttack(ms float64) error {
	if ms < minTransientShaperAttackMs || ms > maxTransientShaperAttackMs || !isFinite(ms) {
		return fmt.Errorf("transient shaper attack must be in [%f, %f]: %f",
			minTransientShaperAttackMs, maxTransientShaperAttackMs, ms)
	}

	t.attackMs = ms
	t.updateCoefficients()

	return nil
}

// SetRelease sets release detector time in milliseconds.
func (t *TransientShaper) SetRelease(ms float64) error {
	if ms < minTransientShaperReleaseMs || ms > maxTransientShaperReleaseMs || !isFinite(ms) {
		return fmt.Errorf("transient shaper release must be in [%f, %f]: %f",
			minTransientShaperReleaseMs, maxTransientShaperReleaseMs, ms)
	}

	t.releaseMs = ms
	t.updateCoefficients()

	return nil
}

// SetSampleRate updates sample rate and detector coefficients.
func (t *TransientShaper) SetSampleRate(sampleRate float64) error {
	if err := validateSampleRate(sampleRate); err != nil {
		return fmt.Errorf("transient shaper %w", err)
	}

	t.sampleRate = sampleRate
	t.updateCoefficients()

	return nil
}

// AttackAmount returns attack shaping amount.
func (t *TransientShaper) AttackAmount() float64 { return t.attackAmount }

// SustainAmount returns sustain shaping amount.
func (t *TransientShaper) SustainAmount() float64 { return t.sustainAmount }

// Attack returns attack detector time in milliseconds.
func (t *TransientShaper) Attack() float64 { return t.attackMs }

// Release returns release detector time in milliseconds.
func (t *TransientShaper) Release() float64 { return t.releaseMs }

// SampleRate returns sample rate in Hz.
func (t *TransientShaper) SampleRate() float64 { return t.sampleRate }

// Reset clears detector state.
func (t *TransientShaper) Reset() {
	t.envelope = 0
}

// ProcessSample processes one sample.
func (t *TransientShaper) ProcessSample(input float64) float64 {
	prevEnv := t.envelope
	x := math.Abs(input)

	coeff := t.releaseCoeff
	if x > prevEnv {
		coeff = t.attackCoeff
	}

	t.envelope = prevEnv + coeff*(x-prevEnv)

	delta := t.envelope - prevEnv
	norm := math.Abs(delta) / (prevEnv + 1e-9)
	if norm > 1 {
		norm = 1
	}

	gain := 1.0
	if delta >= 0 {
		gain += t.attackAmount * norm
	} else {
		gain += t.sustainAmount * norm
	}

	if gain < 0 {
		gain = 0
	}

	return input * gain
}

// ProcessInPlace processes samples in place.
func (t *TransientShaper) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = t.ProcessSample(buf[i])
	}
}

func (t *TransientShaper) updateCoefficients() {
	t.attackCoeff = timeMsToCoeff(t.attackMs, t.sampleRate)
	t.releaseCoeff = timeMsToCoeff(t.releaseMs, t.sampleRate)
}

func timeMsToCoeff(ms, sampleRate float64) float64 {
	seconds := ms / 1000.0
	if seconds <= 0 {
		return 1
	}

	coeff := 1.0 - math.Exp(-1.0/(seconds*sampleRate))
	if coeff < 0 {
		return 0
	}

	if coeff > 1 {
		return 1
	}

	return coeff
}
