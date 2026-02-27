//nolint:funlen,gocognit,gocritic,cyclop
package conv

import (
	"fmt"
	"sync"

	algofft "github.com/cwbudde/algo-fft"
)

// Pool of OverlapSave instances to reduce allocations in one-shot convolutions.
var (
	overlapSavePoolsMu sync.RWMutex
	overlapSavePools   = make(map[int]*sync.Pool) // keyed by FFT size
)

// OverlapSave implements FFT-based convolution using the overlap-save method.
// Also known as "overlap-scrap" or "select-save", this method uses overlapping
// input segments and discards the circular convolution wrap-around portion.
//
// The algorithm:
// 1. Segment input with overlap of (kernelLen - 1) samples
// 2. Each segment has length fftSize (power of 2)
// 3. Convolve via FFT (circular convolution)
// 4. Discard first (kernelLen - 1) samples of each result (wrap-around)
// 5. Concatenate valid portions
//
// Compared to overlap-add:
//   - Overlap-save may be slightly more efficient (no explicit addition step)
//   - Overlap-add is often simpler to understand and implement
//   - Both have similar computational complexity
type OverlapSave struct {
	// Kernel in frequency domain
	kernelFFT []complex128

	// Configuration
	kernelLen int // Original kernel length
	fftSize   int // FFT size (must be power of 2, >= 2 * kernelLen)
	stepSize  int // Valid output samples per block = fftSize - kernelLen + 1

	// FFT plan
	plan *algofft.Plan[complex128]

	// Scratch buffers
	inputBuffer  []complex128 // FFT input buffer
	outputBuffer []complex128 // FFT output buffer
	history      []float64    // Previous input samples for overlap
}

// NewOverlapSave creates a new overlap-save convolver for the given kernel.
// fftSize must be a power of 2 and at least 2 * len(kernel).
// If fftSize is 0, an automatic size is chosen.
func NewOverlapSave(kernel []float64, fftSize int) (*OverlapSave, error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}

	kernelLen := len(kernel)

	// Auto-select FFT size if not specified
	if fftSize <= 0 {
		// Choose FFT size to be at least 2x kernel length for efficiency
		fftSize = max(nextPowerOf2(2*kernelLen), 256)
	}

	// Validate FFT size
	if !isPowerOf2(fftSize) {
		return nil, fmt.Errorf("%w: fftSize must be power of 2, got %d", ErrInvalidBlockSize, fftSize)
	}

	if fftSize < 2*kernelLen {
		fftSize = nextPowerOf2(2 * kernelLen)
	}

	// Step size is the number of valid output samples per block
	stepSize := fftSize - kernelLen + 1

	// Create FFT plan
	plan, err := algofft.NewPlan64(fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to create FFT plan: %w", err)
	}

	os := &OverlapSave{
		kernelFFT:    make([]complex128, fftSize),
		kernelLen:    kernelLen,
		fftSize:      fftSize,
		stepSize:     stepSize,
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

	err = plan.Forward(os.kernelFFT, kernelPadded)
	if err != nil {
		return nil, fmt.Errorf("conv: failed to compute kernel FFT: %w", err)
	}

	return os, nil
}

// FFTSize returns the FFT size used internally.
func (os *OverlapSave) FFTSize() int {
	return os.fftSize
}

// StepSize returns the number of valid output samples per FFT block.
func (os *OverlapSave) StepSize() int {
	return os.stepSize
}

// KernelLen returns the kernel length.
func (os *OverlapSave) KernelLen() int {
	return os.kernelLen
}

// Process convolves the input signal with the kernel.
// Returns the full linear convolution result.
func (os *OverlapSave) Process(input []float64) ([]float64, error) {
	if len(input) == 0 {
		return nil, ErrEmptyInput
	}

	// Output length for full linear convolution
	outputLen := len(input) + os.kernelLen - 1
	output := make([]float64, outputLen)

	// Reset history
	for i := range os.history {
		os.history[i] = 0
	}

	// Process input in steps
	inputPos := 0
	outputPos := 0

	for inputPos < len(input) {
		// Build input block: history + new samples
		for i := range os.inputBuffer {
			os.inputBuffer[i] = 0
		}

		// Copy history (kernelLen - 1 samples)
		for i := range os.kernelLen - 1 {
			os.inputBuffer[i] = complex(os.history[i], 0)
		}

		// Copy new input samples
		newSamples := os.stepSize
		if inputPos+newSamples > len(input) {
			newSamples = len(input) - inputPos
		}

		for i := range newSamples {
			os.inputBuffer[os.kernelLen-1+i] = complex(input[inputPos+i], 0)
		}

		// Forward FFT
		err := os.plan.Forward(os.inputBuffer, os.inputBuffer)
		if err != nil {
			return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
		}

		// Multiply in frequency domain
		for i := range os.outputBuffer {
			os.outputBuffer[i] = os.inputBuffer[i] * os.kernelFFT[i]
		}

		// Inverse FFT
		err = os.plan.Inverse(os.outputBuffer, os.outputBuffer)
		if err != nil {
			return nil, fmt.Errorf("conv: inverse FFT failed: %w", err)
		}

		// Discard wrap-around (first kernelLen - 1 samples) and keep valid portion
		validStart := os.kernelLen - 1
		for i := 0; i < newSamples && outputPos+i < outputLen; i++ {
			output[outputPos+i] = real(os.outputBuffer[validStart+i])
		}

		// Update history for next block
		// History is the last (kernelLen - 1) samples of current input block
		historyStart := max(newSamples, 0)

		for i := range os.kernelLen - 1 {
			idx := historyStart + i
			if idx < os.stepSize && inputPos+idx < len(input) {
				os.history[i] = input[inputPos+idx]
			} else if inputPos+newSamples+i-os.stepSize >= 0 && inputPos+newSamples+i-os.stepSize < len(input) {
				os.history[i] = input[inputPos+newSamples+i-os.stepSize]
			} else {
				os.history[i] = 0
			}
		}

		// Actually, for overlap-save, history should be the last samples that will overlap
		// Let's simplify: history = last (kernelLen-1) samples we've seen
		actualHistoryStart := inputPos + newSamples - (os.kernelLen - 1)
		for i := range os.kernelLen - 1 {
			idx := actualHistoryStart + i
			if idx >= 0 && idx < len(input) {
				os.history[i] = input[idx]
			} else if idx < 0 {
				os.history[i] = 0
			} else {
				os.history[i] = 0
			}
		}

		inputPos += newSamples
		outputPos += newSamples
	}

	// Handle the tail (remaining samples from the convolution)
	// For full convolution, we need kernelLen - 1 more output samples
	// Process one more block with zero-padded input
	if outputPos < outputLen {
		for i := range os.inputBuffer {
			os.inputBuffer[i] = 0
		}

		for i := range os.kernelLen - 1 {
			os.inputBuffer[i] = complex(os.history[i], 0)
		}

		err := os.plan.Forward(os.inputBuffer, os.inputBuffer)
		if err != nil {
			return nil, fmt.Errorf("conv: forward FFT failed: %w", err)
		}

		for i := range os.outputBuffer {
			os.outputBuffer[i] = os.inputBuffer[i] * os.kernelFFT[i]
		}

		err = os.plan.Inverse(os.outputBuffer, os.outputBuffer)
		if err != nil {
			return nil, fmt.Errorf("conv: inverse FFT failed: %w", err)
		}

		validStart := os.kernelLen - 1
		for i := 0; outputPos+i < outputLen && validStart+i < os.fftSize; i++ {
			output[outputPos+i] = real(os.outputBuffer[validStart+i])
		}
	}

	return output, nil
}

// ProcessTo convolves input and writes to pre-allocated output.
// Output must have length len(input) + kernelLen - 1.
func (os *OverlapSave) ProcessTo(output, input []float64) error {
	expectedLen := len(input) + os.kernelLen - 1
	if len(output) != expectedLen {
		return fmt.Errorf("%w: expected %d, got %d", ErrLengthMismatch, expectedLen, len(output))
	}

	result, err := os.Process(input)
	if err != nil {
		return err
	}

	copy(output, result)

	return nil
}

// Reset clears the history buffer for processing a new signal.
func (os *OverlapSave) Reset() {
	for i := range os.history {
		os.history[i] = 0
	}
}

// getOverlapSavePool returns the pool for the given FFT size, creating it if needed.
func getOverlapSavePool(fftSize int) *sync.Pool {
	overlapSavePoolsMu.RLock()

	pool, ok := overlapSavePools[fftSize]

	overlapSavePoolsMu.RUnlock()

	if ok {
		return pool
	}

	overlapSavePoolsMu.Lock()
	defer overlapSavePoolsMu.Unlock()

	// Check again in case another goroutine created it
	if pool, ok := overlapSavePools[fftSize]; ok {
		return pool
	}

	pool = &sync.Pool{
		New: func() any {
			return &OverlapSave{}
		},
	}
	overlapSavePools[fftSize] = pool

	return pool
}

// OverlapSaveConvolve performs one-shot overlap-save convolution.
// This function uses a pool of OverlapSave instances to minimize allocations.
func OverlapSaveConvolve(signal, kernel []float64) ([]float64, error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyKernel
	}

	kernelLen := len(kernel)

	// Determine configuration (same logic as NewOverlapSave)
	fftSize := max(nextPowerOf2(2*kernelLen), 256)

	// Get a pooled instance
	pool := getOverlapSavePool(fftSize)

	v := pool.Get()

	os, ok := v.(*OverlapSave)
	if !ok || os == nil {
		panic("conv: overlap-save pool returned unexpected type")
	}

	defer pool.Put(os)

	// Initialize/reinitialize the instance for this kernel
	err := initOverlapSave(os, kernel, fftSize)
	if err != nil {
		return nil, err
	}

	return os.Process(signal)
}

// initOverlapSave initializes or reinitializes an OverlapSave instance.
func initOverlapSave(os *OverlapSave, kernel []float64, fftSize int) error {
	kernelLen := len(kernel)
	stepSize := fftSize - kernelLen + 1

	// Allocate or resize buffers if needed
	if len(os.kernelFFT) != fftSize {
		os.kernelFFT = make([]complex128, fftSize)
		os.inputBuffer = make([]complex128, fftSize)
		os.outputBuffer = make([]complex128, fftSize)
		os.history = make([]float64, kernelLen-1)

		// Create FFT plan
		plan, err := algofft.NewPlan64(fftSize)
		if err != nil {
			return fmt.Errorf("conv: failed to create FFT plan: %w", err)
		}

		os.plan = plan
	} else {
		// Resize history if kernel length changed
		if len(os.history) != kernelLen-1 {
			os.history = make([]float64, kernelLen-1)
		}
	}

	os.kernelLen = kernelLen
	os.fftSize = fftSize
	os.stepSize = stepSize

	// Compute kernel FFT
	kernelPadded := os.inputBuffer // Reuse inputBuffer as temporary
	for i := range kernelPadded {
		kernelPadded[i] = 0
	}

	for i, v := range kernel {
		kernelPadded[i] = complex(v, 0)
	}

	err := os.plan.Forward(os.kernelFFT, kernelPadded)
	if err != nil {
		return fmt.Errorf("conv: failed to compute kernel FFT: %w", err)
	}

	return nil
}
