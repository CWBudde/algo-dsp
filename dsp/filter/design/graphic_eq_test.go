package design_test

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

func graphicChain(t *testing.T, rate float64, centers, gains []float64, order int) *biquad.Chain {
	t.Helper()

	coeffs, err := design.GraphicEQ(rate, centers, gains, order)
	if err != nil {
		t.Fatal(err)
	}

	return biquad.NewChain(coeffs)
}

func TestGraphicEQEqualGainsAreFlat(t *testing.T) {
	for _, order := range []int{1, 4, 6, 8, 10, 12, 32} {
		for _, gain := range []float64{-24, -12, -.1, 0, .1, 12, 24} {
			t.Run(fmt.Sprintf("order%d/gain%g", order, gain), func(t *testing.T) {
				chain := graphicChain(t, 48000, []float64{31.5, 63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000},
					[]float64{gain, gain, gain, gain, gain, gain, gain, gain, gain, gain}, order)
				for _, hz := range []float64{0, 20, 44.5, 88, 180, 350, 707, 1414, 2800, 5600, 11300, 20000, 24000} {
					if db := chain.MagnitudeDB(hz, 48000); math.Abs(db-gain) > 1e-12 {
						t.Fatalf("%g Hz: %.12g dB, want %g", hz, db, gain)
					}
				}
			})
		}
	}
}

func TestGraphicEQTransitionsAreCenteredMonotonicAndReciprocal(t *testing.T) {
	for _, rate := range []float64{8000, 44100, 48000, 96000, 384000} {
		for _, order := range []int{4, 6, 8, 10, 12} {
			for _, gain := range []float64{.1, 6, 12, 24, 48} {
				t.Run(fmt.Sprintf("rate%g/order%d/gain%g", rate, order, gain), func(t *testing.T) {
					centers := []float64{rate * .1, rate * .2}
					boost := graphicChain(t, rate, centers, []float64{0, gain}, order)
					cut := graphicChain(t, rate, centers, []float64{0, -gain}, order)

					boundary := math.Sqrt(centers[0] * centers[1])
					if db := boost.MagnitudeDB(boundary, rate); math.Abs(db-gain/2) > 1e-8 {
						t.Fatalf("midpoint %.10g dB, want %g", db, gain/2)
					}

					previous := -1.0

					for i := range 513 {
						hz := rate / 2 * float64(i) / 512

						db := boost.MagnitudeDB(hz, rate)
						if math.IsNaN(db) || db < previous-1e-8 || db < -1e-8 || db > gain+1e-8 {
							t.Fatalf("%g Hz: nonmonotonic/out-of-range %.12g dB after %.12g", hz, db, previous)
						}

						if inverse := cut.MagnitudeDB(hz, rate); math.Abs(db+inverse) > 1e-8 {
							t.Fatalf("%g Hz: boost + cut = %g dB", hz, db+inverse)
						}

						previous = db
					}
				})
			}
		}
	}
}

func TestGraphicEQAdjacentPlateauHasNoOverlapSpike(t *testing.T) {
	centers := []float64{31.5, 63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

	for _, order := range []int{4, 6, 8, 10, 12} {
		for _, gain := range []float64{-24, -12, 12, 24} {
			for band := 0; band < len(centers)-1; band++ {
				gains := make([]float64, len(centers))
				gains[band], gains[band+1] = gain, gain
				chain := graphicChain(t, 48000, centers, gains, order)

				for i := range 1025 {
					hz := 20 * math.Pow(1000, float64(i)/1024)

					db := chain.MagnitudeDB(hz, 48000)
					if math.IsNaN(db) || db < math.Min(0, gain)-1e-6 || db > math.Max(0, gain)+1e-6 {
						t.Fatalf("order%d band%d gain%g %g Hz: overshoot %g dB", order, band+1, gain, hz, db)
					}
				}
			}
		}
	}
}

func TestGraphicEQOrderSharpensTransitionAndImpulseMatchesResponse(t *testing.T) {
	centers, gains := []float64{500, 1000, 2000}, []float64{0, 12, 12}
	shallow := graphicChain(t, 48000, centers, gains, 4)

	steep := graphicChain(t, 48000, centers, gains, 12)
	if steep.MagnitudeDB(500, 48000) >= shallow.MagnitudeDB(500, 48000) ||
		steep.MagnitudeDB(1000, 48000) <= shallow.MagnitudeDB(1000, 48000) {
		t.Fatal("higher order did not sharpen transition")
	}

	chain := graphicChain(t, 48000, []float64{125, 500, 1000, 4000}, []float64{-6, 12, 12, -3}, 6)
	impulse := chain.ImpulseResponse(65536)

	for _, hz := range []float64{63, 250, 707, 1000, 3200, 12000} {
		var response complex128
		for i, sample := range impulse {
			response += complex(sample, 0) * cmplx.Exp(complex(0, -2*math.Pi*hz*float64(i)/48000))
		}

		if db := 20 * math.Log10(cmplx.Abs(response)); math.Abs(db-chain.MagnitudeDB(hz, 48000)) > 1e-7 {
			t.Fatalf("%g Hz: actual impulse %g dB, inspected %g dB", hz, db, chain.MagnitudeDB(hz, 48000))
		}
	}

	buffer := make([]float64, 128)

	if allocations := testing.AllocsPerRun(10, func() { chain.ProcessBlock(buffer) }); allocations != 0 {
		t.Fatalf("render allocated %g times", allocations)
	}
}

func TestGraphicEQInvalidGeometry(t *testing.T) {
	for _, test := range []struct {
		name           string
		rate           float64
		centers, gains []float64
		order          int
	}{
		{"zero rate", 0, []float64{1000}, []float64{0}, 4},
		{"NaN rate", math.NaN(), []float64{1000}, []float64{0}, 4},
		{"infinite rate", math.Inf(1), []float64{1000}, []float64{0}, 4},
		{"empty", 48000, nil, nil, 4},
		{"mismatch", 48000, []float64{1000}, nil, 4},
		{"DC", 48000, []float64{0}, []float64{0}, 4},
		{"Nyquist", 48000, []float64{24000}, []float64{0}, 4},
		{"NaN center", 48000, []float64{math.NaN()}, []float64{0}, 4},
		{"unordered", 48000, []float64{1000, 500}, []float64{0, 0}, 4},
		{"duplicate", 48000, []float64{1000, 1000}, []float64{0, 0}, 4},
		{"NaN gain", 48000, []float64{1000}, []float64{math.NaN()}, 4},
		{"infinite gain", 48000, []float64{1000}, []float64{math.Inf(1)}, 4},
		{"excess gain", 48000, []float64{1000}, []float64{49}, 4},
		{"zero order", 48000, []float64{1000}, []float64{0}, 0},
		{"excess order", 48000, []float64{1000}, []float64{0}, 33},
		{"excess bands", 48000, make([]float64, 65), make([]float64, 65), 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := design.GraphicEQ(test.rate, test.centers, test.gains, test.order); err == nil {
				t.Fatal("invalid geometry accepted")
			}
		})
	}
}

func ExampleGraphicEQ() {
	coeffs, err := design.GraphicEQ(48000, []float64{500, 1000, 2000}, []float64{12, 12, 12}, 4)
	if err != nil {
		panic(err)
	}

	chain := biquad.NewChain(coeffs)
	fmt.Printf("500 Hz: %.2f dB; 707 Hz: %.2f dB; 1000 Hz: %.2f dB\n",
		chain.MagnitudeDB(500, 48000), chain.MagnitudeDB(707, 48000), chain.MagnitudeDB(1000, 48000))
	// Output: 500 Hz: 12.00 dB; 707 Hz: 12.00 dB; 1000 Hz: 12.00 dB
}
