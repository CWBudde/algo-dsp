package conv

import algofft "github.com/cwbudde/algo-fft"

// StreamingConvolverT performs block-by-block convolution with persistent state.
// The type parameters F and C select the floating-point precision:
//   - F=float32, C=complex64 for single-precision (typical for audio DSP)
//   - F=float64, C=complex128 for double-precision (typical for measurement/analysis)
//
// Implementations include overlap-add and overlap-save algorithms, which provide
// equivalent results but with different internal buffer management strategies.
//
// Both algorithms:
//   - Process fixed-size input blocks
//   - Maintain internal state for continuity between blocks
//   - Support zero-allocation processing via ProcessBlockTo
//   - Use FFT for efficient convolution
//
// Algorithm selection:
//   - Overlap-add: Maintains output tail buffer, overlaps and adds results
//   - Overlap-save: Maintains input history buffer, discards circular convolution artifacts
//   - Performance is similar; choice is often based on implementation preferences
type StreamingConvolverT[F algofft.Float, C algofft.Complex] interface {
	// ProcessBlock convolves a single input block and returns the output block.
	// Both input and output are blockSize samples.
	// State is maintained between calls to ensure continuity.
	ProcessBlock(input []F) ([]F, error)

	// ProcessBlockTo convolves input block and writes to pre-allocated output.
	// Both input and output must be of size blockSize.
	// This is a zero-allocation version when output is pre-allocated.
	ProcessBlockTo(output, input []F) error

	// Reset clears internal state for processing a new signal stream.
	Reset()

	// BlockSize returns the expected input/output block size.
	BlockSize() int

	// KernelLen returns the convolution kernel length.
	KernelLen() int

	// FFTSize returns the internal FFT size used.
	FFTSize() int
}

// StreamingConvolver is the float64 specialization of StreamingConvolverT.
type StreamingConvolver = StreamingConvolverT[float64, complex128]

// fftRunner is an internal interface that abstracts over Plan and FastPlan.
// Both Forward and Inverse take dst, src slices of at least fftSize length.
type fftRunner[C algofft.Complex] interface {
	Forward(dst, src []C)
	Inverse(dst, src []C)
}

// planAdapter wraps algofft.Plan to satisfy fftRunner (ignores errors since
// the plan is pre-validated and buffer sizes are guaranteed by construction).
type planAdapter[C algofft.Complex] struct {
	plan *algofft.Plan[C]
}

func (a *planAdapter[C]) Forward(dst, src []C) { _ = a.plan.Forward(dst, src) }
func (a *planAdapter[C]) Inverse(dst, src []C) { _ = a.plan.Inverse(dst, src) }

// newFFTRunner tries to create a FastPlan for zero-overhead FFT.
// Falls back to a regular Plan if FastPlan is unavailable for the given size.
func newFFTRunner[C algofft.Complex](n int) (fftRunner[C], error) {
	fp, err := algofft.NewFastPlan[C](n)
	if err == nil {
		return fp, nil
	}
	// FastPlan unavailable (e.g. no codelet for this size), fall back to Plan.
	plan, err := algofft.NewPlanT[C](n)
	if err != nil {
		return nil, err
	}
	return &planAdapter[C]{plan: plan}, nil
}

// copyToComplex copies a float slice into a complex slice (one type switch per call).
func copyToComplex[F algofft.Float, C algofft.Complex](dst []C, src []F) {
	switch s := any(src).(type) {
	case []float32:
		d := any(dst).([]complex64)
		for i, v := range s {
			d[i] = complex(v, 0)
		}
	case []float64:
		d := any(dst).([]complex128)
		for i, v := range s {
			d[i] = complex(v, 0)
		}
	}
}

// copyFromComplex copies real parts of a complex slice into a float slice.
func copyFromComplex[F algofft.Float, C algofft.Complex](dst []F, src []C) {
	switch s := any(src).(type) {
	case []complex64:
		d := any(dst).([]float32)
		for i, v := range s {
			d[i] = real(v)
		}
	case []complex128:
		d := any(dst).([]float64)
		for i, v := range s {
			d[i] = real(v)
		}
	}
}