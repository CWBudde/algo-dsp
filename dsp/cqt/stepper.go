package cqt

import "fmt"

// Stepper computes one constant-Q transform in bounded steps, for callers
// that must not block for a whole [Transform.ProcessInto] call, such as a
// job scheduler with a time budget per step. Each [Stepper.Step] performs
// about maxWork work units (see [Transform.Work]) and returns; the output in
// dst is complete when Step reports done, and it is bit-identical to
// ProcessInto's (or ProcessInto32's) for every sequence of budgets.
//
// Create a Stepper with [Transform.NewStepper] or [Transform.NewStepper32]
// and reuse it for the next signal with [Stepper.Reset]. A Stepper owns its
// scratch buffers and only reads the Transform's immutable configuration, so
// the Transform may run ProcessInto between steps, and several Steppers of
// one Transform may run on different goroutines. A single Stepper is not
// safe for concurrent use. The zero Stepper is done and has no Transform to
// Reset with.
type Stepper[T float32 | float64] struct {
	p   pass
	dst []T
}

// NewStepper returns a Stepper that writes the constant-Q transform of x to
// dst[:OutputLen(len(x))], with the layout of [Transform.ProcessInto]. It
// validates the lengths as ProcessInto does and returns [ErrSignalTooShort]
// or [ErrShortDst]. x is copied, so the caller may reuse it at once; dst is
// written by the steps and must not be modified until the Stepper is done.
// NewStepper allocates the scratch buffers; the steps do not allocate.
func (t *Transform) NewStepper(dst, x []float64) (*Stepper[float64], error) {
	return newStepper(t, dst, x)
}

// NewStepper32 is [Transform.NewStepper] for float32 input and output, the
// stepped form of [Transform.ProcessInto32]: the input is converted to
// float64, the transform is computed in float64, and only the result is
// rounded to float32.
func (t *Transform) NewStepper32(dst, x []float32) (*Stepper[float32], error) {
	return newStepper(t, dst, x)
}

func newStepper[T float32 | float64](t *Transform, dst, x []T) (*Stepper[T], error) {
	s := &Stepper[T]{}
	s.p.t = t

	err := s.Reset(dst, x)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Reset starts a new transform of x into dst with the same Transform,
// discarding any unfinished one. It validates and copies like
// [Transform.NewStepper] and does not allocate if x is not longer than the
// longest input the Stepper has seen. On error the Stepper is unchanged.
func (s *Stepper[T]) Reset(dst, x []T) error {
	t := s.p.t
	if t == nil {
		return fmt.Errorf("%w: zero Stepper has no Transform", ErrStepperDone)
	}

	err := t.checkLengths(len(dst), len(x))
	if err != nil {
		return err
	}

	s.dst = dst
	s.p.start(t, loadInput(&s.p, x))

	return nil
}

// Step performs at least maxWork work units of the transform, or the rest
// of it if fewer remain, and reports whether the transform is complete. The
// unit in progress when the budget is used up is finished, so a step
// exceeds maxWork by less than one unit. A unit is one frame of one octave's
// kernel correlation (the summed kernel lengths, a few thousand work units
// for typical configurations) or one decimated sample (256). A step also
// pads the octaves it starts, which counts no work.
//
// Step returns [ErrMaxWork] if maxWork is less than 1 and [ErrStepperDone]
// once the transform is complete; neither changes the Stepper. Pass
// [math.MaxInt] to finish in one step.
func (s *Stepper[T]) Step(maxWork int) (bool, error) {
	if maxWork < 1 {
		return false, fmt.Errorf("%w: %d", ErrMaxWork, maxWork)
	}

	if s.p.phase == phaseDone {
		return true, ErrStepperDone
	}

	advance(&s.p, s.dst, maxWork)

	return s.p.phase == phaseDone, nil
}

// Progress returns the fraction of the work units performed, from 0 before
// the first step to exactly 1 once the transform is complete. It never
// decreases until [Stepper.Reset].
func (s *Stepper[T]) Progress() float64 {
	if s.p.phase == phaseDone {
		return 1
	}

	return float64(s.p.done) / float64(s.p.total)
}
