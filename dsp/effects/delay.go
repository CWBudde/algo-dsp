package effects

import (
	"fmt"
	"math"
)

const (
	defaultDelayTimeSeconds = 0.25
	defaultDelayFeedback    = 0.35
	defaultDelayMix         = 0.25
	maxDelayTimeSeconds     = 2.0
	minDelayTimeSeconds     = 0.001
)

// Delay is a simple feedback delay with dry/wet mix.
type Delay struct {
	sampleRate   float64
	delaySeconds float64
	feedback     float64
	mix          float64

	delaySamples int
	buffer       []float64
	write        int
}

// NewDelay creates a delay with practical defaults.
func NewDelay(sampleRate float64) (*Delay, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("delay sample rate must be > 0: %f", sampleRate)
	}
	d := &Delay{
		sampleRate:   sampleRate,
		delaySeconds: defaultDelayTimeSeconds,
		feedback:     defaultDelayFeedback,
		mix:          defaultDelayMix,
	}
	if err := d.reconfigureBuffer(); err != nil {
		return nil, err
	}
	return d, nil
}

// SetSampleRate updates sample rate.
func (d *Delay) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("delay sample rate must be > 0: %f", sampleRate)
	}
	d.sampleRate = sampleRate
	return d.reconfigureBuffer()
}

// SetTime sets delay time in seconds.
func (d *Delay) SetTime(seconds float64) error {
	if seconds < minDelayTimeSeconds || seconds > maxDelayTimeSeconds ||
		math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("delay time must be in [%f, %f]: %f",
			minDelayTimeSeconds, maxDelayTimeSeconds, seconds)
	}
	d.delaySeconds = seconds
	return d.reconfigureBuffer()
}

// SetFeedback sets feedback amount in [0, 0.99].
func (d *Delay) SetFeedback(feedback float64) error {
	if feedback < 0 || feedback > 0.99 || math.IsNaN(feedback) || math.IsInf(feedback, 0) {
		return fmt.Errorf("delay feedback must be in [0, 0.99]: %f", feedback)
	}
	d.feedback = feedback
	return nil
}

// SetMix sets wet amount in [0, 1].
func (d *Delay) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("delay mix must be in [0, 1]: %f", mix)
	}
	d.mix = mix
	return nil
}

// Reset clears delay state.
func (d *Delay) Reset() {
	for i := range d.buffer {
		d.buffer[i] = 0
	}
	d.write = 0
}

// ProcessSample processes one sample.
func (d *Delay) ProcessSample(input float64) float64 {
	read := d.write - d.delaySamples
	if read < 0 {
		read += len(d.buffer)
	}
	delayed := d.buffer[read]

	d.buffer[d.write] = input + delayed*d.feedback
	d.write++
	if d.write >= len(d.buffer) {
		d.write = 0
	}

	return input*(1-d.mix) + delayed*d.mix
}

// ProcessInPlace applies delay to buf in place.
func (d *Delay) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = d.ProcessSample(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (d *Delay) SampleRate() float64 { return d.sampleRate }

// Time returns delay time in seconds.
func (d *Delay) Time() float64 { return d.delaySeconds }

// Feedback returns feedback amount in [0, 0.99].
func (d *Delay) Feedback() float64 { return d.feedback }

// Mix returns wet amount in [0, 1].
func (d *Delay) Mix() float64 { return d.mix }

func (d *Delay) reconfigureBuffer() error {
	if d.sampleRate <= 0 {
		return fmt.Errorf("delay sample rate must be > 0: %f", d.sampleRate)
	}
	if d.delaySeconds < minDelayTimeSeconds || d.delaySeconds > maxDelayTimeSeconds {
		return fmt.Errorf("delay time must be in [%f, %f]: %f",
			minDelayTimeSeconds, maxDelayTimeSeconds, d.delaySeconds)
	}

	d.delaySamples = int(math.Round(d.delaySeconds * d.sampleRate))
	if d.delaySamples < 1 {
		d.delaySamples = 1
	}
	maxSamples := int(math.Ceil(maxDelayTimeSeconds*d.sampleRate)) + 1
	if maxSamples < d.delaySamples+1 {
		maxSamples = d.delaySamples + 1
	}
	if maxSamples == len(d.buffer) {
		return nil
	}

	old := d.buffer
	oldWrite := d.write
	d.buffer = make([]float64, maxSamples)
	d.write = 0

	// Preserve the newest available history after sample-rate changes.
	if len(old) > 0 {
		copyCount := len(old)
		if copyCount > len(d.buffer) {
			copyCount = len(d.buffer)
		}
		for i := 0; i < copyCount; i++ {
			src := oldWrite - 1 - i
			if src < 0 {
				src += len(old)
			}
			dst := d.write - 1 - i
			if dst < 0 {
				dst += len(d.buffer)
			}
			d.buffer[dst] = old[src]
		}
	}
	return nil
}
