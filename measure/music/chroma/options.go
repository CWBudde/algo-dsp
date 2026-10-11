package chroma

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// Default analysis parameters.
const (
	// DefaultReferenceHz is the default tuning reference, the frequency of
	// A4 (MIDI note 69) in Hz.
	DefaultReferenceHz = pitch.DefaultReferenceHz
	// DefaultBinsPerOctave is the default CQT resolution: three bins per
	// semitone, centred on the semitone and a third of a semitone either
	// side of it.
	DefaultBinsPerOctave = 36
	// DefaultHop is the default hop size in input samples (23.2 ms at
	// 22.05 kHz, 10.7 ms at 48 kHz).
	DefaultHop = 512
	// DefaultMinHz is the default lower edge of the folded band:
	// C1 − 50 cents at A4 = 440 Hz, 440·2^(−45.5/12) Hz.
	DefaultMinHz = 31.772199163987512
	// DefaultMaxHz is the default upper edge of the folded band:
	// B7 + 50 cents at A4 = 440 Hz, 440·2^(38.5/12) Hz.
	DefaultMaxHz = 4066.8414929904015
)

// tolSemitones is the tolerance in semitones of the band edges and of the
// half-way test: a bin within it of a band edge counts as inside, a bin
// within it of the midpoint between two semitones is split between them.
const tolSemitones = 1e-6

// Normalization selects how each frame's 12 values are scaled.
type Normalization int

const (
	// NormMax divides each frame by its strongest class, so that class is
	// 1, as in melody.Result.Chroma. This is the default.
	NormMax Normalization = iota + 1
	// NormL1 divides each frame by the sum of its classes.
	NormL1
	// NormL2 divides each frame by its Euclidean norm.
	NormL2
	// NormNone leaves the folded power unscaled.
	NormNone
)

// String returns the name of the normalization.
func (n Normalization) String() string {
	switch n {
	case NormMax:
		return "max"
	case NormL1:
		return "l1"
	case NormL2:
		return "l2"
	case NormNone:
		return "none"
	default:
		return fmt.Sprintf("Normalization(%d)", int(n))
	}
}

func (n Normalization) valid() bool { return n >= NormMax && n <= NormNone }

// Option configures a [Chromagram]. Options return an error wrapping
// [ErrInvalidOption] for out-of-range values.
type Option func(*config) error

type config struct {
	referenceHz   float64
	minHz, maxHz  float64
	binsPerOctave int
	cqtOpts       []cqt.Option
	norm          Normalization
}

func defaultConfig() config {
	return config{
		referenceHz:   DefaultReferenceHz,
		minHz:         DefaultMinHz,
		maxHz:         DefaultMaxHz,
		binsPerOctave: DefaultBinsPerOctave,
		norm:          NormMax,
	}
}

func finitePositive(v float64) bool { return v > 0 && !math.IsInf(v, 0) }

// WithReferenceHz sets the tuning reference, the frequency of A4 in Hz
// (default [DefaultReferenceHz]). It moves both the derived CQT bins, which
// are centred on the semitones of the tuned scale, and the folding of bins to
// pitch classes. It must be positive and finite.
func WithReferenceHz(hz float64) Option {
	return func(cfg *config) error {
		if !finitePositive(hz) {
			return fmt.Errorf("%w: reference %g Hz", ErrInvalidOption, hz)
		}

		cfg.referenceHz = hz

		return nil
	}
}

// WithFrequencyRange sets the folded band in Hz (default [DefaultMinHz] to
// [DefaultMaxHz]). Only CQT bins whose centre frequency lies in
// [minHz, maxHz] (to within 1e-6 semitone) are folded; the derived CQT
// covers exactly the bins of the tuned grid inside the band. It needs
// 0 < minHz < maxHz, both finite.
func WithFrequencyRange(minHz, maxHz float64) Option {
	return func(cfg *config) error {
		if !finitePositive(minHz) || !finitePositive(maxHz) || minHz >= maxHz {
			return fmt.Errorf("%w: frequency range %g..%g Hz", ErrInvalidOption, minHz, maxHz)
		}

		cfg.minHz, cfg.maxHz = minHz, maxHz

		return nil
	}
}

// WithBinsPerOctave sets the CQT resolution in bins per octave (default
// [DefaultBinsPerOctave]). It must be a positive multiple of 12, so that the
// bins lie on a fixed grid of k = n/12 bins per semitone.
func WithBinsPerOctave(n int) Option {
	return func(cfg *config) error {
		if n < 12 || n%12 != 0 {
			return fmt.Errorf("%w: bins per octave %d is not a positive multiple of 12", ErrInvalidOption, n)
		}

		cfg.binsPerOctave = n

		return nil
	}
}

// WithCQT adds further options of the underlying constant-Q transform, such
// as [cqt.WithHopLength], [cqt.WithFilterScale], [cqt.WithWindow],
// [cqt.WithCenter] or [cqt.WithEarlyDownsampling]. They are applied after
// the derived options (see the package documentation) and so override them;
// repeated calls accumulate in order. [cqt.WithOutput] is always overridden
// by magnitude output. The options are validated by [New].
func WithCQT(opts ...cqt.Option) Option {
	optsCopy := append([]cqt.Option(nil), opts...)

	return func(cfg *config) error {
		cfg.cqtOpts = append(cfg.cqtOpts, optsCopy...)

		return nil
	}
}

// WithNormalization selects the per-frame normalization (default
// [NormMax]).
func WithNormalization(n Normalization) Option {
	return func(cfg *config) error {
		if !n.valid() {
			return fmt.Errorf("%w: normalization %v", ErrInvalidOption, n)
		}

		cfg.norm = n

		return nil
	}
}
