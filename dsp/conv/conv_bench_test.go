package conv

import (
	"fmt"
	"math"
	"testing"
)

// Benchmark direct convolution with various sizes.
func BenchmarkDirect(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{256, 8},
		{256, 32},
		{256, 64},
		{1024, 8},
		{1024, 32},
		{1024, 64},
		{4096, 8},
		{4096, 32},
		{4096, 64},
	}

	for _, size := range sizes {
		signal := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Direct(signal, kernel)
			}
		})
	}
}

// Benchmark overlap-add convolution with various sizes.
func BenchmarkOverlapAdd(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{1024, 64},
		{1024, 128},
		{1024, 256},
		{4096, 64},
		{4096, 128},
		{4096, 256},
		{16384, 64},
		{16384, 256},
		{16384, 1024},
	}

	for _, size := range sizes {
		signal := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = OverlapAddConvolve(signal, kernel)
			}
		})
	}
}

// Benchmark overlap-add with pre-created convolver.
func BenchmarkOverlapAddReuse(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{4096, 64},
		{4096, 256},
		{16384, 256},
		{16384, 1024},
	}

	for _, size := range sizes {
		signal := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)

		oa, err := NewOverlapAdd(kernel, 0)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = oa.Process(signal)
			}
		})
	}
}

// Benchmark overlap-save convolution.
func BenchmarkOverlapSave(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{1024, 64},
		{1024, 128},
		{4096, 64},
		{4096, 256},
		{16384, 256},
		{16384, 1024},
	}

	for _, size := range sizes {
		signal := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = OverlapSaveConvolve(signal, kernel)
			}
		})
	}
}

// Benchmark auto-selected convolution.
func BenchmarkConvolve(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{1024, 8},    // Should use direct
		{1024, 32},   // Should use direct
		{1024, 64},   // Threshold
		{1024, 128},  // Should use FFT
		{4096, 32},   // Should use direct
		{4096, 128},  // Should use FFT
		{16384, 64},  // Threshold
		{16384, 256}, // Should use FFT
	}

	for _, size := range sizes {
		signal := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Convolve(signal, kernel)
			}
		})
	}
}

// Benchmark correlation.
func BenchmarkCorrelate(b *testing.B) {
	sizes := []int{256, 1024, 4096}

	for _, size := range sizes {
		signal := makeTestSignal(size)
		template := makeTestSignal(size / 4)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Correlate(signal, template)
			}
		})
	}
}

// Benchmark auto-correlation.
func BenchmarkAutoCorrelate(b *testing.B) {
	sizes := []int{256, 1024, 4096}

	for _, size := range sizes {
		signal := makeTestSignal(size)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = AutoCorrelate(signal)
			}
		})
	}
}

// Benchmark deconvolution.
func BenchmarkDeconvolve(b *testing.B) {
	sizes := []struct {
		signal int
		kernel int
	}{
		{256, 16},
		{1024, 32},
		{4096, 64},
	}

	for _, size := range sizes {
		// Create convolved signal
		original := makeTestSignal(size.signal)
		kernel := makeTestKernel(size.kernel)
		convolved, _ := Direct(original, kernel)

		opts := DefaultDeconvOptions()

		b.Run(fmt.Sprintf("signal=%d_kernel=%d", size.signal, size.kernel), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = Deconvolve(convolved, kernel, opts)
			}
		})
	}
}

// CrossoverPointTest compares direct vs FFT-based convolution.
// This is not a benchmark per se, but helps determine crossover points.
func TestCrossoverPoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping crossover analysis in short mode")
	}

	signalSize := 4096
	signal := makeTestSignal(signalSize)

	type result struct {
		kernelSize int
		directNs   int64
		fftNs      int64
		winner     string
	}

	var results []result

	for kernelSize := 4; kernelSize <= 512; kernelSize *= 2 {
		kernel := makeTestKernel(kernelSize)

		// Time direct
		directResult := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = Direct(signal, kernel)
			}
		})

		// Time FFT
		fftResult := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = OverlapAddConvolve(signal, kernel)
			}
		})

		winner := "direct"
		if fftResult.NsPerOp() < directResult.NsPerOp() {
			winner = "fft"
		}

		results = append(results, result{
			kernelSize: kernelSize,
			directNs:   directResult.NsPerOp(),
			fftNs:      fftResult.NsPerOp(),
			winner:     winner,
		})

		t.Logf("kernel=%4d: direct=%10d ns, fft=%10d ns, winner=%s",
			kernelSize, directResult.NsPerOp(), fftResult.NsPerOp(), winner)
	}

	// Log crossover point
	for i := 1; i < len(results); i++ {
		if results[i].winner != results[i-1].winner {
			t.Logf("Crossover between kernel size %d and %d",
				results[i-1].kernelSize, results[i].kernelSize)
		}
	}
}

// Helper to create test signals.
func makeTestSignal(n int) []float64 {
	signal := make([]float64, n)
	for i := range signal {
		signal[i] = math.Sin(2*math.Pi*float64(i)/100) + 0.5*math.Cos(2*math.Pi*float64(i)/30)
	}
	return signal
}

// Helper to create test kernels.
func makeTestKernel(n int) []float64 {
	kernel := make([]float64, n)
	// Simple lowpass-like kernel (sinc-ish)
	center := float64(n-1) / 2
	for i := range kernel {
		x := float64(i) - center
		if x == 0 {
			kernel[i] = 1.0
		} else {
			kernel[i] = math.Sin(math.Pi*x/4) / (math.Pi * x / 4)
		}
		// Apply Hann window
		kernel[i] *= 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return kernel
}
