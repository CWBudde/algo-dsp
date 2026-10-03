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

func ExampleResampleAligned() {
	input := make([]float64, 4800) // 100 ms at 48 kHz
	input[480] = 1                 // impulse at 10 ms

	output, _ := resample.ResampleAligned(input, 48000, 24000, resample.WithQuality(resample.QualityBest))

	peak := 0
	for i, v := range output {
		if v > output[peak] {
			peak = i
		}
	}

	fmt.Printf("out=%d peak=%d (%.0f ms)\n", len(output), peak, 1000*float64(peak)/24000)
	// Output:
	// out=2400 peak=240 (10 ms)
}

func ExampleResampler_ProcessAligned() {
	r, _ := resample.NewForRates(44100, 24000, resample.WithQuality(resample.QualityBest))
	input := make([]float64, 4410) // 100 ms at 44.1 kHz
	input[441] = 1                 // impulse at 10 ms

	output, _ := r.ProcessAligned(input)

	peak := 0
	for i, v := range output {
		if v > output[peak] {
			peak = i
		}
	}

	fmt.Printf("out=%d peak=%d delay compensated=%.2f frames\n", len(output), peak, r.GroupDelayOutput())
	// Output:
	// out=2400 peak=240 delay compensated=17.41 frames
}
