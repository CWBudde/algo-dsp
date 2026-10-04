package spectrum_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

func ExampleNewPowerAverager() {
	average, _ := spectrum.NewPowerAverager(2, 2)
	_ = average.Add([]float64{1, 4})
	_ = average.Add([]float64{3, 8})
	values := make([]float64, 2)
	_ = average.ValuesInto(values)
	fmt.Println(values)
	// Output: [2 6]
}
