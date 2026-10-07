package effectchain

import (
	"fmt"
	"math"
	"testing"
)

func TestDynamicEQDefaultBandsAndSparseRuntime(t *testing.T) {
	for _, sr := range []float64{8000, 16000, 48000, 96000} {
		t.Run(fmt.Sprint(sr), func(t *testing.T) {
			var d Descriptor

			for _, entry := range DefaultDescriptors(sr) {
				if entry.ID == "dyn-eq" {
					d = entry
				}
			}

			preset := d.Presets[0]
			if preset.Num["bands"] != 3 {
				t.Fatal("want three default bands")
			}

			full := New(Context{SampleRate: sr}, DefaultRegistry())

			sparse := New(Context{SampleRate: sr}, DefaultRegistry())
			for _, test := range []struct {
				chain  *Chain
				preset FactoryPreset
			}{{full, preset}, {sparse, FactoryPreset{}}} {
				if err := test.chain.LoadGraph(catalogGraph(t, d, test.preset)); err != nil {
					t.Fatal(err)
				}
			}

			previous := 0.0

			for b := 0; b < 3; b++ {
				cfg, err := sparse.NodeRuntime("fx").(*dynamicEQRuntime).fx.BandConfig(b)
				if err != nil {
					t.Fatal(err)
				}

				hz := preset.Num[fmt.Sprintf("band%dFreqHz", b+1)]
				if hz <= previous || hz >= sr/2 || cfg.FrequencyHz != hz {
					t.Fatalf("inconsistent/spread defaults band%d: %g / %g", b, hz, previous)
				}

				if previous > 0 && hz/previous < 4 {
					t.Fatal("default nodes too close on logarithmic axis")
				}

				previous = hz
			}
		})
	}
}

func TestDynamicEQBandTransferModesRangeAndState(t *testing.T) {
	for _, mode := range []string{"static", "downward", "upward", "upward-below"} {
		for _, gainRange := range []float64{0, 6, 24} {
			t.Run(fmt.Sprintf("%s/range%g", mode, gainRange), func(t *testing.T) {
				preset := FactoryPreset{Num: map[string]float64{"bands": 3}, Str: map[string]string{}}

				for b := 1; b <= 3; b++ {
					prefix := fmt.Sprintf("band%d", b)
					preset.Num[prefix+"ThresholdDB"] = -20
					preset.Num[prefix+"Ratio"] = 4
					preset.Num[prefix+"KneeDB"] = 0
					preset.Num[prefix+"RangeDB"] = gainRange
					preset.Num[prefix+"GainDB"] = float64(b)
					preset.Str[prefix+"Mode"] = mode
				}

				chain := preparedCatalogChain(t, Descriptor{ID: "dyn-eq"}, preset, 128)
				runtime := chain.NodeRuntime("fx").(*dynamicEQRuntime)
				levels := []float64{-80, -40, -20, -10, 0}

				for b := 0; b < 3; b++ {
					preset.Num["responseBand"] = float64(b)
					if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: "dyn-eq", Num: preset.Num, Str: preset.Str}); err != nil {
						t.Fatal(err)
					}

					beforeCoeff, _ := runtime.fx.BandCoefficients(b)
					beforeGain, _ := runtime.fx.BandGainDB(b)

					values, err := chain.Transfer(levels)
					if err != nil {
						t.Fatal(err)
					}

					for i, db := range levels {
						delta := 0.0

						switch mode {
						case "downward":
							delta = -math.Min(gainRange, math.Max(0, (db+20)*.75))
						case "upward":
							delta = math.Min(gainRange, math.Max(0, (db+20)*.75))
						case "upward-below":
							delta = math.Min(gainRange, math.Max(0, (-20-db)*.75))
						}

						want := db + float64(b+1) + delta
						if math.Abs(values[i]-want) > 1e-9 {
							t.Fatalf("band%d %g dB: got%g want%g", b, db, values[i], want)
						}
					}

					afterCoeff, _ := runtime.fx.BandCoefficients(b)

					afterGain, _ := runtime.fx.BandGainDB(b)
					if beforeCoeff != afterCoeff || beforeGain != afterGain {
						t.Fatal("transfer advanced processing state")
					}
				}

				buffer := [][]float64{make([]float64, 128), make([]float64, 128)}

				if allocs := testing.AllocsPerRun(100, func() {
					if err := chain.ProcessPlanar(buffer); err != nil {
						t.Fatal(err)
					}
				}); allocs != 0 {
					t.Fatalf("render allocs %g", allocs)
				}
			})
		}
	}
}
