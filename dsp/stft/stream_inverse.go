package stft

import (
	"fmt"
	"math"

	algofft "github.com/cwbudde/algo-fft"
)

// SynthesisStream overlap-adds ordered STFT frames into a bounded ring. Samples
// are normalized by their accumulated squared windows, as Transform.Inverse.
// Callback samples are borrowed; output offsets are absolute and contiguous.
type SynthesisStream[F algofft.Float, C algofft.Complex] struct {
	t                      *Transform[F, C]
	acc, weights           []float64
	output                 []F
	length, frame, written int64
	closed, closing        bool
	failure                error
}

// InverseStream is the float64 streaming inverse STFT.
type InverseStream = SynthesisStream[float64, complex128]

// InverseStream32 is the float32 streaming inverse STFT.
type InverseStream32 = SynthesisStream[float32, complex64]

// InverseStream creates independent synthesis state for exactly length output
// samples. The matching analysis configuration must be used for input frames.
// NFFT<=65536 and Hop<=NFFT bound storage; no complete source is retained.
func (t *Transform[F, C]) InverseStream(length int64) (*SynthesisStream[F, C], error) {
	if t == nil || t.nfft < 2 || t.hop < 1 || t.nfft > 65536 || t.hop > t.nfft || length < 0 || length > 1<<52 {
		return nil, fmt.Errorf("stft.inverse.stream: %w", ErrInvalidSize)
	}

	return &SynthesisStream[F, C]{t: t.Clone(), acc: make([]float64, 2*t.nfft), weights: make([]float64, 2*t.nfft), output: make([]F, t.nfft), length: length}, nil
}

// Reset rewinds overlap-add state for another signal of the configured length.
func (s *SynthesisStream[F, C]) Reset() {
	clear(s.acc)
	clear(s.weights)
	s.frame = 0
	s.written = 0
	s.closed = false
	s.closing = false
	s.failure = nil
}

// ProcessFrame consumes the next ordered frame without changing the caller's
// bins. Validation is atomic; callback errors are terminal until Reset.
func (s *SynthesisStream[F, C]) ProcessFrame(bins []C, emit func(int64, []F) error) error {
	if s == nil || s.t == nil || s.closed || s.closing || s.failure != nil || emit == nil || len(bins) != s.t.bins {
		return fmt.Errorf("stft.inverse.stream: invalid state, callback or bins")
	}

	for _, bin := range bins {
		value := complex128(bin)
		if math.IsNaN(real(value)) || math.IsNaN(imag(value)) || math.IsInf(real(value), 0) || math.IsInf(imag(value), 0) {
			return fmt.Errorf("stft.inverse.stream: nonfinite bins")
		}
	}

	start := s.frame * int64(s.t.hop)
	if s.t.padding != PadNone {
		start -= int64(s.t.nfft / 2)
	}

	if err := s.t.inverseFrame(bins); err != nil {
		s.failure = err
		return err
	}

	for i := range s.t.frame {
		position := start + int64(i)
		if position < 0 || position >= s.length {
			continue
		}

		at := position % int64(len(s.acc))
		s.acc[at] += float64(s.t.frame[i]) * s.t.synth[i]
		s.weights[at] += s.t.winSq[i]
	}

	s.frame++

	return s.emitUntil(min(max(start+int64(s.t.hop), 0), s.length), emit)
}

func (s *SynthesisStream[F, C]) emitUntil(end int64, emit func(int64, []F) error) error {
	for s.written < end {
		n := int(min(end-s.written, int64(len(s.output))))
		for i := 0; i < n; i++ {
			at := (s.written + int64(i)) % int64(len(s.acc))
			if s.weights[at] < windowSumFloor {
				s.failure = fmt.Errorf("stft.inverse.stream: sample%d: %w", s.written+int64(i), ErrWindowSumZero)
				return s.failure
			}

			s.output[i] = F(s.acc[at] / s.weights[at])
			s.acc[at] = 0
			s.weights[at] = 0
		}

		if err := emit(s.written, s.output[:n]); err != nil {
			s.failure = err
			return err
		}

		s.written += int64(n)
	}

	return nil
}

// FinishStep emits at most maxSamples of the normalized final tail. Missing
// coverage returns ErrWindowSumZero. Budget is bounded to65536 samples.
func (s *SynthesisStream[F, C]) FinishStep(maxSamples int, emit func(int64, []F) error) (bool, error) {
	if s == nil || s.t == nil || emit == nil || maxSamples < 1 || maxSamples > 65536 || s.failure != nil {
		return false, fmt.Errorf("stft.inverse.stream.finish: invalid state or budget")
	}

	if s.closed {
		return true, nil
	}

	s.closing = true
	if err := s.emitUntil(min(s.length, s.written+int64(maxSamples)), emit); err != nil {
		return false, err
	}

	s.closed = s.written == s.length

	return s.closed, nil
}
