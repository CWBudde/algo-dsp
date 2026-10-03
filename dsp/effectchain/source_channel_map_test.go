package effectchain

import (
	"math"
	"testing"
)

func TestSourceChannelMapPreservesPhysicalStereoIRSide(t *testing.T) {
	for _, mapping := range [][]int{{1}, {0, 2}, {1, 2, 3}, {7}, nil} {
		channels := len(mapping)
		if channels == 0 {
			channels = 2
		}

		registry := DefaultRegistry(WithIRProvider(catalogIR{48000}), WithSourceChannelMap(mapping))

		chain := New(Context{SampleRate: 48000}, registry)
		if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{Num: map[string]float64{"wet": 1}})); err != nil {
			t.Fatal(err)
		}

		if err := chain.PreparePlanar(channels, 256); err != nil {
			t.Fatal(err)
		}

		blocks := make([][]float64, channels)
		for ch := range blocks {
			blocks[ch] = make([]float64, 256)
			blocks[ch][0] = 1
		}

		if err := chain.ProcessPlanar(blocks); err != nil {
			t.Fatal(err)
		}

		for ch := range blocks {
			physical := ch
			if len(mapping) > 0 {
				physical = mapping[ch]
			}

			want := 2.
			if physical%2 == 1 {
				want = 1.5
			}

			if math.Abs(blocks[ch][128]-want) > 1e-12 {
				t.Fatalf("map%v packed%d physical%d got%g want%g", mapping, ch, physical, blocks[ch][128], want)
			}
		}
	}
}

func TestSourceChannelMapOwnsOptionAndRegistryInput(t *testing.T) {
	mapping := []int{1}
	option := WithSourceChannelMap(mapping)
	mapping[0] = 0
	registry := DefaultRegistry(WithIRProvider(catalogIR{48000}), option)

	chain := New(Context{SampleRate: 48000}, registry)
	if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{Num: map[string]float64{"wet": 1}})); err != nil {
		t.Fatal(err)
	}

	if err := chain.PreparePlanar(1, 256); err != nil {
		t.Fatal(err)
	}

	block := [][]float64{make([]float64, 256)}

	block[0][0] = 1
	if err := chain.ProcessPlanar(block); err != nil {
		t.Fatal(err)
	}

	if math.Abs(block[0][128]-1.5) > 1e-12 {
		t.Fatal("caller mutation changed owned map")
	}

	if err := chain.PreparePlanar(2, 256); err == nil {
		t.Fatal("mismatched map coverage accepted")
	}
}

func TestSourceChannelMapInvalidIndicesRejected(t *testing.T) {
	for _, mapping := range [][]int{{-1}, {8}, {1, 1}, {0, 1, 2, 3, 4, 5, 6, 7, 8}} {
		chain := New(Context{SampleRate: 48000}, DefaultRegistry(WithSourceChannelMap(mapping)))
		if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{})); err == nil {
			t.Fatalf("map%v accepted", mapping)
		}
	}
}
