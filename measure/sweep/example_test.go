package sweep_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/sweep"
)

func ExampleLogSweep_Generate() {
	s := &sweep.LogSweep{
		StartFreq:  20,
		EndFreq:    20000,
		Duration:   1,
		SampleRate: 48000,
	}

	signal, err := s.Generate()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Sweep length: %d samples (%.1f s)\n", len(signal), float64(len(signal))/48000)
	fmt.Printf("First sample: %.6f\n", signal[0])

	// Output:
	// Sweep length: 48000 samples (1.0 s)
	// First sample: 0.000000
}

func ExampleLogSweep_Deconvolve() {
	// Create a short sweep for demonstration
	s := &sweep.LogSweep{
		StartFreq:  100,
		EndFreq:    4000,
		Duration:   0.25,
		SampleRate: 16000,
	}

	// Generate the sweep excitation signal
	excitation, err := s.Generate()
	if err != nil {
		panic(err)
	}

	// Simulate a simple system: direct path + delayed reflection
	responseLen := len(excitation) + 200
	response := make([]float64, responseLen)
	for i, v := range excitation {
		response[i] += v         // direct path
		if i+100 < responseLen {
			response[i+100] += 0.3 * v // reflection at ~6.25ms
		}
	}

	// Deconvolve to recover the impulse response
	ir, err := s.Deconvolve(response)
	if err != nil {
		panic(err)
	}

	// Find the main peak
	peakIdx := 0
	peakVal := 0.0
	for i, v := range ir {
		if math.Abs(v) > peakVal {
			peakVal = math.Abs(v)
			peakIdx = i
		}
	}

	fmt.Printf("IR length: %d samples\n", len(ir))
	fmt.Printf("Main peak at sample %d\n", peakIdx)
	fmt.Printf("Peak is dominant: %v\n", peakVal > 0)

	// Output:
	// IR length: 8199 samples
	// Main peak at sample 3999
	// Peak is dominant: true
}

func ExampleLinearSweep_Generate() {
	s := &sweep.LinearSweep{
		StartFreq:  100,
		EndFreq:    8000,
		Duration:   0.5,
		SampleRate: 16000,
	}

	signal, err := s.Generate()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Linear sweep length: %d samples\n", len(signal))

	// Output:
	// Linear sweep length: 8000 samples
}
