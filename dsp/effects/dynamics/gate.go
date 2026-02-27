package dynamics

import (
	"fmt"
	"math"
)

const (
	// Default gate parameters.
	defaultGateThresholdDB = -40.0
	defaultGateRatio       = 10.0
	defaultGateKneeDB      = 6.0
	defaultGateAttackMs    = 0.1
	defaultGateHoldMs      = 50.0
	defaultGateReleaseMs   = 100.0
	defaultGateRangeDB     = -80.0
	defaultGateRMSWindowMs = 30.0

	// Gate parameter validation ranges.
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

// Gate implements a soft-knee noise gate with hold support.
type Gate struct {
	// User-configurable parameters
	thresholdDB        float64
	ratio              float64
	kneeDB             float64
	attackMs           float64
	holdMs             float64
	releaseMs          float64
	rangeDB            float64
	topology           DynamicsTopology
	detectorMode       DetectorMode
	rmsWindowMs        float64
	sidechainLowCutHz  float64
	sidechainHighCutHz float64

	// Sample rate
	sampleRate float64

	// Envelope follower state (mirrored from core for test compatibility)
	peakLevel float64

	// Hold counter state
	holdCounter int

	// Computed coefficients (mirrored from core for test compatibility)
	attackCoeff      float64
	releaseCoeff     float64
	thresholdLog2    float64
	kneeWidthLog2    float64
	invKneeWidthLog2 float64
	rangeLin         float64
	holdSamples      int

	// Shared detector architecture
	core *dynamicsCore

	// Optional metering
	metrics GateMetrics
}

// NewGate creates a soft-knee noise gate with professional defaults.
func NewGate(sampleRate float64) (*Gate, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("gate %w", err)
	}

	g := &Gate{
		thresholdDB:        defaultGateThresholdDB,
		ratio:              defaultGateRatio,
		kneeDB:             defaultGateKneeDB,
		attackMs:           defaultGateAttackMs,
		holdMs:             defaultGateHoldMs,
		releaseMs:          defaultGateReleaseMs,
		rangeDB:            defaultGateRangeDB,
		topology:           DynamicsTopologyFeedforward,
		detectorMode:       DetectorModePeak,
		rmsWindowMs:        defaultGateRMSWindowMs,
		sidechainLowCutHz:  0,
		sidechainHighCutHz: 0,
		sampleRate:         sampleRate,
		metrics:            GateMetrics{GainReduction: 1.0},
	}

	core, err := newDynamicsCore(dynamicsCoreConfig{
		sampleRate:         sampleRate,
		topology:           g.topology,
		detectorMode:       g.detectorMode,
		thresholdDB:        g.thresholdDB,
		ratio:              g.ratio,
		kneeDB:             g.kneeDB,
		attackMs:           g.attackMs,
		releaseMs:          g.releaseMs,
		rmsWindowMs:        g.rmsWindowMs,
		autoMakeup:         false,
		manualMakeupGainDB: 0,
		sidechainLowCutHz:  g.sidechainLowCutHz,
		sidechainHighCutHz: g.sidechainHighCutHz,
	})
	if err != nil {
		return nil, fmt.Errorf("gate core init: %w", err)
	}

	g.core = core
	g.updateCoefficients()

	return g, nil
}

// SetThreshold sets the gate threshold in dB.
func (g *Gate) SetThreshold(dB float64) error {
	err := g.core.SetThreshold(dB)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.thresholdDB = dB
	g.syncFromCore()

	return nil
}

// SetRatio sets the expansion ratio.
func (g *Gate) SetRatio(ratio float64) error {
	if ratio < minGateRatio || ratio > maxGateRatio || !isFinite(ratio) {
		return fmt.Errorf("gate ratio must be in [%f, %f]: %f", minGateRatio, maxGateRatio, ratio)
	}

	err := g.core.SetRatio(ratio)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.ratio = ratio
	g.syncFromCore()

	return nil
}

// SetKnee sets the soft-knee width in dB.
func (g *Gate) SetKnee(kneeDB float64) error {
	if kneeDB < minGateKneeDB || kneeDB > maxGateKneeDB || !isFinite(kneeDB) {
		return fmt.Errorf("gate knee must be in [%f, %f]: %f", minGateKneeDB, maxGateKneeDB, kneeDB)
	}

	err := g.core.SetKnee(kneeDB)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.kneeDB = kneeDB
	g.syncFromCore()

	return nil
}

// SetAttack sets the attack time in milliseconds.
func (g *Gate) SetAttack(ms float64) error {
	if ms < minGateAttackMs || ms > maxGateAttackMs || !isFinite(ms) {
		return fmt.Errorf("gate attack must be in [%f, %f]: %f", minGateAttackMs, maxGateAttackMs, ms)
	}

	err := g.core.SetAttack(ms)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.attackMs = ms
	g.updateTimeConstants()

	return nil
}

// SetHold sets hold time in milliseconds.
func (g *Gate) SetHold(ms float64) error {
	if ms < minGateHoldMs || ms > maxGateHoldMs || !isFinite(ms) {
		return fmt.Errorf("gate hold must be in [%f, %f]: %f", minGateHoldMs, maxGateHoldMs, ms)
	}

	g.holdMs = ms
	g.updateTimeConstants()

	return nil
}

// SetRelease sets the release time in milliseconds.
func (g *Gate) SetRelease(ms float64) error {
	if ms < minGateReleaseMs || ms > maxGateReleaseMs || !isFinite(ms) {
		return fmt.Errorf("gate release must be in [%f, %f]: %f", minGateReleaseMs, maxGateReleaseMs, ms)
	}

	err := g.core.SetRelease(ms)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.releaseMs = ms
	g.updateTimeConstants()

	return nil
}

// SetRange sets the maximum attenuation depth in dB.
func (g *Gate) SetRange(dB float64) error {
	if dB < minGateRangeDB || dB > maxGateRangeDB || !isFinite(dB) {
		return fmt.Errorf("gate range must be in [%f, %f]: %f", minGateRangeDB, maxGateRangeDB, dB)
	}

	g.rangeDB = dB
	g.updateCoefficients()

	return nil
}

// SetSampleRate updates sample rate and recalculates time constants.
func (g *Gate) SetSampleRate(sampleRate float64) error {
	err := g.core.SetSampleRate(sampleRate)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.sampleRate = sampleRate
	g.updateTimeConstants()

	return nil
}

// SetTopology selects feedforward or feedback detector topology.
func (g *Gate) SetTopology(topology DynamicsTopology) error {
	err := g.core.SetTopology(topology)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.topology = topology

	return nil
}

// SetDetectorMode selects peak or RMS detector mode.
func (g *Gate) SetDetectorMode(mode DetectorMode) error {
	err := g.core.SetDetectorMode(mode)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.detectorMode = mode

	return nil
}

// SetRMSWindow sets RMS detector window in milliseconds.
func (g *Gate) SetRMSWindow(ms float64) error {
	err := g.core.SetRMSWindow(ms)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.rmsWindowMs = ms

	return nil
}

// SetSidechainLowCut configures detector-only low-cut filter in Hz (0 disables).
func (g *Gate) SetSidechainLowCut(hz float64) error {
	err := g.core.SetSidechainLowCut(hz)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.sidechainLowCutHz = hz

	return nil
}

// SetSidechainHighCut configures detector-only high-cut filter in Hz (0 disables).
func (g *Gate) SetSidechainHighCut(hz float64) error {
	err := g.core.SetSidechainHighCut(hz)
	if err != nil {
		return fmt.Errorf("gate %w", err)
	}

	g.sidechainHighCutHz = hz

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

// Topology returns detector topology.
func (g *Gate) Topology() DynamicsTopology { return g.topology }

// DetectorMode returns current detector mode.
func (g *Gate) DetectorMode() DetectorMode { return g.detectorMode }

// RMSWindow returns RMS detector window in milliseconds.
func (g *Gate) RMSWindow() float64 { return g.rmsWindowMs }

// SidechainLowCut returns detector-only low-cut frequency in Hz.
func (g *Gate) SidechainLowCut() float64 { return g.sidechainLowCutHz }

// SidechainHighCut returns detector-only high-cut frequency in Hz.
func (g *Gate) SidechainHighCut() float64 { return g.sidechainHighCutHz }

// ProcessSample processes one sample through the gate.
func (g *Gate) ProcessSample(input float64) float64 {
	return g.ProcessSampleSidechain(input, input)
}

// ProcessSampleSidechain processes one sample with explicit sidechain.
func (g *Gate) ProcessSampleSidechain(input, sidechain float64) float64 {
	detectorSource := g.core.detectorSource(input, sidechain)
	level := g.core.detectorLevel(detectorSource)
	g.peakLevel = g.core.Envelope()

	gain := g.calculateGain(level)

	if gain >= 1.0 {
		g.holdCounter = g.holdSamples
	} else if g.holdCounter > 0 {
		g.holdCounter--
		gain = 1.0
	}

	if g.topology == DynamicsTopologyFeedback {
		g.core.previousGain = math.Max(gain, minFeedbackGainMemory)
	}

	output := input * gain
	g.updateMetrics(abs(input), abs(output), gain)

	return output
}

// ProcessInPlace applies gating to buf in place.
func (g *Gate) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = g.ProcessSample(buf[i])
	}
}

// CalculateOutputLevel computes steady-state output level for a given input magnitude.
func (g *Gate) CalculateOutputLevel(inputMagnitude float64) float64 {
	inputMagnitude = abs(inputMagnitude)
	gain := g.calculateGain(inputMagnitude)

	return inputMagnitude * gain
}

// Reset clears envelope follower, hold counter, and metrics.
func (g *Gate) Reset() {
	g.core.Reset()
	g.syncFromCore()
	g.holdCounter = 0
	g.metrics = GateMetrics{GainReduction: 1.0}
}

// GetMetrics returns current metering values.
func (g *Gate) GetMetrics() GateMetrics {
	return g.metrics
}

// ResetMetrics clears metering state.
func (g *Gate) ResetMetrics() {
	g.metrics = GateMetrics{GainReduction: 1.0}
}

// updateCoefficients recalculates all internal cached values.
func (g *Gate) updateCoefficients() {
	g.rangeLin = mathPower10(g.rangeDB / 20.0)
	g.syncFromCore()
	g.updateTimeConstants()
}

// updateTimeConstants recalculates attack, release, and hold coefficients.
func (g *Gate) updateTimeConstants() {
	g.syncFromCore()
	g.holdSamples = int(g.holdMs * 0.001 * g.sampleRate)
}

// calculateGain computes the gate gain multiplier using log2-domain soft-knee.
func (g *Gate) calculateGain(peakLevel float64) float64 {
	if peakLevel <= 0 {
		return g.rangeLin
	}

	peakLog2 := mathLog2(peakLevel)
	undershoot := g.thresholdLog2 - peakLog2

	if g.kneeDB <= 0 {
		if undershoot <= 0 {
			return 1.0
		}

		gainLog2 := -undershoot * (g.ratio - 1.0)

		gain := mathPower2(gainLog2)
		if gain < g.rangeLin {
			return g.rangeLin
		}

		return gain
	}

	halfWidth := g.kneeWidthLog2 * 0.5

	var effectiveUndershoot float64

	if undershoot < -halfWidth {
		return 1.0
	} else if undershoot > halfWidth {
		effectiveUndershoot = undershoot
	} else {
		scratch := undershoot + halfWidth
		effectiveUndershoot = scratch * scratch * 0.5 * g.invKneeWidthLog2
	}

	gainLog2 := -effectiveUndershoot * (g.ratio - 1.0)

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

func (g *Gate) syncFromCore() {
	g.attackCoeff = g.core.AttackCoeff()
	g.releaseCoeff = g.core.ReleaseCoeff()
	g.thresholdLog2 = g.core.ThresholdLog2()
	g.kneeWidthLog2 = g.core.KneeWidthLog2()
	g.invKneeWidthLog2 = g.core.invKneeWidthLog2
	g.peakLevel = g.core.Envelope()
}
