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
