package effectchain

import (
	"fmt"
	"math"
	"testing"
)

func TestMultibandIndependentSettingsTransferAndLegacy(t *testing.T) {
	var d Descriptor

	for _, descriptor := range DefaultDescriptors(48000) {
		if descriptor.ID == "dyn-multiband" {
			d = descriptor
		}
	}

	for _, bands := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(bands), func(t *testing.T) {
			preset := FactoryPreset{Num: map[string]float64{
				"bands": float64(bands), "attackMs": 15, "releaseMs": 150, "kneeDB": 10,
				"makeupGainDB": 3, "autoMakeup": 0,
			}}
			chain := preparedCatalogChain(t, d, preset, 128)

			runtime := chain.NodeRuntime("fx").(*multibandRuntime)
			if runtime.fx.NumBands() != bands || len(runtime.fx.CrossoverFreqs()) != bands-1 {
				t.Fatal("requested band count not implemented")
			}

			// New metadata defaults must not override old shared factory/user settings.
			for key, value := range d.Presets[0].Num {
				if _, exists := preset.Num[key]; !exists {
					preset.Num[key] = value
				}
			}

			if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
				t.Fatal(err)
			}

			for b := range bands {
				fx := runtime.fx.Band(b)
				if fx.Attack() != 15 || fx.Release() != 150 || fx.Knee() != 10 || fx.MakeupGain() != 3 {
					t.Fatal("materialized descriptor defaults changed legacy shared settings")
				}
			}

			preset.Num["perBand"] = 1

			for b := range bands {
				prefix := multibandPrefix(b, bands)
				preset.Num[prefix+"AttackMs"] = float64(b + 1)
				preset.Num[prefix+"ReleaseMs"] = float64((b + 1) * 25)
				preset.Num[prefix+"KneeDB"] = float64(b * 2)
				preset.Num[prefix+"MakeupGainDB"] = float64(b + 1)
				preset.Num[prefix+"AutoMakeup"] = 0
			}

			if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
				t.Fatal(err)
			}

			for b := range bands {
				fx := runtime.fx.Band(b)
				if fx.Attack() != float64(b+1) || fx.Release() != float64((b+1)*25) ||
					fx.Knee() != float64(b*2) || fx.MakeupGain() != float64(b+1) {
					t.Fatalf("band %d settings did not reach its actual compressor", b)
				}

				preset.Num["responseBand"] = float64(b)
				if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
					t.Fatal(err)
				}

				levels := []float64{-80, -60, -24, -12, 0}

				transfer, err := chain.Transfer(levels)
				if err != nil {
					t.Fatal(err)
				}

				for i, db := range levels {
					want := 20 * math.Log10(fx.CalculateOutputLevel(math.Pow(10, db/20)))
					if math.Abs(transfer[i]-want) > 1e-10 {
						t.Fatalf("band %d transfer %g: got %g, want %g", b, db, transfer[i], want)
					}
				}
			}

			prefix := multibandPrefix(bands-1, bands)

			preset.Num[prefix+"AutoMakeup"] = 1
			if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
				t.Fatal(err)
			}

			if !runtime.fx.Band(bands-1).AutoMakeup() || runtime.fx.Band(0).AutoMakeup() {
				t.Fatal("automatic gain must be independent per band")
			}

			preset.Num[prefix+"AutoMakeup"] = 0
			if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
				t.Fatal(err)
			}

			if runtime.fx.Band(bands-1).MakeupGain() != float64(bands) {
				t.Fatal("disabling automatic gain lost stored manual makeup")
			}

			independent := preparedCatalogChain(t, d, preset, 128)
			preset.Num["perBand"] = 0
			shared := preparedCatalogChain(t, d, preset, 128)
			maxDifference := 0.0

			for block := range 64 {
				a, b := [][]float64{make([]float64, 128), make([]float64, 128)}, [][]float64{make([]float64, 128), make([]float64, 128)}
				for ch := range a {
					for frame := range a[ch] {
						x := .8 * math.Sin(float64(block*128+frame)*.15)
						a[ch][frame], b[ch][frame] = x, x
					}
				}

				if err := independent.ProcessPlanar(a); err != nil {
					t.Fatal(err)
				}

				if err := shared.ProcessPlanar(b); err != nil {
					t.Fatal(err)
				}

				for frame, sample := range a[0] {
					if math.IsNaN(sample) || math.IsInf(sample, 0) {
						t.Fatal("non-finite independent processing")
					}

					maxDifference = math.Max(maxDifference, math.Abs(sample-b[0][frame]))
				}
			}

			if maxDifference < 1e-6 {
				t.Fatal("independent settings did not change processed audio")
			}

			buffer := [][]float64{make([]float64, 128), make([]float64, 128)}

			if allocations := testing.AllocsPerRun(100, func() {
				if err := independent.ProcessPlanar(buffer); err != nil {
					t.Fatal(err)
				}
			}); allocations != 0 {
				t.Fatalf("processing allocated %g objects", allocations)
			}
		})
	}
}

func TestFourBandWorkspaceBound(t *testing.T) {
	for _, rate := range []float64{8000, 48000, 384000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			graph := catalogGraph(t, Descriptor{ID: "dyn-multiband"}, FactoryPreset{Num: map[string]float64{"bands": 4}})

			estimate, err := EstimateWorkspace(Context{SampleRate: rate}, graph, 2, WithWorkspaceFrames(128))
			if err != nil {
				t.Fatal(err)
			}

			assertWorkspaceBound(t, rate, graph, 2, nil, estimate)
		})
	}
}
