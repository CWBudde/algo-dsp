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
	// ErrInvalidEpsilon is returned when a negative magnitude floor is given.
	ErrInvalidEpsilon = errors.New("mixedphase: epsilon must not be negative")
	// ErrInvalidWindowAlpha is returned when a negative window alpha is given.
	ErrInvalidWindowAlpha = errors.New(
		"mixedphase: window alpha must not be negative",
	)
	// ErrInvalidMethod is returned when an unknown reconstruction method is
	// requested.
	ErrInvalidMethod = errors.New(
		"mixedphase: unknown minimum-phase reconstruction method",
	)
)

// MinimumPhaseMethod selects how a magnitude response is factored into a
// causal minimum-phase spectrum.
//
// Both methods evaluate the same Hilbert relation between log magnitude and
// phase on the same dense grid and therefore agree to within rounding on
// well-conditioned targets. They differ in how the result is assembled, which
// matters once the target spans a large dynamic range.
type MinimumPhaseMethod int

const (
	// MethodCepstrum folds the real cepstrum onto the causal half and
	// exponentiates the resulting complex log spectrum. Magnitude and phase
	// are both produced by the exponential, so the reconstructed magnitude
	// only approximates the target. This is the zero value.
	MethodCepstrum MinimumPhaseMethod = iota

	// MethodHilbert evaluates the discrete Hilbert transform of the log
	// magnitude to obtain the phase alone and pairs it with the (floored)
	// target magnitude. The magnitude is therefore reproduced exactly on the
	// design grid and never passes through an exponential.
	MethodHilbert
)

// String implements [fmt.Stringer].
func (m MinimumPhaseMethod) String() string {
	switch m {
	case MethodCepstrum:
		return "cepstrum"
	case MethodHilbert:
		return "hilbert"
	default:
		return "unknown"
	}
}

func (m MinimumPhaseMethod) valid() bool {
	return m == MethodCepstrum || m == MethodHilbert
}

// MinimumPhaseConfig configures [MinimumPhaseWith].
type MinimumPhaseConfig struct {
	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the prototype length.
	FFTSize int

	// Epsilon is the magnitude floor applied before taking logarithms. Zero
	// selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the reconstruction algorithm. The zero value is
	// [MethodCepstrum].
	Method MinimumPhaseMethod
}

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
	// spectral division. Zero selects a scale-relative default. Negative
	// values are rejected.
	Epsilon float64

	// Window selects the truncation window for both factors. The
	// minimum-phase part receives its right-hand slope; the linear-phase
	// residual receives the symmetric form. The zero value is rectangular.
	Window window.Type

	// WindowAlpha supplies the alpha or beta parameter for parametric
	// windows. Zero uses the window package default. Negative values are
	// rejected.
	WindowAlpha float64

	// Method selects the minimum-phase reconstruction used for the
	// minimum-phase factor. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod

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
	// Zero selects a scale-relative default. Negative values are rejected.
	Epsilon float64

	// Method selects the minimum-phase reconstruction whose phase is
	// interpolated. The zero value is [MethodCepstrum].
	Method MinimumPhaseMethod
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
