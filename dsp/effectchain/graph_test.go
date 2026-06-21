package effectchain

import (
	"strings"
	"testing"
)

func TestParseGraph(t *testing.T) { //nolint:gocyclo // table-driven subtests inflate cyclomatic complexity
	t.Parallel()

	t.Run("empty string returns empty graph", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 0 {
			t.Errorf("expected empty nodes, got %d", len(g.Nodes))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		_, err := parseGraph("{not-json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}

		if !strings.Contains(err.Error(), "invalid chain graph json") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing input node returns empty graph", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [{"id": "_output", "type": "_output"}],
			"connections": []
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 0 {
			t.Errorf("expected empty graph without input node, got %d nodes", len(g.Nodes))
		}
	})

	t.Run("missing output node returns empty graph", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [{"id": "_input", "type": "_input"}],
			"connections": []
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 0 {
			t.Errorf("expected empty graph without output node, got %d nodes", len(g.Nodes))
		}
	})

	t.Run("minimal input-output graph", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
		}

		if len(g.Order) != 2 {
			t.Fatalf("expected 2 nodes in order, got %d", len(g.Order))
		}

		// _input must come before _output.
		if g.Order[0] != InputNodeID {
			t.Errorf("expected _input first in order, got %s", g.Order[0])
		}

		if g.Order[1] != OutputNodeID {
			t.Errorf("expected _output last in order, got %s", g.Order[1])
		}
	})

	t.Run("linear chain preserves topological order", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "fx1", "type": "chorus"},
				{"id": "fx2", "type": "delay"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "fx1"},
				{"from": "fx1", "to": "fx2"},
				{"from": "fx2", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Order) != 4 {
			t.Fatalf("expected 4 nodes in order, got %d", len(g.Order))
		}

		// Verify topological constraints: each node must come after its parents.
		pos := map[string]int{}
		for i, id := range g.Order {
			pos[id] = i
		}

		if pos["_input"] >= pos["fx1"] {
			t.Error("_input must come before fx1")
		}

		if pos["fx1"] >= pos["fx2"] {
			t.Error("fx1 must come before fx2")
		}

		if pos["fx2"] >= pos["_output"] {
			t.Error("fx2 must come before _output")
		}
	})

	t.Run("cycle detection", func(t *testing.T) {
		t.Parallel()

		_, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "a", "type": "chorus"},
				{"id": "b", "type": "delay"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "a"},
				{"from": "a", "to": "b"},
				{"from": "b", "to": "a"},
				{"from": "b", "to": "_output"}
			]
		}`)
		if err == nil {
			t.Fatal("expected error for cyclic graph")
		}

		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("expected cycle error, got: %v", err)
		}
	})

	t.Run("self-loop connection is ignored", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "_input"},
				{"from": "_input", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
		}
	})

	t.Run("nodes with empty id or type are skipped", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "", "type": "chorus"},
				{"id": "fx1", "type": ""},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(g.Nodes) != 2 {
			t.Errorf("expected 2 valid nodes, got %d", len(g.Nodes))
		}
	})

	t.Run("connections to unknown nodes are skipped", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "_output"},
				{"from": "_input", "to": "nonexistent"},
				{"from": "nonexistent", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Only the valid connection should exist.
		if len(g.Outgoing[InputNodeID]) != 1 {
			t.Errorf("expected 1 outgoing edge from _input, got %d", len(g.Outgoing[InputNodeID]))
		}
	})

	t.Run("port indices are parsed", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "split", "type": "split-freq"},
				{"id": "fx1", "type": "chorus"},
				{"id": "fx2", "type": "delay"},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "split"},
				{"from": "split", "to": "fx1", "fromPortIndex": 0},
				{"from": "split", "to": "fx2", "fromPortIndex": 1},
				{"from": "fx1", "to": "_output"},
				{"from": "fx2", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		edges := g.Outgoing["split"]
		if len(edges) != 2 {
			t.Fatalf("expected 2 edges from split, got %d", len(edges))
		}

		portIndices := map[int]bool{}
		for _, e := range edges {
			portIndices[e.FromPortIndex] = true
		}

		if !portIndices[0] || !portIndices[1] {
			t.Error("expected port indices 0 and 1")
		}
	})

	t.Run("bypassed flag is parsed", func(t *testing.T) {
		t.Parallel()

		g, err := parseGraph(`{
			"nodes": [
				{"id": "_input", "type": "_input"},
				{"id": "fx1", "type": "chorus", "bypassed": true},
				{"id": "_output", "type": "_output"}
			],
			"connections": [
				{"from": "_input", "to": "fx1"},
				{"from": "fx1", "to": "_output"}
			]
		}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		node := g.Nodes["fx1"]
		if !node.Bypassed {
			t.Error("expected fx1 to be bypassed")
		}
	})
}

func TestParseNodeParams(t *testing.T) {
	t.Parallel()

	t.Run("extracts numeric values", func(t *testing.T) {
		t.Parallel()

		raw := map[string]any{"gain": 0.5, "freq": 1200.0}
		num, str := parseNodeParams(raw)

		if num["gain"] != 0.5 {
			t.Errorf("expected gain=0.5, got %v", num["gain"])
		}

		if num["freq"] != 1200.0 {
			t.Errorf("expected freq=1200, got %v", num["freq"])
		}

		if len(str) != 0 {
			t.Errorf("expected no string params, got %d", len(str))
		}
	})

	t.Run("extracts string values", func(t *testing.T) {
		t.Parallel()

		raw := map[string]any{"model": "fdn", "family": "rbj"}
		num, str := parseNodeParams(raw)

		if str["model"] != "fdn" {
			t.Errorf("expected model=fdn, got %v", str["model"])
		}

		if str["family"] != "rbj" {
			t.Errorf("expected family=rbj, got %v", str["family"])
		}

		if len(num) != 0 {
			t.Errorf("expected no numeric params, got %d", len(num))
		}
	})

	t.Run("bool true becomes 1", func(t *testing.T) {
		t.Parallel()

		raw := map[string]any{"enabled": true}
		num, _ := parseNodeParams(raw)

		if num["enabled"] != 1 {
			t.Errorf("expected enabled=1, got %v", num["enabled"])
		}
	})

	t.Run("bool false becomes 0", func(t *testing.T) {
		t.Parallel()

		raw := map[string]any{"enabled": false}
		num, _ := parseNodeParams(raw)

		if num["enabled"] != 0 {
			t.Errorf("expected enabled=0, got %v", num["enabled"])
		}
	})

	t.Run("nil raw returns empty maps", func(t *testing.T) {
		t.Parallel()

		num, str := parseNodeParams(nil)
		if len(num) != 0 || len(str) != 0 {
			t.Error("expected empty maps for nil input")
		}
	})

	t.Run("integer types are converted to float64", func(t *testing.T) {
		t.Parallel()

		raw := map[string]any{"order": int(4), "big": int64(8)}
		num, _ := parseNodeParams(raw)

		if num["order"] != 4.0 {
			t.Errorf("expected order=4, got %v", num["order"])
		}

		if num["big"] != 8.0 {
			t.Errorf("expected big=8, got %v", num["big"])
		}
	})
}

func TestIsStructuralNodeType(t *testing.T) {
	t.Parallel()

	structural := []string{"_input", "_output", "split", "sum", "split-freq"}
	for _, nt := range structural {
		if !isStructuralNodeType(nt) {
			t.Errorf("expected %q to be structural", nt)
		}
	}

	nonStructural := []string{"chorus", "delay", "reverb", "filter", ""}
	for _, nt := range nonStructural {
		if isStructuralNodeType(nt) {
			t.Errorf("expected %q to not be structural", nt)
		}
	}
}

func TestIsPassthroughNodeType(t *testing.T) {
	t.Parallel()

	passthrough := []string{"_input", "_output", "split", "sum", "split-freq"}
	for _, nt := range passthrough {
		if !isPassthroughNodeType(nt) {
			t.Errorf("expected %q to be passthrough", nt)
		}
	}

	nonPassthrough := []string{"chorus", "delay", "reverb"}
	for _, nt := range nonPassthrough {
		if isPassthroughNodeType(nt) {
			t.Errorf("expected %q to not be passthrough", nt)
		}
	}
}
