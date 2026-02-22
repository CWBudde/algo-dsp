package ir_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/ir"
)

func ExampleAnalyzer_Analyze() {
	// Create a synthetic exponential decay IR with RT60 = 1.0 s
	sampleRate := 48000.0
	rt60 := 1.0
	decayRate := 6.9078 / rt60 // ensures -60 dB at rt60

	irData := make([]float64, int(sampleRate*3))
	for i := range irData {
		t := float64(i) / sampleRate
		irData[i] = math.Exp(-decayRate * t)
	}

	analyzer := ir.NewAnalyzer(sampleRate)

	metrics, err := analyzer.Analyze(irData)
	if err != nil {
		panic(err)
	}

	fmt.Printf("RT60 = %.2f s\n", metrics.RT60)
	fmt.Printf("EDT  = %.2f s\n", metrics.EDT)
	fmt.Printf("C80  = %.1f dB\n", metrics.C80)
	fmt.Printf("D50  = %.3f\n", metrics.D50)

	// Output:
	// RT60 = 1.00 s
	// EDT  = 1.00 s
	// C80  = 3.1 dB
	// D50  = 0.499
}

func ExampleAnalyzer_SchroederIntegral() {
	sampleRate := 48000.0
	decayRate := 6.9078 / 0.5 // RT60 = 0.5 s

	irData := make([]float64, int(sampleRate*1.5))
	for i := range irData {
		t := float64(i) / sampleRate
		irData[i] = math.Exp(-decayRate * t)
	}

	analyzer := ir.NewAnalyzer(sampleRate)

	schroeder, err := analyzer.SchroederIntegral(irData)
	if err != nil {
		panic(err)
	}

	// Print Schroeder curve at 0, 250, 500, 750, 1000ms
	for _, ms := range []float64{0, 250, 500, 750, 1000} {
		idx := int(ms * 0.001 * sampleRate)
		if idx < len(schroeder) {
			fmt.Printf("t=%4.0fms: %6.1f dB\n", ms, schroeder[idx])
		}
	}

	// Output:
	// t=   0ms:    0.0 dB
	// t= 250ms:  -30.0 dB
	// t= 500ms:  -60.0 dB
	// t= 750ms:  -90.0 dB
	// t=1000ms: -120.0 dB
}
