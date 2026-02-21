package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleDeEsser demonstrates basic de-esser usage with default settings.
func ExampleDeEsser() {
	// Create de-esser with 48kHz sample rate.
	de, err := dynamics.NewDeEsser(48000)
	if err != nil {
		panic(err)
	}

	// Process a buffer of audio.
	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.5 * math.Sin(2*math.Pi*6000*float64(i)/48000)
	}
	de.ProcessInPlace(buf)

	fmt.Println("De-esser processed 256 samples")
	// Output:
	// De-esser processed 256 samples
}

// ExampleDeEsser_configuration demonstrates configuring a de-esser for vocals.
func ExampleDeEsser_configuration() {
	de, err := dynamics.NewDeEsser(48000)
	if err != nil {
		panic(err)
	}

	// Target female vocal sibilance.
	_ = de.SetFrequency(7500)   // Higher center for female voices
	_ = de.SetQ(2.0)            // Narrower band
	_ = de.SetThreshold(-25)    // Moderate threshold
	_ = de.SetRatio(6)          // Moderate ratio
	_ = de.SetAttack(0.3)       // Fast attack to catch transients
	_ = de.SetRelease(30)       // Smooth release
	_ = de.SetRange(-18)        // Limit max reduction to -18 dB
	_ = de.SetMode(dynamics.DeEsserSplitBand) // Only reduce the sibilant band

	fmt.Printf("Frequency: %.0f Hz\n", de.Frequency())
	fmt.Printf("Q: %.1f\n", de.Q())
	fmt.Printf("Threshold: %.0f dB\n", de.Threshold())
	fmt.Printf("Mode: split-band\n")
	// Output:
	// Frequency: 7500 Hz
	// Q: 2.0
	// Threshold: -25 dB
	// Mode: split-band
}

// ExampleDeEsser_wideband demonstrates wideband mode.
func ExampleDeEsser_wideband() {
	de, err := dynamics.NewDeEsser(48000)
	if err != nil {
		panic(err)
	}

	// Configure for wideband reduction.
	_ = de.SetMode(dynamics.DeEsserWideband)
	_ = de.SetThreshold(-20)
	_ = de.SetRatio(4)

	// Process a single sample.
	out := de.ProcessSample(0.3)
	fmt.Printf("Output sample: %.3f\n", out)
	// Output:
	// Output sample: 0.300
}
