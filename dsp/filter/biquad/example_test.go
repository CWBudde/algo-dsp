package biquad_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func ExampleSection_ProcessSample() {
	// Create a lowpass-like biquad section.
	s := biquad.NewSection(biquad.Coefficients{
		B0: 0.25, B1: 0.5, B2: 0.25,
		A1: -0.2, A2: 0.04,
	})

	// Process an impulse.
	for i := range 6 {
		var x float64
		if i == 0 {
			x = 1
		}

		y := s.ProcessSample(x)
		fmt.Printf("y[%d] = %.6f\n", i, y)
	}
	// Output:
	// y[0] = 0.250000
	// y[1] = 0.550000
	// y[2] = 0.350000
	// y[3] = 0.048000
	// y[4] = -0.004400
	// y[5] = -0.002800
}

func ExampleSection_ProcessBlock() {
	c := biquad.Coefficients{
		B0: 0.25, B1: 0.5, B2: 0.25,
		A1: -0.2, A2: 0.04,
	}
	s := biquad.NewSection(c)
	buf := []float64{1, 0, 0, 0}
	s.ProcessBlock(buf)

	fmt.Printf("block: %.3f %.3f %.3f %.3f\n", buf[0], buf[1], buf[2], buf[3])
	fmt.Printf("1 kHz: %+.2f dB\n", c.MagnitudeDB(1000, 48000))
	// Output:
	// block: 0.250 0.550 0.350 0.048
	// 1 kHz: +1.47 dB
}

func ExampleChain_ProcessSample() {
	// Two-section cascade (simulating a 4th-order filter).
	chain := biquad.NewChain([]biquad.Coefficients{
		{B0: 0.25, B1: 0.5, B2: 0.25, A1: -0.2, A2: 0.04},
		{B0: 0.1, B1: 0.2, B2: 0.1, A1: -0.5, A2: 0.1},
	})

	fmt.Printf("Order: %d, Sections: %d\n", chain.Order(), chain.NumSections())

	// Process a step input.
	for i := range 4 {
		y := chain.ProcessSample(1)
		fmt.Printf("y[%d] = %.6f\n", i, y)
	}
	// Output:
	// Order: 4, Sections: 2
	// y[0] = 0.025000
	// y[1] = 0.142500
	// y[2] = 0.368750
	// y[3] = 0.599925
}

func ExampleCoefficients_MagnitudeDB() {
	c := biquad.Coefficients{
		B0: 0.25, B1: 0.5, B2: 0.25,
		A1: -0.2, A2: 0.04,
	}

	sr := 48000.0
	for _, freq := range []float64{100, 1000, 10000, 20000} {
		db := c.MagnitudeDB(freq, sr)
		fmt.Printf("%6.0f Hz: %+.2f dB\n", freq, db)
	}
	// Output:
	//    100 Hz: +1.51 dB
	//   1000 Hz: +1.47 dB
	//  10000 Hz: -3.39 dB
	//  20000 Hz: -25.07 dB
}

func ExamplePoleZeroPairs() {
	coeffs := []biquad.Coefficients{
		{B0: 1, B1: -0.6, B2: 0.25, A1: -1.4, A2: 0.53},
		{B0: 1, B1: -0.2, B2: 0.0, A1: -0.8, A2: 0.0},
	}

	for i, pair := range biquad.PoleZeroPairs(coeffs) {
		fmt.Printf("section %d poles: %.2f%+.2fi, %.2f%+.2fi\n",
			i,
			real(pair.Poles[0]), imag(pair.Poles[0]),
			real(pair.Poles[1]), imag(pair.Poles[1]))
		fmt.Printf("section %d zeros: %.2f%+.2fi, %.2f%+.2fi\n",
			i,
			real(pair.Zeros[0]), imag(pair.Zeros[0]),
			real(pair.Zeros[1]), imag(pair.Zeros[1]))
	}
	// Output:
	// section 0 poles: 0.70+0.20i, 0.70-0.20i
	// section 0 zeros: 0.30+0.40i, 0.30-0.40i
	// section 1 poles: 0.80+0.00i, 0.00-0.00i
	// section 1 zeros: 0.20+0.00i, 0.00-0.00i
}
