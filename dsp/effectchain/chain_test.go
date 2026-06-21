package effectchain

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

const testSampleRate = 44100.0

func testCtx() Context {
	return Context{SampleRate: testSampleRate}
}

// buildGraphJSON is a helper to construct valid JSON graphs for testing.
func buildGraphJSON(nodes []graphNode, connections []graphConnection) string {
	data, err := json.Marshal(graphState{Nodes: nodes, Connections: connections})
	if err != nil {
		panic(err)
	}

	return string(data)
}

func TestChainNew(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	c := New(testCtx(), reg)

	if c == nil {
		t.Fatal("New returned nil")
	}

	if c.HasGraph() {
		t.Error("new chain should not have graph")
	}

	if c.Context().SampleRate != testSampleRate {
		t.Errorf("expected sample rate %v, got %v", testSampleRate, c.Context().SampleRate)
	}
}

func TestChainSetContext(t *testing.T) {
	t.Parallel()

	c := New(testCtx(), testRegistry())
	c.SetContext(Context{SampleRate: 96000})

	if c.Context().SampleRate != 96000 {
		t.Errorf("expected 96000, got %v", c.Context().SampleRate)
	}
}

func TestChainLoadGraph(t *testing.T) {
	t.Parallel()

	t.Run("empty string clears graph", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		err := c.LoadGraph("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if c.HasGraph() {
			t.Error("empty string should result in no valid graph")
		}
	})

	t.Run("valid minimal graph", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !c.HasGraph() {
			t.Error("expected graph to be loaded")
		}
	})

	t.Run("graph with effect node", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if c.NodeRuntime("g1") == nil {
			t.Error("expected runtime for g1")
		}
	})

	t.Run("unknown effect type is silently skipped", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "fx1", Type: "nonexistent_effect"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "fx1"},
				{From: "fx1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if c.NodeRuntime("fx1") != nil {
			t.Error("expected no runtime for unknown effect")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		err := c.LoadGraph("{bad json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("cyclic graph returns error", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "a", Type: "stub"},
				{ID: "b", Type: "stub"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "a"},
				{From: "a", To: "b"},
				{From: "b", To: "a"},
				{From: "b", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err == nil {
			t.Fatal("expected error for cyclic graph")
		}
	})

	t.Run("reload graph removes stale nodes", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		// Load graph with gain node.
		graph1 := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if c.NodeRuntime("g1") == nil {
			t.Fatal("expected g1 runtime after first load")
		}

		// Reload without the gain node.
		graph2 := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "_output"},
			},
		)

		err = c.LoadGraph(graph2)
		if err != nil {
			t.Fatalf("unexpected error on reload: %v", err)
		}

		if c.NodeRuntime("g1") != nil {
			t.Error("expected g1 runtime to be removed after reload")
		}
	})
}

func TestChainReset(t *testing.T) {
	t.Parallel()

	c := New(testCtx(), testRegistry())

	graph := buildGraphJSON(
		[]graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "g1", Type: "gain"},
			{ID: "_output", Type: "_output"},
		},
		[]graphConnection{
			{From: "_input", To: "g1"},
			{From: "g1", To: "_output"},
		},
	)

	_ = c.LoadGraph(graph)
	c.Reset()

	if c.HasGraph() {
		t.Error("expected no graph after reset")
	}

	if c.NodeRuntime("g1") != nil {
		t.Error("expected no runtimes after reset")
	}
}

func TestChainProcess(t *testing.T) { //nolint:gocyclo // table-driven subtests inflate cyclomatic complexity
	t.Parallel()

	t.Run("returns false without graph", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())
		block := []float64{1, 2, 3}

		if c.Process(block) {
			t.Error("expected false without graph")
		}
	})

	t.Run("returns true for empty block", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		if !c.Process(nil) {
			t.Error("expected true for empty block")
		}
	})

	t.Run("passthrough input-output", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3, 4}
		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		expected := []float64{1, 2, 3, 4}
		for i, v := range block {
			if v != expected[i] {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("single gain effect doubles signal", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3, 4}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		expected := []float64{2, 4, 6, 8}
		for i, v := range block {
			if v != expected[i] {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("serial chain of two effects", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "g2", Type: "gain", Params: map[string]any{"gain": 3.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "g2"},
				{From: "g2", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		// 1*2*3=6, 2*2*3=12, 3*2*3=18
		expected := []float64{6, 12, 18}
		for i, v := range block {
			if v != expected[i] {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("bypassed node passes through", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Bypassed: true, Params: map[string]any{"gain": 100.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		// Bypassed: signal should pass through unchanged.
		expected := []float64{1, 2, 3}
		for i, v := range block {
			if v != expected[i] {
				t.Errorf("block[%d] = %v, want %v (bypass should pass through)", i, v, expected[i])
			}
		}
	})

	t.Run("multi-parent mixing averages inputs", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		// Two parallel gain nodes feeding into output:
		// _input -> g1(gain=2) -> _output
		// _input -> g2(gain=4) -> _output
		// Output should be average: (input*2 + input*4) / 2 = input*3
		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "g2", Type: "gain", Params: map[string]any{"gain": 4.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "_input", To: "g2"},
				{From: "g1", To: "_output"},
				{From: "g2", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		expected := []float64{3, 6, 9}
		for i, v := range block {
			if math.Abs(v-expected[i]) > 1e-10 {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("structural split node passes through", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		// _input -> split -> g1(gain=2) -> _output
		//                 -> g2(gain=4) -> _output
		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "s1", Type: "split"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "g2", Type: "gain", Params: map[string]any{"gain": 4.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "s1"},
				{From: "s1", To: "g1"},
				{From: "s1", To: "g2"},
				{From: "g1", To: "_output"},
				{From: "g2", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		// Average of (1*2 + 1*4)/2 = 3, etc.
		expected := []float64{3, 6, 9}
		for i, v := range block {
			if math.Abs(v-expected[i]) > 1e-10 {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})

	t.Run("repeated process calls reuse buffers", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for i := range 5 {
			block := []float64{1, 2, 3}
			if !c.Process(block) {
				t.Fatalf("Process returned false on iteration %d", i)
			}

			for j, v := range block {
				if v != float64((j+1)*2) {
					t.Errorf("iteration %d: block[%d] = %v, want %v", i, j, v, float64((j+1)*2))
				}
			}
		}
	})
}

func TestChainProcessSidechain(t *testing.T) {
	t.Parallel()

	t.Run("lookahead limiter with sidechain", func(t *testing.T) {
		t.Parallel()

		reg := testRegistryWithSidechain()
		c := New(testCtx(), reg)

		// _input -> gain(2x) -> dyn-lookahead -> _output
		//                                 ^
		// _input -> gain(0.5x) ----------/  (sidechain on port 1)
		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "main", Type: "gain", Params: map[string]any{"gain": 2.0}},
				{ID: "side", Type: "gain", Params: map[string]any{"gain": 0.5}},
				{ID: "lim", Type: "dyn-lookahead"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "main"},
				{From: "_input", To: "side"},
				{From: "main", To: "lim", ToPortIndex: 0},
				{From: "side", To: "lim", ToPortIndex: 1},
				{From: "lim", To: "_output"},
			},
		)

		err := c.LoadGraph(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		block := []float64{1, 2, 3, 4}

		if !c.Process(block) {
			t.Fatal("Process returned false")
		}

		// The sidechainStubRuntime mixes: main * 0.5 + side * 0.5
		// main = input * 2 = [2, 4, 6, 8]
		// side = input * 0.5 = [0.5, 1, 1.5, 2]
		// result = [2*0.5+0.5*0.5, 4*0.5+1*0.5, 6*0.5+1.5*0.5, 8*0.5+2*0.5]
		//        = [1.25, 2.5, 3.75, 5]
		expected := []float64{1.25, 2.5, 3.75, 5}
		for i, v := range block {
			if math.Abs(v-expected[i]) > 1e-10 {
				t.Errorf("block[%d] = %v, want %v", i, v, expected[i])
			}
		}
	})
}

func TestChainNodeRuntime(t *testing.T) {
	t.Parallel()

	t.Run("returns nil for unknown node", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		if c.NodeRuntime("nonexistent") != nil {
			t.Error("expected nil for unknown node")
		}
	})

	t.Run("returns runtime for loaded node", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "g1", Type: "gain"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "g1"},
				{From: "g1", To: "_output"},
			},
		)

		_ = c.LoadGraph(graph)

		if c.NodeRuntime("g1") == nil {
			t.Error("expected non-nil runtime for g1")
		}
	})

	t.Run("returns nil for structural nodes", func(t *testing.T) {
		t.Parallel()

		c := New(testCtx(), testRegistry())

		graph := buildGraphJSON(
			[]graphNode{
				{ID: "_input", Type: "_input"},
				{ID: "_output", Type: "_output"},
			},
			[]graphConnection{
				{From: "_input", To: "_output"},
			},
		)

		_ = c.LoadGraph(graph)

		if c.NodeRuntime("_input") != nil {
			t.Error("expected nil for structural node _input")
		}
	})
}

func TestChainConfigureError(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	testErr := errors.New("configure failed")

	reg.MustRegister("failing", func(_ Context) (Runtime, error) {
		return &stubRuntime{configureErr: testErr}, nil
	})

	c := New(testCtx(), reg)

	graph := buildGraphJSON(
		[]graphNode{
			{ID: "_input", Type: "_input"},
			{ID: "f1", Type: "failing"},
			{ID: "_output", Type: "_output"},
		},
		[]graphConnection{
			{From: "_input", To: "f1"},
			{From: "f1", To: "_output"},
		},
	)

	err := c.LoadGraph(graph)
	if !errors.Is(err, testErr) {
		t.Errorf("expected configure error, got: %v", err)
	}
}
