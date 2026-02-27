//nolint:funcorder,funlen
package dynamics

import (
	"fmt"
	"math"
)

const (
	minDynamicsRMSTimeMs  = 1.0
	maxDynamicsRMSTimeMs  = 1000.0
	minSidechainCutoffHz  = 1.0
	minFeedbackGainMemory = 1e-9
)

// DynamicsTopology selects where detector control is measured from.
//
//nolint:revive
type DynamicsTopology int

const (
	// DynamicsTopologyFeedforward detects from the input/sidechain path.
	DynamicsTopologyFeedforward DynamicsTopology = iota
	// DynamicsTopologyFeedback detects from the prior output gain path.
	DynamicsTopologyFeedback
)

// DetectorMode controls detector algorithm.
type DetectorMode int

const (
	// DetectorModePeak uses an absolute-value peak follower.
	DetectorModePeak DetectorMode = iota
	// DetectorModeRMS uses a moving RMS detector with ring-buffer state.
	DetectorModeRMS
)

type dynamicsCoreConfig struct {
	sampleRate         float64
	topology           DynamicsTopology
	detectorMode       DetectorMode
	feedbackRatioScale bool
	thresholdDB        float64
	ratio              float64
	kneeDB             float64
	attackMs           float64
	releaseMs          float64
	rmsWindowMs        float64
	autoMakeup         bool
	manualMakeupGainDB float64
	sidechainLowCutHz  float64
	sidechainHighCutHz float64
}

type dynamicsCore struct {
	cfg dynamicsCoreConfig

	// Detector/envelope state
	envelope             float64
	attackCoeff          float64
	releaseCoeff         float64
	feedbackAttackCoeff  float64
	feedbackReleaseCoeff float64

	// RMS detector state (ring buffer of squared control signal)
	rmsWindowSamples int
	rmsSquares       []float64
	rmsIndex         int
	rmsFilled        int
	rmsSum           float64

	// Gain computer cached values
	thresholdLog2    float64
	kneeWidthLog2    float64
	invKneeWidthLog2 float64
	makeupGainDB     float64
	makeupGainLin    float64

	// Feedback topology state
	previousGain      float64
	previousAbsSample float64

	// Optional sidechain detector-only prefilter
	hp onePoleHighPass
	lp onePoleLowPass
}

func newDynamicsCore(cfg dynamicsCoreConfig) (*dynamicsCore, error) {
	c := &dynamicsCore{cfg: cfg}

	err := c.recalculate()
	if err != nil {
		return nil, err
	}

	c.Reset()

	return c, nil
}

func (c *dynamicsCore) SetSampleRate(sampleRate float64) error {
	c.cfg.sampleRate = sampleRate
	return c.recalculate()
}

func (c *dynamicsCore) SetTopology(topology DynamicsTopology) error {
	if topology != DynamicsTopologyFeedforward && topology != DynamicsTopologyFeedback {
		return fmt.Errorf("invalid dynamics topology: %d", topology)
	}

	c.cfg.topology = topology

	return nil
}

func (c *dynamicsCore) SetDetectorMode(mode DetectorMode) error {
	if mode != DetectorModePeak && mode != DetectorModeRMS {
		return fmt.Errorf("invalid detector mode: %d", mode)
	}

	c.cfg.detectorMode = mode

	return nil
}

func (c *dynamicsCore) SetFeedbackRatioScale(enable bool) error {
	c.cfg.feedbackRatioScale = enable
	return c.recalculateDetectorCoefficients()
}

func (c *dynamicsCore) SetThreshold(dB float64) error {
	if !isFinite(dB) {
		return fmt.Errorf("threshold must be finite: %f", dB)
	}

	c.cfg.thresholdDB = dB

	return c.recalculateGainComputer()
}

func (c *dynamicsCore) SetRatio(ratio float64) error {
	if ratio < minCompressorRatio || ratio > maxCompressorRatio || !isFinite(ratio) {
		return fmt.Errorf("ratio must be in [%f, %f]: %f", minCompressorRatio, maxCompressorRatio, ratio)
	}

	c.cfg.ratio = ratio

	err := c.recalculateGainComputer()
	if err != nil {
		return err
	}

	return c.recalculateDetectorCoefficients()
}

func (c *dynamicsCore) SetKnee(kneeDB float64) error {
	if kneeDB < minCompressorKneeDB || kneeDB > maxCompressorKneeDB || !isFinite(kneeDB) {
		return fmt.Errorf("knee must be in [%f, %f]: %f", minCompressorKneeDB, maxCompressorKneeDB, kneeDB)
	}

	c.cfg.kneeDB = kneeDB

	return c.recalculateGainComputer()
}

func (c *dynamicsCore) SetAttack(ms float64) error {
	if ms < minCompressorAttackMs || ms > maxCompressorAttackMs || !isFinite(ms) {
		return fmt.Errorf("attack must be in [%f, %f]: %f", minCompressorAttackMs, maxCompressorAttackMs, ms)
	}

	c.cfg.attackMs = ms

	return c.recalculateDetectorCoefficients()
}

func (c *dynamicsCore) SetRelease(ms float64) error {
	if ms < minCompressorReleaseMs || ms > maxCompressorReleaseMs || !isFinite(ms) {
		return fmt.Errorf("release must be in [%f, %f]: %f", minCompressorReleaseMs, maxCompressorReleaseMs, ms)
	}

	c.cfg.releaseMs = ms

	return c.recalculateDetectorCoefficients()
}

func (c *dynamicsCore) SetRMSWindow(ms float64) error {
	if ms < minDynamicsRMSTimeMs || ms > maxDynamicsRMSTimeMs || !isFinite(ms) {
		return fmt.Errorf("rms window must be in [%f, %f]: %f", minDynamicsRMSTimeMs, maxDynamicsRMSTimeMs, ms)
	}

	c.cfg.rmsWindowMs = ms

	return c.recalculateRMSBuffer()
}

func (c *dynamicsCore) SetAutoMakeup(auto bool) error {
	c.cfg.autoMakeup = auto
	return c.recalculateGainComputer()
}

func (c *dynamicsCore) SetManualMakeupGain(dB float64) error {
	if !isFinite(dB) {
		return fmt.Errorf("manual makeup gain must be finite: %f", dB)
	}

	c.cfg.manualMakeupGainDB = dB
	c.cfg.autoMakeup = false

	return c.recalculateGainComputer()
}

func (c *dynamicsCore) SetSidechainLowCut(hz float64) error {
	if hz < 0 || !isFinite(hz) {
		return fmt.Errorf("sidechain low-cut must be non-negative and finite: %f", hz)
	}

	prev := c.cfg.sidechainLowCutHz

	c.cfg.sidechainLowCutHz = hz

	err := c.recalculatePrefilter()
	if err != nil {
		c.cfg.sidechainLowCutHz = prev
		_ = c.recalculatePrefilter()

		return err
	}

	return nil
}

func (c *dynamicsCore) SetSidechainHighCut(hz float64) error {
	if hz < 0 || !isFinite(hz) {
		return fmt.Errorf("sidechain high-cut must be non-negative and finite: %f", hz)
	}

	prev := c.cfg.sidechainHighCutHz

	c.cfg.sidechainHighCutHz = hz

	err := c.recalculatePrefilter()
	if err != nil {
		c.cfg.sidechainHighCutHz = prev
		_ = c.recalculatePrefilter()

		return err
	}

	return nil
}

func (c *dynamicsCore) Topology() DynamicsTopology { return c.cfg.topology }
func (c *dynamicsCore) DetectorMode() DetectorMode { return c.cfg.detectorMode }
func (c *dynamicsCore) FeedbackRatioScale() bool   { return c.cfg.feedbackRatioScale }
func (c *dynamicsCore) RMSWindowMs() float64       { return c.cfg.rmsWindowMs }
func (c *dynamicsCore) SidechainLowCutHz() float64 { return c.cfg.sidechainLowCutHz }
func (c *dynamicsCore) SidechainHighCutHz() float64 {
	return c.cfg.sidechainHighCutHz
}
func (c *dynamicsCore) MakeupGainDB() float64 { return c.makeupGainDB }
func (c *dynamicsCore) AutoMakeup() bool      { return c.cfg.autoMakeup }

func (c *dynamicsCore) AttackCoeff() float64  { return c.attackCoeff }
func (c *dynamicsCore) ReleaseCoeff() float64 { return c.releaseCoeff }
func (c *dynamicsCore) ThresholdLog2() float64 {
	return c.thresholdLog2
}

func (c *dynamicsCore) KneeWidthLog2() float64 {
	return c.kneeWidthLog2
}
func (c *dynamicsCore) Envelope() float64 { return c.envelope }

func (c *dynamicsCore) ProcessSample(input float64, sidechain float64) (output float64, gain float64) {
	detectorSource := c.detectorSource(input, sidechain)
	level := c.detectorLevel(detectorSource)
	gain = c.GainForLevel(level)

	output = input * gain * c.makeupGainLin
	if c.cfg.topology == DynamicsTopologyFeedback {
		c.previousGain = math.Max(gain, minFeedbackGainMemory)
		c.previousAbsSample = math.Abs(output)
	}

	return output, gain
}

func (c *dynamicsCore) GainForLevel(level float64) float64 {
	if level <= 0 {
		return 1.0
	}

	levelLog2 := mathLog2(level)
	overshoot := levelLog2 - c.thresholdLog2

	compressionFactor := 1.0 - 1.0/c.cfg.ratio
	if c.cfg.topology == DynamicsTopologyFeedback && c.cfg.feedbackRatioScale {
		compressionFactor = c.cfg.ratio - 1.0
	}

	if c.cfg.kneeDB <= 0 {
		if overshoot <= 0 {
			return 1.0
		}

		gainLog2 := -overshoot * compressionFactor

		return mathPower2(gainLog2)
	}

	halfWidth := c.kneeWidthLog2 * 0.5

	var effectiveOvershoot float64

	if overshoot < -halfWidth {
		return 1.0
	}

	if overshoot > halfWidth {
		effectiveOvershoot = overshoot
	} else {
		scratch := overshoot + halfWidth
		effectiveOvershoot = scratch * scratch * 0.5 * c.invKneeWidthLog2
	}

	gainLog2 := -effectiveOvershoot * compressionFactor

	return mathPower2(gainLog2)
}

func (c *dynamicsCore) detectorSource(_, sidechain float64) float64 {
	if c.cfg.topology == DynamicsTopologyFeedback {
		return c.previousAbsSample
	}

	return math.Abs(c.applyPrefilter(sidechain))
}

func (c *dynamicsCore) detectorLevel(source float64) float64 {
	if c.cfg.detectorMode == DetectorModeRMS {
		source = c.updateRMS(source)
	}

	attackCoeff := c.attackCoeff

	releaseCoeff := c.releaseCoeff
	if c.cfg.topology == DynamicsTopologyFeedback && c.cfg.feedbackRatioScale {
		attackCoeff = c.feedbackAttackCoeff
		releaseCoeff = c.feedbackReleaseCoeff
	}

	if source > c.envelope {
		c.envelope += (source - c.envelope) * attackCoeff
	} else {
		c.envelope = source + (c.envelope-source)*releaseCoeff
	}

	return c.envelope
}

func (c *dynamicsCore) updateRMS(source float64) float64 {
	if len(c.rmsSquares) == 0 {
		return source
	}

	square := source * source

	if c.rmsFilled == len(c.rmsSquares) {
		c.rmsSum -= c.rmsSquares[c.rmsIndex]
	} else {
		c.rmsFilled++
	}

	c.rmsSquares[c.rmsIndex] = square
	c.rmsSum += square

	c.rmsIndex++
	if c.rmsIndex >= len(c.rmsSquares) {
		c.rmsIndex = 0
	}

	mean := c.rmsSum / float64(len(c.rmsSquares))
	if mean <= 0 {
		return 0
	}

	return math.Sqrt(mean)
}

func (c *dynamicsCore) applyPrefilter(x float64) float64 {
	if c.lp.enabled {
		x = c.lp.Process(x)
	}

	if c.hp.enabled {
		x = c.hp.Process(x)
	}

	return x
}

//nolint:cyclop
//nolint:funlen
func (c *dynamicsCore) recalculate() error {
	err := validateSampleRate(c.cfg.sampleRate)
	if err != nil {
		return err
	}

	err = c.SetTopology(c.cfg.topology)
	if err != nil {
		return err
	}

	err = c.SetDetectorMode(c.cfg.detectorMode)
	if err != nil {
		return err
	}

	err = c.SetFeedbackRatioScale(c.cfg.feedbackRatioScale)
	if err != nil {
		return err
	}

	err = c.SetThreshold(c.cfg.thresholdDB)
	if err != nil {
		return err
	}

	err = c.SetRatio(c.cfg.ratio)
	if err != nil {
		return err
	}

	err = c.SetKnee(c.cfg.kneeDB)
	if err != nil {
		return err
	}

	err = c.SetAttack(c.cfg.attackMs)
	if err != nil {
		return err
	}

	err = c.SetRelease(c.cfg.releaseMs)
	if err != nil {
		return err
	}

	err = c.SetRMSWindow(c.cfg.rmsWindowMs)
	if err != nil {
		return err
	}

	if c.cfg.autoMakeup {
		err := c.SetAutoMakeup(true)
		if err != nil {
			return err
		}
	} else {
		err := c.SetManualMakeupGain(c.cfg.manualMakeupGainDB)
		if err != nil {
			return err
		}
	}

	err = c.SetSidechainLowCut(c.cfg.sidechainLowCutHz)
	if err != nil {
		return err
	}

	err = c.SetSidechainHighCut(c.cfg.sidechainHighCutHz)
	if err != nil {
		return err
	}

	return nil
}

func (c *dynamicsCore) recalculateDetectorCoefficients() error {
	err := validateSampleRate(c.cfg.sampleRate)
	if err != nil {
		return err
	}

	c.attackCoeff = 1.0 - math.Exp(-math.Ln2/(c.cfg.attackMs*0.001*c.cfg.sampleRate))
	c.releaseCoeff = math.Exp(-math.Ln2 / (c.cfg.releaseMs * 0.001 * c.cfg.sampleRate))

	if c.cfg.feedbackRatioScale {
		c.feedbackAttackCoeff = 1.0 - math.Exp(-math.Ln2/(c.cfg.attackMs*0.001*c.cfg.sampleRate*c.cfg.ratio))
		c.feedbackReleaseCoeff = math.Exp(-math.Ln2 / (c.cfg.releaseMs * 0.001 * c.cfg.sampleRate * c.cfg.ratio))
	} else {
		c.feedbackAttackCoeff = c.attackCoeff
		c.feedbackReleaseCoeff = c.releaseCoeff
	}

	return nil
}

func (c *dynamicsCore) recalculateRMSBuffer() error {
	err := validateSampleRate(c.cfg.sampleRate)
	if err != nil {
		return err
	}

	samples := max(int(math.Round(c.cfg.rmsWindowMs*0.001*c.cfg.sampleRate)), 1)

	if len(c.rmsSquares) != samples {
		c.rmsSquares = make([]float64, samples)
		c.rmsIndex = 0
		c.rmsFilled = 0
		c.rmsSum = 0
	}

	c.rmsWindowSamples = samples

	return nil
}

func (c *dynamicsCore) recalculateGainComputer() error {
	c.thresholdLog2 = c.cfg.thresholdDB * log2Of10Div20

	c.kneeWidthLog2 = c.cfg.kneeDB * log2Of10Div20
	if c.cfg.kneeDB > 0 {
		c.invKneeWidthLog2 = 1.0 / c.kneeWidthLog2
	} else {
		c.invKneeWidthLog2 = 0
	}

	if c.cfg.autoMakeup {
		reductionDB := c.cfg.thresholdDB * (1.0 - 1.0/c.cfg.ratio)
		c.makeupGainDB = -reductionDB
	} else {
		c.makeupGainDB = c.cfg.manualMakeupGainDB
	}

	c.makeupGainLin = mathPower10(c.makeupGainDB / 20.0)

	return nil
}

func (c *dynamicsCore) recalculatePrefilter() error {
	err := validateSampleRate(c.cfg.sampleRate)
	if err != nil {
		return err
	}

	nyquist := c.cfg.sampleRate * 0.5
	if c.cfg.sidechainLowCutHz > 0 {
		if c.cfg.sidechainLowCutHz < minSidechainCutoffHz || c.cfg.sidechainLowCutHz >= nyquist {
			return fmt.Errorf("sidechain low-cut must be in [%f, nyquist): %f", minSidechainCutoffHz, c.cfg.sidechainLowCutHz)
		}
	}

	if c.cfg.sidechainHighCutHz > 0 {
		if c.cfg.sidechainHighCutHz < minSidechainCutoffHz || c.cfg.sidechainHighCutHz >= nyquist {
			return fmt.Errorf("sidechain high-cut must be in [%f, nyquist): %f", minSidechainCutoffHz, c.cfg.sidechainHighCutHz)
		}
	}

	if c.cfg.sidechainLowCutHz > 0 && c.cfg.sidechainHighCutHz > 0 &&
		c.cfg.sidechainLowCutHz >= c.cfg.sidechainHighCutHz {
		return fmt.Errorf("sidechain low-cut must be below high-cut: low=%f high=%f", c.cfg.sidechainLowCutHz, c.cfg.sidechainHighCutHz)
	}

	c.hp.Configure(c.cfg.sidechainLowCutHz, c.cfg.sampleRate)
	c.lp.Configure(c.cfg.sidechainHighCutHz, c.cfg.sampleRate)

	return nil
}

func (c *dynamicsCore) Reset() {
	c.envelope = 0
	c.previousGain = 1.0
	c.previousAbsSample = 0
	c.rmsIndex = 0
	c.rmsFilled = 0

	c.rmsSum = 0
	for i := range c.rmsSquares {
		c.rmsSquares[i] = 0
	}

	c.hp.Reset()
	c.lp.Reset()
}

func validateSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || !isFinite(sampleRate) {
		return fmt.Errorf("sample rate must be positive and finite: %f", sampleRate)
	}

	return nil
}

func isFinite(v float64) bool {
	return !(math.IsNaN(v) || math.IsInf(v, 0))
}

type onePoleLowPass struct {
	enabled bool
	alpha   float64
	state   float64
}

func (f *onePoleLowPass) Configure(cutoffHz, sampleRate float64) {
	if cutoffHz <= 0 {
		f.enabled = false
		f.alpha = 0
		f.state = 0

		return
	}

	f.enabled = true
	f.alpha = 1.0 - math.Exp(-2.0*math.Pi*cutoffHz/sampleRate)
}

func (f *onePoleLowPass) Process(x float64) float64 {
	if !f.enabled {
		return x
	}

	f.state += f.alpha * (x - f.state)

	return f.state
}

func (f *onePoleLowPass) Reset() {
	f.state = 0
}

type onePoleHighPass struct {
	enabled bool
	lp      onePoleLowPass
}

func (f *onePoleHighPass) Configure(cutoffHz, sampleRate float64) {
	if cutoffHz <= 0 {
		f.enabled = false
		f.lp.enabled = false
		f.lp.alpha = 0
		f.lp.state = 0

		return
	}

	f.enabled = true
	f.lp.Configure(cutoffHz, sampleRate)
}

func (f *onePoleHighPass) Process(x float64) float64 {
	if !f.enabled {
		return x
	}

	return x - f.lp.Process(x)
}

func (f *onePoleHighPass) Reset() {
	f.lp.Reset()
}
