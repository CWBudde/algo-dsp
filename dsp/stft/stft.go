package stft

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrInvalidSize reports an FFT size below 2 or a hop below 1.
	ErrInvalidSize = errors.New("stft: invalid size")
	// ErrNilOption reports a nil [Option] passed to [New] or [New32].
	ErrNilOption = errors.New("stft: nil option")
	// ErrInvalidPadding reports an unknown [Padding] value.
	ErrInvalidPadding = errors.New("stft: invalid padding")
	// ErrInvalidWindow reports an unknown window type, or a window of the
	// wrong length, with non-finite coefficients, or with all coefficients
	// zero.
	ErrInvalidWindow = errors.New("stft: invalid window")
	// ErrShortDst reports a destination slice shorter than required.
	ErrShortDst = errors.New("stft: destination too short")
	// ErrFrameRange reports a frame index outside the valid range.
	ErrFrameRange = errors.New("stft: frame index out of range")
	// ErrSignalTooShort reports a signal too short for reflect padding
	// (PadReflect needs more than nfft/2 samples).
	ErrSignalTooShort = errors.New("stft: signal too short for reflect padding")
	// ErrInvalidSpectrum reports a spectrogram frame whose length is not
	// nfft/2+1, or a negative output length.
	ErrInvalidSpectrum = errors.New("stft: invalid spectrum")
	// ErrWindowSumZero reports an output sample of [Transform.Inverse] that
	// no frame covers with a non-zero window value.
	ErrWindowSumZero = errors.New("stft: window sum is zero")
)

// windowSumFloor is the smallest window-sum-square for which Inverse still
// divides; below it the sample is considered unrecoverable.
const windowSumFloor = 1e-10

// Transform computes the STFT of real signals of precision F and its
// inverse. Use [New] for float64 ([STFT]) or [New32] for float32 ([STFT32]).
//
// A Transform holds scratch buffers and is not safe for concurrent use; see
// [Transform.Clone].
type Transform[F algofft.Float, C algofft.Complex] struct {
	nfft       int
	hop        int
	bins       int
	padding    Padding
	normalized bool

	window []F       // analysis window
	synth  []float64 // synthesis window, including the inverse scaling
	winSq  []float64 // squared analysis window
	plan   *algofft.PlanReal[F, C]

	frame []F // FrameInto / Inverse scratch, length nfft
	spec  []C // Inverse scratch, length bins
}

// STFT is the float64 [Transform].
type STFT = Transform[float64, complex128]

// STFT32 is the float32 [Transform].
type STFT32 = Transform[float32, complex64]

// New returns a float64 STFT with FFT size nfft (>= 2) and hop size hop
// (>= 1). Without options it uses a periodic Hann window, centred framing
// with zero padding and an unscaled forward FFT.
func New(nfft, hop int, opts ...Option) (*STFT, error) {
	return newTransform[float64, complex128](nfft, hop, opts)
}

// New32 is [New] for float32 signals. The window is computed in float64 and
// rounded to float32; the inverse accumulates in float64.
func New32(nfft, hop int, opts ...Option) (*STFT32, error) {
	return newTransform[float32, complex64](nfft, hop, opts)
}

func newTransform[F algofft.Float, C algofft.Complex](nfft, hop int, opts []Option) (*Transform[F, C], error) {
	if nfft < 2 {
		return nil, fmt.Errorf("%w: nfft must be >= 2, got %d", ErrInvalidSize, nfft)
	}

	if hop < 1 {
		return nil, fmt.Errorf("%w: hop must be >= 1, got %d", ErrInvalidSize, hop)
	}

	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	w, err := buildWindow(cfg, nfft)
	if err != nil {
		return nil, err
	}

	plan, err := algofft.NewPlanReal[F, C](nfft)
	if err != nil {
		return nil, fmt.Errorf("stft: create FFT plan: %w", err)
	}

	invScale := 1.0
	if cfg.normalized {
		invScale = math.Sqrt(float64(nfft))
	}

	t := &Transform[F, C]{
		nfft:       nfft,
		hop:        hop,
		bins:       nfft/2 + 1,
		padding:    cfg.padding,
		normalized: cfg.normalized,
		window:     make([]F, nfft),
		synth:      make([]float64, nfft),
		winSq:      make([]float64, nfft),
		plan:       plan,
	}

	for k, v := range w {
		t.window[k] = F(v)
		wk := float64(t.window[k])
		t.synth[k] = wk * invScale
		t.winSq[k] = wk * wk
	}

	t.allocScratch()

	return t, nil
}

func buildWindow(cfg config, nfft int) ([]float64, error) {
	var w []float64

	if cfg.customWindow != nil {
		if len(cfg.customWindow) != nfft {
			return nil, fmt.Errorf("%w: custom window length %d != nfft %d",
				ErrInvalidWindow, len(cfg.customWindow), nfft)
		}

		w = cfg.customWindow
	} else {
		opts := append([]window.Option{window.WithPeriodic()}, cfg.windowOpts...)
		w = window.Generate(cfg.windowType, nfft, opts...)
	}

	nonZero := false

	for _, v := range w {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("%w: non-finite coefficient", ErrInvalidWindow)
		}

		if v != 0 {
			nonZero = true
		}
	}

	if !nonZero {
		return nil, fmt.Errorf("%w: all coefficients are zero", ErrInvalidWindow)
	}

	return w, nil
}

func (t *Transform[F, C]) allocScratch() {
	t.frame = make([]F, t.nfft)
	t.spec = make([]C, t.bins)
}

// Clone returns an independent Transform with the same configuration. The
// window and FFT plan are shared (both are immutable); scratch buffers are
// not, so the clone can be used concurrently with the original.
func (t *Transform[F, C]) Clone() *Transform[F, C] {
	c := *t
	c.allocScratch()

	return &c
}

// NFFT returns the FFT size.
func (t *Transform[F, C]) NFFT() int { return t.nfft }

// Hop returns the hop size in samples.
func (t *Transform[F, C]) Hop() int { return t.hop }

// Bins returns the number of frequency bins per frame, nfft/2+1. Bin k is at
// k*sampleRate/nfft.
func (t *Transform[F, C]) Bins() int { return t.bins }

// Padding returns the framing and padding mode.
func (t *Transform[F, C]) Padding() Padding { return t.padding }

// Normalized reports whether the forward transform is scaled by 1/sqrt(nfft).
func (t *Transform[F, C]) Normalized() bool { return t.normalized }

// Window returns a copy of the analysis window.
func (t *Transform[F, C]) Window() []F {
	return append([]F(nil), t.window...)
}

// FrameCount returns the number of frames [Transform.Forward] produces for a
// signal of n samples: ceil(n/hop) for centred framing (PadZero,
// PadReflect), 1+(n-nfft)/hop for PadNone when n >= nfft, and 0 for n <= 0
// or a PadNone signal shorter than nfft.
func (t *Transform[F, C]) FrameCount(n int) int {
	if n <= 0 {
		return 0
	}

	if t.padding == PadNone {
		if n < t.nfft {
			return 0
		}

		return 1 + (n-t.nfft)/t.hop
	}

	return (n + t.hop - 1) / t.hop
}

// frameStart returns the signal index of sample 0 of the given frame.
func (t *Transform[F, C]) frameStart(frame int) int {
	if t.padding == PadNone {
		return frame * t.hop
	}

	return frame*t.hop - t.nfft/2
}

// FrameInto computes the spectrum of one frame of x into dst[:Bins()].
//
// Valid frames are 0 <= frame < FrameCount(len(x)); with centred framing
// frame == len(x)/hop is accepted too (the extra trailing frame of
// torch.stft when hop divides len(x)). PadReflect with an odd nfft accepts
// frames up to (len(x)-1)/hop only, the last frame the nfft/2 reflected
// samples still cover. FrameInto does not allocate. It uses the
// Transform's scratch buffer and must not be called concurrently.
func (t *Transform[F, C]) FrameInto(dst []C, x []F, frame int) error {
	if len(dst) < t.bins {
		return fmt.Errorf("%w: dst has %d bins, need %d", ErrShortDst, len(dst), t.bins)
	}

	err := t.checkFrame(len(x), frame)
	if err != nil {
		return err
	}

	t.fillFrame(x, frame)

	return t.forward(dst[:t.bins])
}

func (t *Transform[F, C]) checkFrame(n, frame int) error {
	if t.padding == PadReflect && n <= t.nfft/2 {
		return fmt.Errorf("%w: %d samples, need > %d", ErrSignalTooShort, n, t.nfft/2)
	}

	last := t.FrameCount(n) - 1

	switch t.padding {
	case PadReflect:
		// torch.stft reflects nfft/2 samples on each side; the last frame
		// must end inside that padded signal of n + 2*(nfft/2) samples,
		// which for odd nfft is one sample short of n + nfft.
		last = (n - t.nfft%2) / t.hop
	case PadZero:
		if n > 0 {
			last = n / t.hop
		}
	case PadNone:
	}

	if frame < 0 || frame > last {
		return fmt.Errorf("%w: frame %d, valid 0..%d", ErrFrameRange, frame, last)
	}

	return nil
}

// fillFrame writes the windowed, padded samples of frame into t.frame. The
// frame must have been validated with checkFrame.
func (t *Transform[F, C]) fillFrame(x []F, frame int) {
	n := len(x)
	buf, w := t.frame, t.window
	start := t.frameStart(frame)

	// Samples k in [lo, hi) lie inside the signal.
	lo := min(max(-start, 0), t.nfft)
	hi := max(min(n-start, t.nfft), lo)

	if t.padding == PadReflect {
		for k := range lo {
			buf[k] = x[-(start+k)] * w[k]
		}

		for k := hi; k < t.nfft; k++ {
			buf[k] = x[2*(n-1)-(start+k)] * w[k]
		}
	} else {
		clear(buf[:lo])
		clear(buf[hi:])
	}

	for k := lo; k < hi; k++ {
		buf[k] = x[start+k] * w[k]
	}
}

func (t *Transform[F, C]) forward(dst []C) error {
	var err error
	if t.normalized {
		err = t.plan.ForwardUnitary(dst, t.frame)
	} else {
		err = t.plan.Forward(dst, t.frame)
	}

	if err != nil {
		return fmt.Errorf("stft: forward FFT: %w", err)
	}

	return nil
}

// Forward returns the spectrogram of x, frame-major: spec[i][k] is bin k of
// frame i, with FrameCount(len(x)) frames of Bins() bins each. All frames
// share one contiguous backing array.
func (t *Transform[F, C]) Forward(x []F) ([][]C, error) {
	if t.padding == PadReflect && len(x) <= t.nfft/2 {
		return nil, fmt.Errorf("%w: %d samples, need > %d", ErrSignalTooShort, len(x), t.nfft/2)
	}

	frames := t.FrameCount(len(x))
	data := make([]C, frames*t.bins)
	spec := make([][]C, frames)

	for i := range spec {
		spec[i] = data[i*t.bins : (i+1)*t.bins : (i+1)*t.bins]
		t.fillFrame(x, i)

		err := t.forward(spec[i])
		if err != nil {
			return nil, err
		}
	}

	return spec, nil
}

// Inverse reconstructs length samples from a spectrogram laid out as
// [Transform.Forward] returns it: frame i is placed at the same position
// Forward reads it from. Each frame is inverse transformed, multiplied by the
// window and overlap-added; every output sample is then divided by the sum
// of squared window values that overlap it. The imaginary parts of the DC
// bin and (for even nfft) the Nyquist bin are ignored, as in numpy's irfft.
//
// spec is not modified. Every frame must have exactly Bins() bins. Inverse
// returns [ErrWindowSumZero] if a sample in [0, length) has a window sum
// below 1e-10, for example because no frame covers it.
func (t *Transform[F, C]) Inverse(spec [][]C, length int) ([]F, error) {
	if length < 0 {
		return nil, fmt.Errorf("%w: negative length %d", ErrInvalidSpectrum, length)
	}

	for i, frame := range spec {
		if len(frame) != t.bins {
			return nil, fmt.Errorf("%w: frame %d has %d bins, need %d", ErrInvalidSpectrum, i, len(frame), t.bins)
		}
	}

	acc := make([]float64, length)
	wss := make([]float64, length)

	for i, frame := range spec {
		start := t.frameStart(i)
		lo := min(max(-start, 0), t.nfft)
		hi := max(min(length-start, t.nfft), lo)

		if lo == hi {
			continue
		}

		err := t.inverseFrame(frame)
		if err != nil {
			return nil, err
		}

		for k := lo; k < hi; k++ {
			acc[start+k] += float64(t.frame[k]) * t.synth[k]
			wss[start+k] += t.winSq[k]
		}
	}

	out := make([]F, length)

	for j := range out {
		if wss[j] < windowSumFloor {
			return nil, fmt.Errorf("%w: sample %d (sum %g)", ErrWindowSumZero, j, wss[j])
		}

		out[j] = F(acc[j] / wss[j])
	}

	return out, nil
}

// inverseFrame writes the inverse real FFT of frame into t.frame.
func (t *Transform[F, C]) inverseFrame(frame []C) error {
	copy(t.spec, frame)
	dropEdgeImag(t.spec, t.nfft%2 == 0)

	err := t.plan.Inverse(t.frame, t.spec)
	if err != nil {
		return fmt.Errorf("stft: inverse FFT: %w", err)
	}

	return nil
}

// dropEdgeImag zeroes the imaginary part of the DC bin and, if nyquist is
// set, of the last bin, as numpy's irfft ignores them.
func dropEdgeImag[C algofft.Complex](spec []C, nyquist bool) {
	last := len(spec) - 1

	switch s := any(spec).(type) {
	case []complex64:
		s[0] = complex(real(s[0]), 0)
		if nyquist {
			s[last] = complex(real(s[last]), 0)
		}
	case []complex128:
		s[0] = complex(real(s[0]), 0)
		if nyquist {
			s[last] = complex(real(s[last]), 0)
		}
	}
}
