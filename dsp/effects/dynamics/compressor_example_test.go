package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleCompressor demonstrates basic compressor usage with default settings.
func ExampleCompressor() {
	// Create compressor with 48kHz sample rate
	comp, err := dynamics.NewCompressor(48000)
	if err != nil {
		panic(err)
	}

	// Process a single sample
	input := 0.5
	output := comp.ProcessSample(input)

	fmt.Printf("Input: %.3f, Output: %.3f\n", input, output)
	// Output varies due to dynamic processing
}

// ExampleCompressor_configuration demonstrates configuring compressor parameters.
func ExampleCompressor_configuration() {
	comp, _ := dynamics.NewCompressor(48000)

	// Configure for aggressive compression
	_ = comp.SetThreshold(-10.0) // Compress above -10dB
	_ = comp.SetRatio(8.0)       // 8:1 ratio
	_ = comp.SetKnee(3.0)        // 3dB soft knee
	_ = comp.SetAttack(5.0)      // Fast 5ms attack
	_ = comp.SetRelease(50.0)    // 50ms release

	// Process audio buffer
	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}
	comp.ProcessInPlace(buf)

	fmt.Println("Configured compressor parameters:")
	fmt.Printf("Threshold: %.1f dB\n", comp.Threshold())
	fmt.Printf("Ratio: %.1f:1\n", comp.Ratio())
	fmt.Printf("Knee: %.1f dB\n", comp.Knee())
	// Output:
	// Configured compressor parameters:
	// Threshold: -10.0 dB
	// Ratio: 8.0:1
	// Knee: 3.0 dB
}

// ExampleCompressor_metering demonstrates using compressor metering.
func ExampleCompressor_metering() {
	comp, _ := dynamics.NewCompressor(48000)

	// Reset metrics before processing
	comp.ResetMetrics()

	// Process some loud signal
	for i := 0; i < 1000; i++ {
		comp.ProcessSample(0.8)
	}

	// Get metering information
	metrics := comp.GetMetrics()

	fmt.Printf("Input Peak: %.3f\n", metrics.InputPeak)
	fmt.Printf("Output Peak: %.3f\n", metrics.OutputPeak)
	fmt.Printf("Gain Reduction: %.3f (%.1f dB)\n",
		metrics.GainReduction,
		20*math.Log10(metrics.GainReduction))
	// Output varies due to dynamic processing
}
