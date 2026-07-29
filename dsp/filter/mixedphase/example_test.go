package mixedphase_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/mixedphase"
)

func ExampleMinimumPhaseWith() {
	// A symmetric, linear-phase prototype whose energy sits in the middle.
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	// The discrete Hilbert transform reproduces the target magnitude exactly on
	// the design grid; the real cepstrum recovers it through an exponential.
	taps, err := mixedphase.MinimumPhaseWith(
		prototype,
		mixedphase.MinimumPhaseConfig{Method: mixedphase.MethodHilbert},
	)
	if err != nil {
		panic(err)
	}

	metrics, err := mixedphase.Analyze(prototype, taps, 0)
	if err != nil {
		panic(err)
	}

	// The minimum-phase version front-loads its energy.
	fmt.Println(len(taps))
	fmt.Println(metrics.PeakIndex)
	// Output:
	// 9
	// 2
}

func ExampleDesignComplexLeastSquares() {
	prototype := []float64{
		0.01, 0.04, 0.12, 0.20, 0.26, 0.20, 0.12, 0.04, 0.01,
	}

	// Weight the lower half of the spectrum ten times as heavily, then trade
	// mean-square error for peak error with a few Lawson passes.
	weight := make([]float64, 129)
	for i := range weight {
		weight[i] = 1
		if i < len(weight)/2 {
			weight[i] = 10
		}
	}

	result, err := mixedphase.DesignComplexLeastSquares(
		prototype,
		mixedphase.ComplexLeastSquaresConfig{
			Length:            9,
			Mix:               0.5,
			FFTSize:           256,
			Weight:            weight,
			MinimaxIterations: 8,
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(result.Taps))
	fmt.Println(result.ComplexError.Peak < result.ComplexError.RMS*3)
	// Output:
	// 9
	// true
}

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
