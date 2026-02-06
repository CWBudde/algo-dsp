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
