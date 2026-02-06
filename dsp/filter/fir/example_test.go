package fir_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/fir"
)

func ExampleFilter_ProcessSample() {
	// 3-tap moving average filter.
	f := fir.New([]float64{1.0 / 3, 1.0 / 3, 1.0 / 3})

	input := []float64{0, 1, 2, 3, 3, 3}
	for i, x := range input {
		y := f.ProcessSample(x)
		fmt.Printf("y[%d] = %.4f\n", i, y)
	}
	// Output:
	// y[0] = 0.0000
	// y[1] = 0.3333
	// y[2] = 1.0000
	// y[3] = 2.0000
	// y[4] = 2.6667
	// y[5] = 3.0000
}
