package conv

import (
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

// StreamingOverlapAddT implements streaming FFT-based convolution using overlap-add.
// Unlike OverlapAdd which processes entire signals, this maintains state for
// block-by-block processing with minimal allocations.
//
// The type parameters F and C select precision (see StreamingConvolverT).
//
// This is optimized for real-time audio processing where you receive fixed-size
// input blocks and need fixed-size output blocks with continuity between blocks.
type StreamingOverlapAddT[F algofft.Float, C algofft.Complex] struct {
	// Kernel in frequency domain
	kernelFFT []C

	// Configuration
	kernelLen int // Original kernel length
	blockSize int // Input/output block size (fixed)
	fftSize   int // FFT size (blockSize + kernelLen - 1, rounded to power of 2)

	// FFT engine (FastPlan when available, Plan as fallback)
	fft *fftEngine[C]

	// Reusable buffers (pre-allocated to avoid allocations per block)
	inputPadded  []C
	outputPadded []C
	convResult   []F // Full convolution result (blockSize + kernelLen - 1)

	// Overlap state (tail from previous block)
	tail []F
}

// StreamingOverlapAdd is the float64 specialization of StreamingOverlapAddT.
type StreamingOverlapAdd = StreamingOverlapAddT[float64, complex128]

// NewStreamingOverlapAddT creates a generic streaming overlap-add convolver.
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapAddT[F algofft.Float, C algofft.Complex](kernel []F, blockSize int) (*StreamingOverlapAddT[F, C], error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}
	if blockSize <= 0 {
		return nil, fmt.Errorf("conv: blockSize must be positive, got %d", blockSize)
	}

	kernelLen := len(kernel)

	// FFT size must accommodate block + kernel - 1 for linear convolution
	minFFTSize := blockSize + kernelLen - 1
	fftSize := nextPowerOf2(minFFTSize)

	// Create FFT engine (tries FastPlan first, falls back to Plan)
	fft, err := newFFTEngine[C](fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	soa := &StreamingOverlapAddT[F, C]{
		kernelFFT:    make([]C, fftSize),
		kernelLen:    kernelLen,
		blockSize:    blockSize,
		fftSize:      fftSize,
		fft:          fft,
		inputPadded:  make([]C, fftSize),
		outputPadded: make([]C, fftSize),
		convResult:   make([]F, blockSize+kernelLen-1),
		tail:         make([]F, kernelLen-1),
	}

	// Compute kernel FFT
	kernelPadded := make([]C, fftSize)
	packReal[F, C](kernelPadded, kernel)

	fft.Forward(soa.kernelFFT, kernelPadded)

	return soa, nil
}

// NewStreamingOverlapAdd creates a streaming overlap-add convolver (float64).
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapAdd(kernel []float64, blockSize int) (*StreamingOverlapAdd, error) {
	return NewStreamingOverlapAddT[float64, complex128](kernel, blockSize)
}

// NewStreamingOverlapAdd32 creates a streaming overlap-add convolver (float32).
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapAdd32(kernel []float32, blockSize int) (*StreamingOverlapAddT[float32, complex64], error) {
	return NewStreamingOverlapAddT[float32, complex64](kernel, blockSize)
}

// processBlockCore performs the core convolution. Output is written to convResult.
func (soa *StreamingOverlapAddT[F, C]) processBlockCore(input []F) {
	// Zero-pad input to FFT size
	clear(soa.inputPadded)
	packReal[F, C](soa.inputPadded[:soa.blockSize], input)

	// Forward FFT of input block
	soa.fft.Forward(soa.inputPadded, soa.inputPadded)

	// Multiply in frequency domain
	for i := range soa.outputPadded {
		soa.outputPadded[i] = soa.inputPadded[i] * soa.kernelFFT[i]
	}

	// Inverse FFT
	soa.fft.Inverse(soa.outputPadded, soa.outputPadded)

	// Extract real part into convResult
	resultLen := soa.blockSize + soa.kernelLen - 1
	unpackReal[F, C](soa.convResult[:resultLen], soa.outputPadded[:resultLen])

	// Add tail from previous block
	tailLen := len(soa.tail)
	for i := 0; i < tailLen && i < resultLen; i++ {
		soa.convResult[i] += soa.tail[i]
	}

	// Update tail for next block
	newTailLen := resultLen - soa.blockSize
	for i := range newTailLen {
		soa.tail[i] = soa.convResult[soa.blockSize+i]
	}
	for i := newTailLen; i < len(soa.tail); i++ {
		soa.tail[i] = 0
	}
}

// ProcessBlock convolves a single block and returns the output block.
// Input and output are both of size blockSize.
// State is maintained between calls to ensure continuity.
func (soa *StreamingOverlapAddT[F, C]) ProcessBlock(input []F) ([]F, error) {
	if len(input) != soa.blockSize {
		return nil, fmt.Errorf("%w: expected %d samples, got %d", ErrLengthMismatch, soa.blockSize, len(input))
	}

	soa.processBlockCore(input)

	output := make([]F, soa.blockSize)
	copy(output, soa.convResult[:soa.blockSize])
	return output, nil
}

// ProcessBlockTo convolves input block and writes to pre-allocated output.
// Both input and output must be of size blockSize.
// This is a zero-allocation version of ProcessBlock when output is pre-allocated.
func (soa *StreamingOverlapAddT[F, C]) ProcessBlockTo(output, input []F) error {
	if len(input) != soa.blockSize {
		return fmt.Errorf("%w: expected %d input samples, got %d", ErrLengthMismatch, soa.blockSize, len(input))
	}
	if len(output) != soa.blockSize {
		return fmt.Errorf("%w: expected %d output samples, got %d", ErrLengthMismatch, soa.blockSize, len(output))
	}

	soa.processBlockCore(input)

	copy(output, soa.convResult[:soa.blockSize])
	return nil
}

// Reset clears the tail buffer (overlap state from previous blocks).
func (soa *StreamingOverlapAddT[F, C]) Reset() {
	clear(soa.tail)
}

// BlockSize returns the block size.
func (soa *StreamingOverlapAddT[F, C]) BlockSize() int {
	return soa.blockSize
}

// KernelLen returns the kernel length.
func (soa *StreamingOverlapAddT[F, C]) KernelLen() int {
	return soa.kernelLen
}

// FFTSize returns the FFT size.
func (soa *StreamingOverlapAddT[F, C]) FFTSize() int {
	return soa.fftSize
}
