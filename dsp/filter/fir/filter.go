package fir

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/internal/vecmath"
)

// Filter implements a direct-form FIR filter using a circular-buffer delay line.
type Filter struct {
	coeffs []float64
	delay  []float64 // circular delay line
	pos    int       // current write position in delay line
	linear []float64 // double-sized buffer for block processing optimization
}

// New creates a FIR filter from the given coefficient slice.
// The coefficients are copied. The filter order is len(coeffs)-1.
func New(coeffs []float64) *Filter {
	c := make([]float64, len(coeffs))
	copy(c, coeffs)
	n := len(coeffs)
	return &Filter{
		coeffs: c,
		delay:  make([]float64, n),
		linear: make([]float64, n*2), // double-sized for efficient block processing
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

	// Use double-buffered approach: maintain delay values in both halves
	// of f.linear to allow contiguous access without branches or copying.
	for i := range buf {
		x := buf[i]

		// Store sample in both positions of the double buffer
		f.linear[f.pos] = x
		f.linear[f.pos+n] = x

		// Also update the delay line for ProcessSample compatibility
		f.delay[f.pos] = x

		// Compute output using SIMD dot product on contiguous memory.
		// Read from position that gives us the last n samples in reverse order.
		start := f.pos + 1
		buf[i] = vecmath.DotProduct(f.coeffs, f.linear[start:start+n])

		// Advance position
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

	// Use double-buffered approach: maintain delay values in both halves
	// of f.linear to allow contiguous access without branches or copying.
	for i := range src {
		x := src[i]

		// Store sample in both positions of the double buffer
		f.linear[f.pos] = x
		f.linear[f.pos+n] = x

		// Also update the delay line for ProcessSample compatibility
		f.delay[f.pos] = x

		// Compute output using SIMD dot product on contiguous memory
		start := f.pos + 1
		dst[i] = vecmath.DotProduct(f.coeffs, f.linear[start:start+n])

		// Advance position
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
	for i := range f.linear {
		f.linear[i] = 0
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
