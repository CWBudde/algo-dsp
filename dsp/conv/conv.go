// Package conv provides convolution, correlation, and deconvolution routines.
//
// The package offers multiple convolution strategies optimized for different use cases:
//
//   - Direct convolution: Simple O(N*M) time-domain convolution, best for very short kernels
//   - Overlap-add (OLA): FFT-based block convolution, efficient for long signals with medium kernels
//   - Overlap-save (OLS): Alternative FFT-based block convolution with different memory characteristics
//
// The package also provides correlation functions for signal matching and alignment,
// and deconvolution with regularization for inverse filtering.
//
// # Usage
//
// For one-shot convolution, use the simple functions:
//
//	result := conv.Convolve(signal, kernel)  // Linear convolution
//	result := conv.Correlate(a, b)           // Cross-correlation
//
// For repeated convolution with the same kernel, create a reusable convolver:
//
//	c := conv.NewOverlapAdd(kernel, blockSize)
//	result := c.Process(signal)
//
// # Algorithm Selection
//
// The Auto function automatically selects the best algorithm based on input sizes:
//   - Direct convolution for very short kernels (< 32 samples)
//   - FFT-based methods for longer kernels
package conv

import (
	"errors"

	"github.com/cwbudde/algo-dsp/internal/vecmath"
)

// Errors returned by convolution functions.
var (
	ErrEmptyInput       = errors.New("conv: empty input")
	ErrEmptyKernel      = errors.New("conv: empty kernel")
	ErrLengthMismatch   = errors.New("conv: buffer length mismatch")
	ErrInvalidBlockSize = errors.New("conv: invalid block size")
)

// Mode specifies the output mode for convolution and correlation.
type Mode int

const (
	// ModeFull returns the full convolution result with length len(a)+len(b)-1.
	ModeFull Mode = iota

	// ModeSame returns output with the same length as the first input.
	ModeSame

	// ModeValid returns only the portion where signals fully overlap,
	// with length max(len(a), len(b)) - min(len(a), len(b)) + 1.
	ModeValid
)

// Direct performs direct time-domain linear convolution of a and b.
// Returns a new slice of length len(a) + len(b) - 1.
//
// This is an O(N*M) algorithm suitable for short kernels.
// For longer kernels, use FFT-based methods like OverlapAdd.
func Direct(a, b []float64) ([]float64, error) {
	if len(a) == 0 {
		return nil, ErrEmptyInput
	}
	if len(b) == 0 {
		return nil, ErrEmptyKernel
	}

	n := len(a)
	m := len(b)
	resultLen := n + m - 1
	result := make([]float64, resultLen)

	DirectTo(result, a, b)
	return result, nil
}

// DirectTo performs direct convolution, writing to a pre-allocated destination.
// dst must have length len(a) + len(b) - 1.
func DirectTo(dst, a, b []float64) {
	n := len(a)
	m := len(b)

	// Clear destination
	for i := range dst {
		dst[i] = 0
	}

	// Use SIMD-accelerated path for kernels >= 4 samples
	const simdThreshold = 4
	if m >= simdThreshold {
		directToSIMD(dst, a, b, n, m)
	} else {
		directToScalar(dst, a, b, n, m)
	}
}

// directToScalar performs scalar convolution for small kernels.
func directToScalar(dst, a, b []float64, n, m int) {
	for i := 0; i < n; i++ {
		for j := 0; j < m; j++ {
			dst[i+j] += a[i] * b[j]
		}
	}
}

// directToSIMD performs SIMD-accelerated convolution for larger kernels.
// Uses vecmath operations to vectorize the inner loop.
func directToSIMD(dst, a, b []float64, n, m int) {
	// Pre-allocate scratch buffer for scaled kernel
	temp := make([]float64, m)

	for i := 0; i < n; i++ {
		// Scale kernel by current input sample: temp = b * a[i]
		vecmath.ScaleBlock(temp, b, a[i])

		// Accumulate into destination: dst[i:i+m] += temp
		vecmath.AddBlockInPlace(dst[i:i+m], temp)
	}
}

// DirectCircular performs circular convolution of a and b.
// Both inputs must have the same length N, and the result has length N.
func DirectCircular(a, b []float64) ([]float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, ErrEmptyInput
	}
	if len(a) != len(b) {
		return nil, ErrLengthMismatch
	}

	n := len(a)
	result := make([]float64, n)

	DirectCircularTo(result, a, b)
	return result, nil
}

// DirectCircularTo performs circular convolution to a pre-allocated destination.
func DirectCircularTo(dst, a, b []float64) {
	n := len(a)

	for i := range dst {
		dst[i] = 0
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			k := (i + j) % n
			dst[k] += a[i] * b[j]
		}
	}
}

// Convolve performs linear convolution with automatic algorithm selection.
// For short kernels (< 64 samples), uses direct convolution.
// For longer kernels, uses FFT-based overlap-add.
func Convolve(a, b []float64) ([]float64, error) {
	if len(a) == 0 {
		return nil, ErrEmptyInput
	}
	if len(b) == 0 {
		return nil, ErrEmptyKernel
	}

	// Ensure a is the longer signal for efficient processing
	if len(b) > len(a) {
		a, b = b, a
	}

	// Use direct convolution for short kernels
	const directThreshold = 64
	if len(b) <= directThreshold {
		return Direct(a, b)
	}

	// Use FFT-based overlap-add for longer kernels
	return OverlapAddConvolve(a, b)
}

// ConvolveMode performs convolution with specified output mode.
func ConvolveMode(a, b []float64, mode Mode) ([]float64, error) {
	full, err := Convolve(a, b)
	if err != nil {
		return nil, err
	}

	return trimToMode(full, len(a), len(b), mode), nil
}

// trimToMode extracts the appropriate portion of a full convolution result.
func trimToMode(full []float64, lenA, lenB int, mode Mode) []float64 {
	switch mode {
	case ModeFull:
		return full
	case ModeSame:
		// Center the result to match length of first input
		start := (lenB - 1) / 2
		return full[start : start+lenA]
	case ModeValid:
		// Return only fully overlapping portion
		if lenA >= lenB {
			return full[lenB-1 : lenA]
		}
		return full[lenA-1 : lenB]
	default:
		return full
	}
}

// nextPowerOf2 returns the next power of 2 >= n.
func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// isPowerOf2 returns true if n is a power of 2.
func isPowerOf2(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
