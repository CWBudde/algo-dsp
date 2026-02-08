package effects

import "math"

const (
	reverbNumCombs     = 8
	reverbNumAllpasses = 4

	reverbFixedGain = 0.015

	// Legacy tuning values calibrated for 44.1 kHz.
	reverbCombTuningL1 = 1116
	reverbCombTuningL2 = 1188
	reverbCombTuningL3 = 1277
	reverbCombTuningL4 = 1356
	reverbCombTuningL5 = 1422
	reverbCombTuningL6 = 1491
	reverbCombTuningL7 = 1557
	reverbCombTuningL8 = 1617

	reverbAllpassTuningL1 = 556
	reverbAllpassTuningL2 = 441
	reverbAllpassTuningL3 = 341
	reverbAllpassTuningL4 = 225

	defaultReverbWet      = 1.0
	defaultReverbDry      = 1.0
	defaultReverbRoomSize = 0.5
	defaultReverbDamp     = 0.5
)

// Reverb is a lightweight Schroeder/Freeverb-style reverb.
type Reverb struct {
	wet      float64
	dry      float64
	roomSize float64
	damp     float64
	gain     float64

	combs   [reverbNumCombs]reverbComb
	allpass [reverbNumAllpasses]reverbAllpass
}

type reverbAllpass struct {
	feedback float64
	buffer   []float64
	index    int
}

func newReverbAllpass(size int) reverbAllpass {
	return reverbAllpass{
		feedback: 0.5,
		buffer:   make([]float64, size),
	}
}

func (a *reverbAllpass) process(input float64) float64 {
	bufOut := a.buffer[a.index]
	output := bufOut - input
	a.buffer[a.index] = input + bufOut*a.feedback
	a.index++
	if a.index >= len(a.buffer) {
		a.index = 0
	}
	return output
}

func (a *reverbAllpass) reset() {
	for i := range a.buffer {
		a.buffer[i] = 0
	}
	a.index = 0
}

type reverbComb struct {
	feedback    float64
	filterStore float64
	dampA       float64
	dampB       float64
	buffer      []float64
	index       int
}

func newReverbComb(size int) reverbComb {
	c := reverbComb{
		buffer: make([]float64, size),
	}
	c.setDamp(defaultReverbDamp)
	return c
}

func (c *reverbComb) setDamp(v float64) {
	c.dampA = v
	c.dampB = 1 - v
}

func (c *reverbComb) process(input float64) float64 {
	output := c.buffer[c.index]
	c.filterStore = output*c.dampB + c.filterStore*c.dampA
	if math.Abs(c.filterStore) < 1e-23 {
		c.filterStore = 0
	}
	c.buffer[c.index] = input + c.filterStore*c.feedback
	c.index++
	if c.index >= len(c.buffer) {
		c.index = 0
	}
	return output
}

func (c *reverbComb) reset() {
	for i := range c.buffer {
		c.buffer[i] = 0
	}
	c.index = 0
	c.filterStore = 0
}

// NewReverb constructs a reverb with legacy defaults.
func NewReverb() *Reverb {
	r := &Reverb{
		gain: reverbFixedGain,
		combs: [reverbNumCombs]reverbComb{
			newReverbComb(reverbCombTuningL1),
			newReverbComb(reverbCombTuningL2),
			newReverbComb(reverbCombTuningL3),
			newReverbComb(reverbCombTuningL4),
			newReverbComb(reverbCombTuningL5),
			newReverbComb(reverbCombTuningL6),
			newReverbComb(reverbCombTuningL7),
			newReverbComb(reverbCombTuningL8),
		},
		allpass: [reverbNumAllpasses]reverbAllpass{
			newReverbAllpass(reverbAllpassTuningL1),
			newReverbAllpass(reverbAllpassTuningL2),
			newReverbAllpass(reverbAllpassTuningL3),
			newReverbAllpass(reverbAllpassTuningL4),
		},
	}
	r.SetWet(defaultReverbWet)
	r.SetDry(defaultReverbDry)
	r.SetRoomSize(defaultReverbRoomSize)
	r.SetDamp(defaultReverbDamp)
	return r
}

// Reset clears all delay/filter state.
func (r *Reverb) Reset() {
	for i := range r.combs {
		r.combs[i].reset()
	}
	for i := range r.allpass {
		r.allpass[i].reset()
	}
}

// ProcessSample processes one sample.
func (r *Reverb) ProcessSample(input float64) float64 {
	x := r.gain * input

	var acc float64
	for i := range r.combs {
		acc += r.combs[i].process(x)
	}
	for i := range r.allpass {
		acc = r.allpass[i].process(acc)
	}
	return acc*r.wet + input*r.dry
}

// ProcessInPlace applies reverb to buf in place.
func (r *Reverb) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = r.ProcessSample(buf[i])
	}
}

// SetWet sets wet gain.
func (r *Reverb) SetWet(v float64) {
	r.wet = v
}

// SetDry sets dry gain.
func (r *Reverb) SetDry(v float64) {
	r.dry = v
}

// SetRoomSize sets comb feedback amount.
func (r *Reverb) SetRoomSize(v float64) {
	r.roomSize = v
	for i := range r.combs {
		r.combs[i].feedback = r.roomSize
	}
}

// SetDamp sets damping in comb feedback filters.
func (r *Reverb) SetDamp(v float64) {
	r.damp = v
	for i := range r.combs {
		r.combs[i].setDamp(v)
	}
}

// SetGain sets input gain.
func (r *Reverb) SetGain(v float64) {
	r.gain = v
}

// Wet returns wet gain.
func (r *Reverb) Wet() float64 { return r.wet }

// Dry returns dry gain.
func (r *Reverb) Dry() float64 { return r.dry }

// RoomSize returns comb feedback amount.
func (r *Reverb) RoomSize() float64 { return r.roomSize }

// Damp returns comb damping value.
func (r *Reverb) Damp() float64 { return r.damp }

// Gain returns input gain.
func (r *Reverb) Gain() float64 { return r.gain }
