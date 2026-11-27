package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExamplePreisach() {
	p, err := effects.NewPreisach(8)
	if err != nil {
		fmt.Println("error")
		return
	}

	// The same input yields different outputs depending on the history.
	for _, input := range []float32{-0.5, 0, 0.5, 0} {
		fmt.Printf("in=%+.1f out=%+.4f\n", input, p.ProcessSample(input))
	}

	// Reset returns every relay to its positive state.
	p.Reset()
	fmt.Printf("after reset: in=%+.1f out=%+.4f\n", 0.0, p.ProcessSample(0))
	// Output:
	// in=-0.5 out=-0.0714
	// in=+0.0 out=+0.0000
	// in=+0.5 out=+0.3571
	// in=+0.0 out=+0.2857
	// after reset: in=+0.0 out=+0.5714
}
