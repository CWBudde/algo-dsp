package conv

import (
	"fmt"
	"math"

	algofft "github.com/cwbudde/algo-fft"
)

// Correlate computes the full cross-correlation of a and b.
// The result has length len(a) + len(b) - 1.
// Output index k corresponds to lag k - (len(b) - 1).
//
// Cross-correlation is related to convolution: corr(a,b) = conv(a, reverse(b))
// For real signals, this is equivalent to sliding b over a and computing the dot product.
func Correlate(a, b []float64) ([]float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, ErrEmptyInput
	}

	// Cross-correlation is convolution with time-reversed second signal
	bReversed := make([]float64, len(b))
	for i := range b {
		bReversed[i] = b[len(b)-1-i]
	}

	return Convolve(a, bReversed)
}

// CorrelateDirect computes cross-correlation using direct computation.
func CorrelateDirect(a, b []float64) ([]float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, ErrEmptyInput
	}

	bReversed := make([]float64, len(b))
	for i := range b {
		bReversed[i] = b[len(b)-1-i]
	}

	return Direct(a, bReversed)
}

// CorrelateMode computes cross-correlation with specified output mode.
func CorrelateMode(a, b []float64, mode Mode) ([]float64, error) {
	full, err := Correlate(a, b)
	if err != nil {
		return nil, err
	}

	return trimToMode(full, len(a), len(b), mode), nil
}

// AutoCorrelate computes the auto-correlation of signal a.
// The result has length 2*len(a) - 1.
// Output index k corresponds to lag k - (len(a) - 1).
func AutoCorrelate(a []float64) ([]float64, error) {
	return Correlate(a, a)
}

// AutoCorrelateNormalized computes normalized auto-correlation.
// The result is normalized such that the zero-lag value is 1.0.
func AutoCorrelateNormalized(a []float64) ([]float64, error) {
	result, err := AutoCorrelate(a)
	if err != nil {
		return nil, err
	}

	// Find zero-lag value (at center)
	zeroLag := result[len(a)-1]
	if zeroLag == 0 {
		return result, nil
	}

	// Normalize
	for i := range result {
		result[i] /= zeroLag
	}

	return result, nil
}

// CorrelateNormalized computes normalized cross-correlation.
// The result is normalized by the product of the L2 norms of a and b,
// producing values in the range [-1, 1].
func CorrelateNormalized(a, b []float64) ([]float64, error) {
	result, err := Correlate(a, b)
	if err != nil {
		return nil, err
	}

	// Compute L2 norms
	normA := l2Norm(a)
	normB := l2Norm(b)
	normProduct := normA * normB

	if normProduct == 0 {
		return result, nil
	}

	// Normalize
	for i := range result {
		result[i] /= normProduct
	}

	return result, nil
}

// CorrelateFFT computes cross-correlation using FFT.
// This is more efficient for longer signals.
func CorrelateFFT(a, b []float64) ([]float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, ErrEmptyInput
	}

	// For FFT-based correlation: IFFT(FFT(a) * conj(FFT(b)))
	n := len(a)
	m := len(b)
	fftSize := nextPowerOf2(n + m - 1)

	// Create FFT plan
	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	// Zero-pad inputs
	aPadded := make([]complex128, fftSize)
	bPadded := make([]complex128, fftSize)
	for i := 0; i < n; i++ {
		aPadded[i] = complex(a[i], 0)
	}
	for i := 0; i < m; i++ {
		bPadded[i] = complex(b[i], 0)
	}

	// Forward FFT
	aFreq := make([]complex128, fftSize)
	bFreq := make([]complex128, fftSize)

	err = plan.Forward(aFreq, aPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
	}

	err = plan.Forward(bFreq, bPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
	}

	// Multiply A by conjugate of B
	resultFreq := make([]complex128, fftSize)
	for i := range resultFreq {
		// conj(b) = real - imag*i
		bConj := complex(real(bFreq[i]), -imag(bFreq[i]))
		resultFreq[i] = aFreq[i] * bConj
	}

	// Inverse FFT
	resultTime := make([]complex128, fftSize)
	err = plan.Inverse(resultTime, resultFreq)
	if err != nil {
		return nil, fmt.Errorf("conv: inverse FFT failed: %w", err)
	}

	// Extract real part and rearrange for proper correlation output
	// FFT correlation gives circular correlation; we need to rearrange for linear
	outputLen := n + m - 1
	result := make([]float64, outputLen)

	// The correlation result needs to be rearranged
	// Positive lags (0 to n-1) are at the beginning
	// Negative lags (-(m-1) to -1) need to be extracted from the end
	for i := 0; i < n; i++ {
		result[m-1+i] = real(resultTime[i])
	}
	for i := 0; i < m-1; i++ {
		result[i] = real(resultTime[fftSize-m+1+i])
	}

	return result, nil
}

// l2Norm computes the L2 (Euclidean) norm of a signal.
func l2Norm(x []float64) float64 {
	var sum float64
	for _, v := range x {
		sum += v * v
	}
	return math.Sqrt(sum)
}

// FindPeak finds the index and value of the maximum in a correlation result.
// Useful for finding the best alignment between two signals.
func FindPeak(corr []float64) (index int, value float64) {
	if len(corr) == 0 {
		return -1, 0
	}

	index = 0
	value = corr[0]

	for i, v := range corr {
		if v > value {
			index = i
			value = v
		}
	}

	return index, value
}

// LagFromIndex converts a correlation result index to a lag value.
// For a correlation of signals with lengths lenA and lenB,
// the lag at index i is i - (lenB - 1).
func LagFromIndex(index, lenB int) int {
	return index - (lenB - 1)
}

// IndexFromLag converts a lag value to a correlation result index.
// Returns the index in the correlation result array for the given lag.
func IndexFromLag(lag, lenB int) int {
	return lag + (lenB - 1)
}
