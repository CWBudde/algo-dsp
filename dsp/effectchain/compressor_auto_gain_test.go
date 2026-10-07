package effectchain

import (
	"math"
	"testing"
)

func TestCompressorAutoGainProcessingTransferAndToggle(t *testing.T) {
	var descriptor Descriptor

	for _, d := range DefaultDescriptors(48000) {
		if d.ID == "dyn-compressor" {
			descriptor = d
		}
	}

	if descriptor.Presets[0].Num["autoMakeup"] != 0 {
		t.Fatal("automatic gain must remain opt-in")
	}

	preset := FactoryPreset{Num: map[string]float64{
		"thresholdDB": -24, "ratio": 4, "kneeDB": 0, "attackMs": 10,
		"releaseMs": 100, "makeupGainDB": 3,
	}}
	manual := preparedCatalogChain(t, descriptor, preset, 128)
	preset.Num["autoMakeup"] = 1
	automatic := preparedCatalogChain(t, descriptor, preset, 128)
	levels := []float64{-60, -24, 0}

	response, err := automatic.Transfer(levels)
	if err != nil {
		t.Fatal(err)
	}

	for i, wantDB := range []float64{-42, -6, 0} {
		if math.Abs(response[i]-wantDB) > 1e-6 {
			t.Fatalf("auto transfer %g dB: %g, want %g", levels[i], response[i], wantDB)
		}
	}

	for range 64 {
		input, reference := [][]float64{make([]float64, 128), make([]float64, 128)}, [][]float64{make([]float64, 128), make([]float64, 128)}
		for ch := range input {
			for frame := range input[ch] {
				input[ch][frame], reference[ch][frame] = .1, .1
			}
		}

		if err := automatic.ProcessPlanar(input); err != nil {
			t.Fatal(err)
		}

		if err := manual.ProcessPlanar(reference); err != nil {
			t.Fatal(err)
		}

		for ch := range input {
			for frame, sample := range input[ch] {
				// Automatic gain is 18 dB; the stored manual setting is 3 dB.
				if math.Abs(sample-reference[ch][frame]*math.Pow(10, 15.0/20)) > 1e-8 {
					t.Fatalf("automatic processing did not use the same makeup as transfer: %g, manual %g", sample, reference[ch][frame])
				}
			}
		}
	}

	runtime := automatic.NodeRuntime("fx").(*compressorRuntime)
	p := Params{Type: descriptor.ID, Num: preset.Num}

	p.Num["thresholdDB"] = -12
	if err := runtime.Configure(Context{SampleRate: 48000}, p); err != nil {
		t.Fatal(err)
	}

	if !runtime.fx.AutoMakeup() || math.Abs(runtime.fx.MakeupGain()-9) > 1e-12 {
		t.Fatal("automatic gain did not follow the threshold update")
	}

	p.Num["autoMakeup"] = 0
	if err := runtime.Configure(Context{SampleRate: 48000}, p); err != nil {
		t.Fatal(err)
	}

	if runtime.fx.AutoMakeup() || runtime.fx.MakeupGain() != 3 {
		t.Fatal("turning automatic gain off did not restore manual makeup")
	}
}
