package effectchain

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// InputNodeID is the reserved node ID for the chain input.
	InputNodeID = "_input"
	// OutputNodeID is the reserved node ID for the chain output.
	OutputNodeID = "_output"

	NodeTypeSplitFreq = "split-freq"
)

// graphNode is a JSON-serializable node in the effect chain graph.
type graphNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Bypassed bool   `json:"bypassed"`
	Fixed    bool   `json:"fixed"`
	Params   any    `json:"params"`
}

// graphConnection is a JSON-serializable connection between two graph nodes.
type graphConnection struct {
	From          string `json:"from"`
	To            string `json:"to"`
	FromPortIndex int    `json:"fromPortIndex,omitempty"` //nolint:tagliatelle
	ToPortIndex   int    `json:"toPortIndex,omitempty"`   //nolint:tagliatelle
}

// graphState is the root JSON structure for the effect chain graph.
type graphState struct {
	Nodes       []graphNode       `json:"nodes"`
	Connections []graphConnection `json:"connections"`
}

// compiledGraph holds the compiled effect chain graph with adjacency info
// and a topologically sorted traversal order.
type compiledGraph struct {
	Nodes    map[string]Params
	Incoming map[string][]compiledEdge
	Outgoing map[string][]compiledEdge
	Order    []string
}

type compiledEdge struct {
	From          string
	To            string
	FromPortIndex int
	ToPortIndex   int
}

// parseGraph parses the JSON chain graph and performs a topological sort
// (Kahn's algorithm). Returns an empty graph for an empty string.
func parseGraph(raw string) (*compiledGraph, error) {
	if raw == "" {
		return &compiledGraph{}, nil
	}

	var state graphState

	err := json.Unmarshal([]byte(raw), &state)
	if err != nil {
		return nil, fmt.Errorf("invalid chain graph json: %w", err)
	}

	nodes := make(map[string]Params, len(state.Nodes))
	for _, n := range state.Nodes {
		if n.ID == "" || n.Type == "" {
			continue
		}

		num, str := parseNodeParams(n.Params)
		nodes[n.ID] = Params{
			ID:       n.ID,
			Type:     n.Type,
			Bypassed: n.Bypassed,
			Num:      num,
			Str:      str,
		}
	}

	if _, ok := nodes[InputNodeID]; !ok {
		return &compiledGraph{}, nil
	}

	if _, ok := nodes[OutputNodeID]; !ok {
		return &compiledGraph{}, nil
	}

	incoming := make(map[string][]compiledEdge, len(nodes))
	outgoing := make(map[string][]compiledEdge, len(nodes))

	indegree := make(map[string]int, len(nodes))
	for id := range nodes {
		incoming[id] = nil
		outgoing[id] = nil
		indegree[id] = 0
	}

	for _, c := range state.Connections {
		if c.From == "" || c.To == "" || c.From == c.To {
			continue
		}

		if _, ok := nodes[c.From]; !ok {
			continue
		}

		if _, ok := nodes[c.To]; !ok {
			continue
		}

		edge := compiledEdge{
			From: c.From,
			To:   c.To,
		}
		if c.FromPortIndex >= 0 {
			edge.FromPortIndex = c.FromPortIndex
		}

		if c.ToPortIndex >= 0 {
			edge.ToPortIndex = c.ToPortIndex
		}

		outgoing[c.From] = append(outgoing[c.From], edge)
		incoming[c.To] = append(incoming[c.To], edge)
		indegree[c.To]++
	}

	queue := make([]string, 0, len(nodes))

	for id, d := range indegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		order = append(order, id)
		for _, edge := range outgoing[id] {
			indegree[edge.To]--
			if indegree[edge.To] == 0 {
				queue = append(queue, edge.To)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, errors.New("invalid chain graph: contains cycle")
	}

	return &compiledGraph{
		Nodes:    nodes,
		Incoming: incoming,
		Outgoing: outgoing,
		Order:    order,
	}, nil
}

// parseNodeParams extracts numeric and string parameters from a raw JSON params value.
func parseNodeParams(raw any) (map[string]float64, map[string]string) {
	num := map[string]float64{}
	str := map[string]string{}

	params, ok := raw.(map[string]any)
	if !ok || params == nil {
		return num, str
	}

	for k, v := range params {
		switch t := v.(type) {
		case float64:
			num[k] = t
		case float32:
			num[k] = float64(t)
		case int:
			num[k] = float64(t)
		case int64:
			num[k] = float64(t)
		case string:
			str[k] = t
		case bool:
			if t {
				num[k] = 1
			} else {
				num[k] = 0
			}
		}
	}

	return num, str
}

// isStructuralNodeType returns true for I/O and routing nodes that don't need a runtime.
func isStructuralNodeType(nodeType string) bool {
	return nodeType == InputNodeID ||
		nodeType == OutputNodeID ||
		nodeType == "split" ||
		nodeType == "sum" ||
		nodeType == NodeTypeSplitFreq
}

// isPassthroughNodeType returns true for nodes that don't transform audio.
func isPassthroughNodeType(nodeType string) bool {
	return nodeType == "split" || nodeType == NodeTypeSplitFreq || nodeType == "sum" || nodeType == InputNodeID || nodeType == OutputNodeID
}
