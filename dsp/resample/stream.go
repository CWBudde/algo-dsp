package resample

// Stream is a bounded mono resampler that removes ceil(FIR output delay).
// Finite streams append zeros internally and emit exactly FrameCount samples;
// indefinite streams keep continuous filter state for repeated input loops.
// Processing, flushing, and Reset allocate nothing. Distinct clones may run
// concurrently; a single stream must be used sequentially. Input and dst must
// not overlap. Fractional group delay is intentionally rounded upwards rather
// than interpolated, matching integer-frame causal transport alignment.
type Stream struct {
	plan                       StreamPlan
	filter                     *Resampler
	raw, zeros                 []float64
	sourceFrames, targetFrames int64
	read, written              int64
	initialSkip, skip          int
}

// NewStream designs the plan's FIR and reserves bounded scratch. sourceFrames
// is the known finite source length, or -1 for an indefinite stream. Duration
// overflow is checked before any allocation. Check WorkspaceBytes first when
// constructing plans from untrusted rates. Equal rates bypass filtering.
func (p StreamPlan) NewStream(sourceFrames int64) (*Stream, error) {
	if p.inputBlock <= 0 || sourceFrames < -1 {
		return nil, ErrInvalidStream
	}

	target := int64(-1)

	if sourceFrames >= 0 {
		var err error

		target, err = FrameCount(sourceFrames, p.inRate, p.outRate)
		if err != nil {
			return nil, err
		}
	}

	s := &Stream{plan: p, sourceFrames: sourceFrames, targetFrames: target}
	if p.up == p.down {
		return s, nil
	}

	filter, err := NewRational(p.up, p.down, WithQuality(p.quality), WithTapsPerPhase(p.taps))
	if err != nil {
		return nil, err
	}

	s.filter = filter
	// ceil((prototype length - 1)/(2*down)), entirely in integer arithmetic.
	numerator, denominator := uint64(p.taps)*uint64(p.up)-1, 2*uint64(p.down)

	s.initialSkip = int(numerator / denominator)
	if numerator%denominator != 0 {
		s.initialSkip++
	}

	s.skip = s.initialSkip
	s.raw, s.zeros = make([]float64, p.outputBlock), make([]float64, p.inputBlock)

	return s, nil
}

// Clone creates a fresh stream with the same plan and finite duration. It shares
// immutable filter coefficients but reserves independent histories and scratch;
// neither source samples nor the current processing position are copied.
func (s *Stream) Clone() *Stream {
	c := &Stream{plan: s.plan, sourceFrames: s.sourceFrames, targetFrames: s.targetFrames, initialSkip: s.initialSkip, skip: s.initialSkip}
	if s.filter != nil {
		c.filter = s.filter.Clone()
		c.raw, c.zeros = make([]float64, len(s.raw)), make([]float64, len(s.zeros))
	}

	return c
}

// Reset restores the initial position and filter history without allocating.
func (s *Stream) Reset() {
	s.read, s.written, s.skip = 0, 0, s.initialSkip
	if s.filter != nil {
		s.filter.Reset()
	}
}

// InputFrames returns source frames consumed, excluding internal tail zeros.
func (s *Stream) InputFrames() int64 { return s.read }

// OutputFrames returns delay-compensated frames emitted so far.
func (s *Stream) OutputFrames() int64 { return s.written }

// Done reports whether a finite stream has emitted its exact target duration.
// An indefinite stream is never done.
func (s *Stream) Done() bool { return s.targetFrames >= 0 && s.written == s.targetFrames }

// PredictOutputLen returns the compensated output count for the next input
// block, including initial delay removal and finite-duration trimming. Internal
// raw workspace belongs to Stream, so dst may have exactly this many samples.
// Nonpositive lengths return zero. Processing still validates source duration
// and the configured input block limit before touching destination or state.
func (s *Stream) PredictOutputLen(inputLen int) int {
	if inputLen <= 0 {
		return 0
	}

	n := inputLen
	if s.filter != nil {
		n = s.filter.PredictOutputLen(inputLen)
		n -= min(s.skip, n)
	}

	if s.targetFrames >= 0 {
		n = int(minStreamFrames(int64(n), s.targetFrames-s.written))
	}

	return n
}

// ProcessInto consumes an entire source block and emits its compensated output.
// Blocks cannot exceed InputBlockFrames or the remaining finite source length.
// Empty input does nothing. Errors leave destination and stream state unchanged.
func (s *Stream) ProcessInto(dst, input []float64) (int, error) {
	if len(input) > s.plan.inputBlock || (s.sourceFrames >= 0 && int64(len(input)) > s.sourceFrames-s.read) {
		return 0, ErrInvalidStream
	}

	if s.sourceFrames < 0 && int64(len(input)) > int64(^uint64(0)>>1)-s.read {
		return 0, ErrOutputTooLarge
	}

	n := s.PredictOutputLen(len(input))
	if len(dst) < n {
		return 0, ErrShortDst
	}

	if s.sourceFrames < 0 && int64(n) > int64(^uint64(0)>>1)-s.written {
		return 0, ErrOutputTooLarge
	}

	written, err := s.process(dst, input)
	if err != nil {
		return 0, err
	}

	s.read += int64(len(input))

	return written, nil
}

// FlushInto advances one bounded zero-input block after a finite source has
// been completely consumed. It returns only the remaining exact-duration tail.
// done can be false with zero output while filter delay is still being skipped.
// Calls after completion return (0,true,nil). Indefinite or incomplete sources
// return ErrInvalidStream. ErrShortDst leaves destination and state unchanged.
func (s *Stream) FlushInto(dst []float64) (written int, done bool, err error) {
	if s.sourceFrames < 0 || s.read != s.sourceFrames {
		return 0, false, ErrInvalidStream
	}

	if s.Done() {
		return 0, true, nil
	}

	count := s.flushInputFrames()

	n := s.PredictOutputLen(count)
	if len(dst) < n {
		return 0, false, ErrShortDst
	}

	written, err = s.process(dst, s.zeros[:count])

	return written, s.Done(), err
}

func (s *Stream) process(dst, input []float64) (int, error) {
	if len(input) == 0 {
		return 0, nil
	}

	n := len(input)
	start := 0

	if s.filter != nil {
		var err error

		n, err = s.filter.ProcessInto(s.raw, input)
		if err != nil {
			return 0, err
		}

		start = min(n, s.skip)
		s.skip -= start
	}

	kept := n - start
	if s.targetFrames >= 0 {
		kept = int(minStreamFrames(int64(kept), s.targetFrames-s.written))
	}

	if s.filter == nil {
		copy(dst[:kept], input[:kept])
	} else {
		copy(dst[:kept], s.raw[start:start+kept])
	}

	s.written += int64(kept)

	return kept, nil
}

func minStreamFrames(a, b int64) int64 {
	if a < b {
		return a
	}

	return b
}

// flushInputFrames avoids processing a large reserved block when a finite tail
// needs only a few zero frames. Prediction is monotonic at the current rational
// phase, so this bounded search finds the smallest finishing block without any
// wide count multiplication or assumptions about the source block boundaries.
func (s *Stream) flushInputFrames() int {
	remaining := s.targetFrames - s.written

	high := len(s.zeros)
	if int64(s.PredictOutputLen(high)) < remaining {
		return high
	}

	low := 1
	for low < high {
		middle := low + (high-low)/2
		if int64(s.PredictOutputLen(middle)) >= remaining {
			high = middle
		} else {
			low = middle + 1
		}
	}

	return low
}
