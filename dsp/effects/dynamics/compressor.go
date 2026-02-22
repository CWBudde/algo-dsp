package dynamics

import "fmt"

const (
	// Default compressor parameters
	defaultCompressorThresholdDB = -20.0
	defaultCompressorRatio       = 4.0
	defaultCompressorKneeDB      = 6.0
	defaultCompressorAttackMs    = 10.0
	defaultCompressorReleaseMs   = 100.0
	defaultCompressorMakeupDB    = 0.0
	defaultCompressorRMSWindowMs = 30.0

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
	// Used for converting decibel values to log2 domain.
	log2Of10Div20 = 0.166096404744
)

// CompressorMetrics holds metering information for visualization and analysis.
type CompressorMetrics struct {
	InputPeak     float64 // Maximum input level since last reset
	OutputPeak    float64 // Maximum output level since last reset
	GainReduction float64 // Minimum gain (maximum reduction) since last reset
}

// Compressor implements a soft-knee compressor with configurable detector and
// topology while preserving low-allocation streaming usage.
type Compressor struct {
	// User-configurable parameters
	thresholdDB        float64
	ratio              float64
	kneeDB             float64
	attackMs           float64
	releaseMs          float64
	makeupGainDB       float64
	autoMakeup         bool
	topology           DynamicsTopology
	detectorMode       DetectorMode
	feedbackRatioScale bool
	rmsWindowMs        float64
	sidechainLowCutHz  float64
	sidechainHighCutHz float64

	// Sample rate
	sampleRate float64

	// Envelope follower state (mirrored from core for test compatibility)
	peakLevel float64

	// Computed coefficients (mirrored from core for test compatibility)
	attackCoeff      float64
	releaseCoeff     float64
	thresholdLog2    float64
	kneeWidthLog2    float64
	invKneeWidthLog2 float64
	makeupGainLin    float64

	// Shared dynamics architecture
	core *dynamicsCore

	// Optional metering
	metrics CompressorMetrics
}

// NewCompressor creates a compressor with professional defaults.
func NewCompressor(sampleRate float64) (*Compressor, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("compressor %w", err)
	}

	c := &Compressor{
		thresholdDB:        defaultCompressorThresholdDB,
		ratio:              defaultCompressorRatio,
		kneeDB:             defaultCompressorKneeDB,
		attackMs:           defaultCompressorAttackMs,
		releaseMs:          defaultCompressorReleaseMs,
		makeupGainDB:       defaultCompressorMakeupDB,
		autoMakeup:         true,
		topology:           DynamicsTopologyFeedforward,
		detectorMode:       DetectorModePeak,
		feedbackRatioScale: true,
		rmsWindowMs:        defaultCompressorRMSWindowMs,
		sidechainLowCutHz:  0,
		sidechainHighCutHz: 0,
		sampleRate:         sampleRate,
		metrics:            CompressorMetrics{GainReduction: 1.0},
	}

	cfg := dynamicsCoreConfig{
		sampleRate:         sampleRate,
		topology:           c.topology,
		detectorMode:       c.detectorMode,
		feedbackRatioScale: c.feedbackRatioScale,
		thresholdDB:        c.thresholdDB,
		ratio:              c.ratio,
		kneeDB:             c.kneeDB,
		attackMs:           c.attackMs,
		releaseMs:          c.releaseMs,
		rmsWindowMs:        c.rmsWindowMs,
		autoMakeup:         c.autoMakeup,
		manualMakeupGainDB: c.makeupGainDB,
		sidechainLowCutHz:  c.sidechainLowCutHz,
		sidechainHighCutHz: c.sidechainHighCutHz,
	}

	core, err := newDynamicsCore(cfg)
	if err != nil {
		return nil, fmt.Errorf("compressor core init: %w", err)
	}

	c.core = core
	c.syncFromCore()

	return c, nil
}

// SetThreshold sets compression threshold in dB.
func (c *Compressor) SetThreshold(dB float64) error {
	if err := c.core.SetThreshold(dB); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.thresholdDB = dB
	c.syncFromCore()

	return nil
}

// SetRatio sets compression ratio.
func (c *Compressor) SetRatio(ratio float64) error {
	if err := c.core.SetRatio(ratio); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.ratio = ratio
	c.syncFromCore()

	return nil
}

// SetKnee sets soft-knee width in dB.
func (c *Compressor) SetKnee(kneeDB float64) error {
	if err := c.core.SetKnee(kneeDB); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.kneeDB = kneeDB
	c.syncFromCore()

	return nil
}

// SetAttack sets attack time in milliseconds.
func (c *Compressor) SetAttack(ms float64) error {
	if err := c.core.SetAttack(ms); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.attackMs = ms
	c.syncFromCore()

	return nil
}

// SetRelease sets release time in milliseconds.
func (c *Compressor) SetRelease(ms float64) error {
	if err := c.core.SetRelease(ms); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.releaseMs = ms
	c.syncFromCore()

	return nil
}

// SetMakeupGain sets manual makeup gain in dB and disables auto makeup.
func (c *Compressor) SetMakeupGain(dB float64) error {
	if err := c.core.SetManualMakeupGain(dB); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.makeupGainDB = dB
	c.autoMakeup = false
	c.syncFromCore()

	return nil
}

// SetAutoMakeup enables or disables automatic makeup gain.
func (c *Compressor) SetAutoMakeup(enable bool) error {
	if err := c.core.SetAutoMakeup(enable); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.autoMakeup = enable
	c.syncFromCore()

	return nil
}

// SetSampleRate updates sample rate and recalculates coefficients.
func (c *Compressor) SetSampleRate(sampleRate float64) error {
	if err := c.core.SetSampleRate(sampleRate); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.sampleRate = sampleRate
	c.syncFromCore()

	return nil
}

// SetTopology selects feedforward or feedback detector topology.
func (c *Compressor) SetTopology(topology DynamicsTopology) error {
	if err := c.core.SetTopology(topology); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.topology = topology

	return nil
}

// SetDetectorMode selects peak or RMS detector mode.
func (c *Compressor) SetDetectorMode(mode DetectorMode) error {
	if err := c.core.SetDetectorMode(mode); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.detectorMode = mode

	return nil
}

// SetFeedbackRatioScale controls legacy feedback ratio-dependent time scaling.
func (c *Compressor) SetFeedbackRatioScale(enable bool) error {
	if err := c.core.SetFeedbackRatioScale(enable); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.feedbackRatioScale = enable
	c.syncFromCore()

	return nil
}

// SetRMSWindow sets RMS detector window length in milliseconds.
func (c *Compressor) SetRMSWindow(ms float64) error {
	if err := c.core.SetRMSWindow(ms); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.rmsWindowMs = ms

	return nil
}

// SetSidechainLowCut configures detector-only low-cut filter in Hz (0 disables).
func (c *Compressor) SetSidechainLowCut(hz float64) error {
	if err := c.core.SetSidechainLowCut(hz); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.sidechainLowCutHz = hz

	return nil
}

// SetSidechainHighCut configures detector-only high-cut filter in Hz (0 disables).
func (c *Compressor) SetSidechainHighCut(hz float64) error {
	if err := c.core.SetSidechainHighCut(hz); err != nil {
		return fmt.Errorf("compressor %w", err)
	}

	c.sidechainHighCutHz = hz

	return nil
}

// Threshold returns threshold in dB.
func (c *Compressor) Threshold() float64 { return c.thresholdDB }

// Ratio returns compression ratio.
func (c *Compressor) Ratio() float64 { return c.ratio }

// Knee returns soft-knee width in dB.
func (c *Compressor) Knee() float64 { return c.kneeDB }

// Attack returns attack time in milliseconds.
func (c *Compressor) Attack() float64 { return c.attackMs }

// Release returns release time in milliseconds.
func (c *Compressor) Release() float64 { return c.releaseMs }

// MakeupGain returns current makeup gain in dB.
func (c *Compressor) MakeupGain() float64 { return c.makeupGainDB }

// AutoMakeup returns whether automatic makeup gain is enabled.
func (c *Compressor) AutoMakeup() bool { return c.autoMakeup }

// SampleRate returns sample rate in Hz.
func (c *Compressor) SampleRate() float64 { return c.sampleRate }

// Topology returns detector topology.
func (c *Compressor) Topology() DynamicsTopology { return c.topology }

// DetectorMode returns current detector mode.
func (c *Compressor) DetectorMode() DetectorMode { return c.detectorMode }
func (c *Compressor) FeedbackRatioScale() bool   { return c.feedbackRatioScale }

// RMSWindow returns RMS detector window in milliseconds.
func (c *Compressor) RMSWindow() float64 { return c.rmsWindowMs }

// SidechainLowCut returns detector-only low-cut frequency in Hz.
func (c *Compressor) SidechainLowCut() float64 { return c.sidechainLowCutHz }

// SidechainHighCut returns detector-only high-cut frequency in Hz.
func (c *Compressor) SidechainHighCut() float64 { return c.sidechainHighCutHz }

// ProcessSample processes one sample using input as both audio and sidechain.
func (c *Compressor) ProcessSample(input float64) float64 {
	return c.ProcessSampleSidechain(input, input)
}

// ProcessSampleSidechain processes one sample with explicit sidechain control.
func (c *Compressor) ProcessSampleSidechain(input, sidechain float64) float64 {
	output, gain := c.core.ProcessSample(input, sidechain)
	c.syncFromCore()
	c.updateMetrics(abs(input), abs(output), gain)

	return output
}

// ProcessInPlace applies compression to buf in place.
func (c *Compressor) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = c.ProcessSample(buf[i])
	}
}

// CalculateOutputLevel computes steady-state output level for a given input magnitude.
func (c *Compressor) CalculateOutputLevel(inputMagnitude float64) float64 {
	inputMagnitude = abs(inputMagnitude)
	gain := c.calculateGain(inputMagnitude)

	return inputMagnitude * gain * c.makeupGainLin
}

// Reset clears dynamic detector state and metrics.
func (c *Compressor) Reset() {
	c.core.Reset()
	c.syncFromCore()
	c.metrics = CompressorMetrics{GainReduction: 1.0}
}

// GetMetrics returns current metering values.
func (c *Compressor) GetMetrics() CompressorMetrics {
	return c.metrics
}

// ResetMetrics clears metering state.
func (c *Compressor) ResetMetrics() {
	c.metrics = CompressorMetrics{GainReduction: 1.0}
}

// calculateGain computes static compression gain for a detector level.
func (c *Compressor) calculateGain(level float64) float64 {
	return c.core.GainForLevel(level)
}

func (c *Compressor) syncFromCore() {
	c.attackCoeff = c.core.AttackCoeff()
	c.releaseCoeff = c.core.ReleaseCoeff()
	c.thresholdLog2 = c.core.ThresholdLog2()
	c.kneeWidthLog2 = c.core.KneeWidthLog2()
	c.invKneeWidthLog2 = c.core.invKneeWidthLog2
	c.makeupGainDB = c.core.MakeupGainDB()
	c.makeupGainLin = c.core.makeupGainLin
	c.autoMakeup = c.core.AutoMakeup()
	c.feedbackRatioScale = c.core.FeedbackRatioScale()
	c.peakLevel = c.core.Envelope()
}

func (c *Compressor) updateMetrics(inputLevel, outputLevel, gain float64) {
	if inputLevel > c.metrics.InputPeak {
		c.metrics.InputPeak = inputLevel
	}

	if outputLevel > c.metrics.OutputPeak {
		c.metrics.OutputPeak = outputLevel
	}

	if c.metrics.GainReduction == 1.0 || gain < c.metrics.GainReduction {
		c.metrics.GainReduction = gain
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}

	return v
}
