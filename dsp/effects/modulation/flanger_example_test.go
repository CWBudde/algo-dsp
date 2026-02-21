package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExampleFlanger_ProcessInPlace() {
	flanger, err := modulation.NewFlanger(48000,
		modulation.WithFlangerRateHz(0.2),
		modulation.WithFlangerDepthSeconds(0.0012),
		modulation.WithFlangerBaseDelaySeconds(0.001),
		modulation.WithFlangerFeedback(0.3),
		modulation.WithFlangerMix(0.4),
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
