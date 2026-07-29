package mixedphase

import (
	"errors"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

var (
	// ErrEmptyPrototype is returned when a design receives no prototype taps.
	ErrEmptyPrototype = errors.New("mixedphase: empty prototype")
	// ErrInvalidLength is returned when the requested FIR length is invalid.
	ErrInvalidLength = errors.New("mixedphase: invalid filter length")
	// ErrInvalidDelay is returned when the requested pre-delay does not fit.
	ErrInvalidDelay = errors.New("mixedphase: invalid delay")
	// ErrInvalidPhaseMix is returned when phase mix is outside [0, 1].
	ErrInvalidPhaseMix = errors.New("mixedphase: phase mix must be in [0, 1]")
)

// IterativeConfig configures the DAGA 2012 alternating factorisation.
type IterativeConfig struct {
	// Length is the number of taps in the resulting FIR. Zero uses the
	// prototype length.
	Length int

	// Delay is the group delay, in samples, contributed by the linear-phase
	// factor. The linear factor has length 2*Delay+1. Consequently Delay must
	// be in [0, (Length-1)/2].
	Delay int

	// Iterations is the maximum number of alternating correction passes.
	// Zero uses a default of 12. A negative value returns the initial
	// uncorrected factorisation, which is useful for comparisons.
	Iterations int

	// FFTSize controls the dense frequency grid. Zero selects a power of two
	// at least eight times the filter length.
	FFTSize int

	// Epsilon is the magnitude floor used by logarithms and regularised
	// spectral division. Zero selects a scale-relative default.
	Epsilon float64

	// Window selects the truncation window for both factors. The
	// minimum-phase part receives its right-hand slope; the linear-phase
	// residual receives the symmetric form. The zero value is rectangular.
	Window window.Type

	// WindowAlpha supplies the alpha or beta parameter for parametric
	// windows. Zero uses the window package default.
	WindowAlpha float64

	// ToleranceDB stops the iteration once the change in RMS magnitude error
	// falls below this value. Zero uses 1e-7 dB. A negative value disables
	// early stopping.
	ToleranceDB float64
}

// PhaseInterpolationConfig configures the direct frequency-domain baseline.
type PhaseInterpolationConfig struct {
	// Length is the number of output taps. Zero uses the prototype length.
	Length int

	// Mix interpolates the unwrapped target phase: zero is minimum phase and
	// one is linear phase with delay (Length-1)/2.
	Mix float64

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the output length.
	FFTSize int

	// Epsilon is the magnitude floor used by minimum-phase reconstruction.
	// Zero selects a scale-relative default.
	Epsilon float64
}

// Result contains a designed FIR and method-specific intermediate data.
type Result struct {
	// Taps is the final causal FIR.
	Taps []float64

	// MinimumPhasePart and LinearPhasePart contain the two factors produced by
	// [DesignIterative]. They are nil for [DesignPhaseInterpolation].
	MinimumPhasePart []float64
	LinearPhasePart  []float64

	// Iterations is the number of alternating correction passes performed.
	Iterations int

	// Metrics compares Taps with the prototype magnitude response.
	Metrics Metrics
}

// Metrics describes spectral error and the time distribution of an FIR.
type Metrics struct {
	// RMSMagnitudeErrorDB and MaxMagnitudeErrorDB compare dB magnitudes on a
	// dense FFT grid with both responses floored 120 dB below the reference
	// peak.
	RMSMagnitudeErrorDB float64
	MaxMagnitudeErrorDB float64

	// RelativeMagnitudeError is the L2 norm of the linear-magnitude error
	// divided by the L2 norm of the reference magnitude. Unlike the dB
	// metrics, it is not dominated by differences deep in a stopband.
	RelativeMagnitudeError float64

	// PeakIndex is the index of the largest absolute coefficient.
	PeakIndex int

	// EnergyCentroid is the first moment of squared tap magnitude in samples.
	EnergyCentroid float64

	// PrePeakEnergyRatio is the fraction of total energy before PeakIndex.
	PrePeakEnergyRatio float64
}
