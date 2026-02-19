package effects_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

// ExampleSpectralPitchShifter demonstrates frequency-domain pitch shifting.
func ExampleSpectralPitchShifter() {
	shifter, err := effects.NewSpectralPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	// Shift up by a perfect fifth (3:2).
	_ = shifter.SetPitchRatio(1.5)

	in := make([]float64, 2048)
	for i := range in {
		in[i] = 0.25 * math.Sin(2*math.Pi*220*float64(i)/48000)
	}

	out := shifter.Process(in)

	fmt.Printf("in=%d out=%d effective_ratio=%.3f\n", len(in), len(out), shifter.EffectivePitchRatio())
	// Output:
	// in=2048 out=2048 effective_ratio=1.500
}
