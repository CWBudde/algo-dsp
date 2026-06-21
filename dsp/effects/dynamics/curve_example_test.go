package dynamics_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleStaticCurve samples the steady-state characteristic curve of a
// compressor without running the detector, useful for plotting the transfer
// function or validating gain-reduction behavior.
func ExampleStaticCurve() {
	comp, err := dynamics.NewCompressor(48000)
	if err != nil {
		panic(err)
	}

	_ = comp.SetAutoMakeup(false)
	_ = comp.SetThreshold(-20)
	_ = comp.SetRatio(4)
	_ = comp.SetKnee(0)

	curve, err := dynamics.StaticCurve(comp, -40, 0, 10)
	if err != nil {
		panic(err)
	}

	for _, pt := range curve {
		fmt.Printf("in=%+.0f dB  out=%+6.2f dB  gr=%+6.2f dB\n", pt.InputDB, pt.OutputDB, pt.GainReductionDB)
	}
	// Output:
	// in=-40 dB  out=-40.00 dB  gr= +0.00 dB
	// in=-30 dB  out=-30.00 dB  gr= +0.00 dB
	// in=-20 dB  out=-20.00 dB  gr= +0.00 dB
	// in=-10 dB  out=-17.50 dB  gr= -7.50 dB
	// in=+0 dB  out=-15.00 dB  gr=-15.00 dB
}
