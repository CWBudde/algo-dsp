package conv

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

// makeImpulseKernel creates a kernel that is a scaled exponential decay.
func makeImpulseKernel(n int) []float64 {
	k := make([]float64, n)
	k[0] = 1.0
	for i := 1; i < n; i++ {
		k[i] = k[i-1] * 0.99
	}
	return k
}

// makePartitionedTestSignal creates a deterministic signal using a fixed-seed generator.
func makePartitionedTestSignal(n int) []float64 {
	rng := rand.New(rand.NewPCG(42, 0))
	sig := make([]float64, n)
	for i := range sig {
		sig[i] = rng.Float64()*2 - 1
	}
	return sig
}

// convolveWithSOA convolves signal with kernel using StreamingOverlapAdd
// using a fixed block size. Returns the full output.
func convolveWithSOA(t *testing.T, kernel, signal []float64, blockSize int) []float64 {
	t.Helper()

	soa, err := NewStreamingOverlapAdd(kernel, blockSize)
	if err != nil {
		t.Fatalf("NewStreamingOverlapAdd: %v", err)
	}

	total := len(signal)
	out := make([]float64, 0, total)

	for i := 0; i < total; i += blockSize {
		end := i + blockSize
		if end > total {
			// Pad last block with zeros.
			block := make([]float64, blockSize)
			copy(block, signal[i:])
			blkOut, err := soa.ProcessBlock(block)
			if err != nil {
				t.Fatalf("SOA ProcessBlock: %v", err)
			}
			out = append(out, blkOut...)
			break
		}

		blkOut, err := soa.ProcessBlock(signal[i:end])
		if err != nil {
			t.Fatalf("SOA ProcessBlock: %v", err)
		}
		out = append(out, blkOut...)
	}

	return out
}

// convolveWithPartitioned convolves signal using PartitionedConvolution.
// It pads input with latency zeros and skips the first latency output samples
// to compensate for the processing delay.
func convolveWithPartitioned(t *testing.T, kernel, signal []float64, minBlockOrder, maxBlockOrder int) []float64 {
	t.Helper()

	pc, err := NewPartitionedConvolution(kernel, minBlockOrder, maxBlockOrder)
	if err != nil {
		t.Fatalf("NewPartitionedConvolution: %v", err)
	}

	latency := pc.Latency()

	// Append latency zeros to flush the pipeline.
	padded := make([]float64, len(signal)+latency)
	copy(padded, signal)

	out := make([]float64, len(padded))
	if err := pc.ProcessBlock(padded, out); err != nil {
		t.Fatalf("PartitionedConvolution ProcessBlock: %v", err)
	}

	// Skip the first latency samples (the pipeline delay).
	return out[latency:]
}

func TestPartitionedConvolutionLatency(t *testing.T) {
	kernel := makeImpulseKernel(64)

	for _, order := range []int{4, 5, 6, 7} {
		t.Run("order"+string(rune('0'+order)), func(t *testing.T) {
			pc, err := NewPartitionedConvolution(kernel, order, order+4)
			if err != nil {
				t.Fatalf("NewPartitionedConvolution: %v", err)
			}

			want := 1 << order
			if pc.Latency() != want {
				t.Errorf("Latency()=%d, want %d (2^%d)", pc.Latency(), want, order)
			}
		})
	}
}

func TestPartitionedConvolutionMatchesSOA(t *testing.T) {
	tests := []struct {
		name          string
		kernelLen     int
		signalLen     int
		minBlockOrder int
		maxBlockOrder int
		tolerance     float64
	}{
		{"kernel64", 64, 512, 4, 10, 1e-7},
		{"kernel256", 256, 1024, 5, 12, 1e-7},
		{"kernel1024", 1024, 4096, 6, 13, 1e-7},
		{"kernel8192", 8192, 16384, 6, 13, 1e-7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kernel := makeImpulseKernel(tc.kernelLen)
			signal := makePartitionedTestSignal(tc.signalLen)
			latency := 1 << tc.minBlockOrder

			// Reference: StreamingOverlapAdd with blockSize = latency.
			soaOut := convolveWithSOA(t, kernel, signal, latency)

			// Under test: PartitionedConvolution.
			pcOut := convolveWithPartitioned(t, kernel, signal, tc.minBlockOrder, tc.maxBlockOrder)

			// Compare the first len(signal) output samples.
			compareLen := min(len(signal), len(pcOut), len(soaOut))

			maxDiff := 0.0
			for i := range compareLen {
				d := math.Abs(pcOut[i] - soaOut[i])
				if d > maxDiff {
					maxDiff = d
				}
			}

			if maxDiff > tc.tolerance {
				t.Errorf("max diff vs SOA: %e (tolerance %e)", maxDiff, tc.tolerance)
			}
		})
	}
}

func TestPartitionedConvolutionReset(t *testing.T) {
	kernel := makeImpulseKernel(128)
	signal := makePartitionedTestSignal(512)

	pc, err := NewPartitionedConvolution(kernel, 6, 12)
	if err != nil {
		t.Fatalf("NewPartitionedConvolution: %v", err)
	}

	out1 := make([]float64, len(signal))
	if err := pc.ProcessBlock(signal, out1); err != nil {
		t.Fatalf("first ProcessBlock: %v", err)
	}

	pc.Reset()

	out2 := make([]float64, len(signal))
	if err := pc.ProcessBlock(signal, out2); err != nil {
		t.Fatalf("second ProcessBlock after Reset: %v", err)
	}

	for i := range out1 {
		if math.Abs(out1[i]-out2[i]) > 1e-12 {
			t.Errorf("after Reset, sample %d differs: %v vs %v", i, out1[i], out2[i])
		}
	}
}

func TestPartitionedConvolutionErrors(t *testing.T) {
	t.Run("EmptyKernel", func(t *testing.T) {
		_, err := NewPartitionedConvolution([]float64{}, 6, 12)
		if !errors.Is(err, ErrEmptyImpulseResponse) {
			t.Errorf("want ErrEmptyImpulseResponse, got %v", err)
		}
	})

	t.Run("MinOrderZero", func(t *testing.T) {
		_, err := NewPartitionedConvolution([]float64{1, 2, 3}, 0, 12)
		if !errors.Is(err, ErrInvalidBlockOrder) {
			t.Errorf("want ErrInvalidBlockOrder, got %v", err)
		}
	})

	t.Run("MaxLessThanMin", func(t *testing.T) {
		_, err := NewPartitionedConvolution([]float64{1, 2, 3}, 8, 5)
		if !errors.Is(err, ErrInvalidBlockOrder) {
			t.Errorf("want ErrInvalidBlockOrder, got %v", err)
		}
	})

	t.Run("LengthMismatch", func(t *testing.T) {
		pc, err := NewPartitionedConvolution([]float64{1, 2, 3, 4}, 2, 10)
		if err != nil {
			t.Fatalf("NewPartitionedConvolution: %v", err)
		}
		in := make([]float64, 10)
		out := make([]float64, 8) // different length
		err = pc.ProcessBlock(in, out)
		if !errors.Is(err, ErrLengthMismatch) {
			t.Errorf("want ErrLengthMismatch, got %v", err)
		}
	})
}

func TestPartitionedConvolutionStageInfo(t *testing.T) {
	kernel := makeImpulseKernel(1024)

	pc, err := NewPartitionedConvolution(kernel, 6, 13)
	if err != nil {
		t.Fatalf("NewPartitionedConvolution: %v", err)
	}

	count := pc.StageCount()
	if count == 0 {
		t.Fatal("StageCount() = 0, expected at least one stage")
	}

	// Out of bounds.
	_, _, err = pc.StageInfo(-1)
	if !errors.Is(err, ErrStageIndexOutOfRange) {
		t.Errorf("StageInfo(-1): want ErrStageIndexOutOfRange, got %v", err)
	}

	_, _, err = pc.StageInfo(count)
	if !errors.Is(err, ErrStageIndexOutOfRange) {
		t.Errorf("StageInfo(%d): want ErrStageIndexOutOfRange, got %v", count, err)
	}

	// Valid info.
	for i := range count {
		partSize, blockCount, err := pc.StageInfo(i)
		if err != nil {
			t.Errorf("StageInfo(%d): %v", i, err)
			continue
		}
		if partSize <= 0 {
			t.Errorf("StageInfo(%d): partSize=%d, want >0", i, partSize)
		}
		if blockCount <= 0 {
			t.Errorf("StageInfo(%d): blockCount=%d, want >0", i, blockCount)
		}
	}
}

func TestPartitionedConvolutionKernelLen(t *testing.T) {
	kernel := makeImpulseKernel(300)
	pc, err := NewPartitionedConvolution(kernel, 6, 13)
	if err != nil {
		t.Fatalf("NewPartitionedConvolution: %v", err)
	}
	if pc.KernelLen() != len(kernel) {
		t.Errorf("KernelLen()=%d, want %d", pc.KernelLen(), len(kernel))
	}
}

func TestPartitionedConvolutionDiracDelta(t *testing.T) {
	// A dirac-delta kernel should produce a delayed copy of the input.
	kernel := []float64{1.0}
	signal := makePartitionedTestSignal(256)
	minBlockOrder := 4
	latency := 1 << minBlockOrder

	pc, err := NewPartitionedConvolution(kernel, minBlockOrder, 12)
	if err != nil {
		t.Fatalf("NewPartitionedConvolution: %v", err)
	}

	// Pad with latency zeros to flush.
	padded := make([]float64, len(signal)+latency)
	copy(padded, signal)
	out := make([]float64, len(padded))

	if err := pc.ProcessBlock(padded, out); err != nil {
		t.Fatalf("ProcessBlock: %v", err)
	}

	// output[latency:latency+len(signal)] should equal signal.
	for i, want := range signal {
		got := out[latency+i]
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("sample %d: got %v, want %v", i, got, want)
		}
	}
}

func BenchmarkPartitionedConvolution(b *testing.B) {
	kernel := makeImpulseKernel(4096)
	blockSize := 128
	minBlockOrder := 7 // latency = 128
	maxBlockOrder := 13

	pc, err := NewPartitionedConvolution(kernel, minBlockOrder, maxBlockOrder)
	if err != nil {
		b.Fatalf("NewPartitionedConvolution: %v", err)
	}

	input := makePartitionedTestSignal(blockSize)
	output := make([]float64, blockSize)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = pc.ProcessBlock(input, output)
	}
}
