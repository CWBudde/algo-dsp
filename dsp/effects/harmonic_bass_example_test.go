package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleHarmonicBass() {
	bass, err := effects.NewHarmonicBass(48000)
	if err != nil {
		panic(err)
	}

	_ = bass.SetFrequency(90)
	_ = bass.SetHarmonicBassLevel(0.6)

	sample := 0.2
	_ = bass.ProcessSample(sample)

	fmt.Println("processed")

	// Output:
	// processed
}
