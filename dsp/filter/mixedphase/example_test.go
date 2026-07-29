package mixedphase_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/mixedphase"
)

func ExampleDesignIterative() {
	// A symmetric FIR prototype; in practice this can come from any linear
	// phase design method.
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	result, err := mixedphase.DesignIterative(
		prototype,
		mixedphase.IterativeConfig{
			Length: 9,
			Delay:  2,
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(result.Taps))
	fmt.Println(len(result.MinimumPhasePart), len(result.LinearPhasePart))
	// Output:
	// 9
	// 5 5
}
