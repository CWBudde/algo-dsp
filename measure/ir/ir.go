package ir

import (
	"errors"
	"math"
)

// Errors returned by IR analysis functions.
var (
	ErrEmptyIR           = errors.New("ir: impulse response is empty")
	ErrInvalidSampleRate = errors.New("ir: sample rate must be positive")
	ErrInvalidTime       = errors.New("ir: time must be positive")
	ErrNoDecay           = errors.New("ir: insufficient decay for RT calculation")
)

// Metrics holds impulse response analysis results.
type Metrics struct {
	RT60       float64 // reverberation time in seconds (extrapolated from T30 or T20)
	EDT        float64 // early decay time in seconds (0 to -10 dB)
	T20        float64 // RT from -5 to -25 dB slope
	T30        float64 // RT from -5 to -35 dB slope
	C50        float64 // clarity at 50ms in dB
	C80        float64 // clarity at 80ms in dB
	D50        float64 // definition at 50ms (ratio 0-1)
	D80        float64 // definition at 80ms (ratio 0-1)
	CenterTime float64 // energy centroid in seconds
	PeakIndex  int     // sample index of IR peak (absolute maximum)
}

// Analyzer computes IR metrics from impulse response data.
type Analyzer struct {
	SampleRate float64
}

// NewAnalyzer creates an IR analyzer with the given sample rate.
func NewAnalyzer(sampleRate float64) *Analyzer {
	return &Analyzer{SampleRate: sampleRate}
}

// Analyze computes all IR metrics from an impulse response.
// The IR should start near the direct sound arrival.
func (a *Analyzer) Analyze(ir []float64) (Metrics, error) {
	if len(ir) == 0 {
		return Metrics{}, ErrEmptyIR
	}

	if a.SampleRate <= 0 {
		return Metrics{}, ErrInvalidSampleRate
	}

	peakIdx := a.findPeak(ir)

	// Compute metrics starting from the peak
	irFromPeak := ir[peakIdx:]

	schroeder := a.schroederIntegral(irFromPeak)

	m := Metrics{
		PeakIndex:  peakIdx,
		CenterTime: a.centerTime(irFromPeak),
		D50:        a.definition(irFromPeak, 50),
		D80:        a.definition(irFromPeak, 80),
		C50:        a.clarity(irFromPeak, 50),
		C80:        a.clarity(irFromPeak, 80),
	}

	// EDT: extrapolate from 0 to -10 dB slope
	m.EDT = a.reverbTime(schroeder, 0, -10)

	// T20: extrapolate from -5 to -25 dB
	m.T20 = a.reverbTime(schroeder, -5, -25)

	// T30: extrapolate from -5 to -35 dB
	m.T30 = a.reverbTime(schroeder, -5, -35)

	// RT60: prefer T30 (more robust), fall back to T20
	if m.T30 > 0 {
		m.RT60 = m.T30
	} else {
		m.RT60 = m.T20
	}

	return m, nil
}

// SchroederIntegral computes the Schroeder backward integration of the
// squared impulse response, returned in dB.
//
// S(t) = 10*log10( ∫_t^∞ h²(τ) dτ / ∫_0^∞ h²(τ) dτ )
//
// This converts the noisy IR energy decay into a smooth decay curve
// suitable for reverberation time estimation.
func (a *Analyzer) SchroederIntegral(ir []float64) ([]float64, error) {
	if len(ir) == 0 {
		return nil, ErrEmptyIR
	}

	return a.schroederIntegral(ir), nil
}

// schroederIntegral computes the Schroeder integral (unchecked).
func (a *Analyzer) schroederIntegral(ir []float64) []float64 {
	n := len(ir)
	result := make([]float64, n)

	// Backward cumulative sum of squared IR
	var cumSum float64
	for i := n - 1; i >= 0; i-- {
		cumSum += ir[i] * ir[i]
		result[i] = cumSum
	}

	// Normalize and convert to dB
	totalEnergy := result[0]
	if totalEnergy <= 0 {
		return result
	}

	for i := range result {
		ratio := result[i] / totalEnergy
		if ratio <= 0 {
			result[i] = -200 // floor at -200 dB
		} else {
			result[i] = 10 * math.Log10(ratio)
		}
	}

	return result
}

// reverbTime calculates reverberation time by linear regression on the
// Schroeder curve between startDB and endDB, extrapolated to -60 dB.
//
// For EDT: startDB=0, endDB=-10 → extrapolate by factor 6
// For T20: startDB=-5, endDB=-25 → extrapolate by factor 3
// For T30: startDB=-5, endDB=-35 → extrapolate by factor 2
func (a *Analyzer) reverbTime(schroeder []float64, startDB, endDB float64) float64 {
	if len(schroeder) == 0 || a.SampleRate <= 0 {
		return 0
	}

	// Find the sample indices corresponding to startDB and endDB
	startIdx := -1
	endIdx := -1

	for i, v := range schroeder {
		if startIdx < 0 && v <= startDB {
			startIdx = i
		}

		if startIdx >= 0 && v <= endDB {
			endIdx = i
			break
		}
	}

	if startIdx < 0 || endIdx < 0 || endIdx <= startIdx {
		return 0
	}

	// Linear regression on the Schroeder curve in the [startDB, endDB] range
	// y = dB values, x = sample indices
	n := endIdx - startIdx + 1
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXX, sumXY float64

	for i := startIdx; i <= endIdx; i++ {
		x := float64(i - startIdx)
		y := schroeder[i]
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
	}

	nf := float64(n)

	denom := nf*sumXX - sumX*sumX
	if denom == 0 {
		return 0
	}

	// Slope in dB/sample
	slope := (nf*sumXY - sumX*sumY) / denom

	if slope >= 0 {
		return 0 // no decay
	}

	// Convert slope to dB/second
	slopePerSec := slope * a.SampleRate

	// Extrapolate to -60 dB: RT = -60 / slope_per_sec
	rt := -60.0 / slopePerSec

	if rt < 0 {
		return 0
	}

	return rt
}

// Definition computes the definition D(t) at a given time boundary in ms.
//
//	D(t) = ∫₀ᵗ h²(τ)dτ / ∫₀^∞ h²(τ)dτ
//
// Returns a ratio between 0 and 1.
func (a *Analyzer) Definition(ir []float64, timeMs float64) (float64, error) {
	if len(ir) == 0 {
		return 0, ErrEmptyIR
	}

	if a.SampleRate <= 0 {
		return 0, ErrInvalidSampleRate
	}

	if timeMs <= 0 {
		return 0, ErrInvalidTime
	}

	return a.definition(ir, timeMs), nil
}

// definition computes D(t) (unchecked).
func (a *Analyzer) definition(ir []float64, timeMs float64) float64 {
	boundarySample := int(math.Round(timeMs * 0.001 * a.SampleRate))
	if boundarySample <= 0 {
		return 0
	}

	if boundarySample >= len(ir) {
		return 1
	}

	var earlyEnergy, totalEnergy float64

	for i, v := range ir {
		e := v * v

		totalEnergy += e
		if i < boundarySample {
			earlyEnergy += e
		}
	}

	if totalEnergy <= 0 {
		return 0
	}

	return earlyEnergy / totalEnergy
}

// Clarity computes the clarity C(t) at a given time boundary in ms.
//
//	C(t) = 10*log10( ∫₀ᵗ h²(τ)dτ / ∫ₜ^∞ h²(τ)dτ )
//
// Returns the value in dB.
func (a *Analyzer) Clarity(ir []float64, timeMs float64) (float64, error) {
	if len(ir) == 0 {
		return 0, ErrEmptyIR
	}

	if a.SampleRate <= 0 {
		return 0, ErrInvalidSampleRate
	}

	if timeMs <= 0 {
		return 0, ErrInvalidTime
	}

	return a.clarity(ir, timeMs), nil
}

// clarity computes C(t) (unchecked).
func (a *Analyzer) clarity(ir []float64, timeMs float64) float64 {
	boundarySample := int(math.Round(timeMs * 0.001 * a.SampleRate))
	if boundarySample <= 0 {
		return math.Inf(-1)
	}

	if boundarySample >= len(ir) {
		return math.Inf(1)
	}

	var earlyEnergy, lateEnergy float64

	for i, v := range ir {
		e := v * v
		if i < boundarySample {
			earlyEnergy += e
		} else {
			lateEnergy += e
		}
	}

	if lateEnergy <= 0 {
		return math.Inf(1)
	}

	if earlyEnergy <= 0 {
		return math.Inf(-1)
	}

	return 10 * math.Log10(earlyEnergy/lateEnergy)
}

// CenterTime computes the temporal energy centroid of the impulse response.
//
//	Ts = ∫₀^∞ τ·h²(τ)dτ / ∫₀^∞ h²(τ)dτ
//
// Returns the center time in seconds.
func (a *Analyzer) CenterTime(ir []float64) (float64, error) {
	if len(ir) == 0 {
		return 0, ErrEmptyIR
	}

	if a.SampleRate <= 0 {
		return 0, ErrInvalidSampleRate
	}

	return a.centerTime(ir), nil
}

// centerTime computes Ts (unchecked).
func (a *Analyzer) centerTime(ir []float64) float64 {
	var numerator, denominator float64

	for i, v := range ir {
		e := v * v
		t := float64(i) / a.SampleRate
		numerator += t * e
		denominator += e
	}

	if denominator <= 0 {
		return 0
	}

	return numerator / denominator
}

// RT60 computes the reverberation time (time for -60 dB decay).
// Uses T30 extrapolation when possible, falls back to T20.
func (a *Analyzer) RT60(ir []float64) (float64, error) {
	if len(ir) == 0 {
		return 0, ErrEmptyIR
	}

	if a.SampleRate <= 0 {
		return 0, ErrInvalidSampleRate
	}

	schroeder := a.schroederIntegral(ir)

	// Try T30 first
	rt := a.reverbTime(schroeder, -5, -35)
	if rt > 0 {
		return rt, nil
	}

	// Fall back to T20
	rt = a.reverbTime(schroeder, -5, -25)
	if rt > 0 {
		return rt, nil
	}

	return 0, ErrNoDecay
}

// FindImpulseStart finds the index of the first sample that exceeds
// a threshold relative to the peak amplitude.
//
// The default threshold is -20 dB below the peak (0.1 of peak amplitude).
// This is useful for trimming pre-delay silence from recorded IRs.
func (a *Analyzer) FindImpulseStart(ir []float64) (int, error) {
	if len(ir) == 0 {
		return 0, ErrEmptyIR
	}

	return a.findImpulseStart(ir, 0.1), nil
}

// findImpulseStart finds the first sample above threshold*peak.
func (a *Analyzer) findImpulseStart(ir []float64, thresholdRatio float64) int {
	peak := 0.0

	for _, v := range ir {
		av := math.Abs(v)
		if av > peak {
			peak = av
		}
	}

	threshold := peak * thresholdRatio
	for i, v := range ir {
		if math.Abs(v) >= threshold {
			return i
		}
	}

	return 0
}

// findPeak returns the index of the absolute maximum in the IR.
func (a *Analyzer) findPeak(ir []float64) int {
	peakIdx := 0
	peakVal := 0.0

	for i, v := range ir {
		av := math.Abs(v)
		if av > peakVal {
			peakVal = av
			peakIdx = i
		}
	}

	return peakIdx
}
