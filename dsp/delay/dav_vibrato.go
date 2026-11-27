package delay

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

const (
	davVibratoClock   = 44100
	davVibratoOverlap = 4
	davVibratoPadding = 2 * davVibratoOverlap
)

// DAVVibrato32 is a mono fractional delay with the DAV vibrato tap and
// oscillator. Its LFO uses a fixed 44.1 kHz clock. The host sample rate
// determines the span of the delay. An instance belongs to one audio stream.
type DAVVibrato32 struct {
	buffer               []float32
	size, position       int
	sine, cosine         float64
	stepSine, stepCosine float64
}

// NewDAVVibrato32 reserves the maximum modulated delay span in samples.
// The processor can be retuned within that span without allocations.
func NewDAVVibrato32(maxDelaySamples int) (*DAVVibrato32, error) {
	line := &DAVVibrato32{cosine: -1, stepCosine: 1}
	if err := line.ReserveDelay(maxDelaySamples); err != nil {
		return nil, err
	}

	return line, nil
}

// ReserveDelay increases the available delay span while preserving history.
// Call it during setup, while audio processing is stopped.
func (v *DAVVibrato32) ReserveDelay(maxDelaySamples int) error {
	if maxDelaySamples < 0 || maxDelaySamples > math.MaxInt-davVibratoPadding {
		return fmt.Errorf("invalid DAV vibrato capacity %d", maxDelaySamples)
	}

	capacity := maxDelaySamples + davVibratoPadding
	if capacity > len(v.buffer) {
		next := make([]float32, capacity)
		copy(next, v.buffer)
		v.buffer = next
	}

	return nil
}

// Configure sets the sample rate in Hz, depth in [0, 1], and LFO speed in Hz.
// It preserves the oscillator phase and the newest delay history that fits the
// new span. Older positions that a wider span adds start at zero unless the
// line still holds those samples. Invalid values leave state intact.
func (v *DAVVibrato32) Configure(rate, depth, speed float64) error {
	if !(rate > 0 && rate <= math.MaxFloat32 && depth >= 0 && depth <= 1 &&
		speed >= 0 && speed <= davVibratoClock/2) {
		return errors.New("invalid DAV vibrato settings")
	}

	span := math.RoundToEven(depth * depth * .25 * rate)
	if span > float64(len(v.buffer)-davVibratoPadding) {
		return errors.New("DAV vibrato delay exceeds reserved capacity")
	}

	v.resize(int(span))
	v.stepSine, v.stepCosine = math.Sincos(2 * math.Pi * speed / davVibratoClock)

	return nil
}

// resize changes the active span to size samples without allocating. It
// repacks the ring so the newest history keeps its age: buffer[4:position]
// holds the newest samples, buffer[position:end] the older ones, and
// buffer[:4] the four samples that precede buffer[4] in the ring.
func (v *DAVVibrato32) resize(size int) {
	end, nextEnd := v.size+davVibratoPadding, size+davVibratoPadding

	switch {
	case size == v.size:
	case v.position < davVibratoOverlap:
		// First pass after a reset: everything past the write position is zero.
		if nextEnd > end {
			clear(v.buffer[end:nextEnd])
		}
	case size > v.size:
		// Move the older samples to the end of the wider ring and zero the gap.
		// The overlap slots hold the samples that precede buffer[4], so they
		// also fill the slots just before the wrap.
		older := end - v.position
		copy(v.buffer[nextEnd-older:nextEnd], v.buffer[v.position:end])
		clear(v.buffer[v.position : nextEnd-older])

		count := min(davVibratoOverlap, nextEnd-v.position)
		copy(v.buffer[nextEnd-count:nextEnd], v.buffer[davVibratoOverlap-count:davVibratoOverlap])
	case v.position < nextEnd:
		// The newest samples stay in place; keep the youngest of the older ones.
		copy(v.buffer[v.position:nextEnd], v.buffer[end-nextEnd+v.position:end])
	default:
		// The write position falls outside the narrower ring: move the newest
		// period samples to its start and wrap as ProcessSample does.
		period := size + davVibratoOverlap
		copy(v.buffer[davVibratoOverlap:nextEnd], v.buffer[v.position-period:v.position])
		copy(v.buffer[:davVibratoOverlap], v.buffer[period:nextEnd])
		v.position = davVibratoOverlap
	}

	v.size = size
}

// ClearHistory clears the delay while preserving its write position and phase.
func (v *DAVVibrato32) ClearHistory() {
	clear(v.buffer[:v.size+davVibratoPadding])
}

// Reset clears the delay and restarts its oscillator and write position.
func (v *DAVVibrato32) Reset() {
	v.ClearHistory()
	v.position, v.sine, v.cosine = 0, 0, -1
}

// ProcessSample writes one input and returns the modulated delay tap.
func (v *DAVVibrato32) ProcessSample(input float32) float32 {
	v.sine, v.cosine = v.sine*v.stepCosine-v.cosine*v.stepSine, v.cosine*v.stepCosine+v.sine*v.stepSine
	sine := max(-1, min(1, v.sine))
	tap := davVibratoOverlap + float64(v.size)*.5*(1-sine)
	integer := int(math.RoundToEven(tap - .5))
	fraction := float64(float32(tap - float64(integer)))
	index := v.position + integer
	period := v.size + davVibratoOverlap

	end := v.size + davVibratoPadding
	if index >= end {
		index -= period
	} else if index < davVibratoOverlap {
		index += period
	}

	neighbors := v.buffer[index-davVibratoOverlap : index]
	output := interp.Hermite4(fraction,
		float64(neighbors[0]), float64(neighbors[1]), float64(neighbors[2]), float64(neighbors[3]))
	v.buffer[v.position] = input

	v.position++
	if v.position == end {
		copy(v.buffer[:davVibratoOverlap], v.buffer[period:end])
		v.position = davVibratoOverlap
	}

	return float32(output)
}
