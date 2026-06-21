package reverb_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
)

// ExampleConvolutionReverb applies a short impulse response as a wet/dry send
// reverb. The reverb adds latency equal to 2^minBlockOrder samples.
func ExampleConvolutionReverb() {
	kernel := []float64{1.0, 0.5, 0.25} // short impulse response

	r, err := reverb.NewConvolutionReverb(kernel, 6)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	r.SetWetDry(0.3, 1.0) // 30% wet send, full dry pass-through

	block := make([]float64, 256)
	block[0] = 1.0

	if err := r.ProcessInPlace(block); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("latency: %d samples\n", r.Latency())
	// Output:
	// latency: 64 samples
}
