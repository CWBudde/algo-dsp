package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleDAVContinuousTanh4() {
	for _, input := range []float32{-2, -.5, 0, .5, 2, 100} {
		fmt.Printf("%5.1f -> %.4f\n", input, effects.DAVContinuousTanh4(input))
	}
	// Output:
	//  -2.0 -> -0.9640
	//  -0.5 -> -0.4621
	//   0.0 -> 0.0000
	//   0.5 -> 0.4621
	//   2.0 -> 0.9640
	// 100.0 -> 1.0000
}
