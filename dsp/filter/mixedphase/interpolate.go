package mixedphase

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

// DesignPhaseInterpolation interpolates between the unwrapped minimum-phase
// response and a pure-delay linear-phase response, then projects that complex
// response onto the requested finite causal support.
//
// This direct method is intentionally simple. It provides a useful baseline
// for measuring how much the alternating factorisation recovers from the
// truncation error inherent in phase interpolation.
func DesignPhaseInterpolation(
	prototype []float64,
	cfg PhaseInterpolationConfig,
) (Result, error) {
	if len(prototype) == 0 {
		return Result{}, ErrEmptyPrototype
	}

	if cfg.Mix < 0 || cfg.Mix > 1 {
		return Result{}, ErrInvalidPhaseMix
	}

	if cfg.Epsilon < 0 {
		return Result{}, fmt.Errorf("%w: got %g", ErrInvalidEpsilon, cfg.Epsilon)
	}

	if !cfg.Method.valid() {
		return Result{}, fmt.Errorf("%w: %d", ErrInvalidMethod, int(cfg.Method))
	}

	length := cfg.Length
	if length == 0 {
		length = len(prototype)
	}

	if length <= 0 {
		return Result{}, ErrInvalidLength
	}

	fftSize, err := nextDesignFFTSize(max(length, len(prototype)), cfg.FFTSize)
	if err != nil {
		return Result{}, err
	}

	w, err := newFFTWorkspace(fftSize)
	if err != nil {
		return Result{}, err
	}

	targetSpectrum, err := w.forwardReal(prototype)
	if err != nil {
		return Result{}, err
	}

	targetMagnitude := magnitude(targetSpectrum)
	epsilon := defaultEpsilon(targetMagnitude, cfg.Epsilon)

	minimumSpectrum, err := minimumPhaseSpectrum(
		w,
		targetMagnitude,
		epsilon,
		cfg.Method,
	)
	if err != nil {
		return Result{}, err
	}

	half := fftSize / 2

	minimumPhase := make([]float64, half+1)
	for i := range minimumPhase {
		minimumPhase[i] = cmplx.Phase(minimumSpectrum[i])
	}

	minimumPhase = spectrum.UnwrapPhase(minimumPhase)

	delay := float64(length-1) / 2
	desired := make([]complex128, fftSize)

	for i := 0; i <= half; i++ {
		omega := 2 * math.Pi * float64(i) / float64(fftSize)
		linearPhase := -omega * delay
		phase := (1-cfg.Mix)*minimumPhase[i] + cfg.Mix*linearPhase
		value := cmplx.Rect(targetMagnitude[i], phase)

		switch {
		case i == 0:
			desired[i] = complex(real(value), 0)
		case i == half && fftSize%2 == 0:
			desired[i] = complex(real(value), 0)
		default:
			desired[i] = value
			desired[fftSize-i] = cmplx.Conj(value)
		}
	}

	impulse, err := w.inverseReal(desired)
	if err != nil {
		return Result{}, fmt.Errorf(
			"mixedphase: project interpolated phase: %w",
			err,
		)
	}

	taps := append([]float64(nil), impulse[:length]...)

	metrics, err := Analyze(prototype, taps, fftSize)
	if err != nil {
		return Result{}, err
	}

	return Result{Taps: taps, Metrics: metrics}, nil
}
