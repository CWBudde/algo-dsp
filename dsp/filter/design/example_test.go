package design_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

func ExamplePeakCascade() {
	coeffs, _ := design.PeakCascade(48000, 1000, 0.707, 6.0, 3,
		design.WithDCGain(1.0), design.WithNyquistGain(1.0))
	chain := biquad.NewChain(coeffs)

	fmt.Printf("sections=%d order=%d\n", len(coeffs), chain.Order())
	// Output:
	// sections=3 order=6
}

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

func ExampleLinkwitzRileyLP() {
	coeffs := design.LinkwitzRileyLP(1000, 4, 48000)
	chain := biquad.NewChain(coeffs)

	fmt.Printf("sections=%d order=%d\n", len(coeffs), chain.Order())
	fmt.Printf("100 Hz:   %.2f dB\n", chain.MagnitudeDB(100, 48000))
	fmt.Printf("1000 Hz:  %.2f dB\n", chain.MagnitudeDB(1000, 48000))
	fmt.Printf("10000 Hz: %.2f dB\n", chain.MagnitudeDB(10000, 48000))
	// Output:
	// sections=2 order=4
	// 100 Hz:   -0.00 dB
	// 1000 Hz:  -6.02 dB
	// 10000 Hz: -85.48 dB
}

// ExampleFirwin2 designs the 256-tap anti-aliasing low-pass that nnAudio's
// constant-Q transform applies before each factor-2 decimation: unity gain up
// to just below half the Nyquist frequency, zero just above it.
func ExampleFirwin2() {
	taps, err := design.Firwin2(256,
		[]float64{0, 0.5 / 1.001, 0.5 * 1.001, 1},
		[]float64{1, 1, 0, 0})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Sum of taps = gain at DC; alternating sum = gain at Nyquist.
	var dc, nyq float64

	for n, h := range taps {
		dc += h
		if n%2 == 0 {
			nyq += h
		} else {
			nyq -= h
		}
	}

	fmt.Printf("taps=%d\n", len(taps))
	fmt.Printf("centre taps: %.6f %.6f\n", taps[127], taps[128])
	fmt.Printf("DC gain:      %.6f\n", dc)
	fmt.Printf("Nyquist gain: %.6f\n", math.Abs(nyq))
	// Output:
	// taps=256
	// centre taps: 0.450142 0.450142
	// DC gain:      0.999779
	// Nyquist gain: 0.000000
}

// ExampleFirwin2_sampleRate gives the band edges in hertz: a 101-tap low-pass
// for 8 kHz audio passing up to 1 kHz and stopping from 1.5 kHz.
func ExampleFirwin2_sampleRate() {
	const fs = 8000.0

	taps, err := design.Firwin2(101,
		[]float64{0, 1000, 1500, fs / 2},
		[]float64{1, 1, 0, 0},
		design.WithSampleRate(fs))
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, f := range []float64{500, 1000, 1250, 2000, 3000} {
		var re, im float64

		for n, h := range taps {
			w := 2 * math.Pi * f / fs * float64(n)
			re += h * math.Cos(w)
			im -= h * math.Sin(w)
		}

		fmt.Printf("%4.0f Hz: %7.2f dB\n", f, 20*math.Log10(math.Hypot(re, im)))
	}
	// Output:
	//  500 Hz:    0.00 dB
	// 1000 Hz:   -0.39 dB
	// 1250 Hz:   -6.02 dB
	// 2000 Hz:  -89.85 dB
	// 3000 Hz:  -97.49 dB
}
