package shelving_test

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/shelving"
)

// magnitudeDB evaluates the magnitude response of a biquad cascade in decibels.
func magnitudeDB(sections []biquad.Coefficients, freqHz, sampleRate float64) float64 {
	h := complex(1, 0)
	for i := range sections {
		h *= sections[i].Response(freqHz, sampleRate)
	}

	return 20 * math.Log10(cmplx.Abs(h))
}

func ExampleButterworthLowShelf() {
	sections, err := shelving.ButterworthLowShelf(48000, 1000, 12, 4)
	if err != nil {
		panic(err)
	}

	fmt.Printf("sections: %d\n", len(sections))
	fmt.Printf("DC:       %.2f dB\n", magnitudeDB(sections, 0, 48000))
	fmt.Printf("Nyquist:  %.2f dB\n", magnitudeDB(sections, 24000, 48000))
	// Output:
	// sections: 2
	// DC:       12.00 dB
	// Nyquist:  -0.00 dB
}

func ExampleEllipticLowShelf() {
	// A 6th-order elliptic low shelf: +12 dB below 1 kHz, with the flat region
	// held to within 0.5 dB of unity.
	sections, err := shelving.EllipticLowShelf(48000, 1000, 12, 0.5, 6)
	if err != nil {
		panic(err)
	}

	fmt.Printf("sections: %d\n", len(sections))
	fmt.Printf("DC:       %.2f dB\n", magnitudeDB(sections, 0, 48000))
	fmt.Printf("cutoff:   %.2f dB\n", magnitudeDB(sections, 1000, 48000))
	fmt.Printf("Nyquist:  %.2f dB\n", magnitudeDB(sections, 24000, 48000))
	// Output:
	// sections: 3
	// DC:       11.95 dB
	// cutoff:   9.26 dB
	// Nyquist:  0.50 dB
}

func ExampleEllipticHighShelf() {
	// The high-shelf mirror: -12 dB above 5 kHz.
	sections, err := shelving.EllipticHighShelf(48000, 5000, -12, 0.5, 5)
	if err != nil {
		panic(err)
	}

	fmt.Printf("sections: %d\n", len(sections))
	fmt.Printf("DC:       %.2f dB\n", magnitudeDB(sections, 0, 48000))
	fmt.Printf("Nyquist:  %.2f dB\n", magnitudeDB(sections, 24000, 48000))
	// Output:
	// sections: 3
	// DC:       0.00 dB
	// Nyquist:  -12.00 dB
}

func ExampleChebyshev2LowShelf() {
	// Chebyshev II trades ripple in the flat band for a steeper transition,
	// while the shelf itself stays monotonic and reaches the full gain.
	sections, err := shelving.Chebyshev2LowShelf(48000, 1000, 12, 0.5, 6)
	if err != nil {
		panic(err)
	}

	fmt.Printf("sections: %d\n", len(sections))
	fmt.Printf("DC:       %.2f dB\n", magnitudeDB(sections, 0, 48000))
	fmt.Printf("cutoff:   %.2f dB\n", magnitudeDB(sections, 1000, 48000))
	// Output:
	// sections: 3
	// DC:       12.00 dB
	// cutoff:   9.26 dB
}

// ExampleEllipticLowShelf_chain runs audio through the designed cascade.
func ExampleEllipticLowShelf_chain() {
	sections, err := shelving.EllipticLowShelf(48000, 500, 6, 0.25, 4)
	if err != nil {
		panic(err)
	}

	chain := biquad.NewChain(sections)

	// Feed a unit impulse and report the first few samples of the response.
	out := make([]float64, 4)
	out[0] = chain.ProcessSample(1)

	for i := 1; i < len(out); i++ {
		out[i] = chain.ProcessSample(0)
	}

	fmt.Printf("h[0] = %.4f\n", out[0])
	// Output:
	// h[0] = 1.0410
}
