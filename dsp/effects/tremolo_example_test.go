package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleTremolo_ProcessInPlace() {
	tremolo, err := effects.NewTremolo(48000,
		effects.WithTremoloRateHz(5),
		effects.WithTremoloDepth(0.7),
		effects.WithTremoloSmoothingMs(4),
		effects.WithTremoloMix(1),
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
