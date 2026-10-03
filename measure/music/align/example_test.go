package align_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/align"
)

func ExampleCheck() {
	const (
		rate = 24000.0
		n    = 24000
	)

	// Two mono parts and a reference that is their sum, delayed by 12
	// samples (0.5 ms).
	low := make([]float64, n)
	high := make([]float64, n)
	mix := make([]float64, n)

	for i := range low {
		low[i] = 0.4 * math.Sin(2*math.Pi*110*float64(i)/rate)
		high[i] = 0.2 * math.Sin(2*math.Pi*3170*float64(i)/rate+math.Sin(float64(i)/50))
	}

	for i := 12; i < n; i++ {
		mix[i] = low[i-12] + high[i-12]
	}

	res, err := align.Check([][]float64{mix}, [][][]float64{{low}, {high}}, rate, align.WithStride(1))
	if err != nil {
		panic(err)
	}

	fmt.Printf("best lag %.2f ms, correlation %.3f\n", res.BestLag*1000, res.BestCorrelation)
	// Output:
	// best lag -0.50 ms, correlation 1.000
}
