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

// toComplex converts a float value to its corresponding complex type.
func toComplex[F algofft.Float, C algofft.Complex](f F) C {
	switch v := any(f).(type) {
	case float32:
		return any(complex(v, 0)).(C)
	case float64:
		return any(complex(v, 0)).(C)
	}
	panic("unreachable")
}

// toFloat extracts the real part of a complex value as the corresponding float type.
func toFloat[F algofft.Float, C algofft.Complex](c C) F {
	switch v := any(c).(type) {
	case complex64:
		return any(real(v)).(F)
	case complex128:
		return any(real(v)).(F)
	}
	panic("unreachable")
}
