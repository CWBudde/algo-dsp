package dynamics_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

// ExampleNewDAVSoftKneeCompressorGain samples the DAV transfer curve of a
// 4:1 compressor with a -10 dB threshold and a 2 dB knee. The hyperbolic
// curve reaches the full knee depth at the threshold.
func ExampleNewDAVSoftKneeCompressorGain() {
	curve, err := dynamics.NewDAVSoftKneeCompressorGain(-10, 2, 4)
	if err != nil {
		panic(err)
	}

	for _, inputDB := range []float64{-30, -10, 0} {
		gain := curve.Gain(math.Pow(10, inputDB/20))
		fmt.Printf("in=%+3.0f dB  gain=%+6.2f dB\n", inputDB, 20*math.Log10(gain))
	}
	// Output:
	// in=-30 dB  gain= -0.26 dB
	// in=-10 dB  gain= -2.00 dB
	// in= +0 dB  gain= -8.00 dB
}

// ExampleDAVCompressorTiming derives the detector coefficients for a 15 ms
// attack and a 75 ms release at a 2× oversampled 44.1 kHz rate.
func ExampleDAVCompressorTiming() {
	attack, release, err := dynamics.DAVCompressorTiming(15, 75, 88200)
	if err != nil {
		panic(err)
	}

	fmt.Printf("attack=%.6f release=%.6f\n", attack, release)
	// Output:
	// attack=0.000524 release=0.999895
}
