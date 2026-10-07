package effectchain

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
	"testing"
)

func TestStandardFilterOrdersThroughTwenty(t *testing.T) {
	for _, d := range DefaultDescriptors(48000) {
		if !strings.HasPrefix(d.ID, "filter") || d.ID == "filter-moog" {
			continue
		}

		for _, p := range d.Parameters {
			if p.ID == "order" && p.Max != 20 {
				t.Fatalf("%s order maximum: %g", d.ID, p.Max)
			}
		}
	}

	for _, sr := range []float64{8000, 48000, 384000} {
		for _, freq := range []float64{math.Max(20, sr*.001), sr * .1, sr * .49} {
			for _, family := range []string{"butterworth", "chebyshev1", "chebyshev2"} {
				for _, kind := range []string{"lowpass", "highpass", "peak", "lowshelf", "highshelf"} {
					for _, order := range []int{13, 14, 16, 18, 20} {
						if kind == "peak" && order%2 != 0 {
							continue
						}

						t.Run(fmt.Sprintf("%g/%g/%s/%s/%d", sr, freq, family, kind, order), func(t *testing.T) {
							preset := FactoryPreset{Num: map[string]float64{
								"freq": freq, "gain": 6, "q": 1, "order": float64(order),
								"bandwidthHz": math.Min(freq, sr/2-freq), "rippleDB": .5, "stopbandDB": 40,
							}, Str: map[string]string{"family": family, "kind": kind}}

							chain := New(Context{SampleRate: sr}, DefaultRegistry())
							if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "filter"}, preset)); err != nil {
								t.Fatal(err)
							}

							fx := chain.NodeRuntime("fx").(*filterRuntime).fx

							want := (order + 1) / 2
							if kind == "peak" {
								want = order
							}

							if fx.NumSections() != want {
								t.Fatalf("got %d sections, want %d (order must not clamp or fall back)", fx.NumSections(), want)
							}

							for i := range fx.NumSections() {
								c := fx.Section(i).Coefficients
								if math.Abs(c.A2) >= 1 || 1+c.A1+c.A2 <= 0 || 1-c.A1+c.A2 <= 0 {
									t.Fatalf("unstable section %d: %+v", i, c)
								}
							}

							response, err := chain.Response([]float64{0, 20, freq, sr * .4, sr * .49, sr / 2})
							if err != nil {
								t.Fatal(err)
							}

							for _, value := range response {
								if math.IsNaN(value) || math.IsInf(value, 0) || value > 100 {
									t.Fatalf("unsafe response: %g", value)
								}
							}

							impulse := make([]float64, 2048)
							impulse[0] = 1
							fx.ProcessBlock(impulse)

							for _, value := range impulse {
								if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 100 {
									t.Fatalf("unsafe processed impulse: %g", value)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOrderTwentyImpulseMatchesResponse(t *testing.T) {
	for _, family := range []string{"butterworth", "chebyshev1", "chebyshev2"} {
		for _, kind := range []string{"lowpass", "highpass", "peak", "lowshelf", "highshelf"} {
			t.Run(family+"/"+kind, func(t *testing.T) {
				preset := FactoryPreset{Num: map[string]float64{
					"freq": 1200, "gain": 6, "q": 1, "order": 20, "bandwidthHz": 800,
					"rippleDB": .5, "stopbandDB": 40,
				}, Str: map[string]string{"family": family, "kind": kind}}

				chain := preparedCatalogChain(t, Descriptor{ID: "filter"}, preset, 128)
				if err := chain.PreparePlanar(1, 128); err != nil {
					t.Fatal(err)
				}

				frequencies := []float64{100, 800, 1200, 3000, 10000}

				response, err := chain.Response(frequencies)
				if err != nil {
					t.Fatal(err)
				}

				impulse := make([]float64, 65536)
				impulse[0] = 1

				for offset := 0; offset < len(impulse); offset += 128 {
					if err := chain.ProcessPlanar([][]float64{impulse[offset : offset+128]}); err != nil {
						t.Fatal(err)
					}
				}

				for i, hz := range frequencies {
					measured := complex(0, 0)
					for frame, sample := range impulse {
						measured += complex(sample, 0) * cmplx.Rect(1, -2*math.Pi*hz*float64(frame)/48000)
					}

					if delta := math.Abs(cmplx.Abs(measured) - response[i]); delta > 1e-8 {
						t.Fatalf("%g Hz: measured %g, response %g (delta %g)", hz, cmplx.Abs(measured), response[i], delta)
					}
				}
			})
		}
	}
}

func TestFilterFamilyOrderCaps(t *testing.T) {
	for _, tc := range []struct {
		family      string
		order, want int
	}{
		{"butterworth", 20, 20}, {"chebyshev1", 20, 20}, {"chebyshev2", 20, 20},
		{"butterworth", 30, 20}, {"bessel", 20, 10}, {"elliptic", 20, 12}, {"rbj", 20, 1},
	} {
		if got := (BuiltInFilterDesigner{}).NormalizeOrder("lowpass", tc.family, tc.order); got != tc.want {
			t.Errorf("%s order %d: got %d, want %d", tc.family, tc.order, got, tc.want)
		}
	}
}
