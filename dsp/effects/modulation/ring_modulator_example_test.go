package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExampleRingModulator_ProcessInPlace() {
	rm, err := modulation.NewRingModulator(48000,
		modulation.WithRingModCarrierHz(440),
		modulation.WithRingModMix(1),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}
	rm.ProcessInPlace(buf)

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}
