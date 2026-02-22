package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleDistortion_ProcessSample() {
	d, err := effects.NewDistortion(48000,
		effects.WithDistortionMode(effects.DistortionModeTanh),
		effects.WithDistortionDrive(3),
		effects.WithDistortionMix(1),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(d.ProcessSample(0.4) > 0)
	// Output: true
}

func ExampleDistortion_chebyshev() {
	d, err := effects.NewDistortion(48000,
		effects.WithDistortionMode(effects.DistortionModeChebyshev),
		effects.WithChebyshevOrder(3),
		effects.WithChebyshevHarmonicMode(effects.ChebyshevHarmonicOdd),
		effects.WithChebyshevDCBypass(true),
	)
	if err != nil {
		panic(err)
	}

	buf := []float64{0.0, 0.25, 0.5, 0.25, 0.0}
	d.ProcessInPlace(buf)
	fmt.Println(len(buf))
	// Output: 5
}
