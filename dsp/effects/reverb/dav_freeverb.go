package reverb

import (
	"errors"
	"math"
)

const davFreeverbMaxStages = 16

var (
	errDAVFreeverbGeometry = errors.New("invalid DAV Freeverb delay geometry")
	errDAVFreeverbFeedback = errors.New("invalid DAV Freeverb feedback or damping")
)

// DAVFreeverb32 implements the Delphi Win32 Freeverb stereo network, using
// float32 delay histories. Its allpass stages store the scaled input and
// return the delayed sample minus the input. Stereo width and wet/dry mixing
// are applied by the caller.
type DAVFreeverb32 struct {
	combs     [davFreeverbMaxStages][2]davFreeverbComb32
	allpasses [davFreeverbMaxStages][2]davFreeverbAllpass32
	numCombs  int
	numPasses int
}

type davFreeverbDelay32 struct {
	buffer []float32
	pos    int
}

func (d *davFreeverbDelay32) setLength(length int) {
	if length != len(d.buffer) {
		d.buffer = d.buffer[:length]
		d.reset()
	}
}

func (d *davFreeverbDelay32) reset() {
	clear(d.buffer)
	d.pos = 0
}

func (d *davFreeverbDelay32) write(input float32) {
	d.buffer[d.pos] = input
	d.pos++
	if d.pos == len(d.buffer) {
		d.pos = 0
	}
}

func zeroDAVFreeverbSubnormal(value float32) float32 {
	if math.Float32bits(value)&0x7f800000 == 0 {
		return 0
	}
	return value
}

type davFreeverbComb32 struct {
	delay       davFreeverbDelay32
	filterStore float32
	feedback    float32
	dampA       float32
	dampB       float32
}

func (c *davFreeverbComb32) process(input float32) float32 {
	delayed := zeroDAVFreeverbSubnormal(c.delay.buffer[c.delay.pos])
	filtered := float64(delayed)*float64(c.dampB) + float64(c.filterStore)*float64(c.dampA)
	c.filterStore = zeroDAVFreeverbSubnormal(float32(filtered))
	c.delay.write(float32(float64(input) + filtered*float64(c.feedback)))
	return delayed
}

type davFreeverbAllpass32 struct {
	delay    davFreeverbDelay32
	feedback float32
}

func (a *davFreeverbAllpass32) process(input float32) float32 {
	delayed := zeroDAVFreeverbSubnormal(a.delay.buffer[a.delay.pos])
	a.delay.write(float32(float64(input) + float64(input)*float64(a.feedback)))
	return delayed - input
}

// NewDAVFreeverb32 reserves positive per-stage delay lengths for the left
// and right channels. Each bank can have at most sixteen stages. Call
// SetGeometry with the initial active lengths before processing.
func NewDAVFreeverb32(combCapacity, allpassCapacity [][2]int) (*DAVFreeverb32, error) {
	if !davFreeverbValidSizes(combCapacity) || !davFreeverbValidSizes(allpassCapacity) {
		return nil, errDAVFreeverbGeometry
	}

	r := new(DAVFreeverb32)
	for i, lengths := range combCapacity {
		for channel, length := range lengths {
			r.combs[i][channel].delay.buffer = make([]float32, 0, length)
		}
	}
	for i, lengths := range allpassCapacity {
		for channel, length := range lengths {
			r.allpasses[i][channel].delay.buffer = make([]float32, 0, length)
		}
	}
	return r, nil
}

func davFreeverbValidSizes(lengths [][2]int) bool {
	if len(lengths) == 0 || len(lengths) > davFreeverbMaxStages {
		return false
	}
	for _, pair := range lengths {
		if pair[0] <= 0 || pair[1] <= 0 {
			return false
		}
	}
	return true
}

// SetGeometry sets the active left and right delay lengths within the
// reserved capacities. Changing a delay length clears that stage's delay
// history. Adding a stage starts it with zero feedback and damping. An
// invalid setting leaves the current network unchanged.
func (r *DAVFreeverb32) SetGeometry(combLengths, allpassLengths [][2]int) error {
	if !davFreeverbValidSizes(combLengths) || !davFreeverbValidSizes(allpassLengths) {
		return errDAVFreeverbGeometry
	}
	for i, lengths := range combLengths {
		for channel, length := range lengths {
			if length > cap(r.combs[i][channel].delay.buffer) {
				return errDAVFreeverbGeometry
			}
		}
	}
	for i, lengths := range allpassLengths {
		for channel, length := range lengths {
			if length > cap(r.allpasses[i][channel].delay.buffer) {
				return errDAVFreeverbGeometry
			}
		}
	}

	for i, lengths := range combLengths {
		for channel, length := range lengths {
			r.combs[i][channel].delay.setLength(length)
		}
	}
	for i := len(combLengths); i < r.numCombs; i++ {
		for channel := range r.combs[i] {
			delay := r.combs[i][channel].delay
			delay.setLength(0)
			r.combs[i][channel] = davFreeverbComb32{delay: delay}
		}
	}
	for i, lengths := range allpassLengths {
		for channel, length := range lengths {
			r.allpasses[i][channel].delay.setLength(length)
		}
	}
	for i := len(allpassLengths); i < r.numPasses; i++ {
		for channel := range r.allpasses[i] {
			delay := r.allpasses[i][channel].delay
			delay.setLength(0)
			r.allpasses[i][channel] = davFreeverbAllpass32{delay: delay}
		}
	}
	r.numCombs, r.numPasses = len(combLengths), len(allpassLengths)
	return nil
}

// SetCombFeedback sets the feedback and damping from zero to one for all
// currently active comb stages. A feedback of one and damping of zero is
// usable for a frozen tail with muted input.
func (r *DAVFreeverb32) SetCombFeedback(feedback, damping float32) error {
	if !(feedback >= 0 && feedback <= 1 && damping >= 0 && damping <= 1) {
		return errDAVFreeverbFeedback
	}
	dampB := 1 - damping
	for i := range r.numCombs {
		for channel := range r.combs[i] {
			stage := &r.combs[i][channel]
			stage.feedback, stage.dampA, stage.dampB = feedback, damping, dampB
		}
	}
	return nil
}

// SetAllpassFeedback sets the signed shape of each active allpass stage.
// Supply one left/right pair per active stage, with each value in [0, 1].
func (r *DAVFreeverb32) SetAllpassFeedback(feedback [][2]float32) error {
	if len(feedback) != r.numPasses {
		return errDAVFreeverbFeedback
	}
	for _, pair := range feedback {
		for _, value := range pair {
			if !(value >= 0 && value <= 1) {
				return errDAVFreeverbFeedback
			}
		}
	}
	for i, pair := range feedback {
		for channel, value := range pair {
			r.allpasses[i][channel].feedback = value
		}
	}
	return nil
}

// Reset clears the active delay and comb filter history, keeping the
// configured feedback, damping and geometry.
func (r *DAVFreeverb32) Reset() {
	for i := range r.numCombs {
		for channel := range r.combs[i] {
			stage := &r.combs[i][channel]
			stage.delay.reset()
			stage.filterStore = 0
		}
	}
	for i := range r.numPasses {
		for channel := range r.allpasses[i] {
			r.allpasses[i][channel].delay.reset()
		}
	}
}

// ProcessSample sends one summed, scaled input into both networks and
// returns their wet samples without allocating.
func (r *DAVFreeverb32) ProcessSample(input float32) (left, right float32) {
	for i := range r.numCombs {
		left += r.combs[i][0].process(input)
		right += r.combs[i][1].process(input)
	}
	for i := range r.numPasses {
		left = r.allpasses[i][0].process(left)
		right = r.allpasses[i][1].process(right)
	}
	return left, right
}
