package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleNewDAVSoftKneeUpwardGain samples the DAV upward transfer curve with
// a -10 dB threshold, a 2 dB knee and a 1:2 ratio. Quiet peaks receive gain,
// the threshold sits at the full knee boost and loud peaks approach unity.
func ExampleNewDAVSoftKneeUpwardGain() {
	curve, err := dynamics.NewDAVSoftKneeUpwardGain(-10, 2, .5)
	if err != nil {
		panic(err)
	}

	for _, inputDB := range []float64{-30, -10, 0} {
		gain := curve.Gain(math.Pow(10, inputDB/20))
		fmt.Printf("in=%+3.0f dB  gain=%+6.2f dB\n", inputDB, 20*math.Log10(gain))
	}
	// Output:
	// in=-30 dB  gain=+20.20 dB
	// in=-10 dB  gain= +2.00 dB
	// in= +0 dB  gain= +0.39 dB
}
