package webdemo

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// chainGraphNode is a JSON-serializable node in the effect chain graph.
type chainGraphNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Bypassed bool   `json:"bypassed"`
	Fixed    bool   `json:"fixed"`
	Params   any    `json:"params"`
}

// chainGraphConnection is a JSON-serializable connection between two graph nodes.
type chainGraphConnection struct {
	From          string `json:"from"`
	To            string `json:"to"`
	FromPortIndex int    `json:"fromPortIndex,omitempty"`
	ToPortIndex   int    `json:"toPortIndex,omitempty"`
}

// chainGraphState is the root JSON structure for the effect chain graph.
type chainGraphState struct {
	Nodes       []chainGraphNode       `json:"nodes"`
	Connections []chainGraphConnection `json:"connections"`
}

// compiledChainNode is the internal representation of a parsed graph node.
type compiledChainNode struct {
	ID       string
	Type     string
	Bypassed bool
	Num      map[string]float64
	Str      map[string]string
}

// compiledChainGraph holds the compiled effect chain graph with adjacency info
// and a topologically sorted traversal order.
type compiledChainGraph struct {
	Nodes    map[string]compiledChainNode
	Incoming map[string][]compiledChainEdge
	Outgoing map[string][]compiledChainEdge
	Order    []string
}

type compiledChainEdge struct {
	From          string
	To            string
	FromPortIndex int
	ToPortIndex   int
}

// chainEffectRuntime is the per-node processing/configuration contract.
type chainEffectRuntime interface {
	Configure(e *Engine, node compiledChainNode) error
	Process(e *Engine, node compiledChainNode, block []float64)
}

// chainNodeRuntime stores one runtime instance bound to one graph node id.
type chainNodeRuntime struct {
	effectType string
	effect     chainEffectRuntime
}

// chainEffectFactory builds one runtime instance for a node.
type chainEffectFactory func(e *Engine) (chainEffectRuntime, error)

var chainEffectRegistry = map[string]chainEffectFactory{}

func registerChainEffectFactory(effectType string, factory chainEffectFactory) {
	if effectType == "" {
		panic("chain effect registry: empty effect type")
	}

	if factory == nil {
		panic("chain effect registry: nil factory")
	}

	if _, exists := chainEffectRegistry[effectType]; exists {
		panic("chain effect registry: duplicate effect type: " + effectType)
	}

	chainEffectRegistry[effectType] = factory
}

func init() {
	registerChainEffectFactory("chorus", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := modulation.NewChorus()
		if err != nil {
			return nil, err
		}

		return &chorusChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("flanger", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := modulation.NewFlanger(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &flangerChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("ringmod", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := modulation.NewRingModulator(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &ringModChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("bitcrusher", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewBitCrusher(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &bitCrusherChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("distortion", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewDistortion(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &distortionChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("transformer", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewTransformerSimulation(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &transformerChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("widener", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := spatial.NewStereoWidener(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &widenerChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("phaser", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := modulation.NewPhaser(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &phaserChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("tremolo", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := modulation.NewTremolo(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &tremoloChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("delay", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewDelay(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &delayChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("delay-simple", func(_ *Engine) (chainEffectRuntime, error) {
		return &simpleDelayChainRuntime{}, nil
	})
	registerChainEffectFactory("filter", func(e *Engine) (chainEffectRuntime, error) {
		return &filterChainRuntime{fx: biquad.NewChain([]biquad.Coefficients{{B0: 1}})}, nil
	})
	registerChainEffectFactory("bass", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewHarmonicBass(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &bassChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("pitch-time", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := pitch.NewPitchShifter(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &timePitchChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("pitch-spectral", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := pitch.NewSpectralPitchShifter(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &spectralPitchChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("reverb", func(e *Engine) (chainEffectRuntime, error) {
		fdn, err := reverb.NewFDNReverb(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &reverbChainRuntime{freeverb: reverb.NewReverb(), fdn: fdn}, nil
	})
	registerChainEffectFactory("dyn-compressor", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewCompressor(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &compressorChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-limiter", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewLimiter(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &limiterChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-lookahead", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewLookaheadLimiter(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &lookaheadLimiterChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-gate", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewGate(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &gateChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-expander", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewExpander(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &expanderChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-deesser", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewDeEsser(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &deesserChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-transient", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewTransientShaper(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &transientShaperChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("dyn-multiband", func(e *Engine) (chainEffectRuntime, error) {
		return &multibandChainRuntime{}, nil
	})
}

// parseChainGraph parses the JSON chain graph and performs a topological sort
// (Kahn's algorithm). Returns nil, nil for an empty string.
func parseChainGraph(raw string) (*compiledChainGraph, error) {
	if raw == "" {
		return nil, nil
	}

	var state chainGraphState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("invalid chain graph json: %w", err)
	}

	nodes := make(map[string]compiledChainNode, len(state.Nodes))
	for _, n := range state.Nodes {
		if n.ID == "" || n.Type == "" {
			continue
		}

		num, str := parseNodeParams(n.Params)
		nodes[n.ID] = compiledChainNode{
			ID:       n.ID,
			Type:     n.Type,
			Bypassed: n.Bypassed,
			Num:      num,
			Str:      str,
		}
	}

	if _, ok := nodes["_input"]; !ok {
		return nil, nil
	}

	if _, ok := nodes["_output"]; !ok {
		return nil, nil
	}

	incoming := make(map[string][]compiledChainEdge, len(nodes))
	outgoing := make(map[string][]compiledChainEdge, len(nodes))

	indegree := make(map[string]int, len(nodes))
	for id := range nodes {
		incoming[id] = nil
		outgoing[id] = nil
		indegree[id] = 0
	}

	for _, conn := range state.Connections {
		if conn.From == "" || conn.To == "" || conn.From == conn.To {
			continue
		}

		if _, ok := nodes[conn.From]; !ok {
			continue
		}

		if _, ok := nodes[conn.To]; !ok {
			continue
		}

		edge := compiledChainEdge{
			From: conn.From,
			To:   conn.To,
		}
		if conn.FromPortIndex >= 0 {
			edge.FromPortIndex = conn.FromPortIndex
		}

		if conn.ToPortIndex >= 0 {
			edge.ToPortIndex = conn.ToPortIndex
		}

		outgoing[conn.From] = append(outgoing[conn.From], edge)
		incoming[conn.To] = append(incoming[conn.To], edge)
		indegree[conn.To]++
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
		return nil, fmt.Errorf("invalid chain graph: contains cycle")
	}

	return &compiledChainGraph{
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
		switch typed := v.(type) {
		case float64:
			num[k] = typed
		case float32:
			num[k] = float64(typed)
		case int:
			num[k] = float64(typed)
		case int64:
			num[k] = float64(typed)
		case string:
			str[k] = typed
		case bool:
			if typed {
				num[k] = 1
			} else {
				num[k] = 0
			}
		}
	}

	return num, str
}

// syncChainEffectNodes synchronises runtime effect instances with the compiled graph topology.
// Nodes that are no longer present are removed; new or type-changed nodes are (re)created and configured.
func (e *Engine) syncChainEffectNodes(graph *compiledChainGraph) error {
	if graph == nil {
		e.chainNodes = nil
		e.chainCrossover = nil

		return nil
	}

	if e.chainNodes == nil {
		e.chainNodes = map[string]*chainNodeRuntime{}
	}

	seen := map[string]struct{}{}
	seenCrossover := map[string]struct{}{}

	for _, node := range graph.Nodes {
		if node.Type == "_input" || node.Type == "_output" || node.Type == "split" || node.Type == "sum" || node.Type == "split-freq" {
			if node.Type == "split-freq" {
				seenCrossover[node.ID] = struct{}{}
			}

			continue
		}

		seen[node.ID] = struct{}{}

		runtime := e.chainNodes[node.ID]
		if runtime == nil || runtime.effectType != node.Type {
			effect, err := e.newChainEffectRuntime(node.Type)
			if err != nil {
				return err
			}

			if effect == nil {
				continue
			}

			runtime = &chainNodeRuntime{effectType: node.Type, effect: effect}
			e.chainNodes[node.ID] = runtime
		}

		if err := runtime.effect.Configure(e, node); err != nil {
			return err
		}
	}

	for id := range e.chainNodes {
		if _, ok := seen[id]; !ok {
			delete(e.chainNodes, id)
		}
	}

	for id := range e.chainCrossover {
		if _, ok := seenCrossover[id]; !ok {
			delete(e.chainCrossover, id)
		}
	}

	return nil
}

func (e *Engine) newChainEffectRuntime(effectType string) (chainEffectRuntime, error) {
	factory := chainEffectRegistry[effectType]
	if factory == nil {
		return nil, nil
	}

	return factory(e)
}

// getNodeNum safely extracts a numeric parameter from a compiled node, returning def if missing or invalid.
func getNodeNum(node compiledChainNode, key string, def float64) float64 {
	if node.Num == nil {
		return def
	}

	v, ok := node.Num[key]
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}

	return v
}

func configureChorus(fx *modulation.Chorus, sampleRate, mix, depth, speedHz float64, stages int) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetMix(mix); err != nil {
		return err
	}

	if err := fx.SetDepth(depth); err != nil {
		return err
	}

	if err := fx.SetSpeedHz(speedHz); err != nil {
		return err
	}

	return fx.SetStages(stages)
}

func configureFlanger(fx *modulation.Flanger, sampleRate, rateHz, baseDelay, depth, feedback, mix float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetRateHz(rateHz); err != nil {
		return err
	}
	// Apply timing in a transition-safe order to avoid invalid intermediate
	// base+depth combinations during whole-graph parameter updates.
	if err := fx.SetDepthSeconds(0); err != nil {
		return err
	}

	if err := fx.SetBaseDelaySeconds(baseDelay); err != nil {
		return err
	}

	if err := fx.SetDepthSeconds(depth); err != nil {
		return err
	}

	if err := fx.SetFeedback(feedback); err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureRingMod(fx *modulation.RingModulator, sampleRate, carrierHz, mix float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetCarrierHz(carrierHz); err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureBitCrusher(effect *effects.BitCrusher, sampleRate, bitDepth float64, downsample int, mix float64) error {
	if err := effect.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := effect.SetBitDepth(bitDepth); err != nil {
		return err
	}

	if err := effect.SetDownsample(downsample); err != nil {
		return err
	}

	return effect.SetMix(mix)
}

func configureDistortion(
	fx *effects.Distortion,
	sampleRate float64,
	mode effects.DistortionMode,
	approx effects.DistortionApproxMode,
	drive, mix, outputLevel, clipLevel, shape, bias float64,
	chebOrder int,
	chebMode effects.ChebyshevHarmonicMode,
	chebInvert bool,
	chebGain float64,
	chebDCBypass bool,
) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if chebMode == effects.ChebyshevHarmonicOdd && chebOrder%2 == 0 {
		chebOrder++
	}

	if chebMode == effects.ChebyshevHarmonicEven && chebOrder%2 != 0 {
		chebOrder++
	}

	if chebOrder > 16 {
		chebOrder = 16
	}

	if chebMode == effects.ChebyshevHarmonicOdd && chebOrder%2 == 0 {
		chebOrder--
	}

	if chebMode == effects.ChebyshevHarmonicEven && chebOrder%2 != 0 {
		chebOrder--
	}

	if chebOrder < 1 {
		chebOrder = 1
	}

	if err := fx.SetMode(mode); err != nil {
		return err
	}

	if err := fx.SetApproxMode(approx); err != nil {
		return err
	}

	if err := fx.SetDrive(drive); err != nil {
		return err
	}

	if err := fx.SetMix(mix); err != nil {
		return err
	}

	if err := fx.SetOutputLevel(outputLevel); err != nil {
		return err
	}

	if err := fx.SetClipLevel(clipLevel); err != nil {
		return err
	}

	if err := fx.SetShape(shape); err != nil {
		return err
	}

	if err := fx.SetBias(bias); err != nil {
		return err
	}

	if err := fx.SetChebyshevOrder(chebOrder); err != nil {
		return err
	}

	if err := fx.SetChebyshevHarmonicMode(chebMode); err != nil {
		return err
	}

	fx.SetChebyshevInvert(chebInvert)

	if err := fx.SetChebyshevGainLevel(chebGain); err != nil {
		return err
	}

	fx.SetChebyshevDCBypass(chebDCBypass)

	return nil
}

func configureTransformer(
	fx *effects.TransformerSimulation,
	sampleRate float64,
	quality effects.TransformerQuality,
	drive, mix, outputLevel, highpassHz, dampingHz float64,
	oversampling int,
) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetQuality(quality); err != nil {
		return err
	}

	if err := fx.SetDrive(drive); err != nil {
		return err
	}

	if err := fx.SetMix(mix); err != nil {
		return err
	}

	if err := fx.SetOutputLevel(outputLevel); err != nil {
		return err
	}

	if err := fx.SetHighpassHz(highpassHz); err != nil {
		return err
	}

	if err := fx.SetDampingHz(dampingHz); err != nil {
		return err
	}

	return fx.SetOversampling(oversampling)
}

func configureWidener(fx *spatial.StereoWidener, sampleRate, width float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetWidth(width); err != nil {
		return err
	}

	return fx.SetBassMonoFreq(0)
}

func configurePhaser(effect *modulation.Phaser, sampleRate, rateHz, minFreqHz, maxFreqHz float64, stages int, feedback, mix float64) error {
	if err := effect.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := effect.SetRateHz(rateHz); err != nil {
		return err
	}

	if err := effect.SetFrequencyRangeHz(minFreqHz, maxFreqHz); err != nil {
		return err
	}

	if err := effect.SetStages(stages); err != nil {
		return err
	}

	if err := effect.SetFeedback(feedback); err != nil {
		return err
	}

	return effect.SetMix(mix)
}

func configureTremolo(fx *modulation.Tremolo, sampleRate, rateHz, depth, smoothingMs, mix float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetRateHz(rateHz); err != nil {
		return err
	}

	if err := fx.SetDepth(depth); err != nil {
		return err
	}

	if err := fx.SetSmoothingMs(smoothingMs); err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureDelay(fx *effects.Delay, sampleRate, time, feedback, mix float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetTime(time); err != nil {
		return err
	}

	if err := fx.SetFeedback(feedback); err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureTimePitch(fx *pitch.PitchShifter, sampleRate, semitones, sequence, overlap, search float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetPitchSemitones(semitones); err != nil {
		return err
	}

	if err := fx.SetSequence(sequence); err != nil {
		return err
	}

	if err := fx.SetOverlap(overlap); err != nil {
		return err
	}

	return fx.SetSearch(search)
}

func configureSpectralPitch(fx *pitch.SpectralPitchShifter, sampleRate, semitones float64, frameSize, analysisHop int) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetPitchSemitones(semitones); err != nil {
		return err
	}

	if err := fx.SetFrameSize(frameSize); err != nil {
		return err
	}

	return fx.SetAnalysisHop(analysisHop)
}

func configureFDNReverb(fx *reverb.FDNReverb, sampleRate, wet, dry, rt60, preDelay, damp, modDepth, modRate float64) error {
	if err := fx.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := fx.SetWet(wet); err != nil {
		return err
	}

	if err := fx.SetDry(dry); err != nil {
		return err
	}

	if err := fx.SetRT60(rt60); err != nil {
		return err
	}

	if err := fx.SetPreDelay(preDelay); err != nil {
		return err
	}

	if err := fx.SetDamp(damp); err != nil {
		return err
	}

	if err := fx.SetModDepth(modDepth); err != nil {
		return err
	}

	return fx.SetModRate(modRate)
}

func configureFreeverb(fx *reverb.Reverb, wet, dry, roomSize, damp, gain float64) {
	fx.SetWet(wet)
	fx.SetDry(dry)
	fx.SetRoomSize(roomSize)
	fx.SetDamp(damp)
	fx.SetGain(gain)
}

func configureHarmonicBass(effect *effects.HarmonicBass, sampleRate, frequency, inputGain, highGain, original, harmonic, decay, responseMs float64, highpass int) error {
	if err := effect.SetSampleRate(sampleRate); err != nil {
		return err
	}

	if err := effect.SetFrequency(frequency); err != nil {
		return err
	}

	if err := effect.SetInputLevel(inputGain); err != nil {
		return err
	}

	if err := effect.SetHighFrequencyLevel(highGain); err != nil {
		return err
	}

	if err := effect.SetOriginalBassLevel(original); err != nil {
		return err
	}

	if err := effect.SetHarmonicBassLevel(harmonic); err != nil {
		return err
	}

	if err := effect.SetDecay(decay); err != nil {
		return err
	}

	if err := effect.SetResponse(responseMs); err != nil {
		return err
	}

	return effect.SetHighpassMode(effects.HighpassSelect(highpass))
}

func normalizeDistortionMode(raw string) effects.DistortionMode {
	switch raw {
	case "hardclip":
		return effects.DistortionModeHardClip
	case "tanh":
		return effects.DistortionModeTanh
	case "waveshaper1":
		return effects.DistortionModeWaveshaper1
	case "waveshaper2":
		return effects.DistortionModeWaveshaper2
	case "waveshaper3":
		return effects.DistortionModeWaveshaper3
	case "waveshaper4":
		return effects.DistortionModeWaveshaper4
	case "waveshaper5":
		return effects.DistortionModeWaveshaper5
	case "waveshaper6":
		return effects.DistortionModeWaveshaper6
	case "waveshaper7":
		return effects.DistortionModeWaveshaper7
	case "waveshaper8":
		return effects.DistortionModeWaveshaper8
	case "saturate":
		return effects.DistortionModeSaturate
	case "saturate2":
		return effects.DistortionModeSaturate2
	case "softsat":
		return effects.DistortionModeSoftSat
	case "chebyshev":
		return effects.DistortionModeChebyshev
	case "softclip":
		fallthrough
	default:
		return effects.DistortionModeSoftClip
	}
}

func normalizeDistortionApproxMode(raw string) effects.DistortionApproxMode {
	switch raw {
	case "polynomial":
		return effects.DistortionApproxPolynomial
	case "exact":
		fallthrough
	default:
		return effects.DistortionApproxExact
	}
}

func normalizeChebyshevHarmonicMode(raw string) effects.ChebyshevHarmonicMode {
	switch raw {
	case "odd":
		return effects.ChebyshevHarmonicOdd
	case "even":
		return effects.ChebyshevHarmonicEven
	case "all":
		fallthrough
	default:
		return effects.ChebyshevHarmonicAll
	}
}

func normalizeTransformerQuality(raw string) effects.TransformerQuality {
	switch raw {
	case "lightweight":
		return effects.TransformerQualityLightweight
	case "high":
		fallthrough
	default:
		return effects.TransformerQualityHigh
	}
}

func normalizeDynamicsTopology(raw string) dynamics.DynamicsTopology {
	switch raw {
	case "feedback":
		return dynamics.DynamicsTopologyFeedback
	case "feedforward":
		fallthrough
	default:
		return dynamics.DynamicsTopologyFeedforward
	}
}

func normalizeDynamicsDetectorMode(raw string) dynamics.DetectorMode {
	switch raw {
	case "rms":
		return dynamics.DetectorModeRMS
	case "peak":
		fallthrough
	default:
		return dynamics.DetectorModePeak
	}
}

func normalizeDeesserMode(raw string) dynamics.DeEsserMode {
	switch raw {
	case "wideband":
		return dynamics.DeEsserWideband
	case "splitband":
		fallthrough
	default:
		return dynamics.DeEsserSplitBand
	}
}

func normalizeDeesserDetector(raw string) dynamics.DeEsserDetector {
	switch raw {
	case "highpass":
		return dynamics.DeEsserDetectHighpass
	case "bandpass":
		fallthrough
	default:
		return dynamics.DeEsserDetectBandpass
	}
}
