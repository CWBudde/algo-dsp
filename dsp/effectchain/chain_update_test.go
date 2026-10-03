package effectchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type failedUpdateRuntime struct{ failure string }

func (r *failedUpdateRuntime) Configure(Context, Params) error {
	if r.failure == "configure" {
		return errors.New("intentional configure failure")
	}

	return nil
}

func (r *failedUpdateRuntime) Prepare(int) error {
	if r.failure == "prepare" {
		return errors.New("intentional prepare failure")
	}

	return nil
}

func (*failedUpdateRuntime) Process([]float64) {}

func TestTryUpdateGraphStageFailuresLeaveEveryChannelUntouched(t *testing.T) {
	for _, failure := range []string{"constructor", "configure", "prepare", "nil-runtime"} {
		t.Run(failure, func(t *testing.T) {
			active, count := false, 0
			registry := NewRegistry()
			registry.MustRegister("test", func(Context) (Runtime, error) {
				if active {
					count++
					if count == 2 {
						if failure == "constructor" {
							return nil, errors.New("intentional constructor failure")
						}

						if failure == "nil-runtime" {
							return nil, nil
						}

						return &failedUpdateRuntime{failure: failure}, nil
					}
				}

				return &failedUpdateRuntime{}, nil
			})

			chain := New(Context{SampleRate: 48000}, registry)
			if err := chain.LoadGraph(catalogGraph(t, Descriptor{ID: "test"}, FactoryPreset{Num: map[string]float64{"value": 0}})); err != nil {
				t.Fatal(err)
			}

			if err := chain.PreparePlanar(2, 128); err != nil {
				t.Fatal(err)
			}

			graph := chain.graph
			original := []Runtime{chain.planar[0].nodes["fx"].runtime, chain.planar[1].nodes["fx"].runtime}
			active = true

			updated, err := chain.TryUpdateGraph(catalogGraph(t, Descriptor{ID: "test"}, FactoryPreset{Num: map[string]float64{"value": 1}}))
			if updated || err == nil {
				t.Fatalf("stage failure accepted: updated=%v err=%v", updated, err)
			}

			for channel, planar := range chain.planar {
				if planar.graph != graph || planar.nodes["fx"].runtime != original[channel] || planar.preparedFrames != 128 {
					t.Fatal("partial staged update changed prepared channel")
				}
			}
		})
	}
}

func updateGraph(t *testing.T, wet, carrier float64) string {
	t.Helper()

	graph := graphState{Nodes: []graphNode{{ID: InputNodeID, Type: InputNodeID}, {ID: "conv", Type: "reverb-conv", Params: map[string]any{"irIndex": 0, "wet": wet}}, {ID: "ring", Type: "ringmod", Params: map[string]any{"carrierHz": carrier}}, {ID: OutputNodeID, Type: OutputNodeID}}, Connections: []graphConnection{{From: InputNodeID, To: "conv"}, {From: "conv", To: "ring"}, {From: "ring", To: OutputNodeID}}}

	encoded, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}

	return string(encoded)
}

func updateChain(t *testing.T, raw string, provider IRProvider) *Chain {
	t.Helper()

	chain := New(Context{SampleRate: 48000}, DefaultRegistry(WithIRProvider(provider)))
	if err := chain.LoadGraph(raw); err != nil {
		t.Fatal(err)
	}

	if err := chain.PreparePlanar(2, 128); err != nil {
		t.Fatal(err)
	}

	return chain
}

func TestTryUpdateGraphRetainsConvolutionHistoryAndPreparedStorage(t *testing.T) {
	ir := workspaceIR{samples: [][]float64{make([]float64, 4097)}}
	ir.samples[0][128] = .75
	chain := updateChain(t, updateGraph(t, 1, 1000), ir)
	control := updateChain(t, updateGraph(t, .5, 1000), ir)
	original := chain.planar[0].nodes["conv"].runtime.(*convReverbRuntime)
	graph := chain.graph
	block := [][]float64{make([]float64, 128), make([]float64, 128)}
	block[0][0], block[1][0] = 1, .5

	reference := [][]float64{append([]float64(nil), block[0]...), append([]float64(nil), block[1]...)}
	if err := chain.ProcessPlanar(block); err != nil {
		t.Fatal(err)
	}

	if err := control.ProcessPlanar(reference); err != nil {
		t.Fatal(err)
	}

	updated, err := chain.TryUpdateGraph(updateGraph(t, .5, 1000))
	if err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}

	if chain.graph == graph || chain.planar[0].nodes["conv"].runtime != original || chain.preparedFrames != 128 {
		t.Fatal("update rebuilt convolution or discarded preparation")
	}

	nonzero := false

	for range 40 {
		for channel := range block {
			clear(block[channel])
			clear(reference[channel])
		}

		if err := chain.ProcessPlanar(block); err != nil {
			t.Fatal(err)
		}

		if err := control.ProcessPlanar(reference); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(block, reference) {
			t.Fatalf("retained convolution tail differs: %v / %v", block, reference)
		}

		for _, sample := range block[0] {
			nonzero = nonzero || sample != 0
		}
	}

	if !nonzero {
		t.Fatal("oracle did not exercise nonzero convolution tail")
	}

	allocations := testing.AllocsPerRun(3, func() {
		if err := chain.ProcessPlanar(block); err != nil {
			panic(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("updated render allocated %g", allocations)
	}
}

func TestTryUpdateGraphStagesOrdinaryNodeWithoutRebuildingUnchangedIR(t *testing.T) {
	chain := updateChain(t, updateGraph(t, .5, 1000), catalogIR{48000})
	convolutions := []Runtime{chain.planar[0].nodes["conv"].runtime, chain.planar[1].nodes["conv"].runtime}
	rings := []Runtime{chain.planar[0].nodes["ring"].runtime, chain.planar[1].nodes["ring"].runtime}

	updated, err := chain.TryUpdateGraph(updateGraph(t, .25, 2000))
	if err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}

	for channel, planar := range chain.planar {
		if planar.nodes["conv"].runtime != convolutions[channel] || planar.nodes["ring"].runtime == rings[channel] {
			t.Fatal("wrong nodes reconstructed")
		}

		if planar.nodes["conv"].runtime.(*convReverbRuntime).fx == nil || planar.graph.Nodes["ring"].Num["carrierHz"] != 2000 {
			t.Fatal("update not applied on every channel")
		}
	}
}

func TestTryUpdateGraphRejectsAtomically(t *testing.T) {
	raw := updateGraph(t, .5, 1000)
	for _, next := range []string{"{", updateGraph(t, -1, 2000), updateGraph(t, 2, 2000)} {
		chain := updateChain(t, raw, catalogIR{48000})
		graph := chain.graph

		ring := chain.nodes["ring"].runtime
		if updated, err := chain.TryUpdateGraph(next); err == nil || updated {
			t.Fatalf("invalid update accepted updated=%v err=%v", updated, err)
		}

		if chain.graph != graph || chain.nodes["ring"].runtime != ring {
			t.Fatal("invalid update modified live graph")
		}
	}

	chain := updateChain(t, raw, catalogIR{48000})
	chain.planar[1].nodes["conv"].runtime = &convReverbRuntime{}
	graph := chain.graph

	original := chain.nodes["conv"].runtime.(*convReverbRuntime)
	if updated, err := chain.TryUpdateGraph(updateGraph(t, .25, 2000)); err == nil || updated {
		t.Fatal("unprepared second channel update accepted")
	}

	if chain.graph != graph {
		t.Fatal("partial graph publication")
	}

	block := make([]float64, 128)
	block[0] = 1
	original.Process(block)
	block = make([]float64, 128)
	original.Process(block)

	if block[0] != 1.5 {
		t.Fatalf("failed update changed first-channel wet: %.17g", block[0])
	}
}

func TestTryUpdateGraphUnsupportedChangesLeaveState(t *testing.T) {
	for _, next := range []string{
		catalogGraph(t, Descriptor{ID: "pitch-spectral"}, FactoryPreset{Num: map[string]float64{"semitones": 12}}),
		catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{Num: map[string]float64{"irIndex": 1}}),
		"{}",
	} {
		chain := updateChain(t, catalogGraph(t, Descriptor{ID: "pitch-spectral"}, FactoryPreset{Num: map[string]float64{"semitones": -12}}), catalogIR{48000})

		graph := chain.graph
		if updated, err := chain.TryUpdateGraph(next); err != nil || updated {
			t.Fatalf("fallback expected: updated=%v err=%v", updated, err)
		}

		if chain.graph != graph {
			t.Fatal("unsupported update modified graph")
		}
	}

	chain := New(Context{SampleRate: 48000}, DefaultRegistry())
	if updated, err := chain.TryUpdateGraph("{}"); err != nil || updated {
		t.Fatal("unprepared chain update accepted")
	}
}

func ExampleChain_TryUpdateGraph() {
	chain := New(Context{SampleRate: 48000}, DefaultRegistry())
	graph := `{"nodes":[{"id":"_input","type":"_input"},{"id":"fx","type":"ringmod","params":{"carrierHz":1000}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"fx"},{"from":"fx","to":"_output"}]}`
	_ = chain.LoadGraph(graph)
	_ = chain.PreparePlanar(2, 128)
	updated, err := chain.TryUpdateGraph(graph)
	fmt.Println(updated, err)
	// Output: true <nil>
}
