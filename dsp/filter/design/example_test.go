package design_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

func ExampleButterworthLP() {
	coeffs := design.ButterworthLP(1000, 4, 48000)
	chain := biquad.NewChain(coeffs)

	fmt.Printf("sections=%d order=%d\n", len(coeffs), chain.Order())
	fmt.Printf("100 Hz:   %.2f dB\n", chain.MagnitudeDB(100, 48000))
	fmt.Printf("1000 Hz:  %.2f dB\n", chain.MagnitudeDB(1000, 48000))
	fmt.Printf("10000 Hz: %.2f dB\n", chain.MagnitudeDB(10000, 48000))
	// Output:
	// sections=2 order=4
	// 100 Hz:   -0.00 dB
	// 1000 Hz:  -3.01 dB
	// 10000 Hz: -85.48 dB
}
