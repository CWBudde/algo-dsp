package effects_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

// ExampleGate demonstrates basic noise gate usage with default settings.
func ExampleGate() {
	// Create gate with 48kHz sample rate
	gate, err := effects.NewGate(48000)
	if err != nil {
		panic(err)
	}

	// Process a single sample
	input := 0.5
	output := gate.ProcessSample(input)

	fmt.Printf("Input: %.3f, Output: %.3f\n", input, output)
	// Output varies due to dynamic processing
}

// ExampleGate_configuration demonstrates configuring gate parameters.
func ExampleGate_configuration() {
	gate, _ := effects.NewGate(48000)

	// Configure for aggressive noise gating
	_ = gate.SetThreshold(-30.0) // Gate signals below -30 dB
	_ = gate.SetRatio(20.0)      // 20:1 expansion ratio
	_ = gate.SetKnee(6.0)        // 6 dB soft knee
	_ = gate.SetAttack(0.5)      // 0.5 ms attack (fast gate opening)
	_ = gate.SetHold(20.0)       // 20 ms hold (prevent chattering)
	_ = gate.SetRelease(50.0)    // 50 ms release
	_ = gate.SetRange(-60.0)     // -60 dB maximum attenuation

	// Process audio buffer
	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
	}
	gate.ProcessInPlace(buf)

	fmt.Println("Configured gate parameters:")
	fmt.Printf("Threshold: %.1f dB\n", gate.Threshold())
	fmt.Printf("Ratio: %.1f:1\n", gate.Ratio())
	fmt.Printf("Knee: %.1f dB\n", gate.Knee())
	fmt.Printf("Range: %.1f dB\n", gate.Range())
	// Output:
	// Configured gate parameters:
	// Threshold: -30.0 dB
	// Ratio: 20.0:1
	// Knee: 6.0 dB
	// Range: -60.0 dB
}

// ExampleGate_metering demonstrates using gate metering.
func ExampleGate_metering() {
	gate, _ := effects.NewGate(48000)

	// Configure and reset metrics
	_ = gate.SetThreshold(-10)
	_ = gate.SetHold(0)
	gate.ResetMetrics()

	// Process quiet signal (below threshold)
	for i := 0; i < 1000; i++ {
		gate.ProcessSample(0.01)
	}

	// Get metering information
	metrics := gate.GetMetrics()

	fmt.Printf("Input Peak: %.3f\n", metrics.InputPeak)
	fmt.Printf("Output Peak: %.6f\n", metrics.OutputPeak)
	fmt.Printf("Gain Reduction: %.6f (%.1f dB)\n",
		metrics.GainReduction,
		20*math.Log10(metrics.GainReduction))
	// Output varies due to dynamic processing
}
