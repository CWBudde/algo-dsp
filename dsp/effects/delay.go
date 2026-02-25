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

	// delaySmoothSeconds is the one-pole smoother time constant used by
	// SetTargetTime.  At τ = 10 ms the read-pointer reaches 98 % of a new
	// target within ≈ 50 ms, which is inaudible as a click even for large
	// delay-time jumps.
	delaySmoothSeconds = 0.010
)

// Delay is a simple feedback delay with dry/wet mix.
// Use SetTime for immediate (static) changes and SetTargetTime for smooth
// parameter changes during live playback to avoid audible clicks.
type Delay struct {
	sampleRate   float64
	delaySeconds float64
	feedback     float64
	mix          float64

	targetSamples  float64 // desired delay in samples
	currentSamples float64 // current (fractional) delay – ramps toward target
	smoothCoeff    float64 // one-pole LP coefficient derived from delaySmoothSeconds

	buffer []float64
	write  int
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
	d.smoothCoeff = computeSmoothCoeff(sampleRate)
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
	d.smoothCoeff = computeSmoothCoeff(sampleRate)

	return d.reconfigureBuffer()
}

// SetTime sets the delay time in seconds immediately (snaps without ramping).
// Use this for static configuration before playback starts.  For smooth
// in-playback changes that avoid audible clicks, use SetTargetTime.
func (d *Delay) SetTime(seconds float64) error {
	if err := d.validateTime(seconds); err != nil {
		return err
	}

	d.delaySeconds = seconds
	samples := math.Round(seconds * d.sampleRate)
	d.targetSamples = samples
	d.currentSamples = samples // snap – no ramp

	return nil
}

// SetTargetTime sets the delay-time target in seconds.  The effective delay
// is ramped toward the new value during subsequent calls to ProcessSample /
// ProcessInPlace, avoiding the read-pointer jump that would otherwise cause
// an audible click.
func (d *Delay) SetTargetTime(seconds float64) error {
	if err := d.validateTime(seconds); err != nil {
		return err
	}

	d.delaySeconds = seconds
	d.targetSamples = math.Round(seconds * d.sampleRate)
	// currentSamples is intentionally unchanged; ProcessSample ramps it.

	return nil
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
	// Ramp currentSamples toward targetSamples one step at a time.
	d.currentSamples += (d.targetSamples - d.currentSamples) * d.smoothCoeff

	// Fractional read position with linear interpolation.
	readExact := float64(d.write) - d.currentSamples
	bufLen := float64(len(d.buffer))

	for readExact < 0 {
		readExact += bufLen
	}

	r0 := int(math.Floor(readExact))
	frac := readExact - float64(r0)
	r1 := (r0 + 1) % len(d.buffer)

	delayed := d.buffer[r0]*(1-frac) + d.buffer[r1]*frac

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

// Time returns the target delay time in seconds.
func (d *Delay) Time() float64 { return d.delaySeconds }

// Feedback returns feedback amount in [0, 0.99].
func (d *Delay) Feedback() float64 { return d.feedback }

// Mix returns wet amount in [0, 1].
func (d *Delay) Mix() float64 { return d.mix }

// CurrentDelaySamples returns the current effective (possibly mid-ramp)
// delay in fractional samples.  Primarily useful for testing.
func (d *Delay) CurrentDelaySamples() float64 { return d.currentSamples }

func (d *Delay) validateTime(seconds float64) error {
	if seconds < minDelayTimeSeconds || seconds > maxDelayTimeSeconds ||
		math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("delay time must be in [%f, %f]: %f",
			minDelayTimeSeconds, maxDelayTimeSeconds, seconds)
	}

	return nil
}

func (d *Delay) reconfigureBuffer() error {
	if d.sampleRate <= 0 {
		return fmt.Errorf("delay sample rate must be > 0: %f", d.sampleRate)
	}

	maxSamples := int(math.Ceil(maxDelayTimeSeconds*d.sampleRate)) + 1
	if maxSamples == len(d.buffer) {
		// Buffer already sized correctly; just update the sample targets.
		samples := math.Round(d.delaySeconds * d.sampleRate)
		if samples < 1 {
			samples = 1
		}

		d.targetSamples = samples
		d.currentSamples = samples

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

	samples := math.Round(d.delaySeconds * d.sampleRate)
	if samples < 1 {
		samples = 1
	}

	d.targetSamples = samples
	d.currentSamples = samples

	return nil
}

func computeSmoothCoeff(sampleRate float64) float64 {
	return 1 - math.Exp(-1/(sampleRate*delaySmoothSeconds))
}
