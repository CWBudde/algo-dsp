package conv

import (
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

// StreamingOverlapSaveT implements streaming FFT-based convolution using overlap-save.
// Unlike OverlapSave which processes entire signals, this maintains state for
// block-by-block processing with minimal allocations.
//
// The type parameters F and C select precision (see StreamingConvolverT).
//
// The overlap-save method uses overlapping input segments and discards the
// circular convolution wrap-around portion at the start of each block result.
//
// This is optimized for real-time audio processing where you receive fixed-size
// input blocks and need fixed-size output blocks with continuity between blocks.
type StreamingOverlapSaveT[F algofft.Float, C algofft.Complex] struct {
	// Kernel in frequency domain
	kernelFFT []C

	// Configuration
	kernelLen int // Original kernel length
	blockSize int // Input/output block size (fixed)
	fftSize   int // FFT size (power of 2, >= blockSize + kernelLen - 1)

	// FFT plan
	plan *algofft.Plan[C]

	// Reusable buffers (pre-allocated to avoid allocations per block)
	inputBuffer  []C
	outputBuffer []C

	// Input history (last kernelLen-1 samples for overlap)
	history []F
}

// StreamingOverlapSave is the float64 specialization of StreamingOverlapSaveT.
type StreamingOverlapSave = StreamingOverlapSaveT[float64, complex128]

// NewStreamingOverlapSaveT creates a generic streaming overlap-save convolver.
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapSaveT[F algofft.Float, C algofft.Complex](kernel []F, blockSize int) (*StreamingOverlapSaveT[F, C], error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}
	if blockSize <= 0 {
		return nil, fmt.Errorf("conv: blockSize must be positive, got %d", blockSize)
	}

	kernelLen := len(kernel)

	// FFT size must accommodate blockSize + kernelLen - 1 for linear convolution
	minFFTSize := blockSize + kernelLen - 1
	fftSize := nextPowerOf2(minFFTSize)

	// Create FFT plan
	plan, err := algofft.NewPlanT[C](fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	sos := &StreamingOverlapSaveT[F, C]{
		kernelFFT:    make([]C, fftSize),
		kernelLen:    kernelLen,
		blockSize:    blockSize,
		fftSize:      fftSize,
		plan:         plan,
		inputBuffer:  make([]C, fftSize),
		outputBuffer: make([]C, fftSize),
		history:      make([]F, kernelLen-1),
	}

	// Compute kernel FFT (zero-padded to fftSize)
	kernelPadded := make([]C, fftSize)
	for i, v := range kernel {
		kernelPadded[i] = toComplex[F, C](v)
	}

	err = plan.Forward(sos.kernelFFT, kernelPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to compute kernel FFT: %w", err)
	}

	return sos, nil
}

// NewStreamingOverlapSave creates a streaming overlap-save convolver (float64).
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapSave(kernel []float64, blockSize int) (*StreamingOverlapSave, error) {
	return NewStreamingOverlapSaveT[float64, complex128](kernel, blockSize)
}

// NewStreamingOverlapSave32 creates a streaming overlap-save convolver (float32).
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapSave32(kernel []float32, blockSize int) (*StreamingOverlapSaveT[float32, complex64], error) {
	return NewStreamingOverlapSaveT[float32, complex64](kernel, blockSize)
}

// ProcessBlock convolves a single block and returns the output block.
// Input and output are both of size blockSize.
// State is maintained between calls to ensure continuity.
func (sos *StreamingOverlapSaveT[F, C]) ProcessBlock(input []F) ([]F, error) {
	if len(input) != sos.blockSize {
		return nil, fmt.Errorf("%w: expected %d samples, got %d", ErrLengthMismatch, sos.blockSize, len(input))
	}

	// Build input buffer: history + new samples
	// Zero-pad to FFT size
	for i := range sos.inputBuffer {
		sos.inputBuffer[i] = 0
	}

	// Copy history (kernelLen - 1 samples)
	for i := range sos.kernelLen - 1 {
		sos.inputBuffer[i] = toComplex[F, C](sos.history[i])
	}

	// Copy new input samples
	for i := range sos.blockSize {
		sos.inputBuffer[sos.kernelLen-1+i] = toComplex[F, C](input[i])
	}

	// Forward FFT
	err := sos.plan.Forward(sos.inputBuffer, sos.inputBuffer)
	if err != nil {
		return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
	}

	// Multiply in frequency domain
	for i := range sos.outputBuffer {
		sos.outputBuffer[i] = sos.inputBuffer[i] * sos.kernelFFT[i]
	}

	// Inverse FFT
	err = sos.plan.Inverse(sos.outputBuffer, sos.outputBuffer)
	if err != nil {
		return nil, fmt.Errorf("conv: inverse FFT failed: %w", err)
	}

	// Discard first kernelLen-1 samples (circular convolution artifacts)
	// and extract blockSize valid samples
	output := make([]F, sos.blockSize)
	validStart := sos.kernelLen - 1
	for i := range sos.blockSize {
		output[i] = toFloat[F, C](sos.outputBuffer[validStart+i])
	}

	// Update history for next block
	// History is the last (kernelLen-1) samples from the combined input buffer
	// Combined buffer was: [old_history (kernelLen-1)] + [new_input (blockSize)]
	if sos.blockSize >= sos.kernelLen-1 {
		// Take last (kernelLen-1) samples from new input
		copy(sos.history, input[sos.blockSize-sos.kernelLen+1:])
	} else {
		// Shift old history and append new input
		copy(sos.history, sos.history[sos.blockSize:])
		copy(sos.history[sos.kernelLen-1-sos.blockSize:], input)
	}

	return output, nil
}

// ProcessBlockTo convolves input block and writes to pre-allocated output.
// Both input and output must be of size blockSize.
// This is a zero-allocation version of ProcessBlock when output is pre-allocated.
func (sos *StreamingOverlapSaveT[F, C]) ProcessBlockTo(output, input []F) error {
	if len(input) != sos.blockSize {
		return fmt.Errorf("%w: expected %d input samples, got %d", ErrLengthMismatch, sos.blockSize, len(input))
	}
	if len(output) != sos.blockSize {
		return fmt.Errorf("%w: expected %d output samples, got %d", ErrLengthMismatch, sos.blockSize, len(output))
	}

	// Build input buffer: history + new samples
	for i := range sos.inputBuffer {
		sos.inputBuffer[i] = 0
	}

	for i := range sos.kernelLen - 1 {
		sos.inputBuffer[i] = toComplex[F, C](sos.history[i])
	}

	for i := range sos.blockSize {
		sos.inputBuffer[sos.kernelLen-1+i] = toComplex[F, C](input[i])
	}

	// Forward FFT
	err := sos.plan.Forward(sos.inputBuffer, sos.inputBuffer)
	if err != nil {
		return fmt.Errorf("conv: forward FFT failed: %w", err)
	}

	// Multiply in frequency domain
	for i := range sos.outputBuffer {
		sos.outputBuffer[i] = sos.inputBuffer[i] * sos.kernelFFT[i]
	}

	// Inverse FFT
	err = sos.plan.Inverse(sos.outputBuffer, sos.outputBuffer)
	if err != nil {
		return fmt.Errorf("conv: inverse FFT failed: %w", err)
	}

	// Write valid samples to output
	validStart := sos.kernelLen - 1
	for i := range sos.blockSize {
		output[i] = toFloat[F, C](sos.outputBuffer[validStart+i])
	}

	// Update history for next block
	if sos.blockSize >= sos.kernelLen-1 {
		copy(sos.history, input[sos.blockSize-sos.kernelLen+1:])
	} else {
		copy(sos.history, sos.history[sos.blockSize:])
		copy(sos.history[sos.kernelLen-1-sos.blockSize:], input)
	}

	return nil
}

// Reset clears the history buffer (overlap state from previous blocks).
func (sos *StreamingOverlapSaveT[F, C]) Reset() {
	for i := range sos.history {
		sos.history[i] = 0
	}
}

// BlockSize returns the block size.
func (sos *StreamingOverlapSaveT[F, C]) BlockSize() int {
	return sos.blockSize
}

// KernelLen returns the kernel length.
func (sos *StreamingOverlapSaveT[F, C]) KernelLen() int {
	return sos.kernelLen
}

// FFTSize returns the FFT size.
func (sos *StreamingOverlapSaveT[F, C]) FFTSize() int {
	return sos.fftSize
}
