package pitch

import (
	"context"
	"fmt"
	"math"
)

// TimeStretch exposes the existing WSOLA stage without its pitch-shift
// resampling. PitchRatio is the duration multiplier: 2 doubles duration while
// retaining pitch. The returned slice is owned by the caller.
func (p *PitchShifter) TimeStretch(input []float64) ([]float64, error) {
	if p == nil {
		return nil, fmt.Errorf("pitch.stretch: nil processor")
	}

	for _, x := range input {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("pitch.stretch: nonfinite input")
		}
	}

	if len(input) == 0 {
		return []float64{}, nil
	}

	if len(input) > 1<<28 {
		return nil, fmt.Errorf("pitch.stretch: input limit")
	}

	if p.pitchRatio == 1 {
		return append([]float64(nil), input...), nil
	}

	return p.timeStretch(input), nil
}

// StretchReader supplies borrowed valid sample intervals for each channel.
type (
	StretchReader func(channel int, start int64, dst []float64) int
	// StretchStream uses the same WSOLA sequence, correlation and crossfade as
	// PitchShifter.TimeStretch. All channels share one alignment, preserving stereo
	// phase. A Step emits at most one sequence advance with fixed memory usage.
	StretchStream struct {
		p                       *PitchShifter
		read                    StretchReader
		frames, target, written int64
		channels                int
		previous                int64
		step                    int64
		reference, search       []float64
		input, output, tail     [][]float64
		failure                 error
	}
)

// NewStretchStream constructs a bounded stretcher for 1..8 channels and a
// duration ratio in [0.25,4]. Sources and target lengths must fit exact integers.
func NewStretchStream(sampleRate, ratio float64, frames int64, channels int, read StretchReader) (*StretchStream, error) {
	if frames < 1 || frames > 1<<48 || channels < 1 || channels > 8 || read == nil || sampleRate < 8000 || sampleRate > 384000 {
		return nil, fmt.Errorf("pitch.stretch: invalid source")
	}

	p, err := NewPitchShifter(sampleRate)
	if err != nil {
		return nil, err
	}

	if err = p.SetPitchRatio(ratio); err != nil {
		return nil, err
	}

	s := &StretchStream{p: p, read: read, frames: frames, target: int64(math.Round(float64(frames) * ratio)), channels: channels, reference: make([]float64, p.overlapLen), search: make([]float64, 2*p.searchLen+p.overlapLen), input: make([][]float64, channels), output: make([][]float64, channels), tail: make([][]float64, channels)}

	s.target = max(1, s.target)
	for i := range channels {
		s.input[i] = make([]float64, p.sequenceLen)
		s.output[i] = make([]float64, p.stepOut)
		s.tail[i] = make([]float64, p.overlapLen)
	}

	return s, nil
}

// OutputFrames reports the exact rounded target duration.
func (s *StretchStream) OutputFrames() int64 { return s.target }

// Step emits one contiguous planar block on every channel. Its callback borrows
// the buffer. Errors and cancellation are terminal; invalid audio never emits.
func (s *StretchStream) Step(ctx context.Context, emit func(int, int64, []float64) error) (bool, error) {
	if s == nil || ctx == nil || emit == nil {
		return false, fmt.Errorf("pitch.stretch.step: invalid call")
	}

	if s.failure != nil {
		return false, s.failure
	}

	if err := ctx.Err(); err != nil {
		s.failure = err
		return false, err
	}

	if s.written >= s.target {
		return true, nil
	}

	start := int64(0)

	if s.step > 0 {
		nominal := int64(math.Round(float64(s.step) * float64(s.p.stepOut) / s.p.pitchRatio))
		if s.p.pitchRatio == 1 {
			start = nominal
		} else {
			if err := s.readPadded(0, s.previous+int64(s.p.stepOut), s.reference); err != nil {
				s.failure = err
				return false, err
			}

			low := nominal - int64(s.p.searchLen)
			if err := s.readPadded(0, low, s.search); err != nil {
				s.failure = err
				return false, err
			}

			best, score := s.p.searchLen, math.Inf(-1)

			energy := pitchShifterTiny
			for _, x := range s.reference {
				energy += x * x
			}
			// Coarse then local search bounds work without changing the correlation law.
			evaluate := func(at int) {
				dot, other := 0.0, pitchShifterTiny

				for i, x := range s.reference {
					y := s.search[at+i]
					dot += x * y
					other += y * y
				}

				v := dot / math.Sqrt(energy*other)
				if v > score {
					score = v
					best = at
				}
			}
			for at := 0; at <= 2*s.p.searchLen; at += 4 {
				evaluate(at)
			}

			near := best
			for at := max(0, near-3); at <= min(2*s.p.searchLen, near+3); at++ {
				evaluate(at)
			}

			start = low + int64(best)
		}
	}

	for channel := range s.channels {
		if err := s.readPadded(channel, start, s.input[channel]); err != nil {
			s.failure = err
			return false, err
		}
	}

	n := int(min(int64(s.p.stepOut), s.target-s.written))
	for channel := range s.channels {
		copy(s.output[channel], s.input[channel][:s.p.stepOut])

		if s.step > 0 && s.p.pitchRatio != 1 {
			for i := range min(n, s.p.overlapLen) {
				s.output[channel][i] = s.tail[channel][i]*s.p.fadeOut[i] + s.input[channel][i]*s.p.fadeIn[i]
			}
		}

		copy(s.tail[channel], s.input[channel][s.p.stepOut:])
	}

	if err := ctx.Err(); err != nil {
		s.failure = err
		return false, err
	}

	for channel := range s.channels {
		if err := emit(channel, s.written, s.output[channel][:n]); err != nil {
			s.failure = err
			return false, err
		}
	}

	s.previous = start
	s.written += int64(n)
	s.step++

	return s.written == s.target, nil
}

func (s *StretchStream) readPadded(channel int, start int64, dst []float64) error {
	clear(dst)

	lo := max(int64(0), -start)

	hi := min(int64(len(dst)), s.frames-start)
	if hi > lo && s.read(channel, start+lo, dst[lo:hi]) != int(hi-lo) {
		return fmt.Errorf("pitch.stretch: short source read")
	}

	for _, x := range dst {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("pitch.stretch: nonfinite input")
		}
	}

	return nil
}
