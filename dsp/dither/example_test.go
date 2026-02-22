package dither_test

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/cwbudde/algo-dsp/dsp/dither"
)

func ExampleNewQuantizer() {
	quant, err := dither.NewQuantizer(44100,
		dither.WithBitDepth(16),
		dither.WithDitherType(dither.DitherTriangular),
		dither.WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	if err != nil {
		panic(err)
	}

	// Quantize a sine wave sample.
	input := 0.5 * math.Sin(2*math.Pi*1000/44100)
	output := quant.ProcessSample(input)

	fmt.Printf("quantized: %.6f\n", output)
	// Output: quantized: 0.071000
}

func ExampleQuantizer_ProcessInPlace() {
	quant, err := dither.NewQuantizer(44100,
		dither.WithBitDepth(16),
		dither.WithDitherType(dither.DitherNone),
		dither.WithFIRPreset(dither.PresetNone),
	)
	if err != nil {
		panic(err)
	}

	buf := []float64{0.0, 0.25, 0.5, 0.75}
	quant.ProcessInPlace(buf)

	for _, val := range buf {
		fmt.Printf("%.6f ", val)
	}

	fmt.Println()
	// Output: 0.000015 0.249989 0.499992 0.749996
}

func ExampleNewQuantizer_sharpPreset() {
	// The sharp preset automatically selects coefficients
	// optimized for the given sample rate.
	quant, err := dither.NewQuantizer(48000,
		dither.WithSharpPreset(),
		dither.WithBitDepth(16),
		dither.WithRNG(rand.New(rand.NewPCG(42, 0))),
	)
	if err != nil {
		panic(err)
	}

	output := quant.ProcessSample(0.5)

	fmt.Printf("sharp: %.6f\n", output)
	// Output: sharp: 0.499992
}
