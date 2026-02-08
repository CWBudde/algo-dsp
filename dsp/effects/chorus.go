package effects

import (
	"fmt"
	"math"
	"math/rand"
)

const (
	defaultChorusSampleRate = 44100.0
	defaultChorusSpeedHz    = 0.35
	defaultChorusDepth      = 0.02
	defaultChorusMix        = 0.18
	defaultChorusStages     = 3
	defaultChorusSeed       = 1
)

// Chorus is a modulated delay-based chorus effect.
type Chorus struct {
	sampleRate float64
	speedHz    float64
	depth      float64
	mix        float64
	stages     int

	rng  *rand.Rand
	lfos []chorusLFO

	delayLine []float64
	write     int
	maxDelay  int
}

type chorusLFO struct {
	sampleRate float64
	frequency  float64
	phase      float64
}

func (l *chorusLFO) process() float64 {
	y := math.Sin(l.phase)
	l.phase += 2 * math.Pi * l.frequency / l.sampleRate
	if l.phase >= 2*math.Pi {
		l.phase -= 2 * math.Pi
	}
	return y
}

func (l *chorusLFO) reset() {
	l.phase = 0
}

// NewChorus creates a chorus effect with defaults modeled after the legacy implementation.
func NewChorus() (*Chorus, error) {
	c := &Chorus{
		sampleRate: defaultChorusSampleRate,
		speedHz:    defaultChorusSpeedHz,
		depth:      defaultChorusDepth,
		mix:        defaultChorusMix,
		stages:     defaultChorusStages,
		rng:        rand.New(rand.NewSource(defaultChorusSeed)),
	}
	if err := c.reconfigure(); err != nil {
		return nil, err
	}
	return c, nil
}

// SetSampleRate updates sample rate and rebuilds delay/LFO state.
func (c *Chorus) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("chorus sample rate must be > 0: %f", sampleRate)
	}
	c.sampleRate = sampleRate
	return c.reconfigure()
}

// SetSpeedHz updates LFO speed and rebuilds delay/LFO state.
func (c *Chorus) SetSpeedHz(speedHz float64) error {
	if speedHz <= 0 || math.IsNaN(speedHz) || math.IsInf(speedHz, 0) {
		return fmt.Errorf("chorus speed must be > 0: %f", speedHz)
	}
	c.speedHz = speedHz
	return c.reconfigure()
}

// SetDepth updates modulation depth and rebuilds delay state.
func (c *Chorus) SetDepth(depth float64) error {
	if math.IsNaN(depth) || math.IsInf(depth, 0) {
		return fmt.Errorf("chorus depth must be finite: %f", depth)
	}
	c.depth = depth
	return c.reconfigure()
}

// SetStages updates the number of chorus stages and rebuilds LFO state.
func (c *Chorus) SetStages(stages int) error {
	if stages <= 0 {
		return fmt.Errorf("chorus stages must be > 0: %d", stages)
	}
	c.stages = stages
	return c.reconfigure()
}

// SetMix updates wet amount in range [0, 1].
func (c *Chorus) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("chorus mix must be in [0,1]: %f", mix)
	}
	c.mix = mix
	return nil
}

// Reset clears delay state and resets LFO phase.
func (c *Chorus) Reset() {
	for i := range c.delayLine {
		c.delayLine[i] = 0
	}
	c.write = 0
	for i := range c.lfos {
		c.lfos[i].reset()
	}
}

// ProcessSample processes one sample.
func (c *Chorus) ProcessSample(input float64) float64 {
	c.delayLine[c.write] = input
	c.write++
	if c.write >= len(c.delayLine) {
		c.write = 0
	}

	out := (1 - c.mix) * input
	stageGain := math.Pow(c.mix, float64(c.stages))

	for i := range c.lfos {
		// Match legacy mapping: 0.5*(1-lfo)*(maxDelay)
		delay := 0.5 * (1 - c.lfos[i].process()) * float64(c.maxDelay)
		out += stageGain * c.sampleFractionalDelay(delay)
	}
	return out
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

// Depth returns chorus depth.
func (c *Chorus) Depth() float64 { return c.depth }

// Mix returns wet mix amount in [0, 1].
func (c *Chorus) Mix() float64 { return c.mix }

// Stages returns the number of chorus stages.
func (c *Chorus) Stages() int { return c.stages }

func (c *Chorus) reconfigure() error {
	if c.sampleRate <= 0 {
		return fmt.Errorf("chorus sample rate must be > 0: %f", c.sampleRate)
	}
	if c.speedHz <= 0 {
		return fmt.Errorf("chorus speed must be > 0: %f", c.speedHz)
	}
	if c.stages <= 0 {
		return fmt.Errorf("chorus stages must be > 0: %d", c.stages)
	}

	c.maxDelay = int(math.Round(math.Abs(c.depth) * c.sampleRate / c.speedHz))
	if c.maxDelay < 1 {
		c.maxDelay = 1
	}
	c.delayLine = make([]float64, c.maxDelay+4)
	c.write = 0

	c.lfos = make([]chorusLFO, c.stages)
	for i := range c.lfos {
		c.lfos[i] = chorusLFO{
			sampleRate: c.sampleRate,
			frequency:  c.rng.Float64() * c.speedHz,
		}
	}
	return nil
}

func (c *Chorus) sampleFractionalDelay(delay float64) float64 {
	if delay < 0 {
		delay = 0
	}
	if delay > float64(c.maxDelay) {
		delay = float64(c.maxDelay)
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
