package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleFlanger_ProcessInPlace() {
	flanger, err := effects.NewFlanger(48000,
		effects.WithFlangerRateHz(0.2),
		effects.WithFlangerDepthSeconds(0.0012),
		effects.WithFlangerBaseDelaySeconds(0.001),
		effects.WithFlangerFeedback(0.3),
		effects.WithFlangerMix(0.4),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}
	if err := flanger.ProcessInPlace(buf); err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}
