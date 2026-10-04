package stft

import (
	"fmt"
	"math"

	algofft "github.com/cwbudde/algo-fft"
)

// AnalysisStream emits the same frames as Transform.Forward while retaining
// only two FFT windows. Callback bins are borrowed until the next callback and
// must be copied by a retaining caller. One caller owns the stream and callback.
type AnalysisStream[F algofft.Float, C algofft.Complex] struct {
	t               *Transform[F, C]
	ring            []F
	bins            []C
	count, frame    int64
	closing, closed bool
	failure         error
}

// Stream is the float64 streaming STFT.
type Stream = AnalysisStream[float64, complex128]

// Stream32 is the float32 streaming STFT.
type Stream32 = AnalysisStream[float32, complex64]

// NewStream constructs a float64 streaming STFT with the same options as New.
func NewStream(nfft, hop int, opts ...Option) (*Stream, error) {
	if nfft < 2 || nfft > 65536 || hop < 1 || hop > nfft {
		return nil, fmt.Errorf("stft.stream: %w", ErrInvalidSize)
	}

	t, err := New(nfft, hop, opts...)
	if err != nil {
		return nil, err
	}

	return t.AnalysisStream()
}

// NewStream32 constructs a float32 streaming STFT with the same options as New32.
func NewStream32(nfft, hop int, opts ...Option) (*Stream32, error) {
	if nfft < 2 || nfft > 65536 || hop < 1 || hop > nfft {
		return nil, fmt.Errorf("stft.stream: %w", ErrInvalidSize)
	}

	t, err := New32(nfft, hop, opts...)
	if err != nil {
		return nil, err
	}

	return t.AnalysisStream()
}

// AnalysisStream creates an independently owned streaming analyser. Streaming
// sizes are bounded to NFFT<=65536 and Hop<=NFFT. All processing is allocation-free.
func (t *Transform[F, C]) AnalysisStream() (*AnalysisStream[F, C], error) {
	if t == nil || t.nfft < 2 || t.hop < 1 || t.nfft > 65536 || t.hop > t.nfft {
		return nil, fmt.Errorf("stft.stream: %w", ErrInvalidSize)
	}

	return &AnalysisStream[F, C]{t: t.Clone(), ring: make([]F, 2*t.nfft), bins: make([]C, t.bins)}, nil
}

// Reset rewinds framing, errors and EOF state without reallocating.
func (s *AnalysisStream[F, C]) Reset() {
	clear(s.ring)
	s.count = 0
	s.frame = 0
	s.closing = false
	s.closed = false
	s.failure = nil
}

// Process consumes at most65536 finite samples and emits every complete frame.
// Invalid input is rejected before any state changes. A callback error makes the
// stream terminal until Reset; previously emitted frames cannot be rolled back.
func (s *AnalysisStream[F, C]) Process(input []F, emit func(int64, []C) error) error {
	if s == nil || s.t == nil || emit == nil || s.closing || s.failure != nil {
		return fmt.Errorf("stft.stream.process: invalid state or callback")
	}

	if len(input) > 65536 || s.count > 1<<52-int64(len(input)) {
		return fmt.Errorf("stft.stream.process: input limit")
	}

	for _, x := range input {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return fmt.Errorf("stft.stream.process: nonfinite input")
		}
	}

	for _, x := range input {
		s.ring[s.count%int64(len(s.ring))] = x

		s.count++
		for s.ready() {
			if err := s.emitFrame(emit); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *AnalysisStream[F, C]) start() int64 {
	start := s.frame * int64(s.t.hop)
	if s.t.padding != PadNone {
		start -= int64(s.t.nfft / 2)
	}

	return start
}

func (s *AnalysisStream[F, C]) ready() bool {
	if s.t.padding == PadReflect && s.count <= int64(s.t.nfft/2) {
		return false
	}

	return s.start()+int64(s.t.nfft) <= s.count
}

func (s *AnalysisStream[F, C]) emitFrame(emit func(int64, []C) error) error {
	start := s.start()
	for i := range s.t.frame {
		position := start + int64(i)
		x := F(0)

		if s.t.padding == PadReflect {
			if position < 0 {
				position = -position
			}

			if position >= s.count {
				position = 2*(s.count-1) - position
			}
		}

		if position >= 0 && position < s.count {
			x = s.ring[position%int64(len(s.ring))]
		}

		s.t.frame[i] = x * s.t.window[i]
	}

	if err := s.t.forward(s.bins); err != nil {
		s.failure = err
		return err
	}

	if err := emit(s.frame, s.bins); err != nil {
		s.failure = err
		return err
	}

	s.frame++

	return nil
}

// FinishStep zero/reflect pads EOF and emits at most maxFrames trailing frames.
// It returns true once all Forward frames have been emitted. PadNone discards
// incomplete frames. Repeated completed calls are harmless; Process after EOF
// requires Reset. The work budget must be positive and no greater than1024.
func (s *AnalysisStream[F, C]) FinishStep(maxFrames int, emit func(int64, []C) error) (bool, error) {
	if s == nil || s.t == nil || emit == nil || maxFrames < 1 || maxFrames > 1024 || s.failure != nil {
		return false, fmt.Errorf("stft.stream.finish: invalid state or budget")
	}

	if s.closed {
		return true, nil
	}

	if s.t.padding == PadReflect && s.count <= int64(s.t.nfft/2) {
		return false, fmt.Errorf("stft.stream.finish: %w", ErrSignalTooShort)
	}

	s.closing = true

	frames := int64(0)
	if s.count > 0 {
		frames = (s.count + int64(s.t.hop) - 1) / int64(s.t.hop)
	}

	if s.t.padding == PadNone {
		frames = 0
		if s.count >= int64(s.t.nfft) {
			frames = 1 + (s.count-int64(s.t.nfft))/int64(s.t.hop)
		}
	}

	for work := 0; work < maxFrames && s.frame < frames; work++ {
		if err := s.emitFrame(emit); err != nil {
			return false, err
		}
	}

	s.closed = s.frame == frames

	return s.closed, nil
}
