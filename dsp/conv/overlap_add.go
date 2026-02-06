package conv

import (
	"fmt"

	algofft "github.com/MeKo-Christian/algo-fft"
)

// OverlapAdd implements FFT-based convolution using the overlap-add method.
// This is efficient for convolving long signals with shorter kernels.
//
// The algorithm:
// 1. Divide input signal into non-overlapping blocks
// 2. Zero-pad each block and the kernel to FFT size
// 3. Convolve via FFT multiplication in frequency domain
// 4. Overlap-add the results to form the output
type OverlapAdd struct {
	// Kernel in frequency domain
	kernelFFT []complex128

	// Configuration
	kernelLen int // Original kernel length
	blockSize int // Input block size
	fftSize   int // FFT size (blockSize + kernelLen - 1, rounded to power of 2)

	// FFT plan
	plan *algofft.Plan[complex128]

	// Scratch buffers
	inputPadded  []complex128
	outputPadded []complex128
}

// NewOverlapAdd creates a new overlap-add convolver for the given kernel.
// blockSize determines how the input signal is segmented.
// If blockSize is 0, an automatic size is chosen based on kernel length.
func NewOverlapAdd(kernel []float64, blockSize int) (*OverlapAdd, error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}

	kernelLen := len(kernel)

	// Auto-select block size if not specified
	if blockSize <= 0 {
		// Rule of thumb: block size roughly equal to or larger than kernel
		blockSize = nextPowerOf2(kernelLen)
		if blockSize < 256 {
			blockSize = 256
		}
	}

	// FFT size must accommodate block + kernel - 1 for linear convolution
	minFFTSize := blockSize + kernelLen - 1
	fftSize := nextPowerOf2(minFFTSize)

	// Create FFT plan
	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	oa := &OverlapAdd{
		kernelFFT:    make([]complex128, fftSize),
		kernelLen:    kernelLen,
		blockSize:    blockSize,
		fftSize:      fftSize,
		plan:         plan,
		inputPadded:  make([]complex128, fftSize),
		outputPadded: make([]complex128, fftSize),
	}

	// Compute kernel FFT
	kernelPadded := make([]complex128, fftSize)
	for i, v := range kernel {
		kernelPadded[i] = complex(v, 0)
	}

	err = plan.Forward(oa.kernelFFT, kernelPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to compute kernel FFT: %w", err)
	}

	return oa, nil
}

// BlockSize returns the input block size.
func (oa *OverlapAdd) BlockSize() int {
	return oa.blockSize
}

// FFTSize returns the FFT size used internally.
func (oa *OverlapAdd) FFTSize() int {
	return oa.fftSize
}

// KernelLen returns the kernel length.
func (oa *OverlapAdd) KernelLen() int {
	return oa.kernelLen
}

// Process convolves the input signal with the kernel.
// Returns the full linear convolution result.
func (oa *OverlapAdd) Process(input []float64) ([]float64, error) {
	if len(input) == 0 {
		return nil, ErrEmptyInput
	}

	// Output length for full linear convolution
	outputLen := len(input) + oa.kernelLen - 1
	output := make([]float64, outputLen)

	// Process in blocks
	numBlocks := (len(input) + oa.blockSize - 1) / oa.blockSize

	for blockIdx := 0; blockIdx < numBlocks; blockIdx++ {
		// Determine block boundaries
		start := blockIdx * oa.blockSize
		end := start + oa.blockSize
		if end > len(input) {
			end = len(input)
		}
		blockLen := end - start

		// Zero-pad input block to FFT size
		for i := range oa.inputPadded {
			oa.inputPadded[i] = 0
		}
		for i := 0; i < blockLen; i++ {
			oa.inputPadded[i] = complex(input[start+i], 0)
		}

		// Forward FFT of input block
		err := oa.plan.Forward(oa.inputPadded, oa.inputPadded)
		if err != nil {
			return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
		}

		// Multiply in frequency domain
		for i := range oa.outputPadded {
			oa.outputPadded[i] = oa.inputPadded[i] * oa.kernelFFT[i]
		}

		// Inverse FFT
		err = oa.plan.Inverse(oa.outputPadded, oa.outputPadded)
		if err != nil {
			return nil, fmt.Errorf("conv: inverse FFT failed: %w", err)
		}

		// Overlap-add: add the convolution result to the output at position start
		// The result of convolving a block of length L with kernel of length M
		// is L + M - 1 samples long. We add all of these samples to the output.
		resultLen := blockLen + oa.kernelLen - 1
		for i := 0; i < resultLen && start+i < outputLen; i++ {
			output[start+i] += real(oa.outputPadded[i])
		}
	}

	return output, nil
}

// ProcessTo convolves input and writes to pre-allocated output.
// Output must have length len(input) + kernelLen - 1.
func (oa *OverlapAdd) ProcessTo(output, input []float64) error {
	expectedLen := len(input) + oa.kernelLen - 1
	if len(output) != expectedLen {
		return fmt.Errorf("%w: expected %d, got %d", ErrLengthMismatch, expectedLen, len(output))
	}

	result, err := oa.Process(input)
	if err != nil {
		return err
	}

	copy(output, result)
	return nil
}

// Reset clears internal state (no-op for stateless overlap-add).
func (oa *OverlapAdd) Reset() {
	// No persistent state to clear in this implementation
}

// OverlapAddConvolve performs one-shot overlap-add convolution.
// This is a convenience function that creates a temporary OverlapAdd instance.
func OverlapAddConvolve(signal, kernel []float64) ([]float64, error) {
	oa, err := NewOverlapAdd(kernel, 0)
	if err != nil {
		return nil, err
	}
	return oa.Process(signal)
}

// OverlapAddConvolveTo performs one-shot overlap-add convolution to a pre-allocated buffer.
func OverlapAddConvolveTo(output, signal, kernel []float64) error {
	oa, err := NewOverlapAdd(kernel, 0)
	if err != nil {
		return err
	}
	return oa.ProcessTo(output, signal)
}
