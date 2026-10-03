package effectchain

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

// StereoProcessor processes an adjacent pair without folding either channel.
type StereoProcessor interface{ ProcessStereo(left, right []float64) }

// RuntimePreparer reserves storage without processing audio or advancing state.
type RuntimePreparer interface{ Prepare(maxFrames int) error }

// RuntimeResetter clears processing state while retaining configuration/storage.
type RuntimeResetter interface{ Reset() }

// Prepare reserves mono render buffers and effect scratch storage without
// advancing any processor. Later Process calls up to maxFrames allocate no
// storage for built-in runtimes. Call again after loading a changed graph.
func (c *Chain) Prepare(maxFrames int) error {
	if maxFrames < 1 || maxFrames > 1<<20 {
		return errors.New("effectchain: invalid maximum block size")
	}

	if !c.HasGraph() {
		return errors.New("effectchain: no graph to prepare")
	}

	dummy := make([]float64, maxFrames)
	c.prepareBuffers(dummy, c.graph)
	delete(c.outBuf, InputNodeID)
	c.prepareRoutingDelays(maxFrames)

	for id, node := range c.graph.Nodes {
		if node.Type == NodeTypeSplitFreq {
			freq := clamp(node.GetNum("freqHz", 1200), 20, c.ctx.SampleRate*0.475)
			if current := c.crossovers[id]; current != nil && current.Freq() == freq {
				continue
			}

			xo, err := crossover.New(freq, 4, c.ctx.SampleRate)
			if err != nil {
				return fmt.Errorf("effectchain: prepare crossover: %w", err)
			}

			if c.crossovers == nil {
				c.crossovers = map[string]*crossover.Crossover{}
			}

			c.crossovers[id] = xo
		}
	}

	for id, node := range c.nodes {
		if p, ok := node.runtime.(RuntimePreparer); ok {
			if err := p.Prepare(maxFrames); err != nil {
				return fmt.Errorf("effectchain: prepare node %q: %w", id, err)
			}
		}
	}

	c.preparedFrames = maxFrames

	return nil
}

// PreparePlanar reserves independent mono state for each channel and true
// stereo state for every adjacent pair. Stereo nodes require an even channel
// count. Allocation and configuration happen here, never in ProcessPlanar.
func (c *Chain) PreparePlanar(channels, maxFrames int) error {
	if channels < 1 || channels > 8 {
		return errors.New("effectchain: invalid channel count")
	}

	if !c.HasGraph() {
		return errors.New("effectchain: no graph to prepare")
	}

	for _, rt := range c.nodes {
		if convolution, ok := rt.runtime.(*convReverbRuntime); ok && len(convolution.sourceChannelMap) > 0 && len(convolution.sourceChannelMap) != channels {
			return fmt.Errorf("effectchain: source channel map length %d differs from prepared channels %d", len(convolution.sourceChannelMap), channels)
		}
	}

	if channels%2 != 0 {
		for _, rt := range c.nodes {
			if _, ok := rt.runtime.(StereoProcessor); ok {
				return errors.New("effectchain: stereo effect requires complete channel pairs")
			}
		}
	}

	planar := c.planar
	if len(planar) != channels {
		planar = make([]*Chain, channels)
	}

	planar[0] = c
	for ch := 1; ch < channels; ch++ {
		if planar[ch] != nil {
			continue
		}

		other := New(c.ctx, c.registry)
		other.graph = c.graph

		other.channelIndex = ch
		if err := other.syncNodes(c.graph); err != nil {
			return fmt.Errorf("effectchain: prepare channel %d: %w", ch, err)
		}

		other.updateLatency()
		planar[ch] = other
	}

	for _, chain := range planar {
		if err := chain.Prepare(maxFrames); err != nil {
			return err
		}
	}

	c.planar = planar

	return nil
}

// ProcessPlanar processes prepared, equally sized planar channels in place.
// Shape errors are rejected before changing any audio or processor state.
// Adjacent stereo pairs retain independent stereo state; all other processors
// have one independent instance per channel. The block size may vary.
func (c *Chain) ProcessPlanar(block [][]float64) error {
	if len(block) == 0 || len(block) != len(c.planar) {
		return errors.New("effectchain: channels differ from preparation")
	}

	n := len(block[0])
	if n > c.preparedFrames {
		return errors.New("effectchain: block exceeds prepared size")
	}

	for _, channel := range block {
		if len(channel) != n {
			return errors.New("effectchain: unequal channel lengths")
		}
	}

	if n == 0 {
		return nil
	}

	for ch, chain := range c.planar {
		chain.prepareBuffers(block[ch], c.graph)
	}
	defer c.clearPlanarInputs()

	for _, id := range c.graph.Order {
		if id == InputNodeID {
			continue
		}

		node := c.graph.Nodes[id]
		for _, chain := range c.planar {
			dst := chain.outBuf[id]
			src := chain.edgeSource(c.graph, chain.outBuf, chain.splitLowBuf, chain.splitHighBuf)
			mixParentEdgesInto(c.graph.MainIncoming[id], dst, chain.mixBuf[:n], src)
			chain.processSplitFreqNode(id, node, dst, chain.splitLowBuf, chain.splitHighBuf)
		}

		if node.Bypassed || isStructuralNodeType(node.Type) {
			continue
		}

		for ch := 0; ch < len(c.planar); ch++ {
			chain := c.planar[ch]

			rt := chain.nodes[id]
			if rt == nil {
				continue
			}

			if stereo, ok := rt.runtime.(StereoProcessor); ok {
				stereo.ProcessStereo(chain.outBuf[id], c.planar[ch+1].outBuf[id])
				ch++

				continue
			}

			src := chain.edgeSource(c.graph, chain.outBuf, chain.splitLowBuf, chain.splitHighBuf)
			if !chain.processSidechainNode(node, chain.outBuf[id], c.graph.SideIncoming[id], chain.mixBuf[:n], src) {
				if processor, ok := rt.runtime.(ErrorProcessor); ok {
					if err := processor.ProcessWithError(chain.outBuf[id]); err != nil {
						return fmt.Errorf("effectchain: process node %q channel %d: %w", id, ch, err)
					}
				} else {
					rt.runtime.Process(chain.outBuf[id])
				}
			}
		}
	}

	for ch, chain := range c.planar {
		copy(block[ch], chain.outBuf[OutputNodeID])
	}

	return nil
}

func (c *Chain) clearPlanarInputs() {
	for _, chain := range c.planar {
		delete(chain.outBuf, InputNodeID)
	}
}

// ResetProcessing rewinds effects and routing state while retaining the loaded
// graph, parameter configuration and prepared storage. Built-in reset paths
// allocate no memory. Custom runtimes must implement RuntimeResetter.
func (c *Chain) ResetProcessing() error {
	chains := c.planar
	if len(chains) == 0 {
		chains = []*Chain{c}
	}

	for _, chain := range chains {
		for id, rt := range chain.nodes {
			if _, ok := rt.runtime.(RuntimeResetter); !ok {
				return fmt.Errorf("effectchain: node %q does not support reset", id)
			}
		}
	}

	for _, chain := range chains {
		for _, rt := range chain.nodes {
			rt.runtime.(RuntimeResetter).Reset()
		}

		for _, xo := range chain.crossovers {
			xo.Reset()
		}

		for _, buf := range chain.outBuf {
			clear(buf)
		}

		for _, buf := range chain.splitLowBuf {
			clear(buf)
		}

		for _, buf := range chain.splitHighBuf {
			clear(buf)
		}

		clear(chain.mixBuf)

		chain.lastError = nil
		for _, delay := range chain.delayedEdges {
			clear(delay.history)
			clear(delay.output)
			delay.write = 0
			delay.serial = 0
		}

		chain.renderSerial = 0
	}

	return nil
}
