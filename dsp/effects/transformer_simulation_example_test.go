package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleTransformerSimulation_ProcessSample() {
	ts, err := effects.NewTransformerSimulation(48000,
		effects.WithTransformerQuality(effects.TransformerQualityHigh),
		effects.WithTransformerDrive(4),
		effects.WithTransformerMix(1),
	)
	if err != nil {
		panic(err)
	}

	out := ts.ProcessSample(0.4)
	fmt.Println(out != 0)
	// Output: true
}

func ExampleTransformerSimulation_lightweight() {
	ts, err := effects.NewTransformerSimulation(48000,
		effects.WithTransformerQuality(effects.TransformerQualityLightweight),
		effects.WithTransformerDrive(3),
		effects.WithTransformerHighpassHz(30),
		effects.WithTransformerDampingHz(7000),
	)
	if err != nil {
		panic(err)
	}

	buf := []float64{0.1, 0.2, 0.1, -0.1, -0.2, -0.1}
	ts.ProcessInPlace(buf)
	fmt.Println(len(buf))
	// Output: 6
}
