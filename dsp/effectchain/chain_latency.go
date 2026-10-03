package effectchain

// RuntimeLatency reports algorithmic buffering, excluding intentional effects
// such as musical delay, Haas delay or the impulse response's own leading zeros.
type RuntimeLatency interface{ Latency() int }

// Latency returns the maximum algorithmic buffering delay along any graph path.
// It is computed when the graph is configured and is allocation-free to read.
// Hosts may render immutable source ahead and discard these startup samples,
// then feed zeros to recover the selected-duration tail. Branches with different
// buffering delays are aligned before each sum by Prepare/PreparePlanar.
func (c *Chain) Latency() int { return c.latency }

func (c *Chain) updateLatency() {
	positions := make(map[string]int, len(c.graph.Nodes))
	c.edgeCompensation = make(map[compiledEdge]int)

	c.latency = 0
	for _, id := range c.graph.Order {
		position := 0
		for _, edge := range c.graph.Incoming[id] {
			position = max(position, positions[edge.From])
		}

		for _, edge := range c.graph.Incoming[id] {
			if delay := position - positions[edge.From]; delay > 0 {
				c.edgeCompensation[edge] = delay
			}
		}

		node := c.graph.Nodes[id]
		if !node.Bypassed {
			if runtime := c.NodeRuntime(id); runtime != nil {
				if rt, ok := runtime.(RuntimeLatency); ok {
					position += max(rt.Latency(), 0)
				}
			}
		}

		positions[id] = position
	}

	c.latency = positions[OutputNodeID]
}

func (r *timePitchRuntime) Latency() int      { return r.stream.Latency() }
func (r *spectralPitchRuntime) Latency() int  { return r.stream.Latency() }
func (r *spectralFreezeRuntime) Latency() int { return r.stream.Latency() }
func (r *convReverbRuntime) Latency() int     { return r.fx.Latency() }
func (r *lookaheadLimiterRuntime) Latency() int {
	return r.fx.Latency()
}

// ErrorProcessor is an optional checked render interface. Legacy Runtime.Process
// remains available; prepared chains use this interface when implemented.
type ErrorProcessor interface{ ProcessWithError(block []float64) error }

// Err reports the failure from the most recent mono Process call. ProcessPlanar
// reports errors directly instead. A new mono Process clears the prior error.
func (c *Chain) Err() error { return c.lastError }

func (r *timePitchRuntime) ProcessWithError(block []float64) error {
	return r.stream.ProcessInPlace(block)
}

func (r *spectralPitchRuntime) ProcessWithError(block []float64) error {
	return r.stream.ProcessInPlace(block)
}

func (r *spectralFreezeRuntime) ProcessWithError(block []float64) error {
	return r.stream.ProcessInPlace(block)
}

func (r *convReverbRuntime) ProcessWithError(block []float64) error {
	return r.fx.ProcessInPlace(block)
}
