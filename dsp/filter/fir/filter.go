package fir

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/internal/vecmath"
)

// Filter implements a direct-form FIR filter using a circular-buffer delay line.
type Filter struct {
	coeffs []float64
	delay  []float64
	pos    int
}

// New creates a FIR filter from the given coefficient slice.
// The coefficients are copied. The filter order is len(coeffs)-1.
func New(coeffs []float64) *Filter {
	c := make([]float64, len(coeffs))
	copy(c, coeffs)
	return &Filter{
		coeffs: c,
		delay:  make([]float64, len(coeffs)),
	}
}

// ProcessSample filters one input sample using direct convolution
// with a circular delay line.
//
//	y[n] = sum_{k=0}^{N-1} h[k] * x[n-k]
func (f *Filter) ProcessSample(x float64) float64 {
	f.delay[f.pos] = x
	var y float64
	n := len(f.coeffs)
	p := f.pos
	for k := range n {
		y += f.coeffs[k] * f.delay[p]
		p--
		if p < 0 {
			p = n - 1
		}
	}
	f.pos++
	if f.pos >= n {
		f.pos = 0
	}
	return y
}

// ProcessBlock filters a block of samples in-place.
// This optimized version linearizes the delay line and uses SIMD dot product
// for significant performance improvement with large tap counts (128+).
func (f *Filter) ProcessBlock(buf []float64) {
	n := len(f.coeffs)
	if n == 0 {
		return
	}

	// For small tap counts, direct ProcessSample is competitive.
	// For large tap counts (128+), the linearization overhead is worth it.
	const linearizeThreshold = 32
	if n < linearizeThreshold {
		for i, x := range buf {
			buf[i] = f.ProcessSample(x)
		}
		return
	}

	// Allocate linearized buffer for the delay line.
	// This allows us to use a fast SIMD dot product without branches per tap.
	linear := make([]float64, n)

	for i, x := range buf {
		// Store input sample in circular delay line
		f.delay[f.pos] = x

		// Linearize the delay line for convolution without branches.
		// We need: [x[n], x[n-1], x[n-2], ...] = [delay[pos], delay[pos-1], ...]
		// Split into two copies to avoid wraparound branches:
		//   Part 1: delay[pos] down to delay[0]       -> linear[0:pos+1]
		//   Part 2: delay[n-1] down to delay[pos+1]   -> linear[pos+1:n]
		len1 := f.pos + 1
		len2 := n - len1

		// Copy first part: delay[0:pos+1] -> linear[0:pos+1] (in reverse)
		for k := 0; k < len1; k++ {
			linear[k] = f.delay[f.pos-k]
		}

		// Copy second part: delay[pos+1:n] -> linear[pos+1:n] (in reverse)
		if len2 > 0 {
			for k := 0; k < len2; k++ {
				linear[len1+k] = f.delay[n-1-k]
			}
		}

		// Compute output using SIMD dot product
		buf[i] = vecmath.DotProduct(f.coeffs, linear)

		// Advance circular buffer position
		f.pos++
		if f.pos >= n {
			f.pos = 0
		}
	}
}

// ProcessBlockTo filters src into dst. Both slices must have the same length.
// This optimized version linearizes the delay line and uses SIMD dot product
// for significant performance improvement with large tap counts (128+).
func (f *Filter) ProcessBlockTo(dst, src []float64) {
	_ = dst[len(src)-1] // bounds check hint
	n := len(f.coeffs)
	if n == 0 {
		return
	}

	// For small tap counts, direct ProcessSample is competitive.
	const linearizeThreshold = 32
	if n < linearizeThreshold {
		for i, x := range src {
			dst[i] = f.ProcessSample(x)
		}
		return
	}

	// Allocate linearized buffer for the delay line.
	linear := make([]float64, n)

	for i, x := range src {
		// Store input sample in circular delay line
		f.delay[f.pos] = x

		// Linearize the delay line for convolution without branches.
		len1 := f.pos + 1
		len2 := n - len1

		// Copy first part: delay[pos] down to delay[0]
		for k := 0; k < len1; k++ {
			linear[k] = f.delay[f.pos-k]
		}

		// Copy second part: delay[n-1] down to delay[pos+1]
		if len2 > 0 {
			for k := 0; k < len2; k++ {
				linear[len1+k] = f.delay[n-1-k]
			}
		}

		// Compute output using SIMD dot product
		dst[i] = vecmath.DotProduct(f.coeffs, linear)

		// Advance circular buffer position
		f.pos++
		if f.pos >= n {
			f.pos = 0
		}
	}
}

// Reset clears the delay line to zero.
func (f *Filter) Reset() {
	for i := range f.delay {
		f.delay[i] = 0
	}
	f.pos = 0
}

// Order returns the filter order (len(coeffs) - 1).
func (f *Filter) Order() int {
	return len(f.coeffs) - 1
}

// Coefficients returns a copy of the filter coefficients.
func (f *Filter) Coefficients() []float64 {
	c := make([]float64, len(f.coeffs))
	copy(c, f.coeffs)
	return c
}

// Response computes the complex frequency response H(e^{-jw}) at the given
// frequency (Hz) and sample rate (Hz).
func (f *Filter) Response(freqHz, sampleRate float64) complex128 {
	w := 2 * math.Pi * freqHz / sampleRate
	var h complex128
	for k, c := range f.coeffs {
		h += complex(c, 0) * cmplx.Exp(complex(0, -w*float64(k)))
	}
	return h
}

// MagnitudeDB returns the magnitude response in dB at the given frequency.
func (f *Filter) MagnitudeDB(freqHz, sampleRate float64) float64 {
	return 20 * math.Log10(cmplx.Abs(f.Response(freqHz, sampleRate)))
}
