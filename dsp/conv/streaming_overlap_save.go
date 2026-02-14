package conv

import (
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

// StreamingOverlapSave implements streaming FFT-based convolution using overlap-save.
// Unlike OverlapSave which processes entire signals, this maintains state for
// block-by-block processing with minimal allocations.
//
// The overlap-save method uses overlapping input segments and discards the
// circular convolution wrap-around portion at the start of each block result.
//
// This is optimized for real-time audio processing where you receive fixed-size
// input blocks and need fixed-size output blocks with continuity between blocks.
type StreamingOverlapSave struct {
	// Kernel in frequency domain
	kernelFFT []complex128

	// Configuration
	kernelLen int // Original kernel length
	blockSize int // Input/output block size (fixed)
	fftSize   int // FFT size (power of 2, >= blockSize + kernelLen - 1)

	// FFT plan
	plan *algofft.Plan[complex128]

	// Reusable buffers (pre-allocated to avoid allocations per block)
	inputBuffer  []complex128
	outputBuffer []complex128

	// Input history (last kernelLen-1 samples for overlap)
	history []float64
}

// NewStreamingOverlapSave creates a streaming overlap-save convolver.
// blockSize is the fixed size of input and output blocks.
func NewStreamingOverlapSave(kernel []float64, blockSize int) (*StreamingOverlapSave, error) {
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
	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	sos := &StreamingOverlapSave{
		kernelFFT:    make([]complex128, fftSize),
		kernelLen:    kernelLen,
		blockSize:    blockSize,
		fftSize:      fftSize,
		plan:         plan,
		inputBuffer:  make([]complex128, fftSize),
		outputBuffer: make([]complex128, fftSize),
		history:      make([]float64, kernelLen-1),
	}

	// Compute kernel FFT (zero-padded to fftSize)
	kernelPadded := make([]complex128, fftSize)
	for i, v := range kernel {
		kernelPadded[i] = complex(v, 0)
	}

	err = plan.Forward(sos.kernelFFT, kernelPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to compute kernel FFT: %w", err)
	}

	return sos, nil
}

// ProcessBlock convolves a single block and returns the output block.
// Input and output are both of size blockSize.
// State is maintained between calls to ensure continuity.
func (sos *StreamingOverlapSave) ProcessBlock(input []float64) ([]float64, error) {
	if len(input) != sos.blockSize {
		return nil, fmt.Errorf("%w: expected %d samples, got %d", ErrLengthMismatch, sos.blockSize, len(input))
	}

	// Build input buffer: history + new samples
	// Zero-pad to FFT size
	for i := range sos.inputBuffer {
		sos.inputBuffer[i] = 0
	}

	// Copy history (kernelLen - 1 samples)
	for i := 0; i < sos.kernelLen-1; i++ {
		sos.inputBuffer[i] = complex(sos.history[i], 0)
	}

	// Copy new input samples
	for i := 0; i < sos.blockSize; i++ {
		sos.inputBuffer[sos.kernelLen-1+i] = complex(input[i], 0)
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
	output := make([]float64, sos.blockSize)
	validStart := sos.kernelLen - 1
	for i := 0; i < sos.blockSize; i++ {
		output[i] = real(sos.outputBuffer[validStart+i])
	}

	// Update history for next block
	// History is the last (kernelLen-1) samples from the combined input buffer
	// Combined buffer was: [old_history (kernelLen-1)] + [new_input (blockSize)]
	if sos.blockSize >= sos.kernelLen-1 {
		// Take last (kernelLen-1) samples from new input
		copy(sos.history, input[sos.blockSize-sos.kernelLen+1:])
	} else {
		// Shift old history and append new input
		// Keep last (kernelLen-1-blockSize) samples from old history
		copy(sos.history, sos.history[sos.blockSize:])
		// Append all new input samples
		copy(sos.history[sos.kernelLen-1-sos.blockSize:], input)
	}

	return output, nil
}

// ProcessBlockTo convolves input block and writes to pre-allocated output.
// Both input and output must be of size blockSize.
// This is a zero-allocation version of ProcessBlock when output is pre-allocated.
func (sos *StreamingOverlapSave) ProcessBlockTo(output, input []float64) error {
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

	for i := 0; i < sos.kernelLen-1; i++ {
		sos.inputBuffer[i] = complex(sos.history[i], 0)
	}

	for i := 0; i < sos.blockSize; i++ {
		sos.inputBuffer[sos.kernelLen-1+i] = complex(input[i], 0)
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
	for i := 0; i < sos.blockSize; i++ {
		output[i] = real(sos.outputBuffer[validStart+i])
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
func (sos *StreamingOverlapSave) Reset() {
	for i := range sos.history {
		sos.history[i] = 0
	}
}

// BlockSize returns the block size.
func (sos *StreamingOverlapSave) BlockSize() int {
	return sos.blockSize
}

// KernelLen returns the kernel length.
func (sos *StreamingOverlapSave) KernelLen() int {
	return sos.kernelLen
}

// FFTSize returns the FFT size.
func (sos *StreamingOverlapSave) FFTSize() int {
	return sos.fftSize
}
