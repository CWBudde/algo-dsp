package dynamics

import (
	"fmt"
	"math"
)

const (
	defaultExpanderThresholdDB = -35.0
	defaultExpanderRatio       = 2.0
	defaultExpanderKneeDB      = 6.0
	defaultExpanderAttackMs    = 1.0
	defaultExpanderReleaseMs   = 100.0
	defaultExpanderRangeDB     = -60.0
	defaultExpanderRMSWindowMs = 30.0

	minExpanderRatio     = 1.0
	maxExpanderRatio     = 100.0
	minExpanderAttackMs  = 0.1
	maxExpanderAttackMs  = 1000.0
	minExpanderReleaseMs = 1.0
	maxExpanderReleaseMs = 5000.0
	minExpanderKneeDB    = 0.0
	maxExpanderKneeDB    = 24.0
	minExpanderRangeDB   = -120.0
	maxExpanderRangeDB   = 0.0
)

// ExpanderMetrics holds metering information for visualization and analysis.
type ExpanderMetrics struct {
	InputPeak     float64 // Maximum input level since last reset
	OutputPeak    float64 // Maximum output level since last reset
	GainReduction float64 // Minimum gain (maximum attenuation) since last reset
}

// Expander implements a downward expander with soft-knee support.
//
// Signals below threshold are attenuated by expansion ratio and knee shape,
// with optional attenuation floor via range control.
type Expander struct {
	thresholdDB        float64
	ratio              float64
	kneeDB             float64
	attackMs           float64
	releaseMs          float64
	rangeDB            float64
	topology           DynamicsTopology
	detectorMode       DetectorMode
	rmsWindowMs        float64
	sidechainLowCutHz  float64
	sidechainHighCutHz float64
	sampleRate         float64

	peakLevel        float64
	attackCoeff      float64
	releaseCoeff     float64
	thresholdLog2    float64
	kneeWidthLog2    float64
	invKneeWidthLog2 float64
	rangeLin         float64
	core             *dynamicsCore
	metrics          ExpanderMetrics
}

// NewExpander creates a downward expander with production defaults.
func NewExpander(sampleRate float64) (*Expander, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("expander %w", err)
	}

	e := &Expander{
		thresholdDB:        defaultExpanderThresholdDB,
		ratio:              defaultExpanderRatio,
		kneeDB:             defaultExpanderKneeDB,
		attackMs:           defaultExpanderAttackMs,
		releaseMs:          defaultExpanderReleaseMs,
		rangeDB:            defaultExpanderRangeDB,
		topology:           DynamicsTopologyFeedforward,
		detectorMode:       DetectorModePeak,
		rmsWindowMs:        defaultExpanderRMSWindowMs,
		sidechainLowCutHz:  0,
		sidechainHighCutHz: 0,
		sampleRate:         sampleRate,
		metrics:            ExpanderMetrics{GainReduction: 1.0},
	}

	core, err := newDynamicsCore(dynamicsCoreConfig{
		sampleRate:         sampleRate,
		topology:           e.topology,
		detectorMode:       e.detectorMode,
		thresholdDB:        e.thresholdDB,
		ratio:              e.ratio,
		kneeDB:             e.kneeDB,
		attackMs:           e.attackMs,
		releaseMs:          e.releaseMs,
		rmsWindowMs:        e.rmsWindowMs,
		autoMakeup:         false,
		manualMakeupGainDB: 0,
		sidechainLowCutHz:  e.sidechainLowCutHz,
		sidechainHighCutHz: e.sidechainHighCutHz,
	})
	if err != nil {
		return nil, fmt.Errorf("expander core init: %w", err)
	}

	e.core = core
	e.updateCoefficients()
	return e, nil
}

// SetThreshold sets threshold in dB.
func (e *Expander) SetThreshold(dB float64) error {
	if err := e.core.SetThreshold(dB); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.thresholdDB = dB
	e.syncFromCore()
	return nil
}

// SetRatio sets expansion ratio.
func (e *Expander) SetRatio(ratio float64) error {
	if ratio < minExpanderRatio || ratio > maxExpanderRatio || !isFinite(ratio) {
		return fmt.Errorf("expander ratio must be in [%f, %f]: %f", minExpanderRatio, maxExpanderRatio, ratio)
	}
	if err := e.core.SetRatio(ratio); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.ratio = ratio
	e.syncFromCore()
	return nil
}

// SetKnee sets soft-knee width in dB.
func (e *Expander) SetKnee(kneeDB float64) error {
	if kneeDB < minExpanderKneeDB || kneeDB > maxExpanderKneeDB || !isFinite(kneeDB) {
		return fmt.Errorf("expander knee must be in [%f, %f]: %f", minExpanderKneeDB, maxExpanderKneeDB, kneeDB)
	}
	if err := e.core.SetKnee(kneeDB); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.kneeDB = kneeDB
	e.syncFromCore()
	return nil
}

// SetAttack sets attack time in milliseconds.
func (e *Expander) SetAttack(ms float64) error {
	if ms < minExpanderAttackMs || ms > maxExpanderAttackMs || !isFinite(ms) {
		return fmt.Errorf("expander attack must be in [%f, %f]: %f", minExpanderAttackMs, maxExpanderAttackMs, ms)
	}
	if err := e.core.SetAttack(ms); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.attackMs = ms
	e.syncFromCore()
	return nil
}

// SetRelease sets release time in milliseconds.
func (e *Expander) SetRelease(ms float64) error {
	if ms < minExpanderReleaseMs || ms > maxExpanderReleaseMs || !isFinite(ms) {
		return fmt.Errorf("expander release must be in [%f, %f]: %f", minExpanderReleaseMs, maxExpanderReleaseMs, ms)
	}
	if err := e.core.SetRelease(ms); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.releaseMs = ms
	e.syncFromCore()
	return nil
}

// SetRange sets maximum attenuation in dB.
func (e *Expander) SetRange(dB float64) error {
	if dB < minExpanderRangeDB || dB > maxExpanderRangeDB || !isFinite(dB) {
		return fmt.Errorf("expander range must be in [%f, %f]: %f", minExpanderRangeDB, maxExpanderRangeDB, dB)
	}
	e.rangeDB = dB
	e.rangeLin = mathPower10(dB / 20.0)
	return nil
}

// SetSampleRate updates sample rate.
func (e *Expander) SetSampleRate(sampleRate float64) error {
	if err := e.core.SetSampleRate(sampleRate); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.sampleRate = sampleRate
	e.syncFromCore()
	return nil
}

// SetTopology selects feedforward or feedback detector topology.
func (e *Expander) SetTopology(topology DynamicsTopology) error {
	if err := e.core.SetTopology(topology); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.topology = topology
	return nil
}

// SetDetectorMode selects peak or RMS detector mode.
func (e *Expander) SetDetectorMode(mode DetectorMode) error {
	if err := e.core.SetDetectorMode(mode); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.detectorMode = mode
	return nil
}

// SetRMSWindow sets RMS detector window in milliseconds.
func (e *Expander) SetRMSWindow(ms float64) error {
	if err := e.core.SetRMSWindow(ms); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.rmsWindowMs = ms
	return nil
}

// SetSidechainLowCut configures detector-only low-cut filter in Hz (0 disables).
func (e *Expander) SetSidechainLowCut(hz float64) error {
	if err := e.core.SetSidechainLowCut(hz); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.sidechainLowCutHz = hz
	return nil
}

// SetSidechainHighCut configures detector-only high-cut filter in Hz (0 disables).
func (e *Expander) SetSidechainHighCut(hz float64) error {
	if err := e.core.SetSidechainHighCut(hz); err != nil {
		return fmt.Errorf("expander %w", err)
	}
	e.sidechainHighCutHz = hz
	return nil
}

func (e *Expander) Threshold() float64         { return e.thresholdDB }
func (e *Expander) Ratio() float64             { return e.ratio }
func (e *Expander) Knee() float64              { return e.kneeDB }
func (e *Expander) Attack() float64            { return e.attackMs }
func (e *Expander) Release() float64           { return e.releaseMs }
func (e *Expander) Range() float64             { return e.rangeDB }
func (e *Expander) SampleRate() float64        { return e.sampleRate }
func (e *Expander) Topology() DynamicsTopology { return e.topology }
func (e *Expander) DetectorMode() DetectorMode { return e.detectorMode }
func (e *Expander) RMSWindow() float64         { return e.rmsWindowMs }
func (e *Expander) SidechainLowCut() float64   { return e.sidechainLowCutHz }
func (e *Expander) SidechainHighCut() float64  { return e.sidechainHighCutHz }

// ProcessSample processes one sample using input as both audio and sidechain.
func (e *Expander) ProcessSample(input float64) float64 {
	return e.ProcessSampleSidechain(input, input)
}

// ProcessSampleSidechain processes one sample with explicit sidechain signal.
func (e *Expander) ProcessSampleSidechain(input, sidechain float64) float64 {
	detectorSource := e.core.detectorSource(input, sidechain)
	level := e.core.detectorLevel(detectorSource)
	e.peakLevel = e.core.Envelope()

	gain := e.calculateGain(level)
	if e.topology == DynamicsTopologyFeedback {
		e.core.previousGain = math.Max(gain, minFeedbackGainMemory)
	}

	output := input * gain
	e.updateMetrics(abs(input), abs(output), gain)
	return output
}

// ProcessInPlace applies expansion to buf in place.
func (e *Expander) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = e.ProcessSample(buf[i])
	}
}

// CalculateOutputLevel computes steady-state output level for input magnitude.
func (e *Expander) CalculateOutputLevel(inputMagnitude float64) float64 {
	inputMagnitude = abs(inputMagnitude)
	gain := e.calculateGain(inputMagnitude)
	return inputMagnitude * gain
}

// Reset clears dynamic state and metrics.
func (e *Expander) Reset() {
	e.core.Reset()
	e.syncFromCore()
	e.metrics = ExpanderMetrics{GainReduction: 1.0}
}

// GetMetrics returns current metering values.
func (e *Expander) GetMetrics() ExpanderMetrics {
	return e.metrics
}

// ResetMetrics clears metering state.
func (e *Expander) ResetMetrics() {
	e.metrics = ExpanderMetrics{GainReduction: 1.0}
}

func (e *Expander) calculateGain(level float64) float64 {
	if level <= 0 {
		return e.rangeLin
	}

	levelLog2 := mathLog2(level)
	undershoot := e.thresholdLog2 - levelLog2

	if e.kneeDB <= 0 {
		if undershoot <= 0 {
			return 1.0
		}
		gainLog2 := -undershoot * (e.ratio - 1.0)
		gain := mathPower2(gainLog2)
		if gain < e.rangeLin {
			return e.rangeLin
		}
		return gain
	}

	halfWidth := e.kneeWidthLog2 * 0.5
	var effectiveUndershoot float64
	if undershoot < -halfWidth {
		return 1.0
	}
	if undershoot > halfWidth {
		effectiveUndershoot = undershoot
	} else {
		scratch := undershoot + halfWidth
		effectiveUndershoot = scratch * scratch * 0.5 * e.invKneeWidthLog2
	}

	gainLog2 := -effectiveUndershoot * (e.ratio - 1.0)
	gain := mathPower2(gainLog2)
	if gain < e.rangeLin {
		return e.rangeLin
	}
	return gain
}

func (e *Expander) syncFromCore() {
	e.attackCoeff = e.core.AttackCoeff()
	e.releaseCoeff = e.core.ReleaseCoeff()
	e.thresholdLog2 = e.core.ThresholdLog2()
	e.kneeWidthLog2 = e.core.KneeWidthLog2()
	e.invKneeWidthLog2 = e.core.invKneeWidthLog2
	e.peakLevel = e.core.Envelope()
}

func (e *Expander) updateCoefficients() {
	e.rangeLin = mathPower10(e.rangeDB / 20.0)
	e.syncFromCore()
}

func (e *Expander) updateMetrics(inputLevel, outputLevel, gain float64) {
	if inputLevel > e.metrics.InputPeak {
		e.metrics.InputPeak = inputLevel
	}
	if outputLevel > e.metrics.OutputPeak {
		e.metrics.OutputPeak = outputLevel
	}
	if e.metrics.GainReduction == 1.0 || gain < e.metrics.GainReduction {
		e.metrics.GainReduction = gain
	}
}
