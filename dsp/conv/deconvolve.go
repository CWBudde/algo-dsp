package conv

import (
	"errors"
	"fmt"
	"math"
	"math/cmplx"

	algofft "github.com/cwbudde/algo-fft"
)

// Deconvolution errors.
var (
	ErrDivisionByZero  = errors.New("conv: division by zero in deconvolution")
	ErrInvalidEpsilon  = errors.New("conv: epsilon must be positive")
	ErrInvalidNoiseVar = errors.New("conv: noise variance must be positive")
)

// DeconvMethod specifies the deconvolution method.
type DeconvMethod int

const (
	// DeconvNaive performs simple spectral division.
	// Fast but sensitive to noise and zeros in the kernel spectrum.
	DeconvNaive DeconvMethod = iota

	// DeconvRegularized adds a small epsilon to prevent division by zero.
	// output = IFFT(FFT(signal) * conj(FFT(kernel)) / (|FFT(kernel)|^2 + epsilon)).
	DeconvRegularized

	// DeconvWiener applies Wiener deconvolution with noise estimation.
	// Optimal in the MSE sense when signal and noise spectra are known.
	DeconvWiener
)

// DeconvOptions configures deconvolution behavior.
type DeconvOptions struct {
	// Method specifies the deconvolution algorithm.
	Method DeconvMethod

	// Epsilon is the regularization parameter for DeconvRegularized.
	// Prevents division by small values in the frequency domain.
	// Typical values: 1e-6 to 1e-3 depending on SNR.
	Epsilon float64

	// NoiseVariance is the estimated noise variance for Wiener deconvolution.
	// If zero, it will be estimated from the signal.
	NoiseVariance float64

	// SignalVariance is the estimated signal variance for Wiener deconvolution.
	// If zero, it will be estimated from the signal.
	SignalVariance float64
}

// DefaultDeconvOptions returns default deconvolution options.
func DefaultDeconvOptions() DeconvOptions {
	return DeconvOptions{
		Method:  DeconvRegularized,
		Epsilon: 1e-6,
	}
}

// Deconvolve recovers an estimate of the original signal from a convolved result.
// Given y = conv(x, h), this attempts to recover x from y and h.
//
// This is an ill-posed problem, especially when:
//   - The kernel h has zeros or near-zeros in its frequency response
//   - The signal contains noise
//   - The kernel is much shorter than the signal
//
// Use DeconvOptions to specify regularization method and parameters.
func Deconvolve(signal, kernel []float64, opts DeconvOptions) ([]float64, error) {
	if len(signal) == 0 {
		return nil, ErrEmptyInput
	}

	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}

	switch opts.Method {
	case DeconvNaive:
		return deconvolveNaive(signal, kernel)
	case DeconvRegularized:
		if opts.Epsilon <= 0 {
			opts.Epsilon = 1e-6
		}

		return deconvolveRegularized(signal, kernel, opts.Epsilon)
	case DeconvWiener:
		return deconvolveWiener(signal, kernel, opts)
	default:
		return deconvolveRegularized(signal, kernel, 1e-6)
	}
}

// deconvolveNaive performs simple spectral division.
func deconvolveNaive(signal, kernel []float64) ([]float64, error) {
	n := len(signal)
	m := len(kernel)

	// Output length: if y = conv(x, h), then len(y) = len(x) + len(h) - 1
	// So len(x) = len(y) - len(h) + 1
	outputLen := n - m + 1
	if outputLen <= 0 {
		outputLen = n
	}

	fftSize := nextPowerOf2(n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	// Zero-pad inputs
	signalPadded := make([]complex128, fftSize)
	kernelPadded := make([]complex128, fftSize)

	for i := range n {
		signalPadded[i] = complex(signal[i], 0)
	}

	for i := range m {
		kernelPadded[i] = complex(kernel[i], 0)
	}

	// Forward FFT
	signalFreq := make([]complex128, fftSize)
	kernelFreq := make([]complex128, fftSize)

	err = plan.Forward(signalFreq, signalPadded)
	if err != nil {
		return nil, err
	}

	err = plan.Forward(kernelFreq, kernelPadded)
	if err != nil {
		return nil, err
	}

	// Spectral division
	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		mag := cmplx.Abs(kernelFreq[i])
		if mag < 1e-15 {
			return nil, fmt.Errorf("%w: at frequency bin %d", ErrDivisionByZero, i)
		}

		resultFreq[i] = signalFreq[i] / kernelFreq[i]
	}

	// Inverse FFT
	resultTime := make([]complex128, fftSize)

	err = plan.Inverse(resultTime, resultFreq)
	if err != nil {
		return nil, err
	}

	// Extract real part
	result := make([]float64, outputLen)
	for i := range result {
		result[i] = real(resultTime[i])
	}

	return result, nil
}

// deconvolveRegularized performs regularized spectral division.
// output = IFFT(FFT(signal) * conj(FFT(kernel)) / (|FFT(kernel)|^2 + epsilon)).
func deconvolveRegularized(signal, kernel []float64, epsilon float64) ([]float64, error) {
	n := len(signal)
	m := len(kernel)

	outputLen := n - m + 1
	if outputLen <= 0 {
		outputLen = n
	}

	fftSize := nextPowerOf2(n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	// Zero-pad inputs
	signalPadded := make([]complex128, fftSize)
	kernelPadded := make([]complex128, fftSize)

	for i := range n {
		signalPadded[i] = complex(signal[i], 0)
	}

	for i := range m {
		kernelPadded[i] = complex(kernel[i], 0)
	}

	// Forward FFT
	signalFreq := make([]complex128, fftSize)
	kernelFreq := make([]complex128, fftSize)

	err = plan.Forward(signalFreq, signalPadded)
	if err != nil {
		return nil, err
	}

	err = plan.Forward(kernelFreq, kernelPadded)
	if err != nil {
		return nil, err
	}

	// Regularized division: Y * conj(H) / (|H|^2 + epsilon)
	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		hConj := cmplx.Conj(kernelFreq[i])
		hMagSq := real(kernelFreq[i])*real(kernelFreq[i]) + imag(kernelFreq[i])*imag(kernelFreq[i])
		resultFreq[i] = signalFreq[i] * hConj / complex(hMagSq+epsilon, 0)
	}

	// Inverse FFT
	resultTime := make([]complex128, fftSize)

	err = plan.Inverse(resultTime, resultFreq)
	if err != nil {
		return nil, err
	}

	// Extract real part
	result := make([]float64, outputLen)
	for i := range result {
		result[i] = real(resultTime[i])
	}

	return result, nil
}

// deconvolveWiener performs Wiener deconvolution.
// The Wiener filter is: H_w = conj(H) / (|H|^2 + NSR)
// where NSR = noise_variance / signal_variance is the noise-to-signal ratio.
func deconvolveWiener(signal, kernel []float64, opts DeconvOptions) ([]float64, error) {
	n := len(signal)
	m := len(kernel)

	outputLen := n - m + 1
	if outputLen <= 0 {
		outputLen = n
	}

	// Estimate variances if not provided
	noiseVar := opts.NoiseVariance
	signalVar := opts.SignalVariance

	if signalVar <= 0 {
		signalVar = variance(signal)
	}

	if noiseVar <= 0 {
		// Estimate noise as a fraction of signal variance
		// This is a rough heuristic; in practice, noise should be measured
		noiseVar = signalVar * 0.01 // Assume 1% noise
	}

	// Noise-to-signal ratio
	nsr := noiseVar / signalVar
	if nsr <= 0 {
		nsr = 1e-6
	}

	fftSize := nextPowerOf2(n)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	// Zero-pad inputs
	signalPadded := make([]complex128, fftSize)
	kernelPadded := make([]complex128, fftSize)

	for i := range n {
		signalPadded[i] = complex(signal[i], 0)
	}

	for i := range m {
		kernelPadded[i] = complex(kernel[i], 0)
	}

	// Forward FFT
	signalFreq := make([]complex128, fftSize)
	kernelFreq := make([]complex128, fftSize)

	err = plan.Forward(signalFreq, signalPadded)
	if err != nil {
		return nil, err
	}

	err = plan.Forward(kernelFreq, kernelPadded)
	if err != nil {
		return nil, err
	}

	// Wiener filter: Y * conj(H) / (|H|^2 + NSR)
	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		hConj := cmplx.Conj(kernelFreq[i])
		hMagSq := real(kernelFreq[i])*real(kernelFreq[i]) + imag(kernelFreq[i])*imag(kernelFreq[i])
		resultFreq[i] = signalFreq[i] * hConj / complex(hMagSq+nsr, 0)
	}

	// Inverse FFT
	resultTime := make([]complex128, fftSize)

	err = plan.Inverse(resultTime, resultFreq)
	if err != nil {
		return nil, err
	}

	// Extract real part
	result := make([]float64, outputLen)
	for i := range result {
		result[i] = real(resultTime[i])
	}

	return result, nil
}

// variance computes the variance of a signal.
func variance(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}

	// Compute mean
	var mean float64
	for _, v := range x {
		mean += v
	}

	mean /= float64(len(x))

	// Compute variance
	var sum float64

	for _, v := range x {
		d := v - mean
		sum += d * d
	}

	return sum / float64(len(x))
}

// InverseFilter creates an inverse filter from a kernel.
// The inverse filter H_inv satisfies: conv(H, H_inv) ≈ delta
// This is useful for equalizing or undoing the effect of a known filter.
func InverseFilter(kernel []float64, length int, epsilon float64) ([]float64, error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}

	if epsilon <= 0 {
		epsilon = 1e-6
	}

	fftSize := nextPowerOf2(length)

	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	// Zero-pad kernel
	kernelPadded := make([]complex128, fftSize)
	for i := 0; i < len(kernel) && i < fftSize; i++ {
		kernelPadded[i] = complex(kernel[i], 0)
	}

	// Forward FFT of kernel
	kernelFreq := make([]complex128, fftSize)

	err = plan.Forward(kernelFreq, kernelPadded)
	if err != nil {
		return nil, err
	}

	// Compute inverse: conj(H) / (|H|^2 + epsilon)
	invFreq := make([]complex128, fftSize)
	for i := range invFreq {
		hConj := cmplx.Conj(kernelFreq[i])
		hMagSq := real(kernelFreq[i])*real(kernelFreq[i]) + imag(kernelFreq[i])*imag(kernelFreq[i])
		invFreq[i] = hConj / complex(hMagSq+epsilon, 0)
	}

	// Inverse FFT
	invTime := make([]complex128, fftSize)

	err = plan.Inverse(invTime, invFreq)
	if err != nil {
		return nil, err
	}

	// Extract real part
	result := make([]float64, length)
	for i := range result {
		result[i] = real(invTime[i])
	}

	return result, nil
}

// SNR computes the signal-to-noise ratio in dB between original and recovered signals.
// SNR = 10 * log10(signal_power / noise_power)
// where noise = original - recovered.
func SNR(original, recovered []float64) float64 {
	if len(original) != len(recovered) || len(original) == 0 {
		return math.Inf(-1)
	}

	var signalPower, noisePower float64
	for i := range original {
		signalPower += original[i] * original[i]
		noise := original[i] - recovered[i]
		noisePower += noise * noise
	}

	if noisePower == 0 {
		return math.Inf(1) // Perfect recovery
	}

	return 10 * math.Log10(signalPower/noisePower)
}
