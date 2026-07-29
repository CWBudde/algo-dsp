package mixedphase

import (
	"fmt"
	"math"
	"math/cmplx"

	algofft "github.com/cwbudde/algo-fft"
)

type fftWorkspace struct {
	size int
	plan *algofft.Plan[complex128]
}

func newFFTWorkspace(size int) (*fftWorkspace, error) {
	plan, err := algofft.NewPlan64(size)
	if err != nil {
		return nil, fmt.Errorf("mixedphase: create FFT plan: %w", err)
	}

	return &fftWorkspace{size: size, plan: plan}, nil
}

func (w *fftWorkspace) forwardReal(input []float64) ([]complex128, error) {
	src := make([]complex128, w.size)
	for i := 0; i < len(input) && i < w.size; i++ {
		src[i] = complex(input[i], 0)
	}

	dst := make([]complex128, w.size)
	if err := w.plan.Forward(dst, src); err != nil {
		return nil, fmt.Errorf("mixedphase: forward FFT: %w", err)
	}

	return dst, nil
}

func (w *fftWorkspace) forwardComplex(input []complex128) ([]complex128, error) {
	src := make([]complex128, w.size)
	copy(src, input)

	dst := make([]complex128, w.size)

	if err := w.plan.Forward(dst, src); err != nil {
		return nil, fmt.Errorf("mixedphase: forward FFT: %w", err)
	}

	return dst, nil
}

func (w *fftWorkspace) inverseReal(input []complex128) ([]float64, error) {
	src := make([]complex128, w.size)
	copy(src, input)

	dst := make([]complex128, w.size)

	if err := w.plan.Inverse(dst, src); err != nil {
		return nil, fmt.Errorf("mixedphase: inverse FFT: %w", err)
	}

	out := make([]float64, w.size)
	for i := range out {
		out[i] = real(dst[i])
	}

	return out, nil
}

func magnitude(spectrum []complex128) []float64 {
	out := make([]float64, len(spectrum))
	for i, value := range spectrum {
		out[i] = cmplx.Abs(value)
	}

	return out
}

// minimumPhaseSpectrum reconstructs the causal minimum-phase spectrum that
// belongs to targetMagnitude using the requested method.
func minimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
	method MinimumPhaseMethod,
) ([]complex128, error) {
	switch method {
	case MethodCepstrum:
		return cepstrumMinimumPhaseSpectrum(w, targetMagnitude, epsilon)
	case MethodHilbert:
		return hilbertMinimumPhaseSpectrum(w, targetMagnitude, epsilon)
	default:
		return nil, fmt.Errorf("%w: %d", ErrInvalidMethod, int(method))
	}
}

// flooredMagnitude resamples targetMagnitude onto the workspace grid and
// applies the magnitude floor that keeps the logarithm finite.
func flooredMagnitude(
	size int,
	targetMagnitude []float64,
	epsilon float64,
) []float64 {
	out := make([]float64, size)
	for i := range out {
		out[i] = epsilon
		if i < len(targetMagnitude) && targetMagnitude[i] > epsilon {
			out[i] = targetMagnitude[i]
		}
	}

	return out
}

func logMagnitudeSpectrum(floored []float64) []complex128 {
	out := make([]complex128, len(floored))
	for i, value := range floored {
		out[i] = complex(math.Log(value), 0)
	}

	return out
}

// cepstrumMinimumPhaseSpectrum folds the real cepstrum onto its causal half and
// exponentiates the resulting complex log spectrum.
func cepstrumMinimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
) ([]complex128, error) {
	logMagnitude := logMagnitudeSpectrum(
		flooredMagnitude(w.size, targetMagnitude, epsilon),
	)

	cepstrum, err := w.inverseReal(logMagnitude)
	if err != nil {
		return nil, err
	}

	minimumCepstrum := make([]complex128, w.size)
	minimumCepstrum[0] = complex(cepstrum[0], 0)

	half := w.size / 2

	positiveLimit := (w.size + 1) / 2
	for i := 1; i < positiveLimit; i++ {
		minimumCepstrum[i] = complex(2*cepstrum[i], 0)
	}

	if w.size%2 == 0 {
		minimumCepstrum[half] = complex(cepstrum[half], 0)
	}

	logSpectrum, err := w.forwardComplex(minimumCepstrum)
	if err != nil {
		return nil, err
	}

	out := make([]complex128, w.size)
	for i, value := range logSpectrum {
		out[i] = cmplx.Exp(value)
	}

	return out, nil
}

// hilbertMinimumPhaseSpectrum obtains the minimum phase from the discrete
// Hilbert transform of the log magnitude and pairs it with the floored target
// magnitude.
//
// Unlike the cepstral route the magnitude never passes through an exponential,
// so it is reproduced exactly on the design grid.
func hilbertMinimumPhaseSpectrum(
	w *fftWorkspace,
	targetMagnitude []float64,
	epsilon float64,
) ([]complex128, error) {
	floored := flooredMagnitude(w.size, targetMagnitude, epsilon)

	phase, err := discreteHilbertPhase(w, logMagnitudeSpectrum(floored))
	if err != nil {
		return nil, err
	}

	out := make([]complex128, w.size)
	for i := range out {
		out[i] = cmplx.Rect(floored[i], phase[i])
	}

	return out, nil
}

// discreteHilbertPhase evaluates the minimum phase belonging to the given log
// magnitude.
//
// For a minimum-phase spectrum log magnitude and phase form a Hilbert pair. The
// transform is applied as a sign multiplication in the quefrency domain, which
// is the discrete equivalent of convolving the log magnitude with the
// cot(omega/2) kernel while avoiding that kernel's singularity. The DC and
// Nyquist quefrencies are excluded because they carry no quadrature component.
func discreteHilbertPhase(
	w *fftWorkspace,
	logMagnitude []complex128,
) ([]float64, error) {
	quefrency, err := w.inverseReal(logMagnitude)
	if err != nil {
		return nil, err
	}

	half := w.size / 2
	positiveLimit := (w.size + 1) / 2
	signed := make([]complex128, w.size)

	for i := 1; i < positiveLimit; i++ {
		signed[i] = complex(quefrency[i], 0)
	}

	for i := half + 1; i < w.size; i++ {
		signed[i] = complex(-quefrency[i], 0)
	}

	transformed, err := w.forwardComplex(signed)
	if err != nil {
		return nil, err
	}

	phase := make([]float64, w.size)
	for i, value := range transformed {
		phase[i] = imag(value)
	}

	return phase, nil
}

func regularisedMagnitudeDivision(
	numerator, denominator []complex128,
	epsilon float64,
) []float64 {
	out := make([]float64, len(numerator))

	for i := range out {
		num := cmplx.Abs(numerator[i])
		den := cmplx.Abs(denominator[i])
		out[i] = num * den / (den*den + epsilon*epsilon)
	}

	return out
}

func nextDesignFFTSize(filterLength, requested int) (int, error) {
	if requested != 0 {
		if requested < filterLength {
			return 0, fmt.Errorf(
				"%w: FFT size %d is shorter than filter length %d",
				ErrInvalidLength,
				requested,
				filterLength,
			)
		}

		return requested, nil
	}

	target := max(16, 8*filterLength)

	size := 1
	for size < target {
		size <<= 1
	}

	return size, nil
}

func defaultEpsilon(targetMagnitude []float64, requested float64) float64 {
	if requested > 0 {
		return requested
	}

	peak := 0.0
	for _, value := range targetMagnitude {
		peak = max(peak, value)
	}

	if peak == 0 {
		return 1e-12
	}

	return peak * 1e-12
}
