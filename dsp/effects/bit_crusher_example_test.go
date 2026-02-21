package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleBitCrusher_ProcessInPlace() {
	bc, err := effects.NewBitCrusher(48000,
		effects.WithBitCrusherBitDepth(8),
		effects.WithBitCrusherDownsample(4),
		effects.WithBitCrusherMix(1),
	)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := []float64{0.5, 0.75, 0.25, 0.125}
	bc.ProcessInPlace(buf)

	fmt.Printf("len=%d\n", len(buf))
	// Output:
	// len=4
}
