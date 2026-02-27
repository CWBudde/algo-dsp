package conv

import (
	"errors"
	"math"
	"testing"
)

func TestStreamingOverlapSave(t *testing.T) {
	// Simple kernel
	kernel := []float64{1.0, 0.5, 0.25}
	blockSize := 4

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	// Process two blocks
	block1 := []float64{1, 0, 0, 0}
	block2 := []float64{0, 0, 0, 0}

	out1, err := sos.ProcessBlock(block1)
	if err != nil {
		t.Fatalf("ProcessBlock block1 failed: %v", err)
	}

	out2, err := sos.ProcessBlock(block2)
	if err != nil {
		t.Fatalf("ProcessBlock block2 failed: %v", err)
	}

	// Verify continuity across blocks
	if len(out1) != blockSize {
		t.Errorf("out1 length = %d, want %d", len(out1), blockSize)
	}

	if len(out2) != blockSize {
		t.Errorf("out2 length = %d, want %d", len(out2), blockSize)
	}

	// Check expected values
	expected1 := []float64{1.0, 0.5, 0.25, 0}
	for i, want := range expected1 {
		if math.Abs(out1[i]-want) > 1e-10 {
			t.Errorf("out1[%d] = %f, want %f", i, out1[i], want)
		}
	}
}

func TestStreamingOverlapSaveVsBatch(t *testing.T) {
	// Verify streaming produces same result as batch processing
	kernel := []float64{0.5, 1.0, 0.5, 0.2}
	blockSize := 8
	numBlocks := 4

	// Generate test signal
	signal := make([]float64, blockSize*numBlocks)
	for i := range signal {
		signal[i] = math.Sin(float64(i) * 0.1)
	}

	// Batch processing
	batchOS, err := NewOverlapSave(kernel, 0)
	if err != nil {
		t.Fatalf("NewOverlapSave failed: %v", err)
	}

	batchResult, err := batchOS.Process(signal)
	if err != nil {
		t.Fatalf("Batch Process failed: %v", err)
	}

	// Streaming processing
	streamOS, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	streamResult := make([]float64, 0, len(signal))
	for i := range numBlocks {
		block := signal[i*blockSize : (i+1)*blockSize]

		out, err := streamOS.ProcessBlock(block)
		if err != nil {
			t.Fatalf("ProcessBlock failed at block %d: %v", i, err)
		}

		streamResult = append(streamResult, out...)
	}

	// Compare results (first len(signal) samples should match)
	for i := range signal {
		diff := math.Abs(batchResult[i] - streamResult[i])
		if diff > 1e-10 {
			t.Errorf("Sample %d: batch=%f, stream=%f, diff=%e", i, batchResult[i], streamResult[i], diff)
		}
	}
}

func TestStreamingOverlapSaveReset(t *testing.T) {
	kernel := []float64{1.0, 0.5}
	blockSize := 4

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	// Process a block
	block1 := []float64{1, 1, 1, 1}

	_, err = sos.ProcessBlock(block1)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	// Reset
	sos.Reset()

	// Process same block again - should give identical result
	out1, _ := sos.ProcessBlock(block1)
	sos.Reset()
	out2, _ := sos.ProcessBlock(block1)

	for i := range out1 {
		if math.Abs(out1[i]-out2[i]) > 1e-10 {
			t.Errorf("After reset, sample %d differs: %f vs %f", i, out1[i], out2[i])
		}
	}
}

func TestStreamingOverlapSaveProcessBlockTo(t *testing.T) {
	kernel := []float64{1.0, 0.5, 0.25}
	blockSize := 8

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	input := make([]float64, blockSize)
	for i := range input {
		input[i] = float64(i)
	}

	// Test with pre-allocated output
	output := make([]float64, blockSize)

	err = sos.ProcessBlockTo(output, input)
	if err != nil {
		t.Fatalf("ProcessBlockTo failed: %v", err)
	}

	// Compare with ProcessBlock
	sos.Reset()

	expected, err := sos.ProcessBlock(input)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	for i := range output {
		if math.Abs(output[i]-expected[i]) > 1e-10 {
			t.Errorf("ProcessBlockTo vs ProcessBlock differ at %d: %f vs %f", i, output[i], expected[i])
		}
	}
}

func BenchmarkStreamingOverlapSave(b *testing.B) {
	kernel := make([]float64, 4096) // Typical IR length
	for i := range kernel {
		kernel[i] = 1.0 / float64(len(kernel))
	}

	blockSize := 128

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		b.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	input := make([]float64, blockSize)
	for i := range input {
		input[i] = math.Sin(float64(i) * 0.1)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = sos.ProcessBlock(input)
	}
}

func BenchmarkStreamingOverlapSaveTo(b *testing.B) {
	kernel := make([]float64, 4096) // Typical IR length
	for i := range kernel {
		kernel[i] = 1.0 / float64(len(kernel))
	}

	blockSize := 128

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		b.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	input := make([]float64, blockSize)
	output := make([]float64, blockSize)

	for i := range input {
		input[i] = math.Sin(float64(i) * 0.1)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = sos.ProcessBlockTo(output, input)
	}
}

// Test error conditions.
func TestStreamingOverlapSaveErrors(t *testing.T) {
	t.Run("EmptyKernel", func(t *testing.T) {
		_, err := NewStreamingOverlapSave([]float64{}, 128)
		if !errors.Is(err, ErrEmptyKernel) {
			t.Errorf("expected ErrEmptyKernel, got %v", err)
		}
	})

	t.Run("ZeroBlockSize", func(t *testing.T) {
		_, err := NewStreamingOverlapSave([]float64{1.0}, 0)
		if err == nil {
			t.Error("expected error for zero block size")
		}
	})

	t.Run("NegativeBlockSize", func(t *testing.T) {
		_, err := NewStreamingOverlapSave([]float64{1.0}, -1)
		if err == nil {
			t.Error("expected error for negative block size")
		}
	})

	t.Run("WrongInputSize", func(t *testing.T) {
		sos, err := NewStreamingOverlapSave([]float64{1.0, 0.5}, 4)
		if err != nil {
			t.Fatalf("NewStreamingOverlapSave failed: %v", err)
		}

		// Wrong size input
		wrongInput := []float64{1, 2, 3} // Expected 4

		_, err = sos.ProcessBlock(wrongInput)
		if err == nil {
			t.Error("expected error for wrong input size")
		}
	})

	t.Run("WrongOutputSize", func(t *testing.T) {
		sos, err := NewStreamingOverlapSave([]float64{1.0, 0.5}, 4)
		if err != nil {
			t.Fatalf("NewStreamingOverlapSave failed: %v", err)
		}

		input := []float64{1, 2, 3, 4}
		wrongOutput := []float64{0, 0, 0} // Expected 4

		err = sos.ProcessBlockTo(wrongOutput, input)
		if err == nil {
			t.Error("expected error for wrong output size")
		}
	})
}

// Test getter methods.
func TestStreamingOverlapSaveGetters(t *testing.T) {
	kernel := []float64{1.0, 0.5, 0.25, 0.1}
	blockSize := 8

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	if sos.BlockSize() != blockSize {
		t.Errorf("BlockSize() = %d, want %d", sos.BlockSize(), blockSize)
	}

	if sos.KernelLen() != len(kernel) {
		t.Errorf("KernelLen() = %d, want %d", sos.KernelLen(), len(kernel))
	}

	// FFT size should be power of 2 and >= blockSize + kernelLen - 1
	minFFTSize := blockSize + len(kernel) - 1

	expectedFFTSize := nextPowerOf2(minFFTSize)
	if sos.FFTSize() != expectedFFTSize {
		t.Errorf("FFTSize() = %d, want %d", sos.FFTSize(), expectedFFTSize)
	}
}

// Test edge case: single-sample kernel (dirac delta).
func TestStreamingOverlapSaveDiracDelta(t *testing.T) {
	kernel := []float64{1.0}
	blockSize := 8

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	input := []float64{1, 2, 3, 4, 5, 6, 7, 8}

	output, err := sos.ProcessBlock(input)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	// Dirac delta should pass through unchanged
	for i, want := range input {
		if math.Abs(output[i]-want) > 1e-10 {
			t.Errorf("Sample %d: got %f, want %f", i, output[i], want)
		}
	}
}

// Test edge case: very long kernel.
func TestStreamingOverlapSaveLongKernel(t *testing.T) {
	// Kernel longer than block size with slower decay
	kernel := make([]float64, 256)

	kernel[0] = 1.0
	for i := 1; i < len(kernel); i++ {
		kernel[i] = 0.95 * kernel[i-1] // Slower exponential decay
	}

	blockSize := 64

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	// Process impulse
	block1 := make([]float64, blockSize)
	block1[0] = 1.0

	out1, err := sos.ProcessBlock(block1)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	// First output should contain start of kernel
	if math.Abs(out1[0]-kernel[0]) > 1e-10 {
		t.Errorf("Impulse response mismatch at sample 0")
	}

	// Process several more blocks to see continuation
	// With kernel length 256 and block size 64, we expect output for at least 3 blocks
	for i := range 3 {
		zeros := make([]float64, blockSize)

		out, err := sos.ProcessBlock(zeros)
		if err != nil {
			t.Fatalf("ProcessBlock %d failed: %v", i, err)
		}
		// Should still have non-zero output from long kernel (using higher threshold)
		maxVal := 0.0
		for _, v := range out {
			if math.Abs(v) > maxVal {
				maxVal = math.Abs(v)
			}
		}

		if maxVal < 1e-6 {
			t.Errorf("Block %d: expected stronger continuation from long kernel, got max=%e", i, maxVal)
		}
	}
}

// Test continuity across many blocks.
func TestStreamingOverlapSaveContinuity(t *testing.T) {
	kernel := []float64{0.25, 0.5, 1.0, 0.5, 0.25}
	blockSize := 16

	sos, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	// Generate continuous sine wave
	numBlocks := 8
	totalSamples := blockSize * numBlocks

	fullSignal := make([]float64, totalSamples)
	for i := range fullSignal {
		fullSignal[i] = math.Sin(float64(i) * 0.2)
	}

	// Process in blocks
	streamOutput := make([]float64, 0, totalSamples)

	for i := range numBlocks {
		block := fullSignal[i*blockSize : (i+1)*blockSize]

		out, err := sos.ProcessBlock(block)
		if err != nil {
			t.Fatalf("Block %d failed: %v", i, err)
		}

		streamOutput = append(streamOutput, out...)
	}

	// Compare with batch processing
	batchOS, _ := NewOverlapSave(kernel, 0)
	batchResult, _ := batchOS.Process(fullSignal)

	// First len(fullSignal) samples should match
	for i := range totalSamples {
		diff := math.Abs(batchResult[i] - streamOutput[i])
		if diff > 1e-9 {
			t.Errorf("Sample %d: batch=%f, stream=%f, diff=%e", i, batchResult[i], streamOutput[i], diff)
		}
	}
}
