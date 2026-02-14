package conv

// StreamingConvolver performs block-by-block convolution with persistent state.
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
type StreamingConvolver interface {
	// ProcessBlock convolves a single input block and returns the output block.
	// Both input and output are blockSize samples.
	// State is maintained between calls to ensure continuity.
	ProcessBlock(input []float64) ([]float64, error)

	// ProcessBlockTo convolves input block and writes to pre-allocated output.
	// Both input and output must be of size blockSize.
	// This is a zero-allocation version when output is pre-allocated.
	ProcessBlockTo(output, input []float64) error

	// Reset clears internal state for processing a new signal stream.
	Reset()

	// BlockSize returns the expected input/output block size.
	BlockSize() int

	// KernelLen returns the convolution kernel length.
	KernelLen() int

	// FFTSize returns the internal FFT size used.
	FFTSize() int
}
