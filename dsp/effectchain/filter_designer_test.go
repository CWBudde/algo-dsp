package effectchain

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestBuiltInDesignerAllFamiliesAndKinds(t *testing.T) {
	designer := BuiltInFilterDesigner{}

	for _, sr := range []float64{8000, 48000, 384000} {
		for _, family := range []string{"rbj", "butterworth", "bessel", "chebyshev1", "chebyshev2", "elliptic", "unknown"} {
			for _, kind := range []string{"lowpass", "highpass", "bandpass", "notch", "allpass", "peak", "lowshelf", "highshelf"} {
				for _, order := range []int{1, 4, 6, 16} {
					freq := sr * .1
					shape := designer.ClampShape(kind, family, freq, sr, .707)

					chain := designer.BuildChain(family, kind, order, freq, 6, shape, sr)
					if chain == nil || chain.NumSections() == 0 {
						t.Fatalf("empty %s/%s", family, kind)
					}

					for _, probe := range []float64{0, sr * .02, freq, sr * .4, sr * .5} {
						h := complex(chain.Gain(), 0)
						for i := range chain.NumSections() {
							h *= chain.Section(i).Response(probe, sr)
						}

						if math.IsNaN(cmplx.Abs(h)) || math.IsInf(cmplx.Abs(h), 0) {
							t.Fatalf("unsafe %s/%s order%d sr%g", family, kind, order, sr)
						}
					}
				}
			}
		}
	}
}

func TestBuiltInDesignerShelfEndpointsAndSmallGain(t *testing.T) {
	designer := BuiltInFilterDesigner{}

	for _, family := range []string{"butterworth", "chebyshev1", "chebyshev2", "elliptic"} {
		for _, kind := range []string{"lowshelf", "highshelf"} {
			for _, gain := range []float64{-12, -.1, 0, .1, 12} {
				chain := designer.BuildChain(family, kind, 6, 1000, gain, .707, 48000)

				probe := 0.
				if kind == "highshelf" {
					probe = 24000
				}

				h := complex(chain.Gain(), 0)
				for i := range chain.NumSections() {
					h *= chain.Section(i).Response(probe, 48000)
				}

				db := 20 * math.Log10(cmplx.Abs(h))
				if math.Abs(db-gain) > .06 {
					t.Fatalf("%s %s gain%g endpoint%g", family, kind, gain, db)
				}
			}
		}
	}
}
