package effectchain

import (
	"encoding/json"
	"math"
	"testing"
)

// TestRuntimeConfigureAndProcess exercises Configure+Process for all real runtimes
// through the DefaultRegistry. This ensures each runtime can be created, configured
// with representative parameters, and process a block without panicking.
func TestRuntimeConfigureAndProcess(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}

	tests := []struct {
		effectType string
		params     map[string]any
	}{
		{"chorus", map[string]any{"mix": 0.5, "depth": 0.5, "speed": 1.0, "stages": 2.0}},
		{"flanger", map[string]any{"rate": 0.5, "depth": 0.002, "baseDelay": 0.003, "feedback": 0.5, "mix": 0.5}},
		{"ringmod", map[string]any{"carrierHz": 440.0, "mix": 0.5}},
		{"bitcrusher", map[string]any{"bitDepth": 8.0, "downsampleFactor": 2.0, "mix": 0.5}},
		{"distortion", map[string]any{"drive": 5.0, "mix": 0.5, "mode": "softclip", "approx": "exact"}},
		{"dist-cheb", map[string]any{"drive": 5.0, "mix": 0.5, "order": 3.0, "harmonicMode": "all"}},
		{"widener", map[string]any{"width": 1.0, "mix": 0.5}},
		{"phaser", map[string]any{"rate": 0.5, "depth": 0.5, "feedback": 0.3, "stages": 4.0, "mix": 0.5}},
		{"tremolo", map[string]any{"rate": 4.0, "depth": 0.5, "mix": 0.5}},
		{"delay", map[string]any{"delayMs": 200.0, "feedback": 0.3, "mix": 0.4}},
		{"delay-simple", map[string]any{"delaySamples": 100.0, "feedback": 0.3, "mix": 0.5}},
		{"bass", map[string]any{"drive": 3.0, "freq": 100.0, "mix": 0.5}},
		{"reverb-freeverb", map[string]any{"roomSize": 0.7, "damping": 0.5, "wet": 0.3, "dry": 0.7}},
		{"reverb-fdn", map[string]any{"decaySeconds": 1.5, "damping": 0.5, "mix": 0.3}},
		{"reverb", map[string]any{"model": "fdn", "decaySeconds": 1.0, "mix": 0.3}},
		{"dyn-compressor", map[string]any{"threshold": -20.0, "ratio": 4.0, "attackMs": 10.0, "releaseMs": 100.0, "makeupGain": 6.0}},
		{"dyn-limiter", map[string]any{"threshold": -3.0, "attackMs": 1.0, "releaseMs": 50.0}},
		{"dyn-gate", map[string]any{"threshold": -40.0, "attackMs": 1.0, "releaseMs": 50.0, "ratio": 10.0}},
		{"dyn-expander", map[string]any{"threshold": -30.0, "ratio": 2.0, "attackMs": 5.0, "releaseMs": 50.0}},
		{"dyn-deesser", map[string]any{"threshold": -20.0, "freq": 6000.0, "ratio": 4.0, "mode": "splitband", "detector": "bandpass"}},
		{"dyn-transient", map[string]any{"attack": 0.5, "sustain": 0.5}},
		{"dyn-multiband", nil},
		{"pitch-time", map[string]any{"semitones": 2.0, "mix": 0.5}},
		{"pitch-spectral", map[string]any{"semitones": -3.0, "mix": 0.5, "frameSize": 2048.0}},
		{"spectral-freeze", map[string]any{"freeze": 1.0, "mix": 0.5, "phaseMode": "advance"}},
		{"granular", map[string]any{"grainSize": 50.0, "density": 4.0, "pitchShift": 0.0, "mix": 0.5}},
		{"transformer", map[string]any{"drive": 3.0, "mix": 0.5, "quality": "high", "oversampling": 2.0}},
		{"vocoder", map[string]any{"bands": 16.0, "mix": 0.5}},
		{"dyn-lookahead", map[string]any{"threshold": -3.0, "attackMs": 5.0, "releaseMs": 50.0}},
	}

	reg := DefaultRegistry()

	for _, tt := range tests {
		t.Run(tt.effectType, func(t *testing.T) {
			t.Parallel()

			factory := reg.Lookup(tt.effectType)
			if factory == nil {
				t.Fatalf("no factory for %s", tt.effectType)
			}

			rt, err := factory(ctx)
			if err != nil {
				t.Fatalf("factory error: %v", err)
			}

			num := map[string]float64{}
			str := map[string]string{}

			if tt.params != nil {
				for k, v := range tt.params {
					switch val := v.(type) {
					case float64:
						num[k] = val
					case string:
						str[k] = val
					}
				}
			}

			params := Params{
				ID:   "test-node",
				Type: tt.effectType,
				Num:  num,
				Str:  str,
			}

			err = rt.Configure(ctx, params)
			if err != nil {
				t.Fatalf("Configure error: %v", err)
			}

			block := make([]float64, 256)
			for i := range block {
				block[i] = 0.5 * math.Sin(2*math.Pi*440*float64(i)/ctx.SampleRate)
			}

			rt.Process(block)

			// Verify no NaN or Inf in output.
			for i, v := range block {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("block[%d] = %v after Process", i, v)
				}
			}
		})
	}
}

// TestRuntimeReverbFreeverb tests reverb delegate with freeverb model.
func TestRuntimeReverbFreeverb(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()

	factory := reg.Lookup("reverb")

	rt, err := factory(ctx)
	if err != nil {
		t.Fatal(err)
	}

	params := Params{
		ID:   "rev",
		Type: "reverb",
		Num:  map[string]float64{"roomSize": 0.5, "damping": 0.5, "wet": 0.3},
		Str:  map[string]string{"model": "freeverb"},
	}

	err = rt.Configure(ctx, params)
	if err != nil {
		t.Fatal(err)
	}

	block := make([]float64, 128)
	block[0] = 1.0
	rt.Process(block)

	// Verify output has energy (reverb should spread the impulse).
	var sum float64
	for _, v := range block {
		sum += v * v
	}

	if sum < 1e-10 {
		t.Error("expected non-zero output from reverb")
	}
}

// TestChainIntegrationLinear tests a full chain with real effects.
func TestChainIntegrationLinear(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()
	c := New(ctx, reg)

	graph := buildGraphJSON(
		[]graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "comp", Type: "dyn-compressor", Params: map[string]any{
				"threshold": -20.0, "ratio": 4.0, "attackMs": 10.0, "releaseMs": 100.0,
			}},
			{ID: "_output", Type: "_output"},
		},
		[]graphConnection{
			{From: "_input", To: "comp"},
			{From: "comp", To: "_output"},
		},
	)

	err := c.LoadGraph(graph)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	block := make([]float64, 512)
	for i := range block {
		block[i] = 0.8 * math.Sin(2*math.Pi*1000*float64(i)/ctx.SampleRate)
	}

	if !c.Process(block) {
		t.Fatal("Process returned false")
	}

	for i, v := range block {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("block[%d] = %v", i, v)
		}
	}
}

// TestChainIntegrationSplitFreq tests a split-freq topology with real crossover.
func TestChainIntegrationSplitFreq(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()
	c := New(ctx, reg)

	graph := buildGraphJSON(
		[]graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "xo", Type: "split-freq", Params: map[string]any{"freqHz": 1000.0}},
			{ID: "low_comp", Type: "dyn-compressor", Params: map[string]any{
				"threshold": -10.0, "ratio": 2.0, "attackMs": 5.0, "releaseMs": 50.0,
			}},
			{ID: "hi_comp", Type: "dyn-compressor", Params: map[string]any{
				"threshold": -10.0, "ratio": 2.0, "attackMs": 5.0, "releaseMs": 50.0,
			}},
			{ID: "sum", Type: "sum"},
			{ID: "_output", Type: "_output"},
		},
		[]graphConnection{
			{From: "_input", To: "xo"},
			{From: "xo", To: "low_comp", FromPortIndex: 0},
			{From: "xo", To: "hi_comp", FromPortIndex: 1},
			{From: "low_comp", To: "sum"},
			{From: "hi_comp", To: "sum"},
			{From: "sum", To: "_output"},
		},
	)

	err := c.LoadGraph(graph)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	block := make([]float64, 512)
	for i := range block {
		block[i] = 0.5*math.Sin(2*math.Pi*200*float64(i)/ctx.SampleRate) +
			0.3*math.Sin(2*math.Pi*5000*float64(i)/ctx.SampleRate)
	}

	if !c.Process(block) {
		t.Fatal("Process returned false")
	}

	for i, v := range block {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("block[%d] = %v", i, v)
		}
	}
}

// TestFilterRuntimeVariants exercises different filter node types.
func TestFilterRuntimeVariants(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	filterTypes := []string{
		"filter", "filter-lowpass", "filter-highpass", "filter-bandpass",
		"filter-notch", "filter-allpass", "filter-peak",
		"filter-lowshelf", "filter-highshelf",
	}

	reg := DefaultRegistry()

	for _, ft := range filterTypes {
		t.Run(ft, func(t *testing.T) {
			t.Parallel()

			factory := reg.Lookup(ft)

			rt, err := factory(ctx)
			if err != nil {
				t.Fatal(err)
			}

			params := Params{
				ID:   "f1",
				Type: ft,
				Num:  map[string]float64{"freq": 1000, "q": 1.0, "gain": 0, "order": 2},
				Str:  map[string]string{"family": "rbj"},
			}

			err = rt.Configure(ctx, params)
			if err != nil {
				t.Fatalf("Configure: %v", err)
			}

			block := make([]float64, 128)
			for i := range block {
				block[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/ctx.SampleRate)
			}

			rt.Process(block)

			for i, v := range block {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("block[%d] = %v", i, v)
				}
			}
		})
	}
}

// TestChainMultipleProcessCalls verifies stateful processing across calls.
func TestChainMultipleProcessCalls(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()
	c := New(ctx, reg)

	graph := buildGraphJSON(
		[]graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "del", Type: "delay-simple", Params: map[string]any{
				"delaySamples": 10.0, "feedback": 0.3, "mix": 0.5,
			}},
			{ID: "_output", Type: "_output"},
		},
		[]graphConnection{
			{From: "_input", To: "del"},
			{From: "del", To: "_output"},
		},
	)

	err := c.LoadGraph(graph)
	if err != nil {
		t.Fatal(err)
	}

	// Process multiple blocks to exercise stateful delay.
	for call := range 5 {
		block := make([]float64, 64)
		if call == 0 {
			block[0] = 1.0 // impulse
		}

		if !c.Process(block) {
			t.Fatalf("Process returned false on call %d", call)
		}

		for i, v := range block {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("call %d: block[%d] = %v", call, i, v)
			}
		}
	}
}

// TestChainGraphReload verifies runtime reuse when node type doesn't change.
func TestChainGraphReload(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()
	c := New(ctx, reg)

	makeGraph := func(gain float64) string {
		return buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "comp", Type: "dyn-compressor", Params: map[string]any{
					"threshold": gain, "ratio": 4.0, "attackMs": 10.0, "releaseMs": 100.0,
				}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "comp"},
				{From: "comp", To: "_output"},
			},
		)
	}

	err := c.LoadGraph(makeGraph(-20))
	if err != nil {
		t.Fatal(err)
	}

	rt1 := c.NodeRuntime("comp")

	// Reload with different params but same type - runtime should be reused.
	err = c.LoadGraph(makeGraph(-10))
	if err != nil {
		t.Fatal(err)
	}

	rt2 := c.NodeRuntime("comp")

	if rt1 != rt2 {
		t.Error("expected runtime reuse when type doesn't change")
	}
}

// TestChainGraphReloadChangedType verifies new runtime when type changes.
func TestChainGraphReloadChangedType(t *testing.T) {
	t.Parallel()

	ctx := Context{SampleRate: 44100}
	reg := DefaultRegistry()
	c := New(ctx, reg)

	graph1, err := json.Marshal(graphState{
		Nodes: []graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "fx", Type: "dyn-compressor", Params: map[string]any{"threshold": -20.0, "ratio": 4.0}},
			{ID: "_output", Type: "_output"},
		},
		Connections: []graphConnection{
			{From: "_input", To: "fx"},
			{From: "fx", To: "_output"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal graph1: %v", err)
	}

	graph2, err := json.Marshal(graphState{
		Nodes: []graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "fx", Type: "dyn-limiter", Params: map[string]any{"threshold": -3.0}},
			{ID: "_output", Type: "_output"},
		},
		Connections: []graphConnection{
			{From: "_input", To: "fx"},
			{From: "fx", To: "_output"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal graph2: %v", err)
	}

	_ = c.LoadGraph(string(graph1))
	rt1 := c.NodeRuntime("fx")

	_ = c.LoadGraph(string(graph2))
	rt2 := c.NodeRuntime("fx")

	if rt1 == rt2 {
		t.Error("expected new runtime when type changes")
	}
}
