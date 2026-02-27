package spatial

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	defaultCancellerListenerDistance = 1.0
	defaultCancellerSpeakerDistance  = 2.0
	defaultCancellerHeadRadius       = 0.0875
	defaultCancellerAttenuation      = 0.65
	defaultCancellerStages           = 2
	defaultCancellerSpeedOfSound     = 343.0
	defaultCancellerShelfFreq        = 1800.0
	defaultCancellerShelfGainDB      = 3.0
	defaultCancellerShelfQ           = 0.7071067811865476

	minCancellerListenerDistance = 0.1
	minCancellerSpeakerDistance  = 0.15
	minCancellerHeadRadius       = 0.03
	maxCancellerHeadRadius       = 0.2
	minCancellerSpeedOfSound     = 300.0
	maxCancellerSpeedOfSound     = 370.0
	maxCancellerStages           = 8
)

// CrosstalkCancellerOption mutates construction-time parameters.
type CrosstalkCancellerOption func(*crosstalkCancellerConfig) error

type crosstalkCancellerConfig struct {
	listenerDistance float64
	speakerDistance  float64
	headRadius       float64
	attenuation      float64
	stages           int
	speedOfSound     float64
	shelfFreq        float64
	shelfGainDB      float64
}

func defaultCrosstalkCancellerConfig() crosstalkCancellerConfig {
	return crosstalkCancellerConfig{
		listenerDistance: defaultCancellerListenerDistance,
		speakerDistance:  defaultCancellerSpeakerDistance,
		headRadius:       defaultCancellerHeadRadius,
		attenuation:      defaultCancellerAttenuation,
		stages:           defaultCancellerStages,
		speedOfSound:     defaultCancellerSpeedOfSound,
		shelfFreq:        defaultCancellerShelfFreq,
		shelfGainDB:      defaultCancellerShelfGainDB,
	}
}

// WithCancellerListenerDistance sets listener distance to speaker line (meters).
func WithCancellerListenerDistance(distance float64) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if distance < minCancellerListenerDistance || math.IsNaN(distance) || math.IsInf(distance, 0) {
			return fmt.Errorf("crosstalk canceller listener distance must be >= %g: %f",
				minCancellerListenerDistance, distance)
		}

		cfg.listenerDistance = distance

		return nil
	}
}

// WithCancellerSpeakerDistance sets stereo speaker spacing (meters).
func WithCancellerSpeakerDistance(distance float64) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if distance < minCancellerSpeakerDistance || math.IsNaN(distance) || math.IsInf(distance, 0) {
			return fmt.Errorf("crosstalk canceller speaker distance must be >= %g: %f",
				minCancellerSpeakerDistance, distance)
		}

		cfg.speakerDistance = distance

		return nil
	}
}

// WithCancellerHeadRadius sets listener head radius (meters).
func WithCancellerHeadRadius(radius float64) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if radius < minCancellerHeadRadius || radius > maxCancellerHeadRadius ||
			math.IsNaN(radius) || math.IsInf(radius, 0) {
			return fmt.Errorf("crosstalk canceller head radius must be in [%g, %g]: %f",
				minCancellerHeadRadius, maxCancellerHeadRadius, radius)
		}

		cfg.headRadius = radius

		return nil
	}
}

// WithCancellerAttenuation sets per-stage attenuation in [0, 0.99].
func WithCancellerAttenuation(attenuation float64) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if attenuation < 0 || attenuation >= 1 || math.IsNaN(attenuation) || math.IsInf(attenuation, 0) {
			return fmt.Errorf("crosstalk canceller attenuation must be in [0, 1): %f", attenuation)
		}

		cfg.attenuation = attenuation

		return nil
	}
}

// WithCancellerStages sets the cancellation stage count.
func WithCancellerStages(stages int) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if stages <= 0 || stages > maxCancellerStages {
			return fmt.Errorf("crosstalk canceller stages must be in [1, %d]: %d", maxCancellerStages, stages)
		}

		cfg.stages = stages

		return nil
	}
}

// WithCancellerSpeedOfSound sets the speed-of-sound model (m/s).
func WithCancellerSpeedOfSound(speed float64) CrosstalkCancellerOption {
	return func(cfg *crosstalkCancellerConfig) error {
		if speed < minCancellerSpeedOfSound || speed > maxCancellerSpeedOfSound ||
			math.IsNaN(speed) || math.IsInf(speed, 0) {
			return fmt.Errorf("crosstalk canceller speed of sound must be in [%g, %g]: %f",
				minCancellerSpeedOfSound, maxCancellerSpeedOfSound, speed)
		}

		cfg.speedOfSound = speed

		return nil
	}
}

// CrosstalkCanceller is a staged delayed crossfeed cancellation processor.
// It models speaker-to-ear geometric path mismatch and subtracts delayed,
// high-shelf shaped opposite-channel feed from each output channel.
type CrosstalkCanceller struct {
	sampleRate        float64
	listenerDistance  float64
	speakerDistance   float64
	headRadius        float64
	attenuation       float64
	stages            int
	speedOfSound      float64
	shelfFreq         float64
	shelfGainDB       float64
	baseDelaySamples  int
	stageDelaySamples int
	stageGains        []float64
	lineLFromR        []monoDelay
	lineRFromL        []monoDelay
	shelfLFromR       []*biquad.Section
	shelfRFromL       []*biquad.Section
}

// NewCrosstalkCanceller creates a crosstalk canceller with validated options.
func NewCrosstalkCanceller(sampleRate float64, opts ...CrosstalkCancellerOption) (*CrosstalkCanceller, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("crosstalk canceller sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultCrosstalkCancellerConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	c := &CrosstalkCanceller{
		sampleRate:       sampleRate,
		listenerDistance: cfg.listenerDistance,
		speakerDistance:  cfg.speakerDistance,
		headRadius:       cfg.headRadius,
		attenuation:      cfg.attenuation,
		stages:           cfg.stages,
		speedOfSound:     cfg.speedOfSound,
		shelfFreq:        cfg.shelfFreq,
		shelfGainDB:      cfg.shelfGainDB,
	}

	err := c.rebuild()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// ProcessStereo processes one stereo sample pair.
func (c *CrosstalkCanceller) ProcessStereo(left, right float64) (float64, float64) {
	crossToL := 0.0
	crossToR := 0.0

	for i := range c.stages {
		delayedR := c.lineLFromR[i].tick(right)
		delayedL := c.lineRFromL[i].tick(left)

		crossToL += c.shelfLFromR[i].ProcessSample(delayedR) * c.stageGains[i]
		crossToR += c.shelfRFromL[i].ProcessSample(delayedL) * c.stageGains[i]
	}

	return left - crossToL, right - crossToR
}

// ProcessInPlace processes paired left/right buffers in place.
func (c *CrosstalkCanceller) ProcessInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("crosstalk canceller: left and right lengths must match: %d != %d", len(left), len(right))
	}

	for i := range left {
		left[i], right[i] = c.ProcessStereo(left[i], right[i])
	}

	return nil
}

// Reset clears all internal stage states.
func (c *CrosstalkCanceller) Reset() {
	for i := range c.lineLFromR {
		c.lineLFromR[i].reset()
		c.lineRFromL[i].reset()
		c.shelfLFromR[i].Reset()
		c.shelfRFromL[i].Reset()
	}
}

// SetSampleRate updates sample rate and rebuilds delay/filter state.
func (c *CrosstalkCanceller) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("crosstalk canceller sample rate must be > 0 and finite: %f", sampleRate)
	}

	c.sampleRate = sampleRate

	return c.rebuild()
}

// SetGeometry updates listener/speaker geometry and rebuilds state.
func (c *CrosstalkCanceller) SetGeometry(listenerDistance, speakerDistance, headRadius float64) error {
	if listenerDistance < minCancellerListenerDistance || math.IsNaN(listenerDistance) || math.IsInf(listenerDistance, 0) {
		return fmt.Errorf("crosstalk canceller listener distance must be >= %g: %f",
			minCancellerListenerDistance, listenerDistance)
	}

	if speakerDistance < minCancellerSpeakerDistance || math.IsNaN(speakerDistance) || math.IsInf(speakerDistance, 0) {
		return fmt.Errorf("crosstalk canceller speaker distance must be >= %g: %f",
			minCancellerSpeakerDistance, speakerDistance)
	}

	if headRadius < minCancellerHeadRadius || headRadius > maxCancellerHeadRadius || math.IsNaN(headRadius) || math.IsInf(headRadius, 0) {
		return fmt.Errorf("crosstalk canceller head radius must be in [%g, %g]: %f",
			minCancellerHeadRadius, maxCancellerHeadRadius, headRadius)
	}

	c.listenerDistance = listenerDistance
	c.speakerDistance = speakerDistance
	c.headRadius = headRadius

	return c.rebuild()
}

// SetAttenuation updates per-stage attenuation.
func (c *CrosstalkCanceller) SetAttenuation(attenuation float64) error {
	if attenuation < 0 || attenuation >= 1 || math.IsNaN(attenuation) || math.IsInf(attenuation, 0) {
		return fmt.Errorf("crosstalk canceller attenuation must be in [0, 1): %f", attenuation)
	}

	c.attenuation = attenuation

	return c.rebuild()
}

// SetStages updates stage count and rebuilds state.
func (c *CrosstalkCanceller) SetStages(stages int) error {
	if stages <= 0 || stages > maxCancellerStages {
		return fmt.Errorf("crosstalk canceller stages must be in [1, %d]: %d", maxCancellerStages, stages)
	}

	c.stages = stages

	return c.rebuild()
}

// SetSpeedOfSound updates speed-of-sound model and rebuilds state.
func (c *CrosstalkCanceller) SetSpeedOfSound(speed float64) error {
	if speed < minCancellerSpeedOfSound || speed > maxCancellerSpeedOfSound || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return fmt.Errorf("crosstalk canceller speed of sound must be in [%g, %g]: %f",
			minCancellerSpeedOfSound, maxCancellerSpeedOfSound, speed)
	}

	c.speedOfSound = speed

	return c.rebuild()
}

// BaseDelaySamples returns geometric base delay in samples.
func (c *CrosstalkCanceller) BaseDelaySamples() int { return c.baseDelaySamples }

// StageDelaySamples returns per-stage extra delay in samples.
func (c *CrosstalkCanceller) StageDelaySamples() int { return c.stageDelaySamples }

// SampleRate returns sample rate in Hz.
func (c *CrosstalkCanceller) SampleRate() float64 { return c.sampleRate }

func (c *CrosstalkCanceller) rebuild() error {
	err := validateCancellerGeometry(c.listenerDistance, c.speakerDistance, c.headRadius)
	if err != nil {
		return err
	}

	if c.sampleRate <= 0 || math.IsNaN(c.sampleRate) || math.IsInf(c.sampleRate, 0) {
		return fmt.Errorf("crosstalk canceller sample rate must be > 0 and finite: %f", c.sampleRate)
	}

	pathDeltaMeters := c.pathDeltaMeters()
	delaySeconds := pathDeltaMeters / c.speedOfSound

	baseDelay := max(int(math.Round(delaySeconds*c.sampleRate)), 1)

	stageDelay := max(int(math.Round(0.00015*c.sampleRate)), 1)

	c.baseDelaySamples = baseDelay
	c.stageDelaySamples = stageDelay

	c.stageGains = make([]float64, c.stages)
	c.lineLFromR = make([]monoDelay, c.stages)
	c.lineRFromL = make([]monoDelay, c.stages)
	c.shelfLFromR = make([]*biquad.Section, c.stages)
	c.shelfRFromL = make([]*biquad.Section, c.stages)

	shelf := design.HighShelf(c.shelfFreq, c.shelfGainDB, defaultCancellerShelfQ, c.sampleRate)

	gain := c.attenuation
	for i := range c.stages {
		stageSamples := c.baseDelaySamples + i*c.stageDelaySamples
		c.stageGains[i] = gain
		gain *= c.attenuation

		c.lineLFromR[i].init(stageSamples)
		c.lineRFromL[i].init(stageSamples)
		c.shelfLFromR[i] = biquad.NewSection(shelf)
		c.shelfRFromL[i] = biquad.NewSection(shelf)
	}

	return nil
}

func (c *CrosstalkCanceller) pathDeltaMeters() float64 {
	half := c.speakerDistance * 0.5
	near := math.Hypot(c.listenerDistance, half-c.headRadius)
	far := math.Hypot(c.listenerDistance, half+c.headRadius)

	delta := far - near
	if delta <= 0 {
		return 1e-6
	}

	return delta
}

func validateCancellerGeometry(listenerDistance, speakerDistance, headRadius float64) error {
	if speakerDistance <= 2*headRadius {
		return fmt.Errorf("crosstalk canceller speaker distance must be > 2*headRadius: speaker=%f headRadius=%f",
			speakerDistance, headRadius)
	}

	if listenerDistance <= 0 {
		return fmt.Errorf("crosstalk canceller listener distance must be > 0: %f", listenerDistance)
	}

	return nil
}

type monoDelay struct {
	buf   []float64
	write int
}

func (d *monoDelay) init(delaySamples int) {
	if delaySamples < 1 {
		delaySamples = 1
	}

	d.buf = make([]float64, delaySamples+1)
	d.write = 0
}

func (d *monoDelay) tick(x float64) float64 {
	if len(d.buf) == 0 {
		return 0
	}

	out := d.buf[d.write]
	d.buf[d.write] = x

	d.write++
	if d.write >= len(d.buf) {
		d.write = 0
	}

	return out
}

func (d *monoDelay) reset() {
	for i := range d.buf {
		d.buf[i] = 0
	}

	d.write = 0
}
