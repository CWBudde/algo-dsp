package effectchain

import (
	"fmt"
	"math"
	"reflect"
)

// TryUpdateGraph atomically updates parameters in a prepared graph while keeping
// unchanged processors and their histories. Connections, node types, bypass,
// convolution IRs must remain unchanged. Changed ordinary nodes must have zero
// algorithmic latency; buffered processors require full replacement. Convolution
// wet levels update in place; changed ordinary effects are constructed and
// prepared privately before replacing only those nodes. Changed nodes start with
// fresh processing state. Unchanged nodes, including long convolution spectra
// and tails, retain state. It returns false without mutation when full graph
// replacement is required, and an error without mutation for failed preparation.
// As with rendering, callers must serialize control and processing operations.
func (c *Chain) TryUpdateGraph(jsonGraph string) (bool, error) {
	graph, err := parseGraph(jsonGraph)
	if err != nil {
		return false, fmt.Errorf("effectchain: update graph: %w", err)
	}

	if c.graph == nil || !hasRequiredIONodes(graph) || c.preparedFrames < 1 || !sameUpdateTopology(c.graph, graph) {
		return false, nil
	}

	changed := make([]string, 0)

	for id, node := range graph.Nodes {
		previous := c.graph.Nodes[id]
		if reflect.DeepEqual(previous.Num, node.Num) && reflect.DeepEqual(previous.Str, node.Str) {
			continue
		}

		if isStructuralNodeType(node.Type) {
			return false, nil
		}

		if node.Type == "reverb-conv" {
			if !convolutionWetOnly(previous, node) {
				return false, nil
			}

			wet := node.GetNum("wet", .35)
			if math.IsNaN(wet) || math.IsInf(wet, 0) || wet < 0 || wet > 1 {
				return false, fmt.Errorf("effectchain: update convolution wet must be finite in [0,1]")
			}
		}

		changed = append(changed, id)
	}

	chains := c.planar
	if len(chains) == 0 {
		chains = []*Chain{c}
	}

	type replacement struct {
		chain *Chain
		id    string
		node  *nodeRuntime
	}

	type wetChange struct {
		runtime *convReverbRuntime
		wet     float64
	}

	replacements := make([]replacement, 0, len(changed)*len(chains))
	wets := make([]wetChange, 0)

	for _, chain := range chains {
		for _, id := range changed {
			node := graph.Nodes[id]

			current := chain.nodes[id]
			if current == nil || current.runtime == nil {
				return false, fmt.Errorf("effectchain: update missing prepared node %q", id)
			}

			if node.Type == "reverb-conv" {
				convolution, ok := current.runtime.(*convReverbRuntime)
				if !ok || convolution.fx == nil {
					return false, fmt.Errorf("effectchain: update missing prepared convolution %q", id)
				}

				wets = append(wets, wetChange{convolution, node.GetNum("wet", .35)})

				continue
			}

			runtime, err := chain.newRuntime(node.Type)
			if err != nil {
				return false, fmt.Errorf("effectchain: stage update node %q: %w", id, err)
			}

			if runtime == nil {
				return false, fmt.Errorf("effectchain: stage update node %q: missing runtime", id)
			}

			if err := runtime.Configure(chain.ctx, node); err != nil {
				return false, fmt.Errorf("effectchain: configure update node %q: %w", id, err)
			}

			if updateRuntimeLatency(runtime) != 0 || updateRuntimeLatency(current.runtime) != 0 {
				return false, nil
			}

			if preparer, ok := runtime.(RuntimePreparer); ok {
				if err := preparer.Prepare(chain.preparedFrames); err != nil {
					return false, fmt.Errorf("effectchain: prepare update node %q: %w", id, err)
				}
			}

			replacements = append(replacements, replacement{chain, id, &nodeRuntime{effectType: node.Type, runtime: runtime}})
		}
	}
	// Every fallible operation above completed before touching live state.
	for _, change := range wets {
		change.runtime.fx.SetWetDry(change.wet, 1)
	}

	for _, change := range replacements {
		change.chain.nodes[change.id] = change.node
	}

	for _, chain := range chains {
		chain.graph = graph
	}

	return true, nil
}

func sameUpdateTopology(previous, next *compiledGraph) bool {
	if len(previous.Nodes) != len(next.Nodes) || !reflect.DeepEqual(previous.Incoming, next.Incoming) || !reflect.DeepEqual(previous.Outgoing, next.Outgoing) || !reflect.DeepEqual(previous.Order, next.Order) {
		return false
	}

	for id, node := range previous.Nodes {
		other, found := next.Nodes[id]
		if !found || node.Type != other.Type || node.Bypassed != other.Bypassed {
			return false
		}
	}

	return true
}

func convolutionWetOnly(previous, next Params) bool {
	if !reflect.DeepEqual(previous.Str, next.Str) {
		return false
	}

	for id, value := range previous.Num {
		if id != "wet" {
			other, found := next.Num[id]
			if !found || value != other {
				return false
			}
		}
	}

	for id, value := range next.Num {
		if id != "wet" {
			other, found := previous.Num[id]
			if !found || value != other {
				return false
			}
		}
	}

	return true
}

func updateRuntimeLatency(runtime Runtime) int {
	if latency, ok := runtime.(RuntimeLatency); ok {
		return max(latency.Latency(), 0)
	}

	return 0
}
