// Package streamframe provides bounded overlap-add scheduling for streaming
// spectral effects. Sample positions and FFT scheduling do not depend on blocks.
package streamframe

import "errors"

// Transform consumes one complete, unwindowed analysis frame and writes a
// windowed synthesis frame of the same size. Its buffers are reusable scratch.
type Transform func(input, output []float64) error

// Engine owns input history and normalized synthesis overlap-add storage.
type Engine struct {
	window                          []float64
	hop                             int
	transform                       Transform
	input, wet, norm, frame, output []float64
	count                           int64
	nextFrame                       int64
	mask                            int64
}

// New allocates bounded history for a frame/hop combination.
func New(window []float64, hop int, transform Transform) (*Engine, error) {
	if len(window) < 2 || hop < 1 || hop >= len(window) || transform == nil {
		return nil, errors.New("streamframe: invalid configuration")
	}

	size := 1
	for size < 4*len(window) {
		size *= 2
	}

	return &Engine{window: window, hop: hop, transform: transform, input: make([]float64, size), wet: make([]float64, size), norm: make([]float64, size), frame: make([]float64, len(window)), output: make([]float64, len(window)), mask: int64(size - 1)}, nil
}

// Latency returns the lookahead required for a complete first analysis frame.
func (e *Engine) Latency() int { return len(e.window) }

// Reset clears history and rewinds the frame schedule without allocation.
func (e *Engine) Reset() {
	clear(e.input)
	clear(e.wet)
	clear(e.norm)
	clear(e.frame)
	clear(e.output)
	e.count = 0
	e.nextFrame = 0
}

// ProcessInPlace performs normalized overlap-add with latency-aligned dry mix.
// Transform failures are returned immediately; built-in transforms cannot fail
// after their configuration has been validated.
func (e *Engine) ProcessInPlace(block []float64, mix float64) error {
	n := int64(len(e.window))
	for i, x := range block {
		position := e.count
		e.input[position&e.mask] = x

		e.count++
		if e.count >= e.nextFrame+n {
			for j := range e.frame {
				e.frame[j] = e.input[(e.nextFrame+int64(j))&e.mask]
			}

			if err := e.transform(e.frame, e.output); err != nil {
				return err
			}

			for j, y := range e.output {
				slot := (e.nextFrame + int64(j)) & e.mask
				e.wet[slot] += y
				e.norm[slot] += e.window[j] * e.window[j]
			}

			e.nextFrame += int64(e.hop)
		}

		outPosition := position - n
		if outPosition < 0 {
			block[i] = 0
			continue
		}

		slot := outPosition & e.mask

		wet := e.wet[slot]
		if e.norm[slot] > 1e-12 {
			wet /= e.norm[slot]
		}

		block[i] = e.input[slot]*(1-mix) + wet*mix
		e.wet[slot] = 0
		e.norm[slot] = 0
	}

	return nil
}
