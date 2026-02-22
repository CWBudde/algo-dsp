package dynamics

import (
	"fmt"
	"math"
)

const (
	// Default gate parameters
	defaultGateThresholdDB = -40.0
	defaultGateRatio       = 10.0
	defaultGateKneeDB      = 6.0
	defaultGateAttackMs    = 0.1
	defaultGateHoldMs      = 50.0
	defaultGateReleaseMs   = 100.0
	defaultGateRangeDB     = -80.0

	// Gate parameter validation ranges
	minGateRatio     = 1.0
	maxGateRatio     = 100.0
	minGateAttackMs  = 0.1
	maxGateAttackMs  = 1000.0
	minGateHoldMs    = 0.0
	maxGateHoldMs    = 5000.0
	minGateReleaseMs = 1.0
	maxGateReleaseMs = 5000.0
	minGateKneeDB    = 0.0
	maxGateKneeDB    = 24.0
	minGateRangeDB   = -120.0
	maxGateRangeDB   = 0.0
)

// GateMetrics holds metering information for visualization and analysis.
type GateMetrics struct {
	InputPeak     float64 // Maximum input level since last reset
	OutputPeak    float64 // Maximum output level since last reset
	GainReduction float64 // Minimum gain (maximum attenuation) since last reset
}

// Gate implements a soft-knee noise gate with log2-domain gain calculation
// for smooth gating curves.
//
// Signals below the threshold are attenuated according to the expansion ratio.
// The soft-knee algorithm uses the same quadratic smoothing as [Compressor],
// providing a smooth transition around the threshold. The gate operates on
// the undershoot (below-threshold) side rather than the overshoot side.
//
// Parameters:
//   - Threshold: level below which gating is applied
//   - Ratio: expansion ratio (1:1 = no gating, higher = more aggressive)
//   - Knee: soft knee width for smooth transition (same range as Compressor)
//   - Attack: gate opening speed (how quickly gain returns to unity)
//   - Hold: minimum time gate stays open after signal drops below threshold
//   - Release: gate closing speed (how quickly attenuation is applied)
//   - Range: maximum attenuation depth in dB
//
// The gate is mono — for stereo processing, instantiate two gates or implement
// stereo-linking externally.
//
// This implementation is single-threaded and not thread-safe. Parameter
// changes should occur outside audio processing callbacks.
type Gate struct {
	// User-configurable parameters
	thresholdDB float64
	ratio       float64
	kneeDB      float64
	attackMs    float64
	holdMs      float64
	releaseMs   float64
	rangeDB     float64

	// Sample rate
	sampleRate float64

	// Envelope follower state
	peakLevel float64

	// Hold counter state
	holdCounter int

	// Computed coefficients (cached for performance)
	attackCoeff      float64 // Attack time constant
	releaseCoeff     float64 // Release time constant
	thresholdLog2    float64 // Threshold in log2 domain
	kneeWidthLog2    float64 // Width of soft knee in log2 domain (k)
	invKneeWidthLog2 float64 // Reciprocal of knee width (1/k)
	rangeLin         float64 // Minimum gain (linear)
	holdSamples      int     // Hold duration in samples

	// Optional metering
	metrics GateMetrics
}

// NewGate creates a soft-knee noise gate with professional defaults.
//
// Sample rate must be positive and finite.
//
// Default parameters:
//   - Threshold: -40 dB
//   - Ratio: 10:1
//   - Knee: 6 dB
//   - Attack: 0.1 ms
//   - Hold: 50 ms
//   - Release: 100 ms
//   - Range: -80 dB
func NewGate(sampleRate float64) (*Gate, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("gate sample rate must be positive and finite: %f", sampleRate)
	}

	g := &Gate{
		thresholdDB: defaultGateThresholdDB,
		ratio:       defaultGateRatio,
		kneeDB:      defaultGateKneeDB,
		attackMs:    defaultGateAttackMs,
		holdMs:      defaultGateHoldMs,
		releaseMs:   defaultGateReleaseMs,
		rangeDB:     defaultGateRangeDB,
		sampleRate:  sampleRate,
		metrics:     GateMetrics{GainReduction: 1.0},
	}

	g.updateCoefficients()

	return g, nil
}

// SetThreshold sets the gate threshold in dB.
// Signals below this level will be attenuated. Typical range: -60 to -20 dB.
func (g *Gate) SetThreshold(dB float64) error {
	if math.IsNaN(dB) || math.IsInf(dB, 0) {
		return fmt.Errorf("gate threshold must be finite: %f", dB)
	}

	g.thresholdDB = dB
	g.updateCoefficients()

	return nil
}

// SetRatio sets the expansion ratio.
// Range: 1.0 to 100.0
//   - 1.0 = no gating
//   - 2.0 = gentle expansion
//   - 10.0 = aggressive gating
//   - 100.0 ≈ hard gate
func (g *Gate) SetRatio(ratio float64) error {
	if ratio < minGateRatio || ratio > maxGateRatio ||
		math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return fmt.Errorf("gate ratio must be in [%f, %f]: %f",
			minGateRatio, maxGateRatio, ratio)
	}

	g.ratio = ratio
	g.updateCoefficients()

	return nil
}

// SetKnee sets the soft-knee width in dB.
// Range: 0.0 to 24.0 dB
//   - 0 dB = hard knee (abrupt transition)
//   - 6-12 dB = smooth transition (same range as Compressor)
func (g *Gate) SetKnee(kneeDB float64) error {
	if kneeDB < minGateKneeDB || kneeDB > maxGateKneeDB ||
		math.IsNaN(kneeDB) || math.IsInf(kneeDB, 0) {
		return fmt.Errorf("gate knee must be in [%f, %f]: %f",
			minGateKneeDB, maxGateKneeDB, kneeDB)
	}

	g.kneeDB = kneeDB
	g.updateCoefficients()

	return nil
}

// SetAttack sets the attack time in milliseconds (gate opening speed).
// Range: 0.1 to 1000 ms. Faster attack = gate opens more quickly.
func (g *Gate) SetAttack(ms float64) error {
	if ms < minGateAttackMs || ms > maxGateAttackMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("gate attack must be in [%f, %f]: %f",
			minGateAttackMs, maxGateAttackMs, ms)
	}

	g.attackMs = ms
	g.updateTimeConstants()

	return nil
}

// SetHold sets the hold time in milliseconds.
// Range: 0 to 5000 ms. The gate stays open for this duration after the
// signal drops below the threshold, preventing rapid on/off chattering.
func (g *Gate) SetHold(ms float64) error {
	if ms < minGateHoldMs || ms > maxGateHoldMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("gate hold must be in [%f, %f]: %f",
			minGateHoldMs, maxGateHoldMs, ms)
	}

	g.holdMs = ms
	g.updateTimeConstants()

	return nil
}

// SetRelease sets the release time in milliseconds (gate closing speed).
// Range: 1 to 5000 ms. Slower release = smoother gate closing.
func (g *Gate) SetRelease(ms float64) error {
	if ms < minGateReleaseMs || ms > maxGateReleaseMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("gate release must be in [%f, %f]: %f",
			minGateReleaseMs, maxGateReleaseMs, ms)
	}

	g.releaseMs = ms
	g.updateTimeConstants()

	return nil
}

// SetRange sets the maximum attenuation depth in dB.
// Range: -120 to 0 dB
//   - -80 dB = near-silence when gate is fully closed (default)
//   - -20 dB = gentle ducking
//   - 0 dB = no attenuation (gate effectively disabled)
func (g *Gate) SetRange(dB float64) error {
	if dB < minGateRangeDB || dB > maxGateRangeDB ||
		math.IsNaN(dB) || math.IsInf(dB, 0) {
		return fmt.Errorf("gate range must be in [%f, %f]: %f",
			minGateRangeDB, maxGateRangeDB, dB)
	}

	g.rangeDB = dB
	g.updateCoefficients()

	return nil
}

// SetSampleRate updates sample rate and recalculates time constants.
func (g *Gate) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("gate sample rate must be positive and finite: %f", sampleRate)
	}

	g.sampleRate = sampleRate
	g.updateTimeConstants()

	return nil
}

// Threshold returns the current threshold in dB.
func (g *Gate) Threshold() float64 { return g.thresholdDB }

// Ratio returns the current expansion ratio.
func (g *Gate) Ratio() float64 { return g.ratio }

// Knee returns the current knee width in dB.
func (g *Gate) Knee() float64 { return g.kneeDB }

// Attack returns the current attack time in milliseconds.
func (g *Gate) Attack() float64 { return g.attackMs }

// Hold returns the current hold time in milliseconds.
func (g *Gate) Hold() float64 { return g.holdMs }

// Release returns the current release time in milliseconds.
func (g *Gate) Release() float64 { return g.releaseMs }

// Range returns the current maximum attenuation in dB.
func (g *Gate) Range() float64 { return g.rangeDB }

// SampleRate returns the current sample rate in Hz.
func (g *Gate) SampleRate() float64 { return g.sampleRate }

// ProcessSample processes one sample through the gate.
func (g *Gate) ProcessSample(input float64) float64 {
	// Envelope follower (peak detector with attack/release)
	inputLevel := math.Abs(input)

	if inputLevel > g.peakLevel {
		// Attack phase: gate opening
		g.peakLevel += (inputLevel - g.peakLevel) * g.attackCoeff
	} else {
		// Release phase: gate closing
		g.peakLevel = inputLevel + (g.peakLevel-inputLevel)*g.releaseCoeff
	}

	// Calculate gain from static curve
	gain := g.calculateGain(g.peakLevel)

	// Hold mechanism: keep gate open during hold period
	if gain >= 1.0 {
		// Gate is fully open — reset hold counter
		g.holdCounter = g.holdSamples
	} else if g.holdCounter > 0 {
		// Signal dropped below threshold but hold is active
		g.holdCounter--
		gain = 1.0
	}

	// Apply gain
	output := input * gain

	// Update metrics
	g.updateMetrics(inputLevel, math.Abs(output), gain)

	return output
}

// ProcessInPlace applies gating to buf in place.
func (g *Gate) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = g.ProcessSample(buf[i])
	}
}

// CalculateOutputLevel computes the steady-state output level for a given
// input magnitude. This allows visualizing the gating curve without
// envelope or hold dynamics.
func (g *Gate) CalculateOutputLevel(inputMagnitude float64) float64 {
	inputMagnitude = math.Abs(inputMagnitude)
	gain := g.calculateGain(inputMagnitude)

	return inputMagnitude * gain
}

// Reset clears envelope follower, hold counter, and metrics.
func (g *Gate) Reset() {
	g.peakLevel = 0
	g.holdCounter = 0
	g.metrics = GateMetrics{
		GainReduction: 1.0, // Initialize to no reduction
	}
}

// GetMetrics returns current metering values.
func (g *Gate) GetMetrics() GateMetrics {
	return g.metrics
}

// ResetMetrics clears metering state.
func (g *Gate) ResetMetrics() {
	g.metrics = GateMetrics{
		GainReduction: 1.0, // Initialize to no reduction
	}
}

// updateCoefficients recalculates all internal cached values.
func (g *Gate) updateCoefficients() {
	// Threshold in log2 domain
	g.thresholdLog2 = g.thresholdDB * log2Of10Div20

	// Knee width in log2 domain
	g.kneeWidthLog2 = g.kneeDB * log2Of10Div20

	// Reciprocal of knee width
	if g.kneeDB > 0 {
		g.invKneeWidthLog2 = 1.0 / g.kneeWidthLog2
	} else {
		g.invKneeWidthLog2 = 0
	}

	// Range: convert dB to linear minimum gain
	g.rangeLin = mathPower10(g.rangeDB / 20.0)

	g.updateTimeConstants()
}

// updateTimeConstants recalculates attack, release, and hold coefficients.
func (g *Gate) updateTimeConstants() {
	// Attack: 1 - exp(-ln2 / (attack_sec * sample_rate))
	g.attackCoeff = 1.0 - math.Exp(-math.Ln2/(g.attackMs*0.001*g.sampleRate))

	// Release: exp(-ln2 / (release_sec * sample_rate))
	g.releaseCoeff = math.Exp(-math.Ln2 / (g.releaseMs * 0.001 * g.sampleRate))

	// Hold duration in samples
	g.holdSamples = int(g.holdMs * 0.001 * g.sampleRate)
}

// calculateGain computes the gate gain multiplier using log2-domain soft-knee.
//
// This mirrors the [Compressor.calculateGain] algorithm but operates on the
// undershoot side: signals below threshold are attenuated by the expansion
// ratio, with the same quadratic smoothing for the soft knee.
func (g *Gate) calculateGain(peakLevel float64) float64 {
	if peakLevel <= 0 {
		return g.rangeLin
	}

	// Convert peak to log2 domain
	peakLog2 := mathLog2(peakLevel)

	// Undershoot: how far below threshold (positive = below)
	undershoot := g.thresholdLog2 - peakLog2

	// Hard knee case
	if g.kneeDB <= 0 {
		if undershoot <= 0 {
			return 1.0 // At or above threshold
		}
		// Below threshold: apply expansion
		gainLog2 := -undershoot * (g.ratio - 1.0)

		gain := mathPower2(gainLog2)
		if gain < g.rangeLin {
			return g.rangeLin
		}

		return gain
	}

	// Soft knee calculation
	halfWidth := g.kneeWidthLog2 * 0.5

	var effectiveUndershoot float64

	if undershoot < -halfWidth {
		// Above soft knee range: no gating
		return 1.0
	} else if undershoot > halfWidth {
		// Below soft knee range: full expansion
		effectiveUndershoot = undershoot
	} else {
		// Inside soft knee range: quadratic smoothing
		// Same formula as Compressor but on the undershoot side:
		// (undershoot + halfWidth)^2 / (2 * kneeWidth)
		scratch := undershoot + halfWidth
		effectiveUndershoot = scratch * scratch * 0.5 * g.invKneeWidthLog2
	}

	// Apply expansion ratio
	// ratio 1:1 → factor 0 → no gating
	// ratio 2:1 → factor 1 → 1 dB attenuation per dB below threshold
	// ratio 10:1 → factor 9 → aggressive gating
	gainLog2 := -effectiveUndershoot * (g.ratio - 1.0)

	// Convert back to linear, clamping to range
	gain := mathPower2(gainLog2)
	if gain < g.rangeLin {
		return g.rangeLin
	}

	return gain
}

// updateMetrics tracks peak levels and gain reduction.
func (g *Gate) updateMetrics(inputLevel, outputLevel, gain float64) {
	if inputLevel > g.metrics.InputPeak {
		g.metrics.InputPeak = inputLevel
	}

	if outputLevel > g.metrics.OutputPeak {
		g.metrics.OutputPeak = outputLevel
	}

	if g.metrics.GainReduction == 1.0 || gain < g.metrics.GainReduction {
		g.metrics.GainReduction = gain
	}
}
