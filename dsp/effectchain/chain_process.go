package effectchain

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

// Process applies the effect chain to the block in-place.
// Returns false if the chain has no valid graph with I/O nodes.
func (c *Chain) Process(block []float64) bool {
	if len(block) == 0 {
		return true
	}

	g := c.graph
	if g == nil || !hasRequiredIONodes(g) {
		return false
	}

	buffers, splitLow, splitHigh, mixBuf := c.prepareBuffers(block, g)
	edgeSrc := graphEdgeSource(g, buffers, splitLow, splitHigh)

	for _, id := range g.Order {
		if id == InputNodeID {
			continue
		}

		c.processNode(id, g, buffers, splitLow, splitHigh, mixBuf, edgeSrc)
	}

	return copyOutputToBlock(block, buffers)
}

// NodeRuntime returns the Runtime for the given node ID, or nil.
func (c *Chain) NodeRuntime(nodeID string) Runtime {
	rt := c.nodes[nodeID]
	if rt == nil {
		return nil
	}

	return rt.runtime
}

func (c *Chain) prepareBuffers(
	block []float64,
	g *compiledGraph,
) (map[string][]float64, map[string][]float64, map[string][]float64, []float64) {
	if c.outBuf == nil {
		c.outBuf = make(map[string][]float64, len(g.Nodes))
	}

	if c.splitLowBuf == nil {
		c.splitLowBuf = make(map[string][]float64, len(g.Nodes))
	}

	if c.splitHighBuf == nil {
		c.splitHighBuf = make(map[string][]float64, len(g.Nodes))
	}

	if c.mixBuf == nil {
		c.mixBuf = make([]float64, len(block))
	}

	buffers := c.outBuf
	splitLow := c.splitLowBuf
	splitHigh := c.splitHighBuf

	for _, id := range g.Order {
		if id == InputNodeID {
			buffers[id] = block
			continue
		}

		buf := buffers[id]
		if cap(buf) < len(block) {
			buf = make([]float64, len(block))
		}

		buffers[id] = buf[:len(block)]

		low := splitLow[id]
		if cap(low) < len(block) {
			low = make([]float64, len(block))
		}

		splitLow[id] = low[:len(block)]

		high := splitHigh[id]
		if cap(high) < len(block) {
			high = make([]float64, len(block))
		}

		splitHigh[id] = high[:len(block)]
	}

	if len(c.mixBuf) < len(block) {
		c.mixBuf = make([]float64, len(block))
	}

	return buffers, splitLow, splitHigh, c.mixBuf[:len(block)]
}

func graphEdgeSource(
	g *compiledGraph,
	buffers map[string][]float64,
	splitLow map[string][]float64,
	splitHigh map[string][]float64,
) func(edge compiledEdge) []float64 {
	return func(edge compiledEdge) []float64 {
		parentNode := g.Nodes[edge.From]
		if parentNode.Type == NodeTypeSplitFreq {
			if edge.FromPortIndex == 1 {
				return splitHigh[edge.From]
			}

			return splitLow[edge.From]
		}

		return buffers[edge.From]
	}
}

func splitMainAndSideParents(nodeType string, parents []compiledEdge) ([]compiledEdge, []compiledEdge) {
	mainParents := parents
	var sideParents []compiledEdge

	if nodeType != "dyn-lookahead" && nodeType != "vocoder" {
		return mainParents, sideParents
	}

	mainParents = mainParents[:0]

	for _, edge := range parents {
		if edge.ToPortIndex == 1 {
			sideParents = append(sideParents, edge)
			continue
		}

		mainParents = append(mainParents, edge)
	}

	return mainParents, sideParents
}

func (c *Chain) processNode(
	id string,
	g *compiledGraph,
	buffers map[string][]float64,
	splitLow map[string][]float64,
	splitHigh map[string][]float64,
	mixBuf []float64,
	edgeSrc func(edge compiledEdge) []float64,
) {
	node := g.Nodes[id]
	dst := buffers[id]

	parents := g.Incoming[id]
	mainParents, sideParents := splitMainAndSideParents(node.Type, parents)
	mixParentEdgesInto(mainParents, dst, mixBuf, edgeSrc)

	if c.processSplitFreqNode(id, node, dst, splitLow, splitHigh) {
		return
	}

	if id == OutputNodeID || node.Bypassed {
		return
	}

	if c.processSidechainNode(node, dst, sideParents, mixBuf, edgeSrc) {
		return
	}

	c.applyNode(node, dst)
}

func (c *Chain) processSplitFreqNode(
	id string,
	node Params,
	dst []float64,
	splitLow map[string][]float64,
	splitHigh map[string][]float64,
) bool {
	if node.Type != NodeTypeSplitFreq {
		return false
	}

	low := splitLow[id]
	high := splitHigh[id]

	freq := node.GetNum("freqHz", 1200)
	if freq < 20 {
		freq = 20
	}

	nyquist := c.ctx.SampleRate * 0.5

	maxFreq := math.Max(20, nyquist*0.95)
	if freq > maxFreq {
		freq = maxFreq
	}

	xo := c.crossovers[id]
	if xo == nil || math.Abs(xo.Freq()-freq) > 1e-9 {
		newXO, err := crossover.New(freq, 4, c.ctx.SampleRate)
		if err == nil {
			if c.crossovers == nil {
				c.crossovers = map[string]*crossover.Crossover{}
			}

			c.crossovers[id] = newXO
			xo = newXO
		}
	}

	if xo != nil {
		xo.ProcessBlock(dst, low, high)
	} else {
		copy(low, dst)
		copy(high, dst)
	}

	return true
}

// processSidechainNode handles nodes with sidechain inputs (lookahead limiter, vocoder).
func (c *Chain) processSidechainNode(
	node Params,
	dst []float64,
	sideParents []compiledEdge,
	mixBuf []float64,
	edgeSrc func(edge compiledEdge) []float64,
) bool {
	rt := c.nodes[node.ID]
	if rt == nil {
		return false
	}

	sc, ok := rt.runtime.(SidechainProcessor)
	if !ok {
		return false
	}

	// Only process as sidechain if this is a known sidechain node type.
	if node.Type != "dyn-lookahead" && node.Type != "vocoder" {
		return false
	}

	sideBuf := c.mixBuf[:len(dst)]
	mixParentEdgesInto(sideParents, sideBuf, mixBuf, edgeSrc)

	if len(sideParents) == 0 {
		copy(sideBuf, dst)
	}

	sc.ProcessWithSidechain(dst, sideBuf)

	return true
}

func copyOutputToBlock(block []float64, buffers map[string][]float64) bool {
	out := buffers[OutputNodeID]
	if out == nil {
		return false
	}

	copy(block, out)

	return true
}

func (c *Chain) applyNode(node Params, block []float64) {
	if isPassthroughNodeType(node.Type) {
		return
	}

	rt := c.nodes[node.ID]
	if rt == nil || rt.runtime == nil {
		return
	}

	rt.runtime.Process(block)
}

func mixParentEdgesInto(
	parents []compiledEdge,
	dst []float64,
	mixBuf []float64,
	edgeSrc func(edge compiledEdge) []float64,
) {
	if len(parents) == 0 {
		for i := range dst {
			dst[i] = 0
		}

		return
	}

	if len(parents) == 1 {
		copy(dst, edgeSrc(parents[0]))
		return
	}

	for i := range mixBuf {
		mixBuf[i] = 0
	}

	for _, edge := range parents {
		src := edgeSrc(edge)
		for i := range mixBuf {
			mixBuf[i] += src[i]
		}
	}

	scale := 1.0 / float64(len(parents))
	for i := range mixBuf {
		dst[i] = mixBuf[i] * scale
	}
}
