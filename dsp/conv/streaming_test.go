package conv

import (
	"math"
	"testing"
)

// Test that both implementations satisfy the StreamingConvolver interface (float64).
func TestStreamingConvolverInterface(t *testing.T) {
	kernel := []float64{1.0, 0.5, 0.25}
	blockSize := 8

	var (
		_ StreamingConvolver = (*StreamingOverlapAdd)(nil)
		_ StreamingConvolver = (*StreamingOverlapSave)(nil)
	)

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

			if conv.BlockSize() != blockSize {
				t.Errorf("BlockSize() = %d, want %d", conv.BlockSize(), blockSize)
			}

			if conv.KernelLen() != len(kernel) {
				t.Errorf("KernelLen() = %d, want %d", conv.KernelLen(), len(kernel))
			}

			if conv.FFTSize() <= 0 {
				t.Error("FFTSize() should be positive")
			}

			input := make([]float64, blockSize)
			input[0] = 1.0

			output, err := conv.ProcessBlock(input)
			if err != nil {
				t.Fatalf("ProcessBlock failed: %v", err)
			}

			if len(output) != blockSize {
				t.Errorf("output length = %d, want %d", len(output), blockSize)
			}

			conv.Reset()

			outputBuf := make([]float64, blockSize)

			err = conv.ProcessBlockTo(outputBuf, input)
			if err != nil {
				t.Fatalf("ProcessBlockTo failed: %v", err)
			}
		})
	}
}

// Test that both float32 implementations satisfy the StreamingConvolverT interface.
func TestStreamingConvolverInterface32(t *testing.T) {
	kernel := []float32{1.0, 0.5, 0.25}
	blockSize := 8

	var (
		_ StreamingConvolverT[float32, complex64] = (*StreamingOverlapAddT[float32, complex64])(nil)
		_ StreamingConvolverT[float32, complex64] = (*StreamingOverlapSaveT[float32, complex64])(nil)
	)

	implementations := []struct {
		name string
		conv StreamingConvolverT[float32, complex64]
	}{
		{"OverlapAdd32", mustNewStreamingOverlapAdd32(kernel, blockSize)},
		{"OverlapSave32", mustNewStreamingOverlapSave32(kernel, blockSize)},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			conv := impl.conv

			if conv.BlockSize() != blockSize {
				t.Errorf("BlockSize() = %d, want %d", conv.BlockSize(), blockSize)
			}

			if conv.KernelLen() != len(kernel) {
				t.Errorf("KernelLen() = %d, want %d", conv.KernelLen(), len(kernel))
			}

			input := make([]float32, blockSize)
			input[0] = 1.0

			output, err := conv.ProcessBlock(input)
			if err != nil {
				t.Fatalf("ProcessBlock failed: %v", err)
			}

			if len(output) != blockSize {
				t.Errorf("output length = %d, want %d", len(output), blockSize)
			}

			conv.Reset()

			outputBuf := make([]float32, blockSize)

			err = conv.ProcessBlockTo(outputBuf, input)
			if err != nil {
				t.Fatalf("ProcessBlockTo failed: %v", err)
			}
		})
	}
}

// Test that both algorithms produce equivalent results (float64).
func TestStreamingAlgorithmEquivalence(t *testing.T) {
	kernel := []float64{0.5, 1.0, 0.5, 0.2, 0.1}
	blockSize := 16
	numBlocks := 8

	signal := make([]float64, blockSize*numBlocks)
	for i := range signal {
		signal[i] = math.Sin(float64(i)*0.1) + 0.5*math.Cos(float64(i)*0.05)
	}

	ola, err := NewStreamingOverlapAdd(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapAdd failed: %v", err)
	}

	olaResult := make([]float64, 0, len(signal))
	for i := range numBlocks {
		block := signal[i*blockSize : (i+1)*blockSize]

		out, err := ola.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapAdd ProcessBlock failed: %v", err)
		}

		olaResult = append(olaResult, out...)
	}

	ols, err := NewStreamingOverlapSave(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave failed: %v", err)
	}

	olsResult := make([]float64, 0, len(signal))
	for i := range numBlocks {
		block := signal[i*blockSize : (i+1)*blockSize]

		out, err := ols.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapSave ProcessBlock failed: %v", err)
		}

		olsResult = append(olsResult, out...)
	}

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

// Test float32 algorithm equivalence between overlap-add and overlap-save.
func TestStreamingAlgorithmEquivalence32(t *testing.T) {
	kernel := []float32{0.5, 1.0, 0.5, 0.2, 0.1}
	blockSize := 16
	numBlocks := 8

	signal := make([]float32, blockSize*numBlocks)
	for i := range signal {
		signal[i] = float32(math.Sin(float64(i)*0.1) + 0.5*math.Cos(float64(i)*0.05))
	}

	ola, err := NewStreamingOverlapAdd32(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapAdd32 failed: %v", err)
	}

	olaResult := make([]float32, 0, len(signal))
	for i := range numBlocks {
		block := signal[i*blockSize : (i+1)*blockSize]

		out, err := ola.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapAdd32 ProcessBlock failed: %v", err)
		}

		olaResult = append(olaResult, out...)
	}

	ols, err := NewStreamingOverlapSave32(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapSave32 failed: %v", err)
	}

	olsResult := make([]float32, 0, len(signal))
	for i := range numBlocks {
		block := signal[i*blockSize : (i+1)*blockSize]

		out, err := ols.ProcessBlock(block)
		if err != nil {
			t.Fatalf("OverlapSave32 ProcessBlock failed: %v", err)
		}

		olsResult = append(olsResult, out...)
	}

	if len(olaResult) != len(olsResult) {
		t.Fatalf("Result lengths differ: OLA=%d, OLS=%d", len(olaResult), len(olsResult))
	}

	// float32 has less precision, use wider tolerance
	for i := range olaResult {
		diff := math.Abs(float64(olaResult[i] - olsResult[i]))
		if diff > 1e-4 {
			t.Errorf("Sample %d: OLA=%f, OLS=%f, diff=%e", i, olaResult[i], olsResult[i], diff)
		}
	}
}

// Test float32 produces correct impulse response.
func TestStreamingFloat32ImpulseResponse(t *testing.T) {
	kernel := []float32{1.0, 0.5, 0.25}
	blockSize := 8

	implementations := []struct {
		name string
		conv StreamingConvolverT[float32, complex64]
	}{
		{"OverlapAdd32", mustNewStreamingOverlapAdd32(kernel, blockSize)},
		{"OverlapSave32", mustNewStreamingOverlapSave32(kernel, blockSize)},
	}

	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			input := make([]float32, blockSize)
			input[0] = 1.0

			output, err := impl.conv.ProcessBlock(input)
			if err != nil {
				t.Fatalf("ProcessBlock failed: %v", err)
			}

			// Impulse response should match kernel
			expected := []float32{1.0, 0.5, 0.25, 0, 0, 0, 0, 0}
			for i, want := range expected {
				if math.Abs(float64(output[i]-want)) > 1e-5 {
					t.Errorf("output[%d] = %f, want %f", i, output[i], want)
				}
			}
		})
	}
}

// Test ProcessBlockTo equivalence.
func TestStreamingAlgorithmProcessBlockToEquivalence(t *testing.T) {
	kernel := []float64{0.25, 0.5, 1.0, 0.5, 0.25}
	blockSize := 8

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

// Benchmark comparison: float64 vs float32.
func BenchmarkStreamingConvolvers(b *testing.B) {
	kernel64 := make([]float64, 4096)
	kernel32 := make([]float32, 4096)

	for i := range kernel64 {
		kernel64[i] = 1.0 / float64(len(kernel64))
		kernel32[i] = float32(kernel64[i])
	}

	blockSize := 128

	input64 := make([]float64, blockSize)
	input32 := make([]float32, blockSize)

	for i := range input64 {
		input64[i] = math.Sin(float64(i) * 0.1)
		input32[i] = float32(input64[i])
	}

	b.Run("OverlapAdd/f64", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd(kernel64, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_, _ = ola.ProcessBlock(input64)
		}
	})

	b.Run("OverlapAdd/f32", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd32(kernel32, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_, _ = ola.ProcessBlock(input32)
		}
	})

	b.Run("OverlapSave/f64", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave(kernel64, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_, _ = ols.ProcessBlock(input64)
		}
	})

	b.Run("OverlapSave/f32", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave32(kernel32, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_, _ = ols.ProcessBlock(input32)
		}
	})

	b.Run("OverlapAddTo/f64", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd(kernel64, blockSize)
		output := make([]float64, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_ = ola.ProcessBlockTo(output, input64)
		}
	})

	b.Run("OverlapAddTo/f32", func(b *testing.B) {
		ola, _ := NewStreamingOverlapAdd32(kernel32, blockSize)
		output := make([]float32, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_ = ola.ProcessBlockTo(output, input32)
		}
	})

	b.Run("OverlapSaveTo/f64", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave(kernel64, blockSize)
		output := make([]float64, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_ = ols.ProcessBlockTo(output, input64)
		}
	})

	b.Run("OverlapSaveTo/f32", func(b *testing.B) {
		ols, _ := NewStreamingOverlapSave32(kernel32, blockSize)
		output := make([]float32, blockSize)

		b.ReportAllocs()
		b.ResetTimer()

		for range b.N {
			_ = ols.ProcessBlockTo(output, input32)
		}
	})
}

// Helper functions.
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

func mustNewStreamingOverlapAdd32(kernel []float32, blockSize int) *StreamingOverlapAddT[float32, complex64] {
	conv, err := NewStreamingOverlapAdd32(kernel, blockSize)
	if err != nil {
		panic(err)
	}

	return conv
}

func mustNewStreamingOverlapSave32(kernel []float32, blockSize int) *StreamingOverlapSaveT[float32, complex64] {
	conv, err := NewStreamingOverlapSave32(kernel, blockSize)
	if err != nil {
		panic(err)
	}

	return conv
}
