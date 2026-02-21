package modulation

import (
	"fmt"
	"math"
)

const (
	defaultChorusSampleRate   = 44100.0
	defaultChorusSpeedHz      = 0.35
	defaultChorusDepthSeconds = 0.003
	defaultChorusBaseSeconds  = 0.018
	defaultChorusMix          = 0.18
	defaultChorusStages       = 3
	minChorusDelaySeconds     = 0.001
)

// Chorus is a standard multi-voice modulated-delay chorus effect.
//
// Delay time follows:
//
//	d(t) = baseDelay + depth * 0.5 * (1 + sin(phase + voiceOffset))
//
// with independent base delay, depth, and LFO rate controls.
type Chorus struct {
	sampleRate       float64
	speedHz          float64
	depthSeconds     float64
	baseDelaySeconds float64
	mix              float64
	stages           int

	lfoPhase float64

	delayLine []float64
	write     int
	maxDelay  int
}

// NewChorus creates a chorus effect with tuned musical defaults.
func NewChorus() (*Chorus, error) {
	c := &Chorus{
		sampleRate:       defaultChorusSampleRate,
		speedHz:          defaultChorusSpeedHz,
		depthSeconds:     defaultChorusDepthSeconds,
		baseDelaySeconds: defaultChorusBaseSeconds,
		mix:              defaultChorusMix,
		stages:           defaultChorusStages,
	}
	if err := c.reconfigureDelayLine(); err != nil {
		return nil, err
	}
	return c, nil
}

// SetSampleRate updates sample rate.
func (c *Chorus) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("chorus sample rate must be > 0: %f", sampleRate)
	}
	c.sampleRate = sampleRate
	return c.reconfigureDelayLine()
}

// SetSpeedHz updates LFO modulation rate.
func (c *Chorus) SetSpeedHz(speedHz float64) error {
	if speedHz <= 0 || math.IsNaN(speedHz) || math.IsInf(speedHz, 0) {
		return fmt.Errorf("chorus speed must be > 0: %f", speedHz)
	}
	c.speedHz = speedHz
	return nil
}

// SetDepth updates modulation depth in seconds.
func (c *Chorus) SetDepth(depth float64) error {
	if depth < 0 || math.IsNaN(depth) || math.IsInf(depth, 0) {
		return fmt.Errorf("chorus depth must be >= 0 and finite: %f", depth)
	}
	c.depthSeconds = depth
	return c.reconfigureDelayLine()
}

// SetBaseDelay sets the base delay in seconds.
func (c *Chorus) SetBaseDelay(baseDelay float64) error {
	if baseDelay < minChorusDelaySeconds || math.IsNaN(baseDelay) || math.IsInf(baseDelay, 0) {
		return fmt.Errorf("chorus base delay must be >= %f: %f", minChorusDelaySeconds, baseDelay)
	}
	c.baseDelaySeconds = baseDelay
	return c.reconfigureDelayLine()
}

// SetStages updates the number of chorus voices.
func (c *Chorus) SetStages(stages int) error {
	if stages <= 0 {
		return fmt.Errorf("chorus stages must be > 0: %d", stages)
	}
	c.stages = stages
	return nil
}

// SetMix updates wet amount in range [0, 1].
func (c *Chorus) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("chorus mix must be in [0,1]: %f", mix)
	}
	c.mix = mix
	return nil
}

// Reset clears delay state and modulation phase.
func (c *Chorus) Reset() {
	for i := range c.delayLine {
		c.delayLine[i] = 0
	}
	c.write = 0
	c.lfoPhase = 0
}

// ProcessSample processes one sample.
func (c *Chorus) ProcessSample(input float64) float64 {
	c.delayLine[c.write] = input
	c.write++
	if c.write >= len(c.delayLine) {
		c.write = 0
	}

	baseDelaySamples := c.baseDelaySeconds * c.sampleRate
	depthSamples := c.depthSeconds * c.sampleRate

	wetSum := 0.0
	stageCount := float64(c.stages)
	for i := 0; i < c.stages; i++ {
		phaseOffset := (2 * math.Pi * float64(i)) / stageCount
		mod := 0.5 * (1 + math.Sin(c.lfoPhase+phaseOffset)) // 0..1
		delay := baseDelaySamples + depthSamples*mod
		wetSum += c.sampleFractionalDelay(delay)
	}
	wet := wetSum / stageCount

	c.lfoPhase += 2 * math.Pi * c.speedHz / c.sampleRate
	if c.lfoPhase >= 2*math.Pi {
		c.lfoPhase -= 2 * math.Pi
	}

	return input*(1-c.mix) + wet*c.mix
}

// ProcessInPlace applies chorus to buf in place.
func (c *Chorus) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = c.ProcessSample(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (c *Chorus) SampleRate() float64 { return c.sampleRate }

// SpeedHz returns modulation speed in Hz.
func (c *Chorus) SpeedHz() float64 { return c.speedHz }

// Depth returns modulation depth in seconds.
func (c *Chorus) Depth() float64 { return c.depthSeconds }

// BaseDelay returns the base delay in seconds.
func (c *Chorus) BaseDelay() float64 { return c.baseDelaySeconds }

// Mix returns wet mix amount in [0, 1].
func (c *Chorus) Mix() float64 { return c.mix }

// Stages returns number of chorus voices.
func (c *Chorus) Stages() int { return c.stages }

func (c *Chorus) reconfigureDelayLine() error {
	if c.sampleRate <= 0 {
		return fmt.Errorf("chorus sample rate must be > 0: %f", c.sampleRate)
	}
	if c.baseDelaySeconds < minChorusDelaySeconds {
		return fmt.Errorf("chorus base delay must be >= %f: %f", minChorusDelaySeconds, c.baseDelaySeconds)
	}
	if c.depthSeconds < 0 {
		return fmt.Errorf("chorus depth must be >= 0: %f", c.depthSeconds)
	}

	neededMax := int(math.Ceil((c.baseDelaySeconds+c.depthSeconds)*c.sampleRate)) + 3
	if neededMax < 4 {
		neededMax = 4
	}
	if neededMax == len(c.delayLine) {
		return nil
	}

	old := c.delayLine
	oldWrite := c.write
	c.delayLine = make([]float64, neededMax)
	c.write = 0
	c.maxDelay = neededMax - 3

	// Preserve as much recent delay history as possible when resizing.
	if len(old) > 0 {
		copyCount := len(old)
		if copyCount > len(c.delayLine) {
			copyCount = len(c.delayLine)
		}
		for i := 0; i < copyCount; i++ {
			src := oldWrite - 1 - i
			if src < 0 {
				src += len(old)
			}
			dst := c.write - 1 - i
			if dst < 0 {
				dst += len(c.delayLine)
			}
			c.delayLine[dst] = old[src]
		}
	}

	return nil
}

func (c *Chorus) sampleFractionalDelay(delay float64) float64 {
	if delay < 0 {
		delay = 0
	}
	maxDelay := float64(c.maxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	p := int(math.Floor(delay))
	t := delay - float64(p)

	xm1 := c.sampleDelayInt(maxInt(0, p-1))
	x0 := c.sampleDelayInt(p)
	x1 := c.sampleDelayInt(p + 1)
	x2 := c.sampleDelayInt(p + 2)
	return hermite4(t, xm1, x0, x1, x2)
}

func (c *Chorus) sampleDelayInt(delay int) float64 {
	if delay < 0 || delay >= len(c.delayLine) {
		return 0
	}
	idx := c.write - 1 - delay
	if idx < 0 {
		idx += len(c.delayLine)
	}
	return c.delayLine[idx]
}

func hermite4(t, xm1, x0, x1, x2 float64) float64 {
	c0 := x0
	c1 := 0.5 * (x1 - xm1)
	c2 := xm1 - 2.5*x0 + 2*x1 - 0.5*x2
	c3 := 0.5*(x2-xm1) + 1.5*(x0-x1)
	return ((c3*t+c2)*t+c1)*t + c0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
