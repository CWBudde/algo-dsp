package band

import (
	"fmt"
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestButterworthPeakDigitalOrderAndGain(t *testing.T) {
	for _, order := range []int{2, 4, 6, 8, 10, 12, 32} {
		for _, gain := range []float64{-24, -.1, .1, 24} {
			sections, err := ButterworthPeak(48000, 1000, 500, gain, order)
			if err != nil {
				t.Fatal(err)
			}

			if len(sections) != order/2 {
				t.Fatalf("order%d: %d sections", order, len(sections))
			}

			if got := biquad.NewChain(sections).MagnitudeDB(1000, 48000); math.Abs(got-gain) > 1e-8 {
				t.Fatalf("order%d: center %g dB, want %g", order, got, gain)
			}
		}
	}
}

func TestButterworthPeakInvalidParameters(t *testing.T) {
	for _, tc := range []struct {
		rate, hz, width, gain float64
		order                 int
	}{
		{0, 1000, 500, 6, 4}, {48000, 24000, 500, 6, 4},
		{48000, 1000, 2000, 6, 4}, {48000, 1000, 500, math.NaN(), 4},
		{48000, 1000, 500, 6, 3}, {48000, 1000, 500, 6, 34},
		{48000, 1000, 500, 49, 4},
	} {
		if _, err := ButterworthPeak(tc.rate, tc.hz, tc.width, tc.gain, tc.order); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}

func ExampleButterworthPeak() {
	sections, _ := ButterworthPeak(48000, 1000, 500, 6, 6)
	fmt.Println(len(sections))
	// Output: 3
}
