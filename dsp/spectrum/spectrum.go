package spectrum

import (
	"fmt"
	"math"
	"math/cmplx"
	"sort"
	"sync"

	"github.com/cwbudde/algo-vecmath"
)

// scratchBuf holds pooled scratch memory for complex-to-real unpacking.
type scratchBuf struct {
	data []float64
}

var scratchPool = sync.Pool{
	New: func() any { return &scratchBuf{} },
}

func getScratch(n int) (re, im []float64, buf *scratchBuf) {
	buf = scratchPool.Get().(*scratchBuf)
	need := 2 * n
	if cap(buf.data) < need {
		buf.data = make([]float64, need)
	} else {
		buf.data = buf.data[:need]
	}
	return buf.data[:n], buf.data[n:need], buf
}

func putScratch(buf *scratchBuf) {
	scratchPool.Put(buf)
}

// ComplexBins is a read-only adapter for complex spectrum outputs.
//
// This allows integration with different FFT backends without coupling this
// package to any specific implementation.
type ComplexBins interface {
	Len() int
	At(i int) complex128
}

// SliceBins adapts a []complex128 as [ComplexBins].
type SliceBins []complex128

// Len returns the bin count.
func (s SliceBins) Len() int { return len(s) }

// At returns the bin value at index i.
func (s SliceBins) At(i int) complex128 { return s[i] }

// Magnitude returns |X[k]| for each complex spectrum bin.
//
// This function uses SIMD-optimized implementations when available (AVX2, SSE2, NEON)
// for improved performance on large spectrum arrays. Scratch buffers are pooled
// internally, so in steady state this allocates only the output slice.
func Magnitude(in []complex128) []float64 {
	if len(in) == 0 {
		return nil
	}

	out := make([]float64, len(in))
	re, im, buf := getScratch(len(in))

	for i, c := range in {
		re[i] = real(c)
		im[i] = imag(c)
	}

	vecmath.Magnitude(out, re, im)
	putScratch(buf)
	return out
}

// MagnitudeFromParts computes |X[k]| = sqrt(re[k]^2 + im[k]^2) into dst.
//
// This is the zero-allocation fast path for callers that already have real and
// imaginary parts in separate slices. All three slices must have the same length.
func MagnitudeFromParts(dst, re, im []float64) {
	vecmath.Magnitude(dst, re, im)
}

// MagnitudeBins returns |X[k]| for each bin from a [ComplexBins] source.
func MagnitudeBins(in ComplexBins) []float64 {
	if in == nil {
		return nil
	}
	out := make([]float64, in.Len())
	for i := range out {
		out[i] = cmplx.Abs(in.At(i))
	}
	return out
}

// Power returns |X[k]|^2 for each complex spectrum bin.
//
// This function uses SIMD-optimized implementations when available (AVX2, SSE2, NEON)
// for improved performance on large spectrum arrays. Scratch buffers are pooled
// internally, so in steady state this allocates only the output slice.
func Power(in []complex128) []float64 {
	if len(in) == 0 {
		return nil
	}

	out := make([]float64, len(in))
	re, im, buf := getScratch(len(in))

	for i, c := range in {
		re[i] = real(c)
		im[i] = imag(c)
	}

	vecmath.Power(out, re, im)
	putScratch(buf)
	return out
}

// PowerFromParts computes |X[k]|^2 = re[k]^2 + im[k]^2 into dst.
//
// This is the zero-allocation fast path for callers that already have real and
// imaginary parts in separate slices. All three slices must have the same length.
func PowerFromParts(dst, re, im []float64) {
	vecmath.Power(dst, re, im)
}

// PowerBins returns |X[k]|^2 for each bin from a [ComplexBins] source.
func PowerBins(in ComplexBins) []float64 {
	if in == nil {
		return nil
	}
	out := make([]float64, in.Len())
	for i := range out {
		x := in.At(i)
		re := real(x)
		im := imag(x)
		out[i] = re*re + im*im
	}
	return out
}

// Phase returns arg(X[k]) for each complex spectrum bin in radians.
func Phase(in []complex128) []float64 {
	return PhaseBins(SliceBins(in))
}

// PhaseBins returns arg(X[k]) for each bin from a [ComplexBins] source.
func PhaseBins(in ComplexBins) []float64 {
	if in == nil {
		return nil
	}
	out := make([]float64, in.Len())
	for i := range out {
		out[i] = cmplx.Phase(in.At(i))
	}
	return out
}

// UnwrapPhase returns a new phase slice with +/-2*pi discontinuities removed.
func UnwrapPhase(phase []float64) []float64 {
	if len(phase) == 0 {
		return nil
	}
	out := make([]float64, len(phase))
	out[0] = phase[0]
	offset := 0.0
	for i := 1; i < len(phase); i++ {
		d := phase[i] - phase[i-1]
		switch {
		case d > math.Pi:
			offset -= 2 * math.Pi
		case d < -math.Pi:
			offset += 2 * math.Pi
		}
		out[i] = phase[i] + offset
	}
	return out
}

// GroupDelayFromPhase computes group delay in samples from unwrapped phase.
//
// The phase slice is expected over uniformly spaced FFT bins. fftSize is the
// FFT size that produced those bins. A centered finite difference is used for
// interior bins, with one-sided differences at the endpoints.
func GroupDelayFromPhase(unwrapped []float64, fftSize int) ([]float64, error) {
	if len(unwrapped) < 2 {
		return nil, fmt.Errorf("group delay requires at least 2 phase points: %d", len(unwrapped))
	}
	if fftSize <= 0 {
		return nil, fmt.Errorf("group delay fftSize must be > 0: %d", fftSize)
	}
	dw := 2 * math.Pi / float64(fftSize)
	if dw == 0 || math.IsInf(dw, 0) || math.IsNaN(dw) {
		return nil, fmt.Errorf("group delay invalid frequency spacing")
	}
	out := make([]float64, len(unwrapped))
	for i := range unwrapped {
		var dphi float64
		switch i {
		case 0:
			dphi = unwrapped[1] - unwrapped[0]
		case len(unwrapped) - 1:
			dphi = unwrapped[i] - unwrapped[i-1]
		default:
			dphi = (unwrapped[i+1] - unwrapped[i-1]) / 2
		}
		out[i] = -dphi / dw
	}
	return out, nil
}

// GroupDelaySeconds computes group delay in seconds from unwrapped phase.
func GroupDelaySeconds(unwrapped []float64, fftSize int, sampleRate float64) ([]float64, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("group delay sampleRate must be > 0: %f", sampleRate)
	}
	samples, err := GroupDelayFromPhase(unwrapped, fftSize)
	if err != nil {
		return nil, err
	}
	invSR := 1 / sampleRate
	for i := range samples {
		samples[i] *= invSR
	}
	return samples, nil
}

// InterpolateLinear performs piecewise-linear interpolation at queryX.
//
// x must be strictly increasing and have the same length as y.
func InterpolateLinear(x, y, queryX []float64) ([]float64, error) {
	if len(x) == 0 || len(y) == 0 {
		return nil, fmt.Errorf("interpolate requires non-empty x and y")
	}
	if len(x) != len(y) {
		return nil, fmt.Errorf("interpolate x/y length mismatch: %d != %d", len(x), len(y))
	}
	for i := 1; i < len(x); i++ {
		if !(x[i] > x[i-1]) {
			return nil, fmt.Errorf("interpolate x must be strictly increasing at index %d", i)
		}
	}

	out := make([]float64, len(queryX))
	for i, q := range queryX {
		if q <= x[0] {
			out[i] = y[0]
			continue
		}
		if q >= x[len(x)-1] {
			out[i] = y[len(y)-1]
			continue
		}

		j := sort.SearchFloat64s(x, q)
		x0, x1 := x[j-1], x[j]
		t := (q - x0) / (x1 - x0)
		out[i] = y[j-1] + t*(y[j]-y[j-1])
	}
	return out, nil
}

// SmoothFractionalOctave applies simple 1/N-octave smoothing on linear-domain
// values using arithmetic mean over each fractional-octave band.
//
// freqHz and values must have equal length and freqHz must be strictly
// increasing with positive values.
func SmoothFractionalOctave(freqHz, values []float64, fraction int) ([]float64, error) {
	if len(freqHz) == 0 || len(values) == 0 {
		return nil, fmt.Errorf("fractional-octave smoothing requires non-empty inputs")
	}
	if len(freqHz) != len(values) {
		return nil, fmt.Errorf("fractional-octave input length mismatch: %d != %d", len(freqHz), len(values))
	}
	if fraction <= 0 {
		return nil, fmt.Errorf("fractional-octave fraction must be > 0: %d", fraction)
	}
	for i := range freqHz {
		if freqHz[i] <= 0 {
			return nil, fmt.Errorf("fractional-octave frequencies must be > 0 at index %d", i)
		}
		if i > 0 && !(freqHz[i] > freqHz[i-1]) {
			return nil, fmt.Errorf("fractional-octave frequencies must be strictly increasing at index %d", i)
		}
	}

	out := make([]float64, len(values))
	halfBand := math.Pow(2, 1/(2*float64(fraction)))

	for i, f := range freqHz {
		fLo := f / halfBand
		fHi := f * halfBand

		i0 := sort.Search(len(freqHz), func(k int) bool { return freqHz[k] >= fLo })
		i1 := sort.Search(len(freqHz), func(k int) bool { return freqHz[k] > fHi })
		if i0 >= i1 {
			out[i] = values[i]
			continue
		}

		sum := 0.0
		for j := i0; j < i1; j++ {
			sum += values[j]
		}
		out[i] = sum / float64(i1-i0)
	}

	return out, nil
}
