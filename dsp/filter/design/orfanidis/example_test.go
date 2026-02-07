package orfanidis_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/orfanidis"
)

func ExamplePeakingCascade() {
	coeffs, _ := orfanidis.PeakingCascade(48000, 1000, 0.707, 6.0, 3)
	chain := biquad.NewChain(coeffs)

	fmt.Printf("sections=%d order=%d\n", len(coeffs), chain.Order())
	// Output:
	// sections=3 order=6
}
