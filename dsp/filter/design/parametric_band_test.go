package design

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestParametricBandOrders(t *testing.T) {
	for _, sr := range []float64{8000, 44100, 48000, 384000} {
		for _, hz := range []float64{20, sr * .01, sr * .49} {
			for _, kind := range []string{"highpass", "lowpass", "peak", "lowshelf", "highshelf"} {
				for _, order := range []int{2, 4, 6, 8, 10, 12} {
					for _, gain := range []float64{-24, -.1, 0, .1, 24} {
						t.Run(fmt.Sprintf("%g/%g/%s/%d/%g", sr, hz, kind, order, gain), func(t *testing.T) {
							sections, err := ParametricBand(sr, hz, gain, 1, kind, order)
							if err != nil {
								t.Fatal(err)
							}

							isPass := kind == "highpass" || kind == "lowpass"
							if gain != 0 || isPass {
								if len(sections) != order/2 {
									t.Fatalf("%d biquads, want %d", len(sections), order/2)
								}
							}

							chain := biquad.NewChain(sections)

							if order == 2 {
								var legacy biquad.Coefficients

								switch kind {
								case "highpass":
									legacy = Highpass(hz, 1, sr)
								case "lowpass":
									legacy = Lowpass(hz, 1, sr)
								case "peak":
									legacy = Peak(hz, gain, 1, sr)
								case "lowshelf":
									legacy = LowShelf(hz, gain, 1, sr)
								case "highshelf":
									legacy = HighShelf(hz, gain, 1, sr)
								}

								if sections[0] != legacy {
									t.Fatal("order two changed legacy coefficients")
								}
							} else {
								want := gain / 2
								if isPass {
									want = -3.010299956639812
								} else if kind == "peak" {
									want = gain
								}

								if got := chain.MagnitudeDB(hz, sr); math.Abs(got-want) > 1e-4 {
									t.Fatalf("center gain %g dB, want %g", got, want)
								}
							}

							for i := range 257 {
								f := 10 * math.Pow(sr*.499/10, float64(i)/256)

								db := chain.MagnitudeDB(f, sr)
								if math.IsNaN(db) || math.IsInf(db, 0) {
									t.Fatalf("nonfinite response at %g Hz", f)
								}

								if order > 2 && !isPass && (db < min(gain, 0)-1e-4 || db > max(gain, 0)+1e-4) {
									t.Fatalf("gain overshoot at %g Hz: %g dB", f, db)
								}
							}

							block := make([]float64, 4096)
							block[0] = 1
							chain.ProcessBlock(block)

							for _, sample := range block {
								if math.IsNaN(sample) || math.IsInf(sample, 0) {
									t.Fatal("nonfinite impulse")
								}
							}

							if allocs := testing.AllocsPerRun(10, func() { chain.ProcessBlock(block) }); allocs != 0 {
								t.Fatalf("processing allocated %g times", allocs)
							}
						})
					}
				}
			}
		}
	}
}

func TestParametricBandHigherOrdersSharpenTransitions(t *testing.T) {
	for _, kind := range []string{"highpass", "lowpass", "peak", "lowshelf", "highshelf"} {
		low, err := ParametricBand(48000, 1000, 12, 1, kind, 4)
		if err != nil {
			t.Fatal(err)
		}

		high, err := ParametricBand(48000, 1000, 12, 1, kind, 12)
		if err != nil {
			t.Fatal(err)
		}

		f := 3000.0
		if kind == "highpass" || kind == "highshelf" {
			f = 300
		}

		lowDB, highDB := biquad.NewChain(low).MagnitudeDB(f, 48000), biquad.NewChain(high).MagnitudeDB(f, 48000)
		if kind == "lowpass" || kind == "highpass" {
			if highDB >= lowDB-30 {
				t.Fatalf("%s: order12=%g order4=%g", kind, highDB, lowDB)
			}
		} else if highDB >= lowDB {
			t.Fatalf("%s: order12 tail=%g, order4=%g", kind, highDB, lowDB)
		}
	}
}

func TestParametricBandImpulseMatchesResponse(t *testing.T) {
	for _, kind := range []string{"highpass", "lowpass", "peak", "lowshelf", "highshelf"} {
		for _, order := range []int{2, 4, 6, 8, 10, 12} {
			sections, err := ParametricBand(48000, 1000, 12, 1, kind, order)
			if err != nil {
				t.Fatal(err)
			}

			chain := biquad.NewChain(sections)
			impulse := chain.ImpulseResponse(65536)

			for _, f := range []float64{100, 500, 1000, 2000, 10000} {
				response := complex(0, 0)
				for i, sample := range impulse {
					response += complex(sample, 0) * cmplx.Exp(complex(0, -2*math.Pi*f*float64(i)/48000))
				}

				if delta := cmplx.Abs(response - chain.Response(f, 48000)); delta > 1e-8 {
					t.Fatalf("%s/order%d/%g Hz: impulse difference %g", kind, order, f, delta)
				}
			}
		}
	}
}

func TestParametricBandInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		rate, hz, gain, q float64
		kind              string
		order             int
	}{
		{0, 1000, 6, 1, "peak", 4},
		{48000, 24000, 6, 1, "peak", 4},
		{48000, 1000, 25, 1, "peak", 4},
		{48000, 1000, 6, math.NaN(), "peak", 4},
		{48000, 1000, 6, 1, "unknown", 4},
		{48000, 1000, 6, 1, "peak", 3},
		{48000, 1000, 6, 1, "peak", 14},
		{48000, 1000, 6, 1, "peak", 0},
	} {
		if _, err := ParametricBand(tc.rate, tc.hz, tc.gain, tc.q, tc.kind, tc.order); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}

func ExampleParametricBand() {
	sections, _ := ParametricBand(48000, 1000, 6, 1, "peak", 6)
	fmt.Println(len(sections))
	// Output: 3
}
