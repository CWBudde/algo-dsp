package conv

import (
	"math"
	"testing"
)

// Test that both implementations satisfy the StreamingConvolver interface
func TestStreamingConvolverInterface(t *testing.T) {
	kernel := []float64{1.0, 0.5, 0.25}
	blockSize := 8

	var _ StreamingConvolver = (*StreamingOverlapAdd)(nil)
	var _ StreamingConvolver = (*StreamingOverlapSave)(nil)

	// Test both implementations
	implementations := []struct {
		name string
		conv StreamingConvolver
	}{
		{"OverlapAdd", mustNewStreamingOverlapAdd(kernel, blockSize)},
		{"OverlapSave", mustNewStreamingOverlapSave(kernel, blockSize)},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			conv := impl.conv

			// Test interface methods exist and work
			if conv.BlockSize() != blockSize {
				t.Errorf("BlockSize() = %d, want %d", conv.BlockSize(), blockSize)
			}

			if conv.KernelLen() != len(kernel) {
				t.Errorf("KernelLen() = %d, want %d", conv.KernelLen(), len(kernel))
			}

			if conv.FFTSize() <= 0 {
				t.Error("FFTSize() should be positive")
			}

			// Test ProcessBlock
			input := make([]float64, blockSize)
			input[0] = 1.0

			output, err := conv.ProcessBlock(input)
			if err != nil {
				t.Fatalf("ProcessBlock failed: %v", err)
			}
			if len(output) != blockSize {
				t.Errorf("output length = %d, want %d", len(output), blockSize)
			}

			// Test Reset
			conv.Reset()

			// Test ProcessBlockTo
			outputBuf := make([]float64, blockSize)
			err = conv.ProcessBlockTo(outputBuf, input)
			if err != nil {
				t.Fatalf("ProcessBlockTo failed: %v", err)
			}
		})
	}
}

// Test that both algorithms produce equivalent results
func TestStreamingAlgorithmEquivalence(t *testing.T) {
	kernel := []float64{0.5, 1.0, 0.5, 0.2, 0.1}
	blockSize := 16
	numBlocks := 8

	// Generate test signal
	signal := make([]float64, blockSize*numBlocks)
	for i := range signal {
		signal[i] = math.Sin(float64(i)*0.1) + 0.5*math.Cos(float64(i)*0.05)
	}

	// Process with overlap-add
	ola, err := NewStreamingOverlapAdd(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapAdd failed: %v", err)
	}

	olaResult := make([]float64, 0, len(signal))
	for i := 0; i < numBlocks; i++ {
		block := signal[i*blockSize : (i+1)*blockSize]
		out, err := ola.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapAdd ProcessBlock failed: %v", err)
		}
		olaResult = append(olaResult, out...)
	}

	// Process with overlap-save
	ols, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	olsResult := make([]float64, 0, len(signal))
	for i := 0; i < numBlocks; i++ {
		block := signal[i*blockSize : (i+1)*blockSize]
		out, err := ols.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapSave ProcessBlock failed: %v", err)
		}
		olsResult = append(olsResult, out...)
	}

	// Results should be identical (within numerical precision)
	if len(olaResult) != len(olsResult) {
		t.Fatalf("Result lengths differ: OLA=%d, OLS=%d", len(olaResult), len(olsResult))
	}

	for i := range olaResult {
		diff := math.Abs(olaResult[i] - olsResult[i])
		if diff > 1e-9 {
			t.Errorf("Sample %d: OLA=%f, OLS=%f, diff=%e", i, olaResult[i], olsResult[i], diff)
		}
	}
}

// Test ProcessBlockTo equivalence
func TestStreamingAlgorithmProcessBlockToEquivalence(t *testing.T) {
	kernel := []float64{0.25, 0.5, 1.0, 0.5, 0.25}
	blockSize := 8

	// Generate test block
	input := make([]float64, blockSize)
	for i := range input {
		input[i] = float64(i)
	}

	ola, _ := NewStreamingOverlapAdd(kernel, blockSize)
	ols, _ := NewStreamingOverlapSave(kernel, blockSize)

	olaOut := make([]float64, blockSize)
	olsOut := make([]float64, blockSize)

	err := ola.ProcessBlockTo(olaOut, input)
	if err != nil {
		t.Fatalf("OLA ProcessBlockTo failed: %v", err)
	}

	err = ols.ProcessBlockTo(olsOut, input)
	if err != nil {
		t.Fatalf("OLS ProcessBlockTo failed: %v", err)
	}

	for i := range olaOut {
		diff := math.Abs(olaOut[i] - olsOut[i])
		if diff > 1e-9 {
			t.Errorf("Sample %d: OLA=%f, OLS=%f, diff=%e", i, olaOut[i], olsOut[i], diff)
		}
	}
}

// Benchmark comparison
func BenchmarkStreamingConvolvers(b *testing.B) {
	kernel := make([]float64, 4096)
	for i := range kernel {
		kernel[i] = 1.0 / float64(len(kernel))
	}
	blockSize := 128

	input := make([]float64, blockSize)
	for i := range input {
		input[i] = math.Sin(float64(i) * 0.1)
	}

	b.Run("OverlapAdd", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd(kernel, blockSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ola.ProcessBlock(input)
		}
	})

	b.Run("OverlapSave", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave(kernel, blockSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = ols.ProcessBlock(input)
		}
	})

	b.Run("OverlapAddTo", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd(kernel, blockSize)
		output := make([]float64, blockSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ola.ProcessBlockTo(output, input)
		}
	})

	b.Run("OverlapSaveTo", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave(kernel, blockSize)
		output := make([]float64, blockSize)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ols.ProcessBlockTo(output, input)
		}
	})
}

// Helper functions for tests
func mustNewStreamingOverlapAdd(kernel []float64, blockSize int) *StreamingOverlapAdd {
	conv, err := NewStreamingOverlapAdd(kernel, blockSize)
	if err != nil {
		panic(err)
	}
	return conv
}

func mustNewStreamingOverlapSave(kernel []float64, blockSize int) *StreamingOverlapSave {
	conv, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		panic(err)
	}
	return conv
}
