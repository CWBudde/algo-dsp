package signal_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/signal"
)

func ExampleGenerator_Sine() {
	g := signal.NewGenerator(core.WithSampleRate(1000))
	x, err := g.Sine(250, 1, 5)
	if err != nil {
		panic(err)
	}
	if math.Abs(x[4]) < 1e-12 {
		x[4] = 0
	}

	fmt.Printf("%.0f %.0f %.0f %.0f %.0f\n", x[0], x[1], x[2], x[3], x[4])

	// Output:
	// 0 1 0 -1 0
}

func ExampleNormalize() {
	x, err := signal.Normalize([]float64{-0.5, 0.25, 1}, 0.8)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%.2f %.2f %.2f\n", x[0], x[1], x[2])

	// Output:
	// -0.40 0.20 0.80
}
