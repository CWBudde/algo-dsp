package conv

import (
	"unsafe"

	algofft "github.com/cwbudde/algo-fft"
)

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

// fftEngine is a concrete FFT dispatch that avoids interface overhead.
// It stores both a FastPlan (preferred) and a Plan (fallback), using a nil
// check for dispatch instead of a vtable call.
type fftEngine[C algofft.Complex] struct {
	fast *algofft.FastPlan[C]
	plan *algofft.Plan[C]
}

func (e *fftEngine[C]) Forward(dst, src []C) {
	if e.fast != nil {
		e.fast.Forward(dst, src)
		return
	}
	_ = e.plan.Forward(dst, src)
}

func (e *fftEngine[C]) Inverse(dst, src []C) {
	if e.fast != nil {
		e.fast.Inverse(dst, src)
		return
	}
	_ = e.plan.Inverse(dst, src)
}

// newFFTEngine creates an fftEngine that prefers FastPlan over Plan.
func newFFTEngine[C algofft.Complex](n int) (*fftEngine[C], error) {
	e := &fftEngine[C]{}
	fp, err := algofft.NewFastPlan[C](n)
	if err == nil {
		e.fast = fp
		return e, nil
	}
	// FastPlan unavailable (e.g. no codelet for this size), fall back to Plan.
	plan, err := algofft.NewPlanT[C](n)
	if err != nil {
		return nil, err
	}
	e.plan = plan
	return e, nil
}

// packReal writes float values into the real parts of a complex slice.
// Uses unsafe.Sizeof for compile-time-resolvable dispatch instead of any() boxing.
// The destination must be zeroed beforehand if imaginary parts should be zero.
func packReal[F algofft.Float, C algofft.Complex](dst []C, src []F) {
	if unsafe.Sizeof(F(0)) == 4 {
		d := unsafe.Slice((*complex64)(unsafe.Pointer(unsafe.SliceData(dst))), len(src))
		s := unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(src))), len(src))
		for i, v := range s {
			d[i] = complex(v, 0)
		}
	} else {
		d := unsafe.Slice((*complex128)(unsafe.Pointer(unsafe.SliceData(dst))), len(src))
		s := unsafe.Slice((*float64)(unsafe.Pointer(unsafe.SliceData(src))), len(src))
		for i, v := range s {
			d[i] = complex(v, 0)
		}
	}
}

// unpackReal extracts real parts from a complex slice into a float slice.
// Uses unsafe reinterpretation to avoid any() boxing overhead.
func unpackReal[F algofft.Float, C algofft.Complex](dst []F, src []C) {
	if unsafe.Sizeof(F(0)) == 4 {
		d := unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(dst))), len(dst))
		s := unsafe.Slice((*complex64)(unsafe.Pointer(unsafe.SliceData(src))), len(dst))
		for i, v := range s {
			d[i] = real(v)
		}
	} else {
		d := unsafe.Slice((*float64)(unsafe.Pointer(unsafe.SliceData(dst))), len(dst))
		s := unsafe.Slice((*complex128)(unsafe.Pointer(unsafe.SliceData(src))), len(dst))
		for i, v := range s {
			d[i] = real(v)
		}
	}
}
