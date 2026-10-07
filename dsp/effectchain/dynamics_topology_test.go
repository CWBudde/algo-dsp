package effectchain

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
)

func TestDynamicsTopologyCatalogueAndProcessing(t *testing.T) {
	supported := map[string]bool{
		"dyn-compressor": true, "dyn-gate": true, "dyn-expander": true, "dyn-multiband": true,
	}

	for _, d := range DefaultDescriptors(48000) {
		found := false

		for _, parameter := range d.Parameters {
			if parameter.ID != "topology" {
				continue
			}

			found = true

			if parameter.Type != "enum" || parameter.DefaultString != "feedforward" || len(parameter.Options) != 2 ||
				parameter.Options[0].Value != "feedforward" || parameter.Options[1].Value != "feedback" {
				t.Fatalf("%s: unexpected topology descriptor: %+v", d.ID, parameter)
			}
		}

		if found != supported[d.ID] {
			t.Fatalf("%s: topology availability = %v, want %v", d.ID, found, supported[d.ID])
		}

		if !found {
			continue
		}

		t.Run(d.ID, func(t *testing.T) {
			for _, preset := range d.Presets {
				if preset.Str["topology"] != "feedforward" {
					t.Fatalf("%s: factory preset no longer defaults to feedforward", preset.ID)
				}
			}

			const frames = 8192

			preset := FactoryPreset{Num: map[string]float64{
				"thresholdDB": -20, "ratio": 4, "kneeDB": 0, "attackMs": 1,
				"releaseMs": 50, "holdMs": 0,
			}}
			legacy := preparedCatalogChain(t, d, preset, frames)
			preset.Str = map[string]string{"topology": "feedforward"}
			feedforward := preparedCatalogChain(t, d, preset, frames)
			preset.Str["topology"] = "feedback"
			feedback := preparedCatalogChain(t, d, preset, frames)

			input := make([]float64, frames)
			for frame := range input {
				input[frame] = .8 * math.Sin(float64(frame)*.15)
				if d.ID == "dyn-gate" || d.ID == "dyn-expander" {
					input[frame] = .05
				}
			}

			outputs := make([][]float64, 0, 3)

			for _, chain := range []*Chain{legacy, feedforward, feedback} {
				planar := [][]float64{append([]float64(nil), input...), append([]float64(nil), input...)}
				if err := chain.ProcessPlanar(planar); err != nil {
					t.Fatal(err)
				}

				outputs = append(outputs, planar[0])
			}

			maxDifference := 0.0

			for frame := range input {
				if outputs[0][frame] != outputs[1][frame] {
					t.Fatalf("sparse graph changed processing at frame %d", frame)
				}

				if math.IsNaN(outputs[2][frame]) || math.IsInf(outputs[2][frame], 0) {
					t.Fatalf("feedback produced a non-finite sample at frame %d", frame)
				}

				maxDifference = math.Max(maxDifference, math.Abs(outputs[1][frame]-outputs[2][frame]))
			}

			if maxDifference < 1e-8 {
				t.Fatal("topology did not affect actual audio processing")
			}

			runtime := feedback.NodeRuntime("fx")
			checkTopology := func(want dynamics.DynamicsTopology) {
				t.Helper()

				switch r := runtime.(type) {
				case *compressorRuntime:
					if r.fx.Topology() != want {
						t.Fatal("compressor topology not applied")
					}
				case *gateRuntime:
					if r.fx.Topology() != want {
						t.Fatal("gate topology not applied")
					}
				case *expanderRuntime:
					if r.fx.Topology() != want {
						t.Fatal("expander topology not applied")
					}
				case *multibandRuntime:
					for band := range r.fx.NumBands() {
						if r.fx.Band(band).Topology() != want {
							t.Fatalf("multiband band %d topology not applied", band)
						}
					}
				}
			}
			checkTopology(dynamics.DynamicsTopologyFeedback)

			if err := runtime.Configure(Context{SampleRate: 48000}, Params{Type: d.ID, Num: preset.Num}); err != nil {
				t.Fatal(err)
			}

			checkTopology(dynamics.DynamicsTopologyFeedforward)
		})
	}
}
