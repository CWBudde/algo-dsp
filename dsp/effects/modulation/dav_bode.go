package modulation

import (
	"errors"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

const (
	davBodeClock        = 44100
	davBodeMaximumOrder = 32
)

type davBodeHistory struct {
	input, output [davBodeMaximumOrder]float32
}

// DAVBode32 implements the DAV Win32 Bode frequency shifter. It uses wide
// filter coefficients, float32 phase histories and the original fixed
// 44.1 kHz oscillator clock. Each instance processes one channel.
type DAVBode32 struct {
	coefficients [davBodeMaximumOrder]float64
	history      [2]davBodeHistory
	order        int
	transition   float64

	speed                              float64
	sine, cosine, stepSine, stepCosine float32
	previous                           float32
	phase                              uint8
}

// Configure sets the shift in Hz, the coefficient count and the normalized
// transition bandwidth without allocating. An order change clears both
// filter banks. Bandwidth and frequency changes preserve phase and history.
func (d *DAVBode32) Configure(speed float64, order int, transition float64) error {
	if !(speed >= -davBodeClock/2 && speed <= davBodeClock/2) ||
		order < 1 || order > davBodeMaximumOrder {
		return errors.New("invalid DAV Bode frequency or order")
	}

	if d.order != order || d.transition != transition {
		var design [davBodeMaximumOrder]float64
		if err := hilbert.DesignCoefficientsTo(design[:order], transition); err != nil {
			return err
		}

		if d.order == 0 {
			d.cosine = -1
		}

		if d.order != order {
			d.history = [2]davBodeHistory{}
		}

		d.order, d.transition, d.coefficients = order, transition, design
	}

	if d.stepCosine == 0 || d.speed != speed {
		sine, cosine := math.Sincos(2 * math.Pi * float64(float32(speed)) / davBodeClock)
		d.speed = speed
		d.stepSine, d.stepCosine = float32(sine), float32(cosine)
	}

	return nil
}

// Reset clears the filter histories and returns the oscillator to its initial
// phase while retaining the configured settings.
func (d *DAVBode32) Reset() {
	d.history = [2]davBodeHistory{}
	d.previous, d.phase, d.sine, d.cosine = 0, 0, 0, -1
}

// ProcessSample returns the two legacy sideband samples.
func (d *DAVBode32) ProcessSample(input float32) (up, down float32) {
	if d.order == 0 {
		return 0, 0
	}

	state := &d.history[d.phase]
	previous := d.previous
	d.previous = input

	d.phase ^= 1
	for i, coefficient := range d.coefficients[:d.order] {
		sample := input

		switch {
		case i == 1:
			sample = previous
		case i >= 2:
			sample = state.output[i-2]
		}

		state.output[i] = float32((float64(sample)+float64(state.output[i]))*coefficient -
			float64(state.input[i]))
		state.input[i] = sample
	}

	re, im := state.output[0], previous
	if d.order > 1 {
		re, im = state.output[d.order-2], state.output[d.order-1]
		if d.order > 4 {
			re, im = im, re
		}
	}

	re *= d.cosine
	im *= d.sine

	sine := float64(d.sine)*float64(d.stepCosine) - float64(d.cosine)*float64(d.stepSine)
	cosine := float64(d.cosine)*float64(d.stepCosine) + float64(d.sine)*float64(d.stepSine)
	d.sine, d.cosine = float32(sine), float32(cosine)

	const sqrtHalf32 = float64(float32(.7071067811865475244))

	return float32((float64(re) - float64(im)) * sqrtHalf32),
		float32((float64(re) + float64(im)) * sqrtHalf32)
}
