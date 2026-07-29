package mixedphase

import (
	"fmt"
	"math"
	"math/cmplx"
)

const (
	defaultMinimaxTolerance = 1e-4

	// minimaxWeightFloor keeps reweighting from zeroing a band completely,
	// which would make the normal equations singular.
	minimaxWeightFloor = 1e-9
)

// DesignComplexLeastSquares approximates a prescribed complex response by
// weighted least squares, optionally followed by Lawson reweighting for
// peak-error control.
//
// The target is the same mixed-phase family [DesignPhaseInterpolation] uses:
// the prototype magnitude paired with a phase interpolated between minimum
// phase and a pure delay. The two designs differ only in how that target is
// projected onto the finite tap budget. Phase interpolation truncates the
// inverse transform, which is the least-squares solution only for uniform
// weighting; this design solves the weighted normal equations, so a frequency
// weight actually shapes where the error goes.
//
// With MinimaxIterations above zero the weight is multiplied by the current
// error magnitude after every solve. That is Lawson's algorithm: the weighted
// least-squares sequence converges towards the Chebyshev solution, trading RMS
// error for a flatter error envelope. The iterate with the smallest peak error
// is returned, because the sequence is not monotone.
func DesignComplexLeastSquares(
	prototype []float64,
	cfg ComplexLeastSquaresConfig,
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

	if cfg.MinimaxIterations < 0 {
		return Result{}, fmt.Errorf(
			"%w: minimax iterations must not be negative, got %d",
			ErrInvalidLength,
			cfg.MinimaxIterations,
		)
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

	weight, err := expandWeight(cfg.Weight, fftSize)
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

	desired := prescribedResponse(
		w,
		targetMagnitude,
		minimumSpectrum,
		cfg.Mix,
		float64(length-1)/2,
	)

	reference := 0.0
	for i := range fftSize/2 + 1 {
		reference = max(reference, cmplx.Abs(desired[i]))
	}

	if reference == 0 {
		return Result{}, fmt.Errorf(
			"mixedphase: prescribed response is identically zero",
		)
	}

	taps, complexError, iterations, err := solveWeighted(
		w,
		desired,
		weight,
		length,
		reference,
		cfg,
	)
	if err != nil {
		return Result{}, err
	}

	metrics, err := analyzeAgainstSpectrum(w, targetSpectrum, taps)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Taps:         taps,
		Iterations:   iterations,
		Metrics:      metrics,
		ComplexError: complexError,
	}, nil
}

// solveWeighted performs the initial least-squares solve and the requested
// Lawson reweighting passes, returning the iterate with the smallest peak
// error.
func solveWeighted(
	w *fftWorkspace,
	desired []complex128,
	weight []float64,
	length int,
	reference float64,
	cfg ComplexLeastSquaresConfig,
) ([]float64, ComplexErrorMetrics, int, error) {
	tolerance := cfg.MinimaxTolerance
	if tolerance == 0 {
		tolerance = defaultMinimaxTolerance
	}

	var (
		bestTaps  []float64
		bestError ComplexErrorMetrics
		performed int
	)

	bestError.Peak = math.Inf(1)
	previousPeak := math.Inf(1)

	for pass := 0; pass <= cfg.MinimaxIterations; pass++ {
		taps, err := leastSquaresTaps(w, desired, weight, length)
		if err != nil {
			return nil, ComplexErrorMetrics{}, 0, err
		}

		deviation, err := complexDeviation(w, taps, desired)
		if err != nil {
			return nil, ComplexErrorMetrics{}, 0, err
		}

		current := summariseDeviation(deviation, reference)
		if current.Peak < bestError.Peak {
			bestTaps, bestError = taps, current
		}

		if pass == cfg.MinimaxIterations {
			break
		}

		performed = pass + 1

		if tolerance >= 0 &&
			math.Abs(previousPeak-current.Peak) <= tolerance*current.Peak {
			break
		}

		previousPeak = current.Peak

		reweight(weight, deviation)
	}

	return bestTaps, bestError, performed, nil
}

// leastSquaresTaps solves the weighted normal equations for the given weight.
//
// Both the autocorrelation of the weight and the weighted cross-correlation
// with the target are single inverse transforms, because the design grid is a
// uniform DFT grid: R[m][n] collapses to a Toeplitz sequence and the
// right-hand side is the real part of the inverse transform of weight*desired.
// The shared 1/N scaling of the two transforms cancels in the solve.
func leastSquaresTaps(
	w *fftWorkspace,
	desired []complex128,
	weight []float64,
	length int,
) ([]float64, error) {
	weightSpectrum := make([]complex128, w.size)
	weighted := make([]complex128, w.size)

	for i := range weightSpectrum {
		weightSpectrum[i] = complex(weight[i], 0)
		weighted[i] = complex(weight[i], 0) * desired[i]
	}

	autocorrelation, err := w.inverseReal(weightSpectrum)
	if err != nil {
		return nil, err
	}

	crossCorrelation, err := w.inverseReal(weighted)
	if err != nil {
		return nil, err
	}

	return solveNormalEquations(autocorrelation, crossCorrelation[:length])
}

// complexDeviation evaluates |H(omega) - D(omega)| on [0, Nyquist].
func complexDeviation(
	w *fftWorkspace,
	taps []float64,
	desired []complex128,
) ([]float64, error) {
	achieved, err := w.forwardReal(taps)
	if err != nil {
		return nil, err
	}

	out := make([]float64, w.size/2+1)
	for i := range out {
		out[i] = cmplx.Abs(achieved[i] - desired[i])
	}

	return out, nil
}

func summariseDeviation(
	deviation []float64,
	reference float64,
) ComplexErrorMetrics {
	sumSquared := 0.0
	peak := 0.0

	for _, value := range deviation {
		sumSquared += value * value
		peak = max(peak, value)
	}

	return ComplexErrorMetrics{
		RMS:  math.Sqrt(sumSquared/float64(len(deviation))) / reference,
		Peak: peak / reference,
	}
}

// reweight applies one Lawson step: the weight is multiplied by the current
// error magnitude, mirrored onto the negative frequencies and renormalised.
//
// Bins are floored relative to the largest weight so that a band the current
// iterate happens to match exactly does not drop out of the normal equations
// altogether.
func reweight(weight []float64, deviation []float64) {
	size := len(weight)
	half := size / 2

	largest := 0.0

	for i, value := range deviation {
		weight[i] *= value
		largest = max(largest, weight[i])
	}

	if largest == 0 {
		return
	}

	floor := largest * minimaxWeightFloor
	total := 0.0

	for i := 0; i <= half; i++ {
		weight[i] = max(weight[i], floor)
		total += weight[i]
	}

	if total == 0 {
		return
	}

	for i := 0; i <= half; i++ {
		weight[i] /= total
		if i > 0 && size-i > half {
			weight[size-i] = weight[i]
		}
	}
}

// expandWeight validates the half-spectrum weight and mirrors it onto the full
// design grid, where the normal equations are evaluated.
func expandWeight(requested []float64, size int) ([]float64, error) {
	half := size / 2

	out := make([]float64, size)

	if requested == nil {
		for i := range out {
			out[i] = 1
		}

		return out, nil
	}

	if len(requested) != half+1 {
		return nil, fmt.Errorf(
			"%w: got %d weights, want %d for an FFT size of %d",
			ErrInvalidWeight,
			len(requested),
			half+1,
			size,
		)
	}

	total := 0.0

	for i, value := range requested {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf(
				"%w: weight[%d] = %g",
				ErrInvalidWeight,
				i,
				value,
			)
		}

		total += value
		out[i] = value

		if i > 0 && size-i > half {
			out[size-i] = value
		}
	}

	if total == 0 {
		return nil, fmt.Errorf("%w: all weights are zero", ErrInvalidWeight)
	}

	return out, nil
}
