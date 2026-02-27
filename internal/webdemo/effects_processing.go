package webdemo

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/crossover"
)

// processEffectsInPlace routes to graph-based or legacy serial processing.
func (e *Engine) processEffectsInPlace(block []float64) {
	if len(block) == 0 {
		return
	}

	if e.chainGraph != nil {
		if e.processEffectsByGraphInPlace(block, e.chainGraph) {
			return
		}
	}

	e.processEffectsLegacyInPlace(block)
}

// processEffectsLegacyInPlace applies the fixed-order serial effect chain.
func (e *Engine) processEffectsLegacyInPlace(block []float64) {
	if e.effects.HarmonicBassEnabled {
		e.bass.ProcessInPlace(block)
	}

	if e.effects.DelayEnabled {
		e.delay.ProcessInPlace(block)
	}

	if e.effects.ChorusEnabled {
		e.chorus.ProcessInPlace(block)
	}

	if e.effects.FlangerEnabled {
		_ = e.flanger.ProcessInPlace(block)
	}

	if e.effects.RingModEnabled {
		e.ringMod.ProcessInPlace(block)
	}

	if e.effects.BitCrusherEnabled {
		e.crusher.ProcessInPlace(block)
	}

	if e.effects.WidenerEnabled {
		e.processWidenerMonoInPlace(block)
	}

	if e.effects.PhaserEnabled {
		_ = e.phaser.ProcessInPlace(block)
	}

	if e.effects.TremoloEnabled {
		_ = e.tremolo.ProcessInPlace(block)
	}

	if e.effects.TimePitchEnabled {
		e.tp.ProcessInPlace(block)
	}

	if e.effects.SpectralPitchEnabled {
		e.sp.ProcessInPlace(block)
	}

	if e.effects.ReverbEnabled {
		if e.effects.ReverbModel == "fdn" {
			e.fdn.ProcessInPlace(block)
		} else {
			e.reverb.ProcessInPlace(block)
		}
	}
}

// processEffectsByGraphInPlace traverses the DAG in topological order and applies
// each effect node. Returns false if the graph is missing required I/O nodes.
func (e *Engine) processEffectsByGraphInPlace(block []float64, g *compiledChainGraph) bool {
	if g == nil {
		return false
	}

	const (
		inputID  = "_input"
		outputID = "_output"
	)

	if _, ok := g.Nodes[inputID]; !ok {
		return false
	}

	if _, ok := g.Nodes[outputID]; !ok {
		return false
	}

	if e.chainOutBuf == nil {
		e.chainOutBuf = make(map[string][]float64, len(g.Nodes))
	}

	if e.chainSplitLowBuf == nil {
		e.chainSplitLowBuf = make(map[string][]float64, len(g.Nodes))
	}

	if e.chainSplitHighBuf == nil {
		e.chainSplitHighBuf = make(map[string][]float64, len(g.Nodes))
	}

	if e.chainMixBuf == nil {
		e.chainMixBuf = make([]float64, len(block))
	}

	buffers := e.chainOutBuf
	splitLow := e.chainSplitLowBuf
	splitHigh := e.chainSplitHighBuf

	for _, id := range g.Order {
		if id == inputID {
			buffers[id] = block
			continue
		}

		buf := buffers[id]
		if cap(buf) < len(block) {
			buf = make([]float64, len(block))
		}

		buf = buf[:len(block)]
		buffers[id] = buf

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

	if len(e.chainMixBuf) < len(block) {
		e.chainMixBuf = make([]float64, len(block))
	}

	mixBuf := e.chainMixBuf[:len(block)]

	for _, id := range g.Order {
		if id == inputID {
			continue
		}

		node := g.Nodes[id]
		dst := buffers[id]

		parents := g.Incoming[id]
		edgeSrc := func(edge compiledChainEdge) []float64 {
			parentNode := g.Nodes[edge.From]
			if parentNode.Type == "split-freq" {
				if edge.FromPortIndex == 1 {
					return splitHigh[edge.From]
				}

				return splitLow[edge.From]
			}

			return buffers[edge.From]
		}

		mainParents := parents
		sideParents := []compiledChainEdge(nil)

		if node.Type == "dyn-lookahead" || node.Type == "vocoder" {
			mainParents = mainParents[:0]

			for _, edge := range parents {
				if edge.ToPortIndex == 1 {
					sideParents = append(sideParents, edge)
				} else {
					mainParents = append(mainParents, edge)
				}
			}
		}

		mixParentEdgesInto(mainParents, dst, mixBuf, edgeSrc)

		if node.Type == "split-freq" {
			low := splitLow[id]
			high := splitHigh[id]

			freq := getNodeNum(node, "freqHz", 1200)
			if freq < 20 {
				freq = 20
			}

			nyquist := e.sampleRate * 0.5

			maxFreq := math.Max(20, nyquist*0.95)
			if freq > maxFreq {
				freq = maxFreq
			}

			xo := e.chainCrossover[id]
			if xo == nil || math.Abs(xo.Freq()-freq) > 1e-9 {
				newXO, err := crossover.New(freq, 4, e.sampleRate)
				if err == nil {
					if e.chainCrossover == nil {
						e.chainCrossover = map[string]*crossover.Crossover{}
					}

					e.chainCrossover[id] = newXO
					xo = newXO
				}
			}

			if xo != nil {
				xo.ProcessBlock(dst, low, high)
			} else {
				copy(low, dst)
				copy(high, dst)
			}

			continue
		}

		if id == outputID || node.Bypassed {
			continue
		}

		if node.Type == "dyn-lookahead" {
			rt := e.chainNodes[node.ID]
			if rt != nil {
				if lookahead, ok := rt.effect.(*lookaheadLimiterChainRuntime); ok {
					sideBuf := e.chainMixBuf[:len(dst)]
					mixParentEdgesInto(sideParents, sideBuf, mixBuf, edgeSrc)

					if len(sideParents) == 0 {
						copy(sideBuf, dst)
					}

					lookahead.ProcessWithSidechain(dst, sideBuf)

					continue
				}
			}
		}

		if node.Type == "vocoder" {
			rt := e.chainNodes[node.ID]
			if rt != nil {
				if voc, ok := rt.effect.(*vocoderChainRuntime); ok {
					// Ensure carrier buffer is large enough.
					if len(voc.carrierBuf) < len(dst) {
						voc.carrierBuf = make([]float64, len(dst))
					}

					carrierBuf := voc.carrierBuf[:len(dst)]
					mixParentEdgesInto(sideParents, carrierBuf, mixBuf, edgeSrc)

					if len(sideParents) == 0 {
						copy(carrierBuf, dst)
					}

					// dst = modulator (port 0), carrierBuf = carrier (port 1).
					voc.processVocoder(dst, carrierBuf)

					continue
				}
			}
		}

		e.applyCompiledNode(node, dst)
	}

	out := buffers[outputID]
	if out == nil {
		return false
	}

	copy(block, out)

	return true
}

func mixParentEdgesInto(
	parents []compiledChainEdge,
	dst []float64,
	mixBuf []float64,
	edgeSrc func(edge compiledChainEdge) []float64,
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

// applyCompiledNode dispatches a single compiled graph node to its runtime effect.
func (e *Engine) applyCompiledNode(node compiledChainNode, block []float64) {
	if node.Type == "split" || node.Type == "split-freq" || node.Type == "sum" || node.Type == "_input" || node.Type == "_output" {
		return
	}

	rt := e.chainNodes[node.ID]
	if rt == nil || rt.effect == nil {
		return
	}

	rt.effect.Process(e, node, block)
}

// processWidenerMonoInPlace applies the stereo widener to a mono signal using a
// short decorrelation delay to approximate a stereo side signal, then folds back to mono.
func (e *Engine) processWidenerMonoInPlace(block []float64) {
	if len(block) == 0 {
		return
	}

	if len(e.chainBuf) < len(block) {
		e.chainBuf = make([]float64, len(block))
	}

	dry := e.chainBuf[:len(block)]
	copy(dry, block)

	// Mono fold-down approximation:
	// Build a decorrelated side signal from a short delay, run through stereo widener,
	// then fold back to mono with user-controlled wet mix.
	delaySamples := int(e.sampleRate * 0.001) // 1 ms
	if delaySamples < 1 {
		delaySamples = 1
	}

	for i := range block {
		left := dry[i]

		right := dry[i]
		if i >= delaySamples {
			right = dry[i-delaySamples]
		}

		l2, r2 := e.widener.ProcessStereo(left, right)
		wet := 0.5 * (l2 + r2)
		block[i] = dry[i]*(1-e.effects.WidenerMix) + wet*e.effects.WidenerMix
	}
}

// processNodeWidenerMonoInPlace is the per-chain-node variant of processWidenerMonoInPlace,
// using the node's own widener instance and a caller-supplied mix value.
func (e *Engine) processNodeWidenerMonoInPlace(block []float64, widener *spatial.StereoWidener, mix float64) {
	if len(block) == 0 || widener == nil {
		return
	}

	if len(e.chainBuf) < len(block) {
		e.chainBuf = make([]float64, len(block))
	}

	dry := e.chainBuf[:len(block)]
	copy(dry, block)

	delaySamples := int(e.sampleRate * 0.001)
	if delaySamples < 1 {
		delaySamples = 1
	}

	for i := range block {
		left := dry[i]

		right := dry[i]
		if i >= delaySamples {
			right = dry[i-delaySamples]
		}

		l2, r2 := widener.ProcessStereo(left, right)
		wet := 0.5 * (l2 + r2)
		block[i] = dry[i]*(1-mix) + wet*mix
	}
}
