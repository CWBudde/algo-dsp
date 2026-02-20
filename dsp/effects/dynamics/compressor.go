package dynamics

import (
	"fmt"
	"math"
)

const (
	// Default compressor parameters
	defaultCompressorThresholdDB = -20.0
	defaultCompressorRatio       = 4.0
	defaultCompressorKneeDB      = 6.0
	defaultCompressorAttackMs    = 10.0
	defaultCompressorReleaseMs   = 100.0
	defaultCompressorMakeupDB    = 0.0

	// Parameter validation ranges
	minCompressorRatio     = 1.0
	maxCompressorRatio     = 100.0
	minCompressorAttackMs  = 0.1
	maxCompressorAttackMs  = 1000.0
	minCompressorReleaseMs = 1.0
	maxCompressorReleaseMs = 5000.0
	minCompressorKneeDB    = 0.0
	maxCompressorKneeDB    = 24.0

	// log2Of10Div20 is the conversion factor for dB to log2: log2(10) / 20
	// Used for converting decibel values to log2 domain
	log2Of10Div20 = 0.166096404744
)

// CompressorMetrics holds metering information for visualization and analysis.
type CompressorMetrics struct {
	InputPeak     float64 // Maximum input level since last reset
	OutputPeak    float64 // Maximum output level since last reset
	GainReduction float64 // Minimum gain (maximum reduction) since last reset
}

// Compressor implements a professional-quality soft-knee compressor with
// logarithmic-domain gain calculation for smooth compression curves.
//
// The algorithm uses log2-domain processing for the soft-knee characteristic,
// which provides smooth transition around the threshold. The compressor is
// mono - for stereo processing, instantiate two compressors or implement
// stereo-linking externally.
//
// This implementation is single-threaded and not thread-safe. Parameter
// changes should occur outside audio processing callbacks.
type Compressor struct {
	// User-configurable parameters
	thresholdDB  float64
	ratio        float64
	kneeDB       float64
	attackMs     float64
	releaseMs    float64
	makeupGainDB float64
	autoMakeup   bool

	// Sample rate
	sampleRate float64

	// Envelope follower state
	peakLevel float64

	// Computed coefficients (cached for performance)
	attackCoeff      float64 // Attack time constant
	releaseCoeff     float64 // Release time constant
	thresholdLog2    float64 // Threshold in log2 domain
	kneeWidthLog2    float64 // Width of soft knee in log2 domain (k)
	invKneeWidthLog2 float64 // Reciprocal of knee width (1/k)
	makeupGainLin    float64 // Linear makeup gain

	// Optional metering
	metrics CompressorMetrics
}

// NewCompressor creates a soft-knee compressor with professional defaults.
//
// Sample rate must be positive and finite.
//
// Default parameters:
//   - Threshold: -20 dB
//   - Ratio: 4:1
//   - Knee: 6 dB
//   - Attack: 10 ms
//   - Release: 100 ms
//   - Auto makeup gain: enabled
func NewCompressor(sampleRate float64) (*Compressor, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("compressor sample rate must be positive and finite: %f", sampleRate)
	}

	c := &Compressor{
		thresholdDB:  defaultCompressorThresholdDB,
		ratio:        defaultCompressorRatio,
		kneeDB:       defaultCompressorKneeDB,
		attackMs:     defaultCompressorAttackMs,
		releaseMs:    defaultCompressorReleaseMs,
		makeupGainDB: defaultCompressorMakeupDB,
		autoMakeup:   true,
		sampleRate:   sampleRate,
	}

	c.updateCoefficients()
	return c, nil
}

// SetThreshold sets compression threshold in dB.
// Typical range: -60 to 0 dB. Signals above this level will be compressed.
func (c *Compressor) SetThreshold(dB float64) error {
	if math.IsNaN(dB) || math.IsInf(dB, 0) {
		return fmt.Errorf("compressor threshold must be finite: %f", dB)
	}
	c.thresholdDB = dB
	c.updateCoefficients()
	return nil
}

// SetRatio sets compression ratio.
// Range: 1.0 to 100.0
//   - 1.0 = no compression
//   - 4.0 = 4:1 (musical compression)
//   - 100.0 ≈ limiting
func (c *Compressor) SetRatio(ratio float64) error {
	if ratio < minCompressorRatio || ratio > maxCompressorRatio ||
		math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return fmt.Errorf("compressor ratio must be in [%f, %f]: %f",
			minCompressorRatio, maxCompressorRatio, ratio)
	}
	c.ratio = ratio
	c.updateCoefficients()
	return nil
}

// SetKnee sets soft-knee width in dB.
// Range: 0.0 to 24.0 dB
//   - 0 dB = hard knee (abrupt transition)
//   - 6-12 dB = typical for musical compression (smooth transition)
func (c *Compressor) SetKnee(kneeDB float64) error {
	if kneeDB < minCompressorKneeDB || kneeDB > maxCompressorKneeDB ||
		math.IsNaN(kneeDB) || math.IsInf(kneeDB, 0) {
		return fmt.Errorf("compressor knee must be in [%f, %f]: %f",
			minCompressorKneeDB, maxCompressorKneeDB, kneeDB)
	}
	c.kneeDB = kneeDB
	c.updateCoefficients()
	return nil
}

// SetAttack sets attack time in milliseconds.
// Range: 0.1 to 1000 ms. Faster attack = quicker compression response.
func (c *Compressor) SetAttack(ms float64) error {
	if ms < minCompressorAttackMs || ms > maxCompressorAttackMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("compressor attack must be in [%f, %f]: %f",
			minCompressorAttackMs, maxCompressorAttackMs, ms)
	}
	c.attackMs = ms
	c.updateTimeConstants()
	return nil
}

// SetRelease sets release time in milliseconds.
// Range: 1 to 5000 ms. Slower release = smoother gain recovery.
func (c *Compressor) SetRelease(ms float64) error {
	if ms < minCompressorReleaseMs || ms > maxCompressorReleaseMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("compressor release must be in [%f, %f]: %f",
			minCompressorReleaseMs, maxCompressorReleaseMs, ms)
	}
	c.releaseMs = ms
	c.updateTimeConstants()
	return nil
}

// SetMakeupGain sets manual makeup gain in dB and disables auto makeup.
func (c *Compressor) SetMakeupGain(dB float64) error {
	if math.IsNaN(dB) || math.IsInf(dB, 0) {
		return fmt.Errorf("compressor makeup gain must be finite: %f", dB)
	}
	c.makeupGainDB = dB
	c.autoMakeup = false
	c.updateCoefficients()
	return nil
}

// SetAutoMakeup enables or disables automatic makeup gain calculation.
// When enabled, makeup gain compensates for gain reduction at threshold.
func (c *Compressor) SetAutoMakeup(enable bool) error {
	c.autoMakeup = enable
	c.updateCoefficients()
	return nil
}

// SetSampleRate updates sample rate and recalculates time constants.
func (c *Compressor) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("compressor sample rate must be positive and finite: %f", sampleRate)
	}
	c.sampleRate = sampleRate
	c.updateTimeConstants()
	return nil
}

// Threshold returns the current threshold in dB.
func (c *Compressor) Threshold() float64 { return c.thresholdDB }

// Ratio returns the current compression ratio.
func (c *Compressor) Ratio() float64 { return c.ratio }

// Knee returns the current knee width in dB.
func (c *Compressor) Knee() float64 { return c.kneeDB }

// Attack returns the current attack time in milliseconds.
func (c *Compressor) Attack() float64 { return c.attackMs }

// Release returns the current release time in milliseconds.
func (c *Compressor) Release() float64 { return c.releaseMs }

// MakeupGain returns the current makeup gain in dB.
func (c *Compressor) MakeupGain() float64 { return c.makeupGainDB }

// AutoMakeup returns whether automatic makeup gain is enabled.
func (c *Compressor) AutoMakeup() bool { return c.autoMakeup }

// SampleRate returns the current sample rate in Hz.
func (c *Compressor) SampleRate() float64 { return c.sampleRate }

// ProcessSample processes one sample through the compressor.
func (c *Compressor) ProcessSample(input float64) float64 {
	// Envelope follower (peak detector with attack/release)
	inputLevel := math.Abs(input)

	if inputLevel > c.peakLevel {
		// Attack phase
		c.peakLevel += (inputLevel - c.peakLevel) * c.attackCoeff
	} else {
		// Release phase
		c.peakLevel = inputLevel + (c.peakLevel-inputLevel)*c.releaseCoeff
	}

	// Calculate gain reduction
	gain := c.calculateGain(c.peakLevel)

	// Apply gain and makeup
	output := input * gain * c.makeupGainLin

	// Update metrics
	c.updateMetrics(inputLevel, math.Abs(output), gain)

	return output
}

// ProcessInPlace applies compression to buf in place.
func (c *Compressor) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = c.ProcessSample(buf[i])
	}
}

// CalculateOutputLevel computes the steady-state output level for a given input magnitude.
// This allows visualizing the compression curve.
func (c *Compressor) CalculateOutputLevel(inputMagnitude float64) float64 {
	inputMagnitude = math.Abs(inputMagnitude)
	gain := c.calculateGain(inputMagnitude)
	return inputMagnitude * gain * c.makeupGainLin
}

// Reset clears envelope follower and metrics.
func (c *Compressor) Reset() {
	c.peakLevel = 0
	c.metrics = CompressorMetrics{
		GainReduction: 1.0, // Initialize to no reduction
	}
}

// GetMetrics returns current metering values.
func (c *Compressor) GetMetrics() CompressorMetrics {
	return c.metrics
}

// ResetMetrics clears metering state.
func (c *Compressor) ResetMetrics() {
	c.metrics = CompressorMetrics{
		GainReduction: 1.0, // Initialize to no reduction
	}
}

// updateCoefficients recalculates all internal cached values.
func (c *Compressor) updateCoefficients() {
	// Threshold in log2 domain
	c.thresholdLog2 = c.thresholdDB * log2Of10Div20

	// Knee width in log2 domain (k)
	// This corresponds to FSoftKnee[0] in the reference algorithm
	c.kneeWidthLog2 = c.kneeDB * log2Of10Div20

	// Reciprocal of knee width (1/k)
	// This corresponds to FSoftKnee[1] in the reference algorithm
	if c.kneeDB > 0 {
		c.invKneeWidthLog2 = 1.0 / c.kneeWidthLog2
	} else {
		c.invKneeWidthLog2 = 0
	}

	// Auto makeup gain calculation
	if c.autoMakeup {
		// Compensate for gain reduction at threshold
		// Formula: -threshold * (1 - 1/ratio)
		gainReductionDB := c.thresholdDB * (1.0 - 1.0/c.ratio)
		c.makeupGainDB = -gainReductionDB
	}

	// Convert makeup gain to linear
	c.makeupGainLin = mathPower10(c.makeupGainDB / 20.0)

	c.updateTimeConstants()
}

// updateTimeConstants recalculates attack and release coefficients.
func (c *Compressor) updateTimeConstants() {
	// Attack: 1 - exp(-ln2 / (attack_sec * sample_rate))
	c.attackCoeff = 1.0 - math.Exp(-math.Ln2/(c.attackMs*0.001*c.sampleRate))

	// Release: exp(-ln2 / (release_sec * sample_rate))
	c.releaseCoeff = math.Exp(-math.Ln2 / (c.releaseMs * 0.001 * c.sampleRate))
}

// calculateGain computes gain multiplier using log2-domain soft-knee formula.
// This implements a quadratic smoothing around the threshold using the
// parameters k (kneeWidth) and 1/k.
func (c *Compressor) calculateGain(peakLevel float64) float64 {
	if peakLevel <= 0 {
		return 1.0
	}

	// Convert peak to log2 domain
	peakLog2 := mathLog2(peakLevel)

	// Calculate overshoot relative to threshold
	// Positive overshoot means signal is above threshold
	overshoot := peakLog2 - c.thresholdLog2

	// Check for hard knee case
	if c.kneeDB <= 0 {
		if overshoot <= 0 {
			return 1.0
		}
		// Hard knee: full ratio above threshold
		gainLog2 := -overshoot * (1.0 - 1.0/c.ratio)
		return mathPower2(gainLog2)
	}

	// Soft knee calculation
	halfWidth := c.kneeWidthLog2 * 0.5
	var effectiveOvershoot float64

	if overshoot < -halfWidth {
		// Below soft knee range: no compression
		return 1.0
	} else if overshoot > halfWidth {
		// Above soft knee range: linear compression (standard ratio)
		effectiveOvershoot = overshoot
	} else {
		// Inside soft knee range: quadratic smoothing
		// Formula: (overshoot + w/2)^2 / (2*w)
		//        = (overshoot + halfWidth)^2 * 0.5 * (1/w)
		scratch := overshoot + halfWidth
		effectiveOvershoot = scratch * scratch * 0.5 * c.invKneeWidthLog2
	}

	// Apply compression ratio
	// ratio 1:1 → factor 0 → no compression
	// ratio 4:1 → factor 0.75 → 75% of reduction
	gainLog2 := -effectiveOvershoot * (1.0 - 1.0/c.ratio)

	// Convert back to linear
	return mathPower2(gainLog2)
}

// updateMetrics tracks peak levels and gain reduction.
func (c *Compressor) updateMetrics(inputLevel, outputLevel, gain float64) {
	// Track peak values
	if inputLevel > c.metrics.InputPeak {
		c.metrics.InputPeak = inputLevel
	}
	if outputLevel > c.metrics.OutputPeak {
		c.metrics.OutputPeak = outputLevel
	}
	// Track minimum gain (maximum reduction)
	if c.metrics.GainReduction == 1.0 || gain < c.metrics.GainReduction {
		c.metrics.GainReduction = gain
	}
}
