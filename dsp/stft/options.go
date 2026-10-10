package stft

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

// Padding selects how frames are placed and how samples outside the signal
// are filled.
type Padding int

const (
	// PadNone places frame i at samples [i*hop, i*hop+nfft) and only emits
	// frames that lie completely inside the signal.
	PadNone Padding = iota
	// PadZero centres frame i on sample i*hop and treats samples outside the
	// signal as zero. This is the default.
	PadZero
	// PadReflect centres frame i on sample i*hop and mirrors the signal at
	// its edges without repeating the edge sample (numpy/torch "reflect").
	PadReflect
)

// String returns the name of the padding mode.
func (p Padding) String() string {
	switch p {
	case PadNone:
		return "none"
	case PadZero:
		return "zero"
	case PadReflect:
		return "reflect"
	default:
		return fmt.Sprintf("Padding(%d)", int(p))
	}
}

func (p Padding) valid() bool {
	return p == PadNone || p == PadZero || p == PadReflect
}

// Option configures a [Transform].
type Option func(*config) error

type config struct {
	windowType   window.Type
	windowOpts   []window.Option
	customWindow []float64
	padding      Padding
	normalized   bool
}

func defaultConfig() config {
	return config{
		windowType: window.TypeHann,
		padding:    PadZero,
	}
}

// WithWindow selects the analysis/synthesis window type. The window is
// generated in its periodic form, window.Generate(t, nfft,
// window.WithPeriodic(), opts...); opts are passed through, for example
// window.WithAlpha for Kaiser or Tukey. The default is a periodic Hann window.
// An unknown window type (see [window.Type.Valid]) is rejected with
// [ErrInvalidWindow].
func WithWindow(t window.Type, opts ...window.Option) Option {
	optsCopy := append([]window.Option(nil), opts...)

	return func(cfg *config) error {
		// window.Generate silently treats unknown types as rectangular, so
		// reject them here.
		if !t.Valid() {
			return fmt.Errorf("%w: unknown window type %v", ErrInvalidWindow, t)
		}

		cfg.windowType = t
		cfg.windowOpts = optsCopy
		cfg.customWindow = nil

		return nil
	}
}

// WithCustomWindow uses the given coefficients as window. The slice is
// copied; its length must equal nfft and all values must be finite.
func WithCustomWindow(w []float64) Option {
	wCopy := append([]float64(nil), w...)

	return func(cfg *config) error {
		if len(wCopy) == 0 {
			return fmt.Errorf("%w: empty custom window", ErrInvalidWindow)
		}

		cfg.customWindow = wCopy

		return nil
	}
}

// WithCenter selects the framing and padding mode (default [PadZero]).
func WithCenter(p Padding) Option {
	return func(cfg *config) error {
		if !p.valid() {
			return fmt.Errorf("%w: %v", ErrInvalidPadding, p)
		}

		cfg.padding = p

		return nil
	}
}

// WithNormalized scales the forward transform by 1/sqrt(nfft), as
// torch.stft(normalized=True) does. Inverse undoes the scaling, so
// Forward/Inverse remain a reconstructing pair.
func WithNormalized() Option {
	return func(cfg *config) error {
		cfg.normalized = true

		return nil
	}
}
