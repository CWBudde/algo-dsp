package effectchain

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"testing"
)

type workspaceIR struct{ samples [][]float64 }

func (ir workspaceIR) GetIR(int) ([][]float64, float64, bool) {
	return ir.samples, 48000, true
}

func TestWorkspaceConvolutionPartitionsAndHistoryBound(t *testing.T) {
	graph := catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{})
	for _, frames := range []int{129, 8193, 300000} {
		t.Run(fmt.Sprint(frames), func(t *testing.T) {
			ir := workspaceIR{samples: [][]float64{make([]float64, frames)}}
			ir.samples[0][0] = 1

			estimate, err := EstimateWorkspace(Context{SampleRate: 48000}, graph, 1, WithWorkspaceFrames(128), WithWorkspaceIRFrames(map[int]int{0: frames}))
			if err != nil {
				t.Fatal(err)
			}

			assertWorkspaceBound(t, 48000, graph, 1, ir, estimate)
		})
	}
}

func TestWorkspaceIncludesParallelLatencyCompensation(t *testing.T) {
	graph := graphState{Nodes: []graphNode{{ID: InputNodeID, Type: InputNodeID}, {ID: "fx", Type: "pitch-time", Params: map[string]any{"semitones": -24}}, {ID: OutputNodeID, Type: OutputNodeID}}, Connections: []graphConnection{{From: InputNodeID, To: "fx"}, {From: "fx", To: OutputNodeID}}}

	serial, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}

	graph.Connections = append(graph.Connections, graphConnection{From: InputNodeID, To: OutputNodeID})

	parallel, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}

	context := Context{SampleRate: 48000}

	one, err := EstimateWorkspace(context, string(serial), 2, WithWorkspaceFrames(128))
	if err != nil {
		t.Fatal(err)
	}

	two, err := EstimateWorkspace(context, string(parallel), 2, WithWorkspaceFrames(128))
	if err != nil {
		t.Fatal(err)
	}

	if two <= one {
		t.Fatal("parallel path did not reserve latency-compensation history")
	}

	assertWorkspaceBound(t, context.SampleRate, string(parallel), 2, nil, two)
}

func TestWorkspaceBoundsCatalogueConstruction(t *testing.T) {
	for _, rate := range []float64{8000, 48000, 384000} {
		for _, descriptor := range DefaultDescriptors(rate) {
			for _, preset := range descriptor.Presets {
				t.Run(fmt.Sprintf("%s/%s/%g", descriptor.ID, preset.ID, rate), func(t *testing.T) {
					graph := catalogGraph(t, descriptor, preset)

					estimate, err := EstimateWorkspace(Context{SampleRate: rate}, graph, 2, WithWorkspaceFrames(128), WithWorkspaceIRFrames(map[int]int{0: 4}))
					if err != nil {
						t.Fatal(err)
					}

					assertWorkspaceBound(t, rate, graph, 2, catalogIR{rate}, estimate)
				})
			}
		}
	}
}

func assertWorkspaceBound(t *testing.T, rate float64, graph string, channels int, provider IRProvider, estimate int64) {
	t.Helper()
	runtime.GC()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	chain := New(Context{SampleRate: rate}, DefaultRegistry(WithIRProvider(provider)))
	if err := chain.LoadGraph(graph); err != nil {
		t.Fatal(err)
	}

	if err := chain.PreparePlanar(channels, 128); err != nil {
		t.Fatal(err)
	}

	runtime.ReadMemStats(&after)
	runtime.KeepAlive(chain)

	actual := after.TotalAlloc - before.TotalAlloc
	if actual > uint64(estimate) {
		t.Errorf("cold constructor/preparation allocated %d bytes above %d-byte estimate", actual, estimate)
	}
}

func TestWorkspaceValidation(t *testing.T) {
	graph := catalogGraph(t, Descriptor{ID: "ringmod"}, FactoryPreset{})
	for _, rate := range []float64{0, -1, math.NaN(), math.Inf(1), 1e10} {
		if _, err := EstimateWorkspace(Context{SampleRate: rate}, graph, 2); err == nil {
			t.Errorf("accepted rate %g", rate)
		}
	}

	for _, channels := range []int{0, -1, 9} {
		if _, err := EstimateWorkspace(Context{SampleRate: 48000}, graph, channels); err == nil {
			t.Errorf("accepted channels %d", channels)
		}
	}

	for _, size := range []int{0, -1, 1 << 21} {
		if _, err := EstimateWorkspace(Context{SampleRate: 48000}, graph, 2, WithWorkspaceFrames(size)); err == nil {
			t.Errorf("accepted block %d", size)
		}
	}

	for _, invalid := range []string{"{", "{}", catalogGraph(t, Descriptor{ID: "unknown"}, FactoryPreset{})} {
		if _, err := EstimateWorkspace(Context{SampleRate: 48000}, invalid, 2); err == nil {
			t.Errorf("accepted graph %s", invalid)
		}
	}

	if _, err := EstimateWorkspace(Context{SampleRate: 48000}, graph, 2, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceImpulseBoundsAndOwnership(t *testing.T) {
	graph := catalogGraph(t, Descriptor{ID: "reverb-conv"}, FactoryPreset{Num: map[string]float64{"irIndex": 7}})

	context := Context{SampleRate: 48000}
	for _, frames := range []map[int]int{nil, {7: 0}, {7: -1}, {8: 100}} {
		if _, err := EstimateWorkspace(context, graph, 2, WithWorkspaceIRFrames(frames)); err == nil {
			t.Errorf("accepted missing/invalid IR bound %v", frames)
		}
	}

	bounds := map[int]int{7: 300000}
	option := WithWorkspaceIRFrames(bounds)

	first, err := EstimateWorkspace(context, graph, 2, option)
	if err != nil {
		t.Fatal(err)
	}

	bounds[7] = 4

	second, err := EstimateWorkspace(context, graph, 2, option)
	if err != nil {
		t.Fatal(err)
	}

	if first <= second {
		t.Fatal("IR geometry does not affect convolution budget")
	}

	if bounds[7] != 4 {
		t.Fatal("estimator mutated caller geometry")
	}
}

func TestWorkspaceWorstCaseTimePitchAndMultiplicity(t *testing.T) {
	graph := catalogGraph(t, Descriptor{ID: "pitch-time"}, FactoryPreset{Num: map[string]float64{"sequence": 120, "overlap": 4, "search": 40, "semitones": -24}})
	context := Context{SampleRate: 384000}

	mono, err := EstimateWorkspace(context, graph, 1, WithWorkspaceFrames(128))
	if err != nil {
		t.Fatal(err)
	}

	assertWorkspaceBound(t, context.SampleRate, graph, 1, nil, mono)

	eight, err := EstimateWorkspace(context, graph, 8, WithWorkspaceFrames(128))
	if err != nil {
		t.Fatal(err)
	}

	if eight != mono*8 || mono < 16<<20 {
		t.Fatalf("pitch history budget mono=%d eight=%d", mono, eight)
	}
}

func TestWorkspaceBoundsLargestBuiltInHistories(t *testing.T) {
	for _, tc := range []struct {
		typeID string
		num    map[string]float64
		str    map[string]string
	}{
		{"granular", map[string]float64{"grainSeconds": .5, "baseDelay": 2}, nil},
		{"chorus", map[string]float64{"stages": 6, "depth": .01}, nil},
		{"reverb", map[string]float64{"modDepth": .01, "preDelay": .1}, map[string]string{"model": "fdn"}},
		{"pitch-spectral", map[string]float64{"frameSize": 8192, "semitones": 12}, nil},
		{"spectral-freeze", map[string]float64{"frameSize": 8192}, nil},
		{"dyn-expander", map[string]float64{"rmsWindowMs": 1000}, nil},
		{"dyn-lookahead", map[string]float64{"lookaheadMs": 200}, nil},
		{"dyn-eq", map[string]float64{"bands": 8}, nil},
	} {
		t.Run(tc.typeID, func(t *testing.T) {
			graph := catalogGraph(t, Descriptor{ID: tc.typeID}, FactoryPreset{Num: tc.num, Str: tc.str})

			estimate, err := EstimateWorkspace(Context{SampleRate: 384000}, graph, 2, WithWorkspaceFrames(128))
			if err != nil {
				t.Fatal(err)
			}

			assertWorkspaceBound(t, 384000, graph, 2, nil, estimate)
		})
	}
}

func ExampleEstimateWorkspace() {
	graph := `{"nodes":[{"id":"_input","type":"_input"},{"id":"delay","type":"delay","params":{"time":0.01}},{"id":"_output","type":"_output"}],"connections":[{"from":"_input","to":"delay"},{"from":"delay","to":"_output"}]}`
	bytes, err := EstimateWorkspace(Context{SampleRate: 48000}, graph, 2, WithWorkspaceFrames(128))
	fmt.Println(bytes > 1000000, err)
	// Output: true <nil>
}
