package cqt

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// Norm selects how each kernel of the top octave is normalized before use
// (nnAudio's basis_norm). The numeric values match nnAudio's integers.
type Norm int

const (
	// NormNone leaves the kernels unnormalized (basis_norm=0). Each kernel
	// is still divided by its length, as in nnAudio.
	NormNone Norm = 0
	// NormL1 divides each kernel by the sum of its complex magnitudes
	// (basis_norm=1). This is the default.
	NormL1 Norm = 1
	// NormL2 divides each kernel by its Euclidean norm (basis_norm=2).
	NormL2 Norm = 2
)

// String returns the name of the kernel normalization.
func (n Norm) String() string {
	switch n {
	case NormNone:
		return "none"
	case NormL1:
		return "l1"
	case NormL2:
		return "l2"
	default:
		return fmt.Sprintf("Norm(%d)", int(n))
	}
}

func (n Norm) valid() bool { return n == NormNone || n == NormL1 || n == NormL2 }

// Padding selects how each octave's signal is extended by nfft/2 samples on
// both sides before the kernels are applied (nnAudio's pad_mode).
type Padding int

const (
	// PadReflect mirrors the signal at its edges without repeating the edge
	// sample (numpy/torch "reflect"). This is the default. An octave whose
	// signal is not longer than nfft/2 samples is zero padded instead, as
	// nnAudio does when torch's reflection pad rejects it.
	PadReflect Padding = iota
	// PadConstant pads with zeros.
	PadConstant
)

// String returns the name of the padding mode.
func (p Padding) String() string {
	switch p {
	case PadReflect:
		return "reflect"
	case PadConstant:
		return "constant"
	default:
		return fmt.Sprintf("Padding(%d)", int(p))
	}
}

func (p Padding) valid() bool { return p == PadReflect || p == PadConstant }

// Normalization selects the final per-bin scaling (nnAudio's
// normalization_type). Every variant also multiplies by the early
// downsampling factor.
type Normalization int

const (
	// NormalizationLibrosa multiplies bin b by sqrt(Lengths()[b]), matching
	// the magnitude scale of librosa's CQT. This is the default.
	NormalizationLibrosa Normalization = iota
	// NormalizationConvolutional applies no further scaling.
	NormalizationConvolutional
	// NormalizationWrap multiplies every bin by 2.
	NormalizationWrap
)

// String returns the name of the normalization.
func (n Normalization) String() string {
	switch n {
	case NormalizationLibrosa:
		return "librosa"
	case NormalizationConvolutional:
		return "convolutional"
	case NormalizationWrap:
		return "wrap"
	default:
		return fmt.Sprintf("Normalization(%d)", int(n))
	}
}

func (n Normalization) valid() bool {
	return n == NormalizationLibrosa || n == NormalizationConvolutional || n == NormalizationWrap
}

// Output selects the output representation.
type Output int

const (
	// OutputMagnitude writes one magnitude per frame and bin. This is the
	// default.
	OutputMagnitude Output = iota
	// OutputComplex writes the real and imaginary part per frame and bin,
	// interleaved.
	OutputComplex
)

// String returns the name of the output representation.
func (o Output) String() string {
	switch o {
	case OutputMagnitude:
		return "magnitude"
	case OutputComplex:
		return "complex"
	default:
		return fmt.Sprintf("Output(%d)", int(o))
	}
}

func (o Output) valid() bool { return o == OutputMagnitude || o == OutputComplex }

// Option configures a [Transform].
type Option func(*config) error

type config struct {
	hop             int
	fmin            float64
	nBins           int
	binsPerOctave   int
	filterScale     float64
	windowType      window.Type
	windowOpts      []window.Option
	basisNorm       Norm
	padding         Padding
	earlyDownsample bool
	normalization   Normalization
	output          Output
}

// defaultConfig returns nnAudio CQT2010v2's defaults.
func defaultConfig() config {
	return config{
		hop:             512,
		fmin:            32.70,
		nBins:           84,
		binsPerOctave:   12,
		filterScale:     1,
		windowType:      window.TypeHann,
		basisNorm:       NormL1,
		padding:         PadReflect,
		earlyDownsample: true,
		normalization:   NormalizationLibrosa,
		output:          OutputMagnitude,
	}
}

func finitePositive(v float64) bool { return v > 0 && !math.IsInf(v, 0) }

// WithHopLength sets the hop size in input samples (default 512). It must be
// at least 1, and after early downsampling it must be divisible by
// 2^(octaves-1); see [New].
func WithHopLength(hop int) Option {
	return func(cfg *config) error {
		if hop < 1 {
			return fmt.Errorf("%w: hop length %d < 1", ErrInvalidOption, hop)
		}

		cfg.hop = hop

		return nil
	}
}

// WithFMin sets the centre frequency of the lowest bin in Hz (default 32.70).
func WithFMin(fmin float64) Option {
	return func(cfg *config) error {
		if !finitePositive(fmin) {
			return fmt.Errorf("%w: fmin %g", ErrInvalidOption, fmin)
		}

		cfg.fmin = fmin

		return nil
	}
}

// WithBins sets the number of frequency bins (nnAudio's n_bins, default 84).
func WithBins(n int) Option {
	return func(cfg *config) error {
		if n < 1 {
			return fmt.Errorf("%w: bins %d < 1", ErrInvalidOption, n)
		}

		cfg.nBins = n

		return nil
	}
}

// WithBinsPerOctave sets the number of bins per octave (default 12).
func WithBinsPerOctave(n int) Option {
	return func(cfg *config) error {
		if n < 1 {
			return fmt.Errorf("%w: bins per octave %d < 1", ErrInvalidOption, n)
		}

		cfg.binsPerOctave = n

		return nil
	}
}

// WithFilterScale scales the kernel lengths (default 1). The quality factor
// is Q = filterScale / (2^(1/binsPerOctave) - 1).
func WithFilterScale(s float64) Option {
	return func(cfg *config) error {
		if !finitePositive(s) {
			return fmt.Errorf("%w: filter scale %g", ErrInvalidOption, s)
		}

		cfg.filterScale = s

		return nil
	}
}

// WithWindow selects the kernel window (default [window.TypeHann]). Each
// kernel of length l uses the periodic window
// window.Generate(t, l, opts..., window.WithPeriodic()), as scipy's
// get_window(window, l, fftbins=True) does; a kernel of length 1 uses the
// coefficient 1. opts are passed through, for example window.WithAlpha for
// Kaiser.
//
// nnAudio's CQT2010v2 accepts a window argument but never forwards it to its
// kernel builder, so nnAudio (and basic-pitch) always use Hann. This package
// honours the selected window.
func WithWindow(t window.Type, opts ...window.Option) Option {
	optsCopy := append([]window.Option(nil), opts...)

	return func(cfg *config) error {
		// TypeFreeCosine is the last window.Type; window.Generate silently
		// treats unknown types as rectangular, so reject them here.
		if t < window.TypeRectangular || t > window.TypeFreeCosine {
			return fmt.Errorf("%w: window type %d", ErrInvalidOption, int(t))
		}

		cfg.windowType = t
		cfg.windowOpts = optsCopy

		return nil
	}
}

// WithBasisNorm selects the kernel normalization (default [NormL1]).
func WithBasisNorm(n Norm) Option {
	return func(cfg *config) error {
		if !n.valid() {
			return fmt.Errorf("%w: basis norm %v", ErrInvalidOption, n)
		}

		cfg.basisNorm = n

		return nil
	}
}

// WithPadding selects the padding mode (default [PadReflect]).
func WithPadding(p Padding) Option {
	return func(cfg *config) error {
		if !p.valid() {
			return fmt.Errorf("%w: padding %v", ErrInvalidOption, p)
		}

		cfg.padding = p

		return nil
	}
}

// WithEarlyDownsampling enables or disables early downsampling (default
// true). When enabled, [New] decides as nnAudio does whether the input can be
// decimated by a power of two before the top octave is analysed; the hop
// size and sample rate are divided by that factor and the output is
// multiplied by it.
func WithEarlyDownsampling(enabled bool) Option {
	return func(cfg *config) error {
		cfg.earlyDownsample = enabled

		return nil
	}
}

// WithNormalization selects the final per-bin scaling (default
// [NormalizationLibrosa]).
func WithNormalization(n Normalization) Option {
	return func(cfg *config) error {
		if !n.valid() {
			return fmt.Errorf("%w: normalization %v", ErrInvalidOption, n)
		}

		cfg.normalization = n

		return nil
	}
}

// WithOutput selects magnitude or complex output (default
// [OutputMagnitude]).
func WithOutput(o Output) Option {
	return func(cfg *config) error {
		if !o.valid() {
			return fmt.Errorf("%w: output %v", ErrInvalidOption, o)
		}

		cfg.output = o

		return nil
	}
}
