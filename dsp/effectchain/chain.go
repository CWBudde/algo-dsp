package effectchain

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

// ErrUnknownEffect is returned when a node references an unregistered effect type.
var ErrUnknownEffect = errors.New("unknown effect type")

type nodeRuntime struct {
	effectType string
	runtime    Runtime
}

// Chain owns a graph-based effect chain: topology, node runtimes,
// processing buffers, and crossovers. It is independent of any application engine.
type Chain struct {
	ctx      Context
	registry *Registry

	graph *compiledGraph
	nodes map[string]*nodeRuntime

	outBuf       map[string][]float64
	splitLowBuf  map[string][]float64
	splitHighBuf map[string][]float64
	crossovers   map[string]*crossover.Crossover
	mixBuf       []float64
}

// New creates a Chain with the given context and registry.
func New(ctx Context, registry *Registry) *Chain {
	return &Chain{
		ctx:      ctx,
		registry: registry,
		nodes:    make(map[string]*nodeRuntime),
	}
}

// SetContext updates the chain context (e.g., after sample rate change).
func (c *Chain) SetContext(ctx Context) {
	c.ctx = ctx
}

// Context returns the current chain context.
func (c *Chain) Context() Context {
	return c.ctx
}

// HasGraph returns true if the chain has a loaded graph with valid I/O nodes.
func (c *Chain) HasGraph() bool {
	return c.graph != nil && hasRequiredIONodes(c.graph)
}

// LoadGraph parses a JSON graph string, compiles the topology, and
// synchronizes node runtimes. An empty string clears the graph.
func (c *Chain) LoadGraph(jsonGraph string) error {
	graph, err := parseGraph(jsonGraph)
	if err != nil {
		return err
	}

	err = c.syncNodes(graph)
	if err != nil {
		return err
	}

	c.graph = graph

	return nil
}

// Reset clears all node runtimes and processing state.
func (c *Chain) Reset() {
	c.graph = nil
	c.nodes = make(map[string]*nodeRuntime)
	c.crossovers = nil
	c.outBuf = nil
	c.splitLowBuf = nil
	c.splitHighBuf = nil
	c.mixBuf = nil
}

// syncNodes synchronises runtime effect instances with the compiled graph topology.
// Nodes that are no longer present are removed; new or type-changed nodes are (re)created and configured.
//
//nolint:cyclop
func (c *Chain) syncNodes(graph *compiledGraph) error {
	if graph == nil {
		c.nodes = nil
		c.crossovers = nil

		return nil
	}

	if c.nodes == nil {
		c.nodes = map[string]*nodeRuntime{}
	}

	seen := map[string]struct{}{}
	seenCrossover := map[string]struct{}{}

	for _, node := range graph.Nodes {
		if isStructuralNodeType(node.Type) {
			if node.Type == NodeTypeSplitFreq {
				seenCrossover[node.ID] = struct{}{}
			}

			continue
		}

		seen[node.ID] = struct{}{}

		rt := c.nodes[node.ID]
		if rt == nil || rt.effectType != node.Type {
			runtime, err := c.newRuntime(node.Type)
			if err != nil {
				if errors.Is(err, ErrUnknownEffect) {
					continue
				}

				return err
			}

			if runtime == nil {
				continue
			}

			rt = &nodeRuntime{effectType: node.Type, runtime: runtime}
			c.nodes[node.ID] = rt
		}

		err := rt.runtime.Configure(c.ctx, node)
		if err != nil {
			return fmt.Errorf("effectchain: configure node %q (%s): %w", node.ID, node.Type, err)
		}
	}

	for id := range c.nodes {
		if _, ok := seen[id]; !ok {
			delete(c.nodes, id)
		}
	}

	for id := range c.crossovers {
		if _, ok := seenCrossover[id]; !ok {
			delete(c.crossovers, id)
		}
	}

	return nil
}

func (c *Chain) newRuntime(effectType string) (Runtime, error) {
	factory := c.registry.Lookup(effectType)
	if factory == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEffect, effectType)
	}

	return factory(c.ctx)
}

func hasRequiredIONodes(g *compiledGraph) bool {
	if g == nil {
		return false
	}

	if _, ok := g.Nodes[InputNodeID]; !ok {
		return false
	}

	if _, ok := g.Nodes[OutputNodeID]; !ok {
		return false
	}

	return true
}
