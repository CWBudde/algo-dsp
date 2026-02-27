package conv_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/conv"
)

func ExampleDirect() {
	// Simple moving average filter
	signal := []float64{1, 2, 3, 4, 5, 4, 3, 2, 1}
	kernel := []float64{0.25, 0.5, 0.25}

	result, _ := conv.Direct(signal, kernel)

	fmt.Printf("Input length: %d\n", len(signal))
	fmt.Printf("Kernel length: %d\n", len(kernel))
	fmt.Printf("Output length: %d\n", len(result))
	fmt.Printf("First few values: %.2f, %.2f, %.2f\n", result[0], result[1], result[2])

	// Output:
	// Input length: 9
	// Kernel length: 3
	// Output length: 11
	// First few values: 0.25, 1.00, 2.00
}

func ExampleConvolve() {
	// Convolve automatically selects the best algorithm
	signal := make([]float64, 1000)
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * float64(i) / 50)
	}

	// Short kernel uses direct convolution
	shortKernel := []float64{0.2, 0.3, 0.3, 0.2}
	result1, _ := conv.Convolve(signal, shortKernel)
	fmt.Printf("Short kernel result length: %d\n", len(result1))

	// Longer kernel uses FFT-based convolution
	longKernel := make([]float64, 100)
	for i := range longKernel {
		longKernel[i] = math.Exp(-float64(i) / 20)
	}

	result2, _ := conv.Convolve(signal, longKernel)
	fmt.Printf("Long kernel result length: %d\n", len(result2))

	// Output:
	// Short kernel result length: 1003
	// Long kernel result length: 1099
}

func ExampleOverlapAdd() {
	// Create a reusable convolver for repeated processing
	kernel := make([]float64, 64)
	for i := range kernel {
		kernel[i] = math.Exp(-float64(i) / 10)
	}

	kernel[0] = 1 // Normalize

	convolver, _ := conv.NewOverlapAdd(kernel, 256)
	fmt.Printf("Block size: %d\n", convolver.BlockSize())
	fmt.Printf("FFT size: %d\n", convolver.FFTSize())

	// Process multiple signals efficiently
	for i := range 3 {
		signal := make([]float64, 500)
		for j := range signal {
			signal[j] = math.Sin(2 * math.Pi * float64(j) / 20)
		}

		result, _ := convolver.Process(signal)
		fmt.Printf("Result %d length: %d\n", i+1, len(result))
	}

	// Output:
	// Block size: 256
	// FFT size: 512
	// Result 1 length: 563
	// Result 2 length: 563
	// Result 3 length: 563
}

func ExampleCorrelate() {
	// Find the position of a template in a signal
	signal := []float64{0, 0, 0, 1, 2, 3, 2, 1, 0, 0, 0}
	template := []float64{1, 2, 3, 2, 1}

	result, _ := conv.Correlate(signal, template)

	// Find the peak (best match location)
	peakIdx, peakVal := conv.FindPeak(result)
	lag := conv.LagFromIndex(peakIdx, len(template))

	fmt.Printf("Peak at index %d (lag %d) with value %.2f\n", peakIdx, lag, peakVal)

	// Output:
	// Peak at index 7 (lag 3) with value 19.00
}

func ExampleAutoCorrelate() {
	// Compute auto-correlation of a periodic signal
	n := 100
	signal := make([]float64, n)

	period := 20
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * float64(i) / float64(period))
	}

	result, _ := conv.AutoCorrelateNormalized(signal)

	// The normalized auto-correlation at zero lag is always 1.0
	zeroLag := result[n-1]
	fmt.Printf("Zero-lag correlation: %.4f\n", zeroLag)

	// For periodic signals, there should be peaks at multiples of the period
	onePeriodLag := result[n-1+period]
	fmt.Printf("One-period lag correlation: %.4f\n", onePeriodLag)

	// Output:
	// Zero-lag correlation: 1.0000
	// One-period lag correlation: 0.8000
}

func ExampleDeconvolve() {
	// Create a simple signal
	original := make([]float64, 50)
	for i := range original {
		original[i] = math.Sin(2 * math.Pi * float64(i) / 10)
	}

	// Convolve with a smoothing kernel
	kernel := []float64{0.25, 0.5, 0.25}
	convolved, _ := conv.Direct(original, kernel)

	// Attempt to recover the original
	opts := conv.DefaultDeconvOptions()
	opts.Epsilon = 1e-3 // Regularization

	recovered, _ := conv.Deconvolve(convolved, kernel, opts)

	// Measure recovery quality
	snr := conv.SNR(original, recovered)
	fmt.Printf("Recovery SNR: %.1f dB\n", snr)
	fmt.Printf("Original length: %d\n", len(original))
	fmt.Printf("Recovered length: %d\n", len(recovered))

	// Output:
	// Recovery SNR: 39.6 dB
	// Original length: 50
	// Recovered length: 50
}
