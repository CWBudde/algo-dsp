package resample_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/resample"
)

func ExampleResample() {
	in := []float64{0, 1, 0, -1, 0, 1, 0, -1}
	out, _ := resample.Resample(in, 2, 1, resample.WithQuality(resample.QualityBalanced))
	fmt.Printf("in=%d out=%d\n", len(in), len(out))
	// Output:
	// in=8 out=16
}

func ExampleNewForRates() {
	r, _ := resample.NewForRates(44100, 48000, resample.WithQuality(resample.QualityBest))
	up, down := r.Ratio()
	fmt.Printf("ratio=%d/%d\n", up, down)
	// Output:
	// ratio=160/147
}

func ExampleResampler_ProcessInto() {
	r, _ := resample.NewRational(2, 1)
	input := []float64{1, -1, 1, -1}
	dst := make([]float64, r.PredictOutputLen(len(input)))
	written, _ := r.ProcessInto(dst, input)
	fmt.Printf("in=%d out=%d\n", len(input), written)
	// Output:
	// in=4 out=8
}

func ExampleResampler_Clone() {
	left, _ := resample.NewRational(160, 147)
	right := left.Clone()
	up, down := right.Ratio()
	fmt.Printf("right=%d/%d\n", up, down)
	// Output:
	// right=160/147
}

func ExampleResampler_GroupDelayInput() {
	r, _ := resample.NewRational(2, 1)
	fmt.Printf("input delay=%.2f frames\n", r.GroupDelayInput())
	// Output:
	// input delay=15.75 frames
}

func ExampleResampler_GroupDelayOutput() {
	r, _ := resample.NewRational(2, 1)
	fmt.Printf("output delay=%.2f frames\n", r.GroupDelayOutput())
	// Output:
	// output delay=31.50 frames
}
