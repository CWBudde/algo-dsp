package pass

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestButterworthCutResponse(t *testing.T) {
	for _, rate := range []float64{44100, 48000, 96000} {
		for _, order := range []int{0, 1, 2, 3, 8, 16} {
			for _, highpass := range []bool{false, true} {
				const hz = 1000

				var filter ButterworthCut
				filter.Configure(hz, rate, order, highpass)

				var reference []biquad.Coefficients
				if highpass {
					reference = ButterworthHP(hz, order, rate)
				} else {
					reference = ButterworthLP(hz, order, rate)
				}

				chain := make([]*biquad.Section, len(reference))
				for i, coefficients := range reference {
					chain[i] = biquad.NewSection(coefficients)
				}

				for i := range 250 {
					var input float32
					if i == 0 {
						input = 1
					}

					want := float64(input)
					for _, section := range chain {
						want = section.ProcessSample(want)
					}

					got := filter.ProcessSample(input)
					if math.Abs(float64(got)-want) > 1e-7 {
						t.Fatalf("rate %g order %d highpass %t sample %d: got %g, want %g",
							rate, order, highpass, i, got, want)
					}
				}
			}
		}
	}
}

func TestButterworthCutBoundaries(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)

	for _, tc := range []struct {
		cutoff, rate float64
		lp, hp       float32
	}{
		{nan, 48000, 1, 1},
		{1000, nan, 1, 1},
		{1000, inf, 1, 1},
		{1000, -inf, 1, 1},
		{1000, 0, 1, 1},
		{1000, -48000, 1, 1},
		{24000, 48000, 1, 0},
		{inf, 48000, 1, 0},
		{0, 48000, 0, 1},
		{-1000, 48000, 0, 1},
		{-inf, 48000, 0, 1},
	} {
		for _, highpass := range []bool{false, true} {
			want := tc.lp
			if highpass {
				want = tc.hp
			}

			var filter ButterworthCut
			// Leave state behind that a broken design could amplify.
			filter.Configure(1000, 48000, 5, highpass)

			for range 16 {
				filter.ProcessSample(1)
			}

			filter.Configure(tc.cutoff, tc.rate, 5, highpass)

			for i, x := range []float32{1, -0.5, 0.25} {
				if got := filter.ProcessSample(x); got != want*x {
					t.Errorf("cutoff %g rate %g highpass %t sample %d: got %g, want %g",
						tc.cutoff, tc.rate, highpass, i, got, want*x)
				}
			}
		}
	}
}
