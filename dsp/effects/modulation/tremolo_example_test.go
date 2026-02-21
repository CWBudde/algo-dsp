package modulation_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExampleTremolo_ProcessInPlace() {
	tremolo, err := modulation.NewTremolo(48000,
		modulation.WithTremoloRateHz(5),
		modulation.WithTremoloDepth(0.7),
		modulation.WithTremoloSmoothingMs(4),
		modulation.WithTremoloMix(1),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{1, 0, 0, 0}
	if err := tremolo.ProcessInPlace(buf); err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}
