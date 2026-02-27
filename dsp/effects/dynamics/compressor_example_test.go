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
	_ = comp.ProcessSample(0.5)

	fmt.Println("Compressor processed one sample")
	// Output:
	// Compressor processed one sample
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
	for range 1000 {
		comp.ProcessSample(0.8)
	}

	// Get metering information
	metrics := comp.GetMetrics()
	if metrics.InputPeak > 0 && metrics.OutputPeak > 0 {
		fmt.Println("Compressor metering updated")
	}

	// Output:
	// Compressor metering updated
}
