package effectchain

import (
	"fmt"
	"math"
	"testing"
)

func TestGraphicEQRuntimeEqualGainsStayFlatAcrossRatesAndOrders(t *testing.T) {
	for _, rate := range []float64{8000, 16000, 32000, 44100, 48000, 96000, 384000} {
		for _, order := range []int{4, 6, 8, 10, 12} {
			for _, gain := range []float64{-24, -12, 0, 12, 24} {
				t.Run(fmt.Sprintf("rate%g/order%d/gain%g", rate, order, gain), func(t *testing.T) {
					params := map[string]float64{"order": float64(order)}
					for band := 1; band <= 10; band++ {
						params[fmt.Sprintf("gain%dDB", band)] = gain
					}

					chain := New(Context{SampleRate: rate}, DefaultRegistry())
					if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "eq-graphic"}, FactoryPreset{Num: params})); err != nil {
						t.Fatal(err)
					}

					frequencies := []float64{0, 44.5, 88, 180, 350, 707, rate * .25, rate * .49, rate / 2}

					response, err := chain.Response(frequencies)
					if err != nil {
						t.Fatal(err)
					}

					for i, magnitude := range response {
						if db := 20 * math.Log10(magnitude); math.Abs(db-gain) > 1e-10 {
							t.Fatalf("%g Hz: %.12g dB, want %g", frequencies[i], db, gain)
						}
					}

					if err := chain.PreparePlanar(1, 128); err != nil {
						t.Fatal(err)
					}

					block := [][]float64{{.1, -.1, 0, .03}}
					if err := chain.ProcessPlanar(block); err != nil {
						t.Fatal(err)
					}

					for i, input := range []float64{.1, -.1, 0, .03} {
						if want := input * math.Pow(10, gain/20); math.Abs(block[0][i]-want) > 1e-12 {
							t.Fatalf("sample%d: %g, want %g", i, block[0][i], want)
						}
					}
				})
			}
		}
	}
}

func TestGraphicEQCoincidentHighBandsAreAveraged(t *testing.T) {
	chain := New(Context{SampleRate: 8000}, DefaultRegistry())

	params := map[string]float64{"gain8DB": 24, "gain9DB": 12, "gain10DB": -6}
	if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "eq-graphic"}, FactoryPreset{Num: params})); err != nil {
		t.Fatal(err)
	}

	response, err := chain.Response([]float64{4000})
	if err != nil {
		t.Fatal(err)
	}

	if db := 20 * math.Log10(response[0]); math.Abs(db-10) > 1e-9 {
		t.Fatalf("coincident bands: %g dB, want mean 10 dB", db)
	}
}
