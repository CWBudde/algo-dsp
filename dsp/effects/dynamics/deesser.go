//nolint:gocritic
package dynamics

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

// DeEsserMode selects the gain reduction strategy.
type DeEsserMode int

const (
	// DeEsserSplitBand applies gain reduction only to the detected
	// sibilance band, leaving other frequencies untouched. This is the
	// most transparent mode and is recommended for most use cases.
	DeEsserSplitBand DeEsserMode = iota

	// DeEsserWideband applies gain reduction to the entire signal when
	// sibilance is detected. This is simpler but can cause pumping
	// artifacts on non-sibilant content.
	DeEsserWideband
)

// DeEsserDetector selects the filter type used for sibilance detection.
type DeEsserDetector int

const (
	// DeEsserDetectBandpass uses a bandpass filter centered on the
	// detection frequency. Best for targeting a specific sibilance band.
	DeEsserDetectBandpass DeEsserDetector = iota

	// DeEsserDetectHighpass uses a highpass filter at the detection
	// frequency. Detects all energy above the frequency, which can be
	// useful for broadband sibilance.
	DeEsserDetectHighpass
)

const (
	// Default de-esser parameters.
	defaultDeEsserFreqHz      = 6000.0
	defaultDeEsserQ           = 1.5
	defaultDeEsserThreshDB    = -20.0
	defaultDeEsserRatio       = 4.0
	defaultDeEsserKneeDB      = 3.0
	defaultDeEsserAttackMs    = 0.5
	defaultDeEsserReleaseMs   = 20.0
	defaultDeEsserRangeDB     = -24.0
	defaultDeEsserMode        = DeEsserSplitBand
	defaultDeEsserDetector    = DeEsserDetectBandpass
	defaultDeEsserListen      = false
	defaultDeEsserFilterOrder = 2

	// Validation ranges.
	minDeEsserFreqHz      = 1000.0
	maxDeEsserFreqHz      = 20000.0
	minDeEsserQ           = 0.1
	maxDeEsserQ           = 10.0
	minDeEsserRatio       = 1.0
	maxDeEsserRatio       = 100.0
	minDeEsserKneeDB      = 0.0
	maxDeEsserKneeDB      = 12.0
	minDeEsserAttackMs    = 0.01
	maxDeEsserAttackMs    = 50.0
	minDeEsserReleaseMs   = 1.0
	maxDeEsserReleaseMs   = 500.0
	minDeEsserRangeDB     = -60.0
	maxDeEsserRangeDB     = 0.0
	minDeEsserFilterOrder = 1
	maxDeEsserFilterOrder = 4
)

// DeEsserMetrics holds metering information for visualization and analysis.
type DeEsserMetrics struct {
	// DetectionLevel is the peak detected sibilance level since last reset.
	DetectionLevel float64
	// GainReduction is the minimum gain (maximum reduction) since last reset.
	GainReduction float64
}

// DeEsser implements a split-band sibilance detector and reducer.
//
// The de-esser extracts a sibilance band from the input using a configurable
// detection filter (bandpass or highpass), computes the envelope of that band,
// and applies gain reduction when the sibilance energy exceeds the threshold.
//
// Two reduction modes are supported:
//   - Split-band: only the sibilance band is attenuated (transparent, default).
//   - Wideband: the entire signal is attenuated when sibilance is detected.
//
// The gain computer uses the same log2-domain soft-knee algorithm as
// [Compressor], providing smooth and musical de-essing behavior.
//
// A "listen" mode outputs the detection band in isolation, allowing the user
// to monitor exactly what the de-esser is responding to.
//
// This processor is mono, real-time safe, and not thread-safe.
type DeEsser struct {
	// User-configurable parameters.
	freqHz      float64
	q           float64
	thresholdDB float64
	ratio       float64
	kneeDB      float64
	attackMs    float64
	releaseMs   float64
	rangeDB     float64
	mode        DeEsserMode
	detector    DeEsserDetector
	listen      bool
	filterOrder int

	sampleRate float64

	// Detection filters — cascaded for steeper slopes.
	detectFilters []*biquad.Section

	// Split-band reduction filters — matched to detection for band extraction.
	bandFilters []*biquad.Section
	// bandNorm is the reciprocal of the band filters' passband gain so that
	// the extracted band is normalised to unity before recombination.
	bandNorm float64

	// Envelope follower state.
	envLevel float64

	// Cached coefficients.
	attackCoeff      float64
	releaseCoeff     float64
	thresholdLog2    float64
	kneeWidthLog2    float64
	invKneeWidthLog2 float64
	rangeLin         float64

	// Metering.
	metrics DeEsserMetrics
}

// NewDeEsser creates a de-esser with sensible vocal defaults.
//
// Default parameters:
//   - Frequency: 6000 Hz
//   - Q: 1.5
//   - Threshold: -20 dB
//   - Ratio: 4:1
//   - Knee: 3 dB
//   - Attack: 0.5 ms
//   - Release: 20 ms
//   - Range: -24 dB
//   - Mode: split-band
//   - Detector: bandpass
//   - Filter order: 2 (cascaded second-order sections)
func NewDeEsser(sampleRate float64) (*DeEsser, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("de-esser sample rate must be positive and finite: %f", sampleRate)
	}

	d := &DeEsser{
		freqHz:      defaultDeEsserFreqHz,
		q:           defaultDeEsserQ,
		thresholdDB: defaultDeEsserThreshDB,
		ratio:       defaultDeEsserRatio,
		kneeDB:      defaultDeEsserKneeDB,
		attackMs:    defaultDeEsserAttackMs,
		releaseMs:   defaultDeEsserReleaseMs,
		rangeDB:     defaultDeEsserRangeDB,
		mode:        defaultDeEsserMode,
		detector:    defaultDeEsserDetector,
		listen:      defaultDeEsserListen,
		filterOrder: defaultDeEsserFilterOrder,
		sampleRate:  sampleRate,
		metrics:     DeEsserMetrics{GainReduction: 1.0},
	}

	d.updateCoefficients()
	d.rebuildFilters()

	return d, nil
}

// --- Getters ---

// Frequency returns the detection center frequency in Hz.
func (d *DeEsser) Frequency() float64 { return d.freqHz }

// Q returns the detection filter Q factor.
func (d *DeEsser) Q() float64 { return d.q }

// Threshold returns the detection threshold in dB.
func (d *DeEsser) Threshold() float64 { return d.thresholdDB }

// Ratio returns the compression ratio applied to detected sibilance.
func (d *DeEsser) Ratio() float64 { return d.ratio }

// Knee returns the soft-knee width in dB.
func (d *DeEsser) Knee() float64 { return d.kneeDB }

// Attack returns the envelope attack time in milliseconds.
func (d *DeEsser) Attack() float64 { return d.attackMs }

// Release returns the envelope release time in milliseconds.
func (d *DeEsser) Release() float64 { return d.releaseMs }

// Range returns the maximum gain reduction depth in dB.
func (d *DeEsser) Range() float64 { return d.rangeDB }

// Mode returns the current reduction mode (split-band or wideband).
func (d *DeEsser) Mode() DeEsserMode { return d.mode }

// Detector returns the current detection filter type.
func (d *DeEsser) Detector() DeEsserDetector { return d.detector }

// Listen returns whether listen mode is active.
func (d *DeEsser) Listen() bool { return d.listen }

// FilterOrder returns the detection filter order (number of cascaded sections).
func (d *DeEsser) FilterOrder() int { return d.filterOrder }

// SampleRate returns the current sample rate in Hz.
func (d *DeEsser) SampleRate() float64 { return d.sampleRate }

// --- Setters ---

// SetFrequency sets the detection center frequency in Hz.
// Range: [1000, 20000] Hz.
func (d *DeEsser) SetFrequency(hz float64) error {
	if hz < minDeEsserFreqHz || hz > maxDeEsserFreqHz ||
		math.IsNaN(hz) || math.IsInf(hz, 0) {
		return fmt.Errorf("de-esser frequency must be in [%g, %g]: %f",
			minDeEsserFreqHz, maxDeEsserFreqHz, hz)
	}

	if hz >= d.sampleRate/2 {
		return fmt.Errorf("de-esser frequency must be < Nyquist (%g): %f",
			d.sampleRate/2, hz)
	}

	d.freqHz = hz
	d.rebuildFilters()

	return nil
}

// SetQ sets the detection filter Q (bandwidth) factor.
// Range: [0.1, 10]. Lower Q = wider band, higher Q = narrower targeting.
func (d *DeEsser) SetQ(q float64) error {
	if q < minDeEsserQ || q > maxDeEsserQ ||
		math.IsNaN(q) || math.IsInf(q, 0) {
		return fmt.Errorf("de-esser Q must be in [%g, %g]: %f",
			minDeEsserQ, maxDeEsserQ, q)
	}

	d.q = q
	d.rebuildFilters()

	return nil
}

// SetThreshold sets the detection threshold in dB.
// Sibilance must exceed this level to trigger gain reduction.
func (d *DeEsser) SetThreshold(dB float64) error {
	if math.IsNaN(dB) || math.IsInf(dB, 0) {
		return fmt.Errorf("de-esser threshold must be finite: %f", dB)
	}

	d.thresholdDB = dB
	d.updateCoefficients()

	return nil
}

// SetRatio sets the compression ratio applied to detected sibilance.
// Range: [1, 100]. Higher ratio = more aggressive de-essing.
func (d *DeEsser) SetRatio(ratio float64) error {
	if ratio < minDeEsserRatio || ratio > maxDeEsserRatio ||
		math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return fmt.Errorf("de-esser ratio must be in [%g, %g]: %f",
			minDeEsserRatio, maxDeEsserRatio, ratio)
	}

	d.ratio = ratio
	d.updateCoefficients()

	return nil
}

// SetKnee sets the soft-knee width in dB.
// Range: [0, 12]. 0 = hard knee, larger = smoother transition.
func (d *DeEsser) SetKnee(kneeDB float64) error {
	if kneeDB < minDeEsserKneeDB || kneeDB > maxDeEsserKneeDB ||
		math.IsNaN(kneeDB) || math.IsInf(kneeDB, 0) {
		return fmt.Errorf("de-esser knee must be in [%g, %g]: %f",
			minDeEsserKneeDB, maxDeEsserKneeDB, kneeDB)
	}

	d.kneeDB = kneeDB
	d.updateCoefficients()

	return nil
}

// SetAttack sets the envelope attack time in milliseconds.
// Range: [0.01, 50]. Fast attack captures the onset of sibilance quickly.
func (d *DeEsser) SetAttack(ms float64) error {
	if ms < minDeEsserAttackMs || ms > maxDeEsserAttackMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("de-esser attack must be in [%g, %g]: %f",
			minDeEsserAttackMs, maxDeEsserAttackMs, ms)
	}

	d.attackMs = ms
	d.updateTimeConstants()

	return nil
}

// SetRelease sets the envelope release time in milliseconds.
// Range: [1, 500]. Controls how quickly gain returns after sibilance stops.
func (d *DeEsser) SetRelease(ms float64) error {
	if ms < minDeEsserReleaseMs || ms > maxDeEsserReleaseMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("de-esser release must be in [%g, %g]: %f",
			minDeEsserReleaseMs, maxDeEsserReleaseMs, ms)
	}

	d.releaseMs = ms
	d.updateTimeConstants()

	return nil
}

// SetRange sets the maximum gain reduction depth in dB.
// Range: [-60, 0]. Limits how much the de-esser can attenuate.
func (d *DeEsser) SetRange(valDB float64) error {
	if valDB < minDeEsserRangeDB || valDB > maxDeEsserRangeDB ||
		math.IsNaN(valDB) || math.IsInf(valDB, 0) {
		return fmt.Errorf("de-esser range must be in [%g, %g]: %f",
			minDeEsserRangeDB, maxDeEsserRangeDB, valDB)
	}

	d.rangeDB = valDB
	d.updateCoefficients()

	return nil
}

// SetMode sets the reduction mode (split-band or wideband).
func (d *DeEsser) SetMode(mode DeEsserMode) error {
	if mode < DeEsserSplitBand || mode > DeEsserWideband {
		return fmt.Errorf("de-esser mode invalid: %d", mode)
	}

	d.mode = mode

	return nil
}

// SetDetector sets the detection filter type (bandpass or highpass).
func (d *DeEsser) SetDetector(det DeEsserDetector) error {
	if det < DeEsserDetectBandpass || det > DeEsserDetectHighpass {
		return fmt.Errorf("de-esser detector invalid: %d", det)
	}

	d.detector = det
	d.rebuildFilters()

	return nil
}

// SetListen enables or disables listen mode.
// When enabled, the output is the isolated detection band, allowing
// monitoring of what the de-esser is responding to.
func (d *DeEsser) SetListen(listen bool) {
	d.listen = listen
}

// SetFilterOrder sets the detection filter order (cascaded sections).
// Range: [1, 4]. Higher order = steeper filter slopes.
func (d *DeEsser) SetFilterOrder(order int) error {
	if order < minDeEsserFilterOrder || order > maxDeEsserFilterOrder {
		return fmt.Errorf("de-esser filter order must be in [%d, %d]: %d",
			minDeEsserFilterOrder, maxDeEsserFilterOrder, order)
	}

	d.filterOrder = order
	d.rebuildFilters()

	return nil
}

// SetSampleRate updates the sample rate and rebuilds all internal state.
func (d *DeEsser) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("de-esser sample rate must be positive and finite: %f", sampleRate)
	}

	if d.freqHz >= sampleRate/2 {
		return fmt.Errorf("de-esser frequency %g Hz exceeds Nyquist for sample rate %g Hz; lower frequency before changing sample rate",
			d.freqHz, sampleRate)
	}

	d.sampleRate = sampleRate
	d.updateCoefficients()
	d.rebuildFilters()

	return nil
}

// --- Processing ---

// ProcessSample processes one sample through the de-esser.
func (d *DeEsser) ProcessSample(input float64) float64 {
	// Run detection filters to extract sibilance band energy.
	detected := input
	for _, f := range d.detectFilters {
		detected = f.ProcessSample(detected)
	}

	// Listen mode: output the detection band.
	if d.listen {
		return detected
	}

	// Envelope follower on the detected band.
	detLevel := math.Abs(detected)
	if detLevel > d.envLevel {
		d.envLevel += (detLevel - d.envLevel) * d.attackCoeff
	} else {
		d.envLevel = detLevel + (d.envLevel-detLevel)*d.releaseCoeff
	}

	// Calculate gain reduction from the detected level.
	gain := d.calculateGain(d.envLevel)

	// Update metrics.
	if detLevel > d.metrics.DetectionLevel {
		d.metrics.DetectionLevel = detLevel
	}

	if d.metrics.GainReduction == 1.0 || gain < d.metrics.GainReduction {
		d.metrics.GainReduction = gain
	}

	// Apply gain reduction according to mode.
	switch d.mode {
	case DeEsserWideband:
		return input * gain
	case DeEsserSplitBand:
		// Extract sibilance band from the input using the band filters.
		band := input
		for _, f := range d.bandFilters {
			band = f.ProcessSample(band)
		}
		// Normalise the extracted band to unity peak gain so the filter's
		// native passband gain doesn't skew the recombination arithmetic.
		band *= d.bandNorm
		// Reduce only the band, then recombine.
		// output = (input - band) + band * gain
		//        = input + band * (gain - 1)
		out := input + band*(gain-1)
		// The split-band formula can add energy when the band filter has
		// significant phase shift (e.g. highpass at off-centre frequencies).
		// Clamp to the wideband-limited output so we never increase level.
		widebandOut := input * gain
		if math.Abs(out) > math.Abs(widebandOut) {
			return widebandOut
		}

		return out
	default:
		return input * gain
	}
}

// ProcessInPlace applies de-essing to buf in place.
func (d *DeEsser) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = d.ProcessSample(buf[i])
	}
}

// Reset clears all internal filter and envelope state.
func (d *DeEsser) Reset() {
	d.envLevel = 0
	for _, f := range d.detectFilters {
		f.Reset()
	}

	for _, f := range d.bandFilters {
		f.Reset()
	}

	d.metrics = DeEsserMetrics{GainReduction: 1.0}
}

// GetMetrics returns current metering values.
func (d *DeEsser) GetMetrics() DeEsserMetrics {
	return d.metrics
}

// ResetMetrics clears metering state.
func (d *DeEsser) ResetMetrics() {
	d.metrics = DeEsserMetrics{GainReduction: 1.0}
}

// --- Internal ---

// calculateGain computes the gain multiplier using the same log2-domain
// soft-knee compressor algorithm used by [Compressor]. Sibilance energy
// above the threshold is compressed according to the ratio.
func (d *DeEsser) calculateGain(envLevel float64) float64 {
	if envLevel <= 0 {
		return 1.0
	}

	envLog2 := mathLog2(envLevel)
	overshoot := envLog2 - d.thresholdLog2

	if d.kneeDB <= 0 {
		if overshoot <= 0 {
			return 1.0
		}

		gainLog2 := -overshoot * (1.0 - 1.0/d.ratio)

		gain := mathPower2(gainLog2)
		if gain < d.rangeLin {
			return d.rangeLin
		}

		return gain
	}

	halfWidth := d.kneeWidthLog2 * 0.5

	var effectiveOvershoot float64

	if overshoot < -halfWidth {
		return 1.0
	} else if overshoot > halfWidth {
		effectiveOvershoot = overshoot
	} else {
		scratch := overshoot + halfWidth
		effectiveOvershoot = scratch * scratch * 0.5 * d.invKneeWidthLog2
	}

	gainLog2 := -effectiveOvershoot * (1.0 - 1.0/d.ratio)

	gain := mathPower2(gainLog2)
	if gain < d.rangeLin {
		return d.rangeLin
	}

	return gain
}

func (d *DeEsser) updateCoefficients() {
	d.thresholdLog2 = d.thresholdDB * log2Of10Div20

	d.kneeWidthLog2 = d.kneeDB * log2Of10Div20
	if d.kneeDB > 0 {
		d.invKneeWidthLog2 = 1.0 / d.kneeWidthLog2
	} else {
		d.invKneeWidthLog2 = 0
	}

	d.rangeLin = mathPower10(d.rangeDB / 20.0)
	d.updateTimeConstants()
}

func (d *DeEsser) updateTimeConstants() {
	d.attackCoeff = 1.0 - math.Exp(-math.Ln2/(d.attackMs*0.001*d.sampleRate))
	d.releaseCoeff = math.Exp(-math.Ln2 / (d.releaseMs * 0.001 * d.sampleRate))
}

func (d *DeEsser) rebuildFilters() {
	freq := d.freqHz

	// Build detection filter cascade.
	d.detectFilters = make([]*biquad.Section, d.filterOrder)
	for i := range d.detectFilters {
		var coeffs biquad.Coefficients

		switch d.detector {
		case DeEsserDetectBandpass:
			coeffs = design.Bandpass(freq, d.q, d.sampleRate)
		case DeEsserDetectHighpass:
			coeffs = design.Highpass(freq, d.q, d.sampleRate)
		default:
			coeffs = design.Bandpass(freq, d.q, d.sampleRate)
		}

		d.detectFilters[i] = biquad.NewSection(coeffs)
	}

	// Build band extraction filters for split-band mode.
	// Uses the same filter design as detection so band extraction matches.
	d.bandFilters = make([]*biquad.Section, d.filterOrder)
	for i := range d.bandFilters {
		var coeffs biquad.Coefficients

		switch d.detector {
		case DeEsserDetectBandpass:
			coeffs = design.Bandpass(freq, d.q, d.sampleRate)
		case DeEsserDetectHighpass:
			coeffs = design.Highpass(freq, d.q, d.sampleRate)
		default:
			coeffs = design.Bandpass(freq, d.q, d.sampleRate)
		}

		d.bandFilters[i] = biquad.NewSection(coeffs)
	}

	// Compute the normalisation factor for the extracted band so that it has
	// unity peak gain before recombination in split-band mode.
	// Both design.Bandpass and design.Highpass use bilinear-transformed
	// Butterworth/constant-skirt designs whose resonance peak equals Q per
	// section; N cascaded sections give a peak of Q^N.
	d.bandNorm = 1.0 / math.Pow(d.q, float64(d.filterOrder))
}
