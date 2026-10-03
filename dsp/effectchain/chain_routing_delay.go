package effectchain

type routingDelay struct {
	history, output []float64
	write           int
	serial          uint64
}

func (c *Chain) prepareRoutingDelays(maxFrames int) {
	if c.delayedEdges == nil {
		c.delayedEdges = make(map[compiledEdge]*routingDelay, len(c.edgeCompensation))
	}

	for edge, delay := range c.edgeCompensation {
		current := c.delayedEdges[edge]
		if current == nil || len(current.history) != delay {
			current = &routingDelay{history: make([]float64, delay)}
			c.delayedEdges[edge] = current
		}

		if len(current.output) < maxFrames {
			current.output = make([]float64, maxFrames)
		}
	}

	for edge := range c.delayedEdges {
		if c.edgeCompensation[edge] == 0 {
			delete(c.delayedEdges, edge)
		}
	}
}

func (c *Chain) edgeSource(g *compiledGraph, buffers, low, high map[string][]float64) func(compiledEdge) []float64 {
	raw := graphEdgeSource(g, buffers, low, high)

	return func(edge compiledEdge) []float64 {
		source := raw(edge)

		delay := c.delayedEdges[edge]
		if delay == nil {
			return source
		}

		output := delay.output[:len(source)]
		if delay.serial != c.renderSerial {
			for i, x := range source {
				output[i] = delay.history[delay.write]
				delay.history[delay.write] = x

				delay.write++
				if delay.write == len(delay.history) {
					delay.write = 0
				}
			}

			delay.serial = c.renderSerial
		}

		return output
	}
}
