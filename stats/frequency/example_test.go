package frequency_test

import (
	"fmt"

	frequencystats "github.com/cwbudde/algo-dsp/stats/frequency"
)

func ExampleCalculate() {
	mag := []float64{0, 1, 2, 1, 0}
	s := frequencystats.Calculate(mag, 8000)
	fmt.Printf("centroid=%.0f rolloff=%.0f\n", s.Centroid, s.Rolloff)

	// Output:
	// centroid=2000 rolloff=3000
}

func ExampleFlatness() {
	flat := frequencystats.Flatness([]float64{0, 1, 1, 1, 1})
	fmt.Printf("flatness=%.1f\n", flat)

	// Output:
	// flatness=1.0
}
