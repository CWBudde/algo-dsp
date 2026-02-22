package weighting_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/weighting"
)

func ExampleNew() {
	// Create an A-weighting filter for 48 kHz audio.
	chain := weighting.New(weighting.TypeA, 48000)

	// Print the magnitude response at key frequencies.
	for _, freq := range []float64{100, 1000, 4000, 10000} {
		dB := chain.MagnitudeDB(freq, 48000)
		fmt.Printf("%6.0f Hz: %+.1f dB\n", freq, dB)
	}
	// Output:
	//    100 Hz: -19.2 dB
	//   1000 Hz: -0.0 dB
	//   4000 Hz: +1.3 dB
	//  10000 Hz: -1.9 dB
}

func ExampleNew_processBlock() {
	// Apply C-weighting to a 1 kHz sine tone.
	sr := 48000.0
	chain := weighting.New(weighting.TypeC, sr)

	buf := make([]float64, 4800)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / sr)
	}

	chain.ProcessBlock(buf)

	// Measure peak amplitude after weighting (should be ~1.0 at 1 kHz).
	var peak float64
	for _, v := range buf {
		if a := math.Abs(v); a > peak {
			peak = a
		}
	}

	fmt.Printf("Peak after C-weighting: %.2f\n", peak)
	// Output:
	// Peak after C-weighting: 1.03
}
