package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleDAVQuarticSaturation() {
	for _, x := range []float64{0, 0.5, 1, -2, 1e6} {
		fmt.Printf("%g -> %.4f\n", x, effects.DAVQuarticSaturation(x))
	}
	// Output:
	// 0 -> 0.0000
	// 0.5 -> 0.2485
	// 1 -> 0.4783
	// -2 -> -0.7857
	// 1e+06 -> 1.0000
}
