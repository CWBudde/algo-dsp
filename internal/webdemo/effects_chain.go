package webdemo

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/moog"
	"github.com/cwbudde/algo-dsp/dsp/window"
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
	FromPortIndex int    `json:"fromPortIndex,omitempty"` //nolint:tagliatelle
	ToPortIndex   int    `json:"toPortIndex,omitempty"`   //nolint:tagliatelle
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
	registerChainEffectFactory("chorus", func(_ *Engine) (chainEffectRuntime, error) {
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
	registerChainEffectFactory("dist-cheb", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewDistortion(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &distChebChainRuntime{fx: fx}, nil
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

	filterNodeTypes := []string{
		"filter",
		"filter-lowpass",
		"filter-highpass",
		"filter-bandpass",
		"filter-notch",
		"filter-allpass",
		"filter-peak",
		"filter-lowshelf",
		"filter-highshelf",
		"filter-moog",
	}
	for _, effectType := range filterNodeTypes {
		t := effectType
		registerChainEffectFactory(t, func(_ *Engine) (chainEffectRuntime, error) {
			return &filterChainRuntime{fx: biquad.NewChain([]biquad.Coefficients{{B0: 1}})}, nil
		})
	}

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
	registerChainEffectFactory("spectral-freeze", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewSpectralFreeze(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &spectralFreezeChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("granular", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewGranular(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &granularChainRuntime{fx: fx}, nil
	})
	registerChainEffectFactory("reverb", func(e *Engine) (chainEffectRuntime, error) {
		fdn, err := reverb.NewFDNReverb(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &reverbChainRuntime{
			freeverb: &freeverbChainRuntime{fx: reverb.NewReverb()},
			fdn:      &fdnReverbChainRuntime{fx: fdn},
		}, nil
	})
	registerChainEffectFactory("reverb-freeverb", func(_ *Engine) (chainEffectRuntime, error) {
		return &freeverbChainRuntime{fx: reverb.NewReverb()}, nil
	})
	registerChainEffectFactory("reverb-fdn", func(e *Engine) (chainEffectRuntime, error) {
		fdn, err := reverb.NewFDNReverb(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &fdnReverbChainRuntime{fx: fdn}, nil
	})
	registerChainEffectFactory("reverb-conv", func(_ *Engine) (chainEffectRuntime, error) {
		return &convReverbChainRuntime{irIndex: -1}, nil
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
	registerChainEffectFactory("dyn-multiband", func(_ *Engine) (chainEffectRuntime, error) {
		return &multibandChainRuntime{}, nil
	})
	registerChainEffectFactory("vocoder", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := effects.NewVocoder(e.sampleRate)
		if err != nil {
			return nil, err
		}

		return &vocoderChainRuntime{fx: fx}, nil
	})
}

// parseChainGraph parses the JSON chain graph and performs a topological sort
// (Kahn's algorithm). Returns nil, nil for an empty string.
func parseChainGraph(raw string) (*compiledChainGraph, error) {
	if raw == "" {
		return nil, nil
	}

	var state chainGraphState

	err := json.Unmarshal([]byte(raw), &state)
	if err != nil {
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

		edge := compiledChainEdge{
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

// syncChainEffectNodes synchronises runtime effect instances with the compiled graph topology.
// Nodes that are no longer present are removed; new or type-changed nodes are (re)created and configured.
//
//nolint:cyclop
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

		rt := e.chainNodes[node.ID]
		if rt == nil || rt.effectType != node.Type {
			effect, err := e.newChainEffectRuntime(node.Type)
			if err != nil {
				return err
			}

			if effect == nil {
				continue
			}

			rt = &chainNodeRuntime{effectType: node.Type, effect: effect}
			e.chainNodes[node.ID] = rt
		}

		err := rt.effect.Configure(e, node)
		if err != nil {
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
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return err
	}

	err = fx.SetSpeedHz(speedHz)
	if err != nil {
		return err
	}

	return fx.SetStages(stages)
}

func configureFlanger(fx *modulation.Flanger, sampleRate, rateHz, baseDelay, depth, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}
	// Apply timing in a transition-safe order to avoid invalid intermediate
	// base+depth combinations during whole-graph parameter updates.
	err = fx.SetDepthSeconds(0)
	if err != nil {
		return err
	}

	err = fx.SetBaseDelaySeconds(baseDelay)
	if err != nil {
		return err
	}

	err = fx.SetDepthSeconds(depth)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureRingMod(fx *modulation.RingModulator, sampleRate, carrierHz, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetCarrierHz(carrierHz)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureBitCrusher(fx *effects.BitCrusher, sampleRate, bitDepth float64, downsample int, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetBitDepth(bitDepth)
	if err != nil {
		return err
	}

	err = fx.SetDownsample(downsample)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

//nolint:cyclop
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
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
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

	err = fx.SetMode(mode)
	if err != nil {
		return err
	}

	err = fx.SetApproxMode(approx)
	if err != nil {
		return err
	}

	err = fx.SetDrive(drive)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	err = fx.SetOutputLevel(outputLevel)
	if err != nil {
		return err
	}

	err = fx.SetClipLevel(clipLevel)
	if err != nil {
		return err
	}

	err = fx.SetShape(shape)
	if err != nil {
		return err
	}

	err = fx.SetBias(bias)
	if err != nil {
		return err
	}

	err = fx.SetChebyshevOrder(chebOrder)
	if err != nil {
		return err
	}

	err = fx.SetChebyshevHarmonicMode(chebMode)
	if err != nil {
		return err
	}

	fx.SetChebyshevInvert(chebInvert)

	err = fx.SetChebyshevGainLevel(chebGain)
	if err != nil {
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
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetQuality(quality)
	if err != nil {
		return err
	}

	err = fx.SetDrive(drive)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	err = fx.SetOutputLevel(outputLevel)
	if err != nil {
		return err
	}

	err = fx.SetHighpassHz(highpassHz)
	if err != nil {
		return err
	}

	err = fx.SetDampingHz(dampingHz)
	if err != nil {
		return err
	}

	return fx.SetOversampling(oversampling)
}

func configureWidener(fx *spatial.StereoWidener, sampleRate, width float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetWidth(width)
	if err != nil {
		return err
	}

	return fx.SetBassMonoFreq(0)
}

func configurePhaser(fx *modulation.Phaser, sampleRate, rateHz, minFreqHz, maxFreqHz float64, stages int, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}

	err = fx.SetFrequencyRangeHz(minFreqHz, maxFreqHz)
	if err != nil {
		return err
	}

	err = fx.SetStages(stages)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureTremolo(fx *modulation.Tremolo, sampleRate, rateHz, depth, smoothingMs, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetRateHz(rateHz)
	if err != nil {
		return err
	}

	err = fx.SetDepth(depth)
	if err != nil {
		return err
	}

	err = fx.SetSmoothingMs(smoothingMs)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureDelay(fx *effects.Delay, sampleRate, time, feedback, mix float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	// Use SetTargetTime so the read-pointer ramps smoothly to the new
	// delay time during subsequent processing, avoiding an audible click.
	err = fx.SetTargetTime(time)
	if err != nil {
		return err
	}

	err = fx.SetFeedback(feedback)
	if err != nil {
		return err
	}

	return fx.SetMix(mix)
}

func configureTimePitch(fx *pitch.PitchShifter, sampleRate, semitones, sequence, overlap, search float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return err
	}

	err = fx.SetSequence(sequence)
	if err != nil {
		return err
	}

	err = fx.SetOverlap(overlap)
	if err != nil {
		return err
	}

	return fx.SetSearch(search)
}

func configureSpectralPitch(fx *pitch.SpectralPitchShifter, sampleRate, semitones float64, frameSize, analysisHop int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetPitchSemitones(semitones)
	if err != nil {
		return err
	}

	err = fx.SetFrameSize(frameSize)
	if err != nil {
		return err
	}

	return fx.SetAnalysisHop(analysisHop)
}

func configureSpectralFreeze(
	fx *effects.SpectralFreeze,
	sampleRate float64,
	frameSize, hopSize int,
	mix float64,
	phaseMode effects.SpectralFreezePhaseMode,
	frozen bool,
	windowType window.Type,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetFrameSize(frameSize)
	if err != nil {
		return err
	}

	err = fx.SetHopSize(hopSize)
	if err != nil {
		return err
	}

	err = fx.SetWindowType(windowType)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	err = fx.SetPhaseMode(phaseMode)
	if err != nil {
		return err
	}

	fx.SetFrozen(frozen)

	return nil
}

func configureGranular(
	fx *effects.Granular,
	sampleRate, grainSeconds, overlap, pitchRatio, spray, baseDelay, mix float64,
) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetGrainSeconds(grainSeconds)
	if err != nil {
		return err
	}

	err = fx.SetOverlap(overlap)
	if err != nil {
		return err
	}

	err = fx.SetPitch(pitchRatio)
	if err != nil {
		return err
	}

	err = fx.SetSpray(spray)
	if err != nil {
		return err
	}

	err = fx.SetBaseDelay(baseDelay)
	if err != nil {
		return err
	}

	err = fx.SetMix(mix)
	if err != nil {
		return err
	}

	return nil
}

func configureFDNReverb(fx *reverb.FDNReverb, sampleRate, wet, dry, rt60, preDelay, damp, modDepth, modRate float64) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetWet(wet)
	if err != nil {
		return err
	}

	err = fx.SetDry(dry)
	if err != nil {
		return err
	}

	err = fx.SetRT60(rt60)
	if err != nil {
		return err
	}

	err = fx.SetPreDelay(preDelay)
	if err != nil {
		return err
	}

	err = fx.SetDamp(damp)
	if err != nil {
		return err
	}

	err = fx.SetModDepth(modDepth)
	if err != nil {
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

func configureHarmonicBass(fx *effects.HarmonicBass, sampleRate, frequency, inputGain, highGain, original, harmonic, decay, responseMs float64, highpass int) error {
	err := fx.SetSampleRate(sampleRate)
	if err != nil {
		return err
	}

	err = fx.SetFrequency(frequency)
	if err != nil {
		return err
	}

	err = fx.SetInputLevel(inputGain)
	if err != nil {
		return err
	}

	err = fx.SetHighFrequencyLevel(highGain)
	if err != nil {
		return err
	}

	err = fx.SetOriginalBassLevel(original)
	if err != nil {
		return err
	}

	err = fx.SetHarmonicBassLevel(harmonic)
	if err != nil {
		return err
	}

	err = fx.SetDecay(decay)
	if err != nil {
		return err
	}

	err = fx.SetResponse(responseMs)
	if err != nil {
		return err
	}

	return fx.SetHighpassMode(effects.HighpassSelect(highpass))
}

type chorusChainRuntime struct {
	fx *modulation.Chorus
}

func (r *chorusChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	stages := min(max(int(math.Round(getNodeNum(node, "stages", 3))), 1), 6)

	return configureChorus(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "mix", 0.18), 0, 1),
		clamp(getNodeNum(node, "depth", 0.003), 0, 0.01),
		clamp(getNodeNum(node, "speedHz", 0.35), 0.05, 5),
		stages,
	)
}

func (r *chorusChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type flangerChainRuntime struct {
	fx *modulation.Flanger
}

func (r *flangerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureFlanger(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "rateHz", 0.25), 0.05, 5),
		clamp(getNodeNum(node, "baseDelay", 0.001), 0.0001, 0.01),
		clamp(getNodeNum(node, "depth", 0.0015), 0, 0.0099),
		clamp(getNodeNum(node, "feedback", 0.25), -0.99, 0.99),
		clamp(getNodeNum(node, "mix", 0.5), 0, 1),
	)
}

func (r *flangerChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type ringModChainRuntime struct {
	fx *modulation.RingModulator
}

func (r *ringModChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureRingMod(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "carrierHz", 440), 1, e.sampleRate*0.49),
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *ringModChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type bitCrusherChainRuntime struct {
	fx *effects.BitCrusher
}

func (r *bitCrusherChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	ds := min(max(int(math.Round(getNodeNum(node, "downsample", 4))), 1), 256)

	return configureBitCrusher(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "bitDepth", 8), 1, 32),
		ds,
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *bitCrusherChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type distortionChainRuntime struct {
	fx *effects.Distortion
}

func (r *distortionChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	mode := normalizeDistortionMode(node.Str["mode"])
	approx := normalizeDistortionApproxMode(node.Str["approx"])

	return configureDistortion(
		r.fx,
		e.sampleRate,
		mode,
		approx,
		clamp(getNodeNum(node, "drive", 1.8), 0.01, 20),
		clamp(getNodeNum(node, "mix", 1.0), 0, 1),
		clamp(getNodeNum(node, "output", 1.0), 0, 4),
		clamp(getNodeNum(node, "clip", 1.0), 0.05, 1),
		clamp(getNodeNum(node, "shape", 0.5), 0, 1),
		clamp(getNodeNum(node, "bias", 0), -1, 1),
		3,
		effects.ChebyshevHarmonicAll,
		false,
		1.0,
		false,
	)
}

func (r *distortionChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type distChebChainRuntime struct {
	fx *effects.Distortion
}

func (r *distChebChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	approx := normalizeDistortionApproxMode(node.Str["approx"])
	chebMode := normalizeChebyshevHarmonicMode(node.Str["harmonic"])

	chebOrder := min(max(int(math.Round(getNodeNum(node, "order", 3))), 1), 16)

	chebInvert := getNodeNum(node, "invert", 0) >= 0.5
	chebDCBypass := getNodeNum(node, "dcBypass", 0) >= 0.5

	err := configureDistortion(
		r.fx,
		e.sampleRate,
		effects.DistortionModeChebyshev,
		approx,
		clamp(getNodeNum(node, "drive", 1.0), 0.01, 20),
		clamp(getNodeNum(node, "mix", 1.0), 0, 1),
		clamp(getNodeNum(node, "output", 1.0), 0, 4),
		1.0,
		0.5,
		0.0,
		chebOrder,
		chebMode,
		chebInvert,
		clamp(getNodeNum(node, "gain", 1.0), 0, 4),
		chebDCBypass,
	)
	if err != nil {
		return err
	}

	// Per-harmonic weights w1..w16
	weights := make([]float64, 16)
	for k := range 16 {
		weights[k] = getNodeNum(node, fmt.Sprintf("w%d", k+1), 0)
	}

	return r.fx.SetChebyshevWeights(weights)
}

func (r *distChebChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type transformerChainRuntime struct {
	fx *effects.TransformerSimulation
}

func (r *transformerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	quality := normalizeTransformerQuality(node.Str["quality"])

	oversampling := int(math.Round(getNodeNum(node, "oversampling", 4)))
	switch oversampling {
	case 2, 4, 8:
	default:
		if oversampling <= 3 {
			oversampling = 2
		} else if oversampling <= 6 {
			oversampling = 4
		} else {
			oversampling = 8
		}
	}

	return configureTransformer(
		r.fx,
		e.sampleRate,
		quality,
		clamp(getNodeNum(node, "drive", 2.0), 0.1, 30),
		clamp(getNodeNum(node, "mix", 1.0), 0, 1),
		clamp(getNodeNum(node, "output", 1.0), 0, 4),
		clamp(getNodeNum(node, "highpassHz", 25), 5, e.sampleRate*0.45),
		clamp(getNodeNum(node, "dampingHz", 9000), 200, e.sampleRate*0.49),
		oversampling,
	)
}

func (r *transformerChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type widenerChainRuntime struct {
	fx *spatial.StereoWidener
}

func (r *widenerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureWidener(r.fx, e.sampleRate, clamp(getNodeNum(node, "width", 1), 0, 4))
}

func (r *widenerChainRuntime) Process(e *Engine, node compiledChainNode, block []float64) {
	e.processNodeWidenerMonoInPlace(block, r.fx, clamp(getNodeNum(node, "mix", 0.5), 0, 1))
}

type phaserChainRuntime struct {
	fx *modulation.Phaser
}

func (r *phaserChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	minHz := clamp(getNodeNum(node, "minFreqHz", 300), 20, e.sampleRate*0.45)
	maxHz := clamp(getNodeNum(node, "maxFreqHz", 1600), minHz+1, e.sampleRate*0.49)

	stages := min(max(int(math.Round(getNodeNum(node, "stages", 6))), 1), 12)

	return configurePhaser(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "rateHz", 0.4), 0.05, 5),
		minHz,
		maxHz,
		stages,
		clamp(getNodeNum(node, "feedback", 0.2), -0.99, 0.99),
		clamp(getNodeNum(node, "mix", 0.5), 0, 1),
	)
}

func (r *phaserChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type tremoloChainRuntime struct {
	fx *modulation.Tremolo
}

func (r *tremoloChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureTremolo(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "rateHz", 4), 0.05, 20),
		clamp(getNodeNum(node, "depth", 0.6), 0, 1),
		clamp(getNodeNum(node, "smoothingMs", 5), 0, 200),
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *tremoloChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type delayChainRuntime struct {
	fx *effects.Delay
}

func (r *delayChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureDelay(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "time", 0.25), 0.001, 2),
		clamp(getNodeNum(node, "feedback", 0.35), 0, 0.99),
		clamp(getNodeNum(node, "mix", 0.25), 0, 1),
	)
}

func (r *delayChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type simpleDelayChainRuntime struct {
	sampleRate   float64
	delayMs      float64
	delaySamples int
	write        int
	buf          []float64
}

func (r *simpleDelayChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	r.sampleRate = e.sampleRate
	r.delayMs = clamp(getNodeNum(node, "delayMs", 20), 0, 500)

	r.delaySamples = max(int(math.Round(r.delayMs*r.sampleRate/1000.0)), 0)

	size := max(r.delaySamples+1, 1)

	if len(r.buf) != size {
		r.buf = make([]float64, size)
		r.write = 0
	}

	return nil
}

func (r *simpleDelayChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if len(r.buf) <= 1 {
		return
	}

	for i := range block {
		r.buf[r.write] = block[i]

		readPos := r.write + 1
		if readPos >= len(r.buf) {
			readPos = 0
		}

		block[i] = r.buf[readPos]
		r.write = readPos
	}
}

type filterChainRuntime struct {
	fx     *biquad.Chain
	moogLP *moog.Filter

	hasConfig      bool
	lastFamily     string
	lastKind       string
	lastOrder      int
	lastFreq       float64
	lastGainDB     float64
	lastShape      float64
	lastSampleRate float64
}

//nolint:cyclop
func (r *filterChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	family := normalizeChainFilterFamily(node.Str["family"], node.Type)
	kind := normalizeChainFilterKind(node.Type, node.Str["kind"])
	freq := clamp(getNodeNum(node, "freq", 1200), 20, e.sampleRate*0.49)
	gainDB := clamp(getNodeNum(node, "gain", 0), -24, 24)
	shape := clamp(getNodeNum(node, "q", 0.707), 0.2, 8)

	if family == "moog" {
		order := int(math.Round(getNodeNum(node, "order", 8)))
		oversampling := moogOversamplingFromOrder(order)
		resonance := clamp(shape, 0, 4)

		drive := clamp(math.Pow(10, gainDB/20), 0.1, 24)
		if r.hasConfig &&
			r.lastFamily == family &&
			r.lastKind == kind &&
			r.lastOrder == order &&
			eqFloat(r.lastFreq, freq) &&
			eqFloat(r.lastGainDB, gainDB) &&
			eqFloat(r.lastShape, shape) &&
			eqFloat(r.lastSampleRate, e.sampleRate) {
			return nil
		}

		if r.moogLP == nil {
			fx, err := moog.New(
				e.sampleRate,
				moog.WithVariant(moog.VariantHuovilainen),
				moog.WithOversampling(oversampling),
				moog.WithCutoffHz(freq),
				moog.WithResonance(resonance),
				moog.WithDrive(drive),
				moog.WithInputGain(1),
				moog.WithOutputGain(1),
				moog.WithNormalizeOutput(true),
			)
			if err != nil {
				return err
			}

			r.moogLP = fx
		} else {
			err := r.moogLP.SetSampleRate(e.sampleRate)
			if err != nil {
				return err
			}

			err = r.moogLP.SetOversampling(oversampling)
			if err != nil {
				return err
			}

			err = r.moogLP.SetCutoffHz(freq)
			if err != nil {
				return err
			}

			err = r.moogLP.SetResonance(resonance)
			if err != nil {
				return err
			}

			err = r.moogLP.SetDrive(drive)
			if err != nil {
				return err
			}
		}

		r.fx = nil
		r.hasConfig = true
		r.lastFamily = family
		r.lastKind = kind
		r.lastOrder = order
		r.lastFreq = freq
		r.lastGainDB = gainDB
		r.lastShape = shape
		r.lastSampleRate = e.sampleRate

		return nil
	}

	r.moogLP = nil

	family = normalizeEQFamily(family)
	family = normalizeEQFamilyForType(kind, family)
	order := normalizeEQOrder(kind, family, int(math.Round(getNodeNum(node, "order", 2))))

	shape = clampEQShape(kind, family, freq, e.sampleRate, shape)
	if r.hasConfig &&
		r.lastFamily == family &&
		r.lastKind == kind &&
		r.lastOrder == order &&
		eqFloat(r.lastFreq, freq) &&
		eqFloat(r.lastGainDB, gainDB) &&
		eqFloat(r.lastShape, shape) &&
		eqFloat(r.lastSampleRate, e.sampleRate) {
		return nil
	}

	next := buildEQChain(family, kind, order, freq, gainDB, shape, e.sampleRate)
	if r.fx == nil {
		r.fx = next
	} else if r.fx.NumSections() == next.NumSections() {
		r.fx.SetGain(next.Gain())

		for i := 0; i < r.fx.NumSections(); i++ {
			r.fx.Section(i).Coefficients = next.Section(i).Coefficients
		}
	} else {
		// Preserve as much per-section delay state as possible across topology changes.
		oldState := r.fx.State()
		newState := make([][2]float64, next.NumSections())
		copy(newState, oldState)
		next.SetState(newState)
		r.fx = next
	}

	r.hasConfig = true
	r.lastFamily = family
	r.lastKind = kind
	r.lastOrder = order
	r.lastFreq = freq
	r.lastGainDB = gainDB
	r.lastShape = shape
	r.lastSampleRate = e.sampleRate

	return nil
}

func eqFloat(a, b float64) bool {
	return math.Abs(a-b) <= 1e-12
}

func (r *filterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if r.moogLP != nil {
		r.moogLP.ProcessInPlace(block)
		return
	}

	if r.fx != nil {
		r.fx.ProcessBlock(block)
	}
}

type bassChainRuntime struct {
	fx *effects.HarmonicBass
}

func (r *bassChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	hp := min(max(int(math.Round(getNodeNum(node, "highpass", 0))), 0), 2)

	return configureHarmonicBass(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "frequency", 80), 10, 500),
		clamp(getNodeNum(node, "inputGain", 1), 0, 2),
		clamp(getNodeNum(node, "highGain", 1), 0, 2),
		clamp(getNodeNum(node, "original", 1), 0, 2),
		clamp(getNodeNum(node, "harmonic", 0), 0, 2),
		clamp(getNodeNum(node, "decay", 0), -1, 1),
		clamp(getNodeNum(node, "responseMs", 20), 1, 200),
		hp,
	)
}

func (r *bassChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type timePitchChainRuntime struct {
	fx *pitch.PitchShifter
}

func (r *timePitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	seq := clamp(getNodeNum(node, "sequence", 40), 20, 120)

	ov := clamp(getNodeNum(node, "overlap", 10), 4, 60)
	if ov >= seq {
		ov = seq - 1
	}

	return configureTimePitch(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "semitones", 0), -24, 24),
		seq,
		ov,
		clamp(getNodeNum(node, "search", 15), 2, 40),
	)
}

func (r *timePitchChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralPitchChainRuntime struct {
	fx *pitch.SpectralPitchShifter
}

func (r *spectralPitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(getNodeNum(node, "frameSize", 1024))))

	hop := max(int(math.Round(float64(frame)*clamp(getNodeNum(node, "hopRatio", 0.25), 0.01, 0.99))), 1)

	if hop >= frame {
		hop = frame - 1
	}

	return configureSpectralPitch(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "semitones", 0), -24, 24),
		frame,
		hop,
	)
}

func (r *spectralPitchChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralFreezeChainRuntime struct {
	fx *effects.SpectralFreeze
}

func (r *spectralFreezeChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(getNodeNum(node, "frameSize", 1024))))

	hop := max(int(math.Round(float64(frame)*clamp(getNodeNum(node, "hopRatio", 0.25), 0.01, 0.99))), 1)

	if hop >= frame {
		hop = frame - 1
	}

	frozen := getNodeNum(node, "frozen", 1) >= 0.5

	return configureSpectralFreeze(
		r.fx,
		e.sampleRate,
		frame,
		hop,
		clamp(getNodeNum(node, "mix", 1), 0, 1),
		normalizeSpectralFreezePhaseMode(node.Str["phaseMode"]),
		frozen,
		window.TypeHann,
	)
}

func (r *spectralFreezeChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type granularChainRuntime struct {
	fx *effects.Granular
}

func (r *granularChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureGranular(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "grainSeconds", 0.08), 0.005, 0.5),
		clamp(getNodeNum(node, "overlap", 0.5), 0, 0.95),
		clamp(getNodeNum(node, "pitch", 1), 0.25, 4),
		clamp(getNodeNum(node, "spray", 0.1), 0, 1),
		clamp(getNodeNum(node, "baseDelay", 0.08), 0, 2),
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *granularChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type freeverbChainRuntime struct {
	fx *reverb.Reverb
}

func (r *freeverbChainRuntime) Configure(_ *Engine, node compiledChainNode) error {
	configureFreeverb(
		r.fx,
		clamp(getNodeNum(node, "wet", 0.22), 0, 1.5),
		clamp(getNodeNum(node, "dry", 1), 0, 1.5),
		clamp(getNodeNum(node, "roomSize", 0.72), 0, 0.98),
		clamp(getNodeNum(node, "damp", 0.45), 0, 0.99),
		clamp(getNodeNum(node, "gain", 0.015), 0, 0.1),
	)

	return nil
}

func (r *freeverbChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type fdnReverbChainRuntime struct {
	fx *reverb.FDNReverb
}

func (r *fdnReverbChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureFDNReverb(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "wet", 0.22), 0, 1.5),
		clamp(getNodeNum(node, "dry", 1), 0, 1.5),
		clamp(getNodeNum(node, "rt60", 1.8), 0.2, 8),
		clamp(getNodeNum(node, "preDelay", 0.01), 0, 0.1),
		clamp(getNodeNum(node, "damp", 0.45), 0, 0.99),
		clamp(getNodeNum(node, "modDepth", 0.002), 0, 0.01),
		clamp(getNodeNum(node, "modRate", 0.1), 0, 1),
	)
}

func (r *fdnReverbChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

// reverbChainRuntime supports legacy "reverb" nodes with a model selector.
type reverbChainRuntime struct {
	freeverb *freeverbChainRuntime
	fdn      *fdnReverbChainRuntime
}

func (r *reverbChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	model := node.Str["model"]
	if model == "fdn" {
		return r.fdn.Configure(e, node)
	}

	return r.freeverb.Configure(e, node)
}

func (r *reverbChainRuntime) Process(e *Engine, node compiledChainNode, block []float64) {
	model := node.Str["model"]
	if model == "fdn" {
		r.fdn.Process(e, node, block)
		return
	}

	r.freeverb.Process(e, node, block)
}

type compressorChainRuntime struct {
	fx *dynamics.Compressor
}

func (r *compressorChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -20), -60, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 4), 1, 100))
	if err != nil {
		return err
	}

	err = r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24))
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 10), 0.1, 1000))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
	if err != nil {
		return err
	}

	err = r.fx.SetAutoMakeup(false)
	if err != nil {
		return err
	}

	return r.fx.SetMakeupGain(clamp(getNodeNum(node, "makeupGainDB", 0), 0, 24))
}

func (r *compressorChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type limiterChainRuntime struct {
	fx *dynamics.Limiter
}

func (r *limiterChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -0.1), -24, 0))
	if err != nil {
		return err
	}

	return r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
}

func (r *limiterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type lookaheadLimiterChainRuntime struct {
	fx *dynamics.LookaheadLimiter
}

func (r *lookaheadLimiterChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -1), -24, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
	if err != nil {
		return err
	}

	return r.fx.SetLookahead(clamp(getNodeNum(node, "lookaheadMs", 3), 0, 200))
}

func (r *lookaheadLimiterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

func (r *lookaheadLimiterChainRuntime) ProcessWithSidechain(program, sidechain []float64) {
	r.fx.ProcessInPlaceSidechain(program, sidechain)
}

type gateChainRuntime struct {
	fx *dynamics.Gate
}

func (r *gateChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -40), -80, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 10), 1, 100))
	if err != nil {
		return err
	}

	err = r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24))
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 0.1), 0.1, 1000))
	if err != nil {
		return err
	}

	err = r.fx.SetHold(clamp(getNodeNum(node, "holdMs", 50), 0, 5000))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
	if err != nil {
		return err
	}

	return r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -80), -120, 0))
}

func (r *gateChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type expanderChainRuntime struct {
	fx *dynamics.Expander
}

func (r *expanderChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -35), -80, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 2), 1, 100))
	if err != nil {
		return err
	}

	err = r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24))
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 1), 0.1, 1000))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
	if err != nil {
		return err
	}

	err = r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -60), -120, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetTopology(normalizeDynamicsTopology(node.Str["topology"]))
	if err != nil {
		return err
	}

	err = r.fx.SetDetectorMode(normalizeDynamicsDetectorMode(node.Str["detector"]))
	if err != nil {
		return err
	}

	return r.fx.SetRMSWindow(clamp(getNodeNum(node, "rmsWindowMs", 30), 1, 1000))
}

func (r *expanderChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type deesserChainRuntime struct {
	fx *dynamics.DeEsser
}

func (r *deesserChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetFrequency(clamp(getNodeNum(node, "freqHz", 6000), 1000, e.sampleRate*0.49))
	if err != nil {
		return err
	}

	err = r.fx.SetQ(clamp(getNodeNum(node, "q", 1.5), 0.1, 10))
	if err != nil {
		return err
	}

	err = r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -20), -80, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 4), 1, 100))
	if err != nil {
		return err
	}

	err = r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 3), 0, 12))
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 0.5), 0.01, 50))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 20), 1, 500))
	if err != nil {
		return err
	}

	err = r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -24), -60, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetMode(normalizeDeesserMode(node.Str["mode"]))
	if err != nil {
		return err
	}

	err = r.fx.SetDetector(normalizeDeesserDetector(node.Str["detector"]))
	if err != nil {
		return err
	}

	order := min(max(int(math.Round(getNodeNum(node, "filterOrder", 2))), 1), 4)

	err = r.fx.SetFilterOrder(order)
	if err != nil {
		return err
	}

	r.fx.SetListen(getNodeNum(node, "listen", 0) >= 0.5)

	return nil
}

func (r *deesserChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type transientShaperChainRuntime struct {
	fx *dynamics.TransientShaper
}

func (r *transientShaperChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetAttackAmount(clamp(getNodeNum(node, "attack", 0), -1, 1))
	if err != nil {
		return err
	}

	err = r.fx.SetSustainAmount(clamp(getNodeNum(node, "sustain", 0), -1, 1))
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 10), 0.1, 200))
	if err != nil {
		return err
	}

	return r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 120), 1, 2000))
}

func (r *transientShaperChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type multibandChainRuntime struct {
	fx        *dynamics.MultibandCompressor
	lastBands int
	lastOrder int
	lastC1    float64
	lastC2    float64
	lastSR    float64
}

//nolint:cyclop
func (r *multibandChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	bands := min(max(int(math.Round(getNodeNum(node, "bands", 3))), 2), 3)

	order := min(max(int(math.Round(getNodeNum(node, "order", 4))), 2), 24)

	if order%2 != 0 {
		order++
	}

	c1 := clamp(getNodeNum(node, "cross1Hz", 250), 40, e.sampleRate*0.2)
	c2 := clamp(getNodeNum(node, "cross2Hz", 3000), c1+100, e.sampleRate*0.45)

	rebuild := r.fx == nil ||
		r.lastBands != bands ||
		r.lastOrder != order ||
		math.Abs(r.lastC1-c1) > 1e-9 ||
		math.Abs(r.lastC2-c2) > 1e-9 ||
		math.Abs(r.lastSR-e.sampleRate) > 1e-9

	if rebuild {
		freqs := []float64{c1}
		if bands == 3 {
			freqs = append(freqs, c2)
		}

		fx, err := dynamics.NewMultibandCompressor(freqs, order, e.sampleRate)
		if err != nil {
			return err
		}

		r.fx = fx
		r.lastBands = bands
		r.lastOrder = order
		r.lastC1 = c1
		r.lastC2 = c2
		r.lastSR = e.sampleRate
	}

	// Band 1 (low)
	err := r.fx.SetBandThreshold(0, clamp(getNodeNum(node, "lowThresholdDB", -20), -80, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetBandRatio(0, clamp(getNodeNum(node, "lowRatio", 2.5), 1, 20))
	if err != nil {
		return err
	}

	// Band 2 (mid / high for 2-band)
	err = r.fx.SetBandThreshold(1, clamp(getNodeNum(node, "midThresholdDB", -18), -80, 0))
	if err != nil {
		return err
	}

	err = r.fx.SetBandRatio(1, clamp(getNodeNum(node, "midRatio", 3.0), 1, 20))
	if err != nil {
		return err
	}

	// Optional band 3 (high)
	if bands == 3 {
		err = r.fx.SetBandThreshold(2, clamp(getNodeNum(node, "highThresholdDB", -14), -80, 0))
		if err != nil {
			return err
		}

		err = r.fx.SetBandRatio(2, clamp(getNodeNum(node, "highRatio", 4.0), 1, 20))
		if err != nil {
			return err
		}
	}

	attack := clamp(getNodeNum(node, "attackMs", 8), 0.1, 1000)
	release := clamp(getNodeNum(node, "releaseMs", 120), 1, 5000)
	knee := clamp(getNodeNum(node, "kneeDB", 6), 0, 24)
	makeup := clamp(getNodeNum(node, "makeupGainDB", 0), 0, 24)
	autoMakeup := getNodeNum(node, "autoMakeup", 0) >= 0.5

	for b := 0; b < r.fx.NumBands(); b++ {
		err := r.fx.SetBandAttack(b, attack)
		if err != nil {
			return err
		}

		err = r.fx.SetBandRelease(b, release)
		if err != nil {
			return err
		}

		err = r.fx.SetBandKnee(b, knee)
		if err != nil {
			return err
		}

		err = r.fx.SetBandAutoMakeup(b, autoMakeup)
		if err != nil {
			return err
		}

		if !autoMakeup {
			err = r.fx.SetBandMakeupGain(b, makeup)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *multibandChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if r.fx == nil {
		return
	}

	r.fx.ProcessInPlace(block)
}

func normalizeChainFilterFamily(raw, nodeType string) string {
	if nodeType == "filter-moog" {
		return "moog"
	}

	family := strings.ToLower(strings.TrimSpace(raw))
	if family == "" {
		return "rbj"
	}

	switch family {
	case "rbj", "butterworth", "bessel", "chebyshev1", "chebyshev2", "elliptic", "moog":
		return family
	default:
		return "rbj"
	}
}

func normalizeChainFilterKind(nodeType, raw string) string {
	if nodeType == "filter-moog" {
		return "lowpass"
	}

	kind := normalizeEQType("mid", raw)
	if strings.TrimSpace(raw) != "" {
		return kind
	}

	switch nodeType {
	case "filter-highpass":
		return "highpass"
	case "filter-bandpass":
		return "bandpass"
	case "filter-notch":
		return "notch"
	case "filter-allpass":
		return "allpass"
	case "filter-peak":
		return "peak"
	case "filter-lowshelf":
		return "lowshelf"
	case "filter-highshelf":
		return "highshelf"
	default:
		return "lowpass"
	}
}

func moogOversamplingFromOrder(order int) int {
	switch {
	case order >= 12:
		return 8
	case order >= 8:
		return 4
	case order >= 4:
		return 2
	default:
		return 1
	}
}

//nolint:cyclop
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

func normalizeSpectralFreezePhaseMode(raw string) effects.SpectralFreezePhaseMode {
	switch raw {
	case "hold":
		return effects.SpectralFreezePhaseHold
	case "advance":
		fallthrough
	default:
		return effects.SpectralFreezePhaseAdvance
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

// convReverbChainRuntime handles the "reverb-conv" node type using partitioned convolution.
type convReverbChainRuntime struct {
	fx      *reverb.ConvolutionReverb
	irIndex int
}

func (r *convReverbChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	irIndex := int(getNodeNum(node, "irIndex", 0))
	wet := getNodeNum(node, "wet", 0.35)

	if r.fx == nil || r.irIndex != irIndex {
		if e.irLib == nil {
			return nil
		}

		ir := e.irLib.GetIR(irIndex)
		if ir == nil || len(ir.Samples) == 0 {
			return nil
		}

		ch0 := ir.Samples[0]
		kernel := make([]float64, len(ch0))
		copy(kernel, ch0)

		if len(ir.Samples) > 1 {
			ch1 := ir.Samples[1]

			n := min(len(ch0), len(ch1))
			for i := range n {
				kernel[i] = (ch0[i] + ch1[i]) * 0.5
			}
		}

		cr, err := reverb.NewConvolutionReverb(kernel, 7) // 128-sample latency
		if err != nil {
			return err
		}

		r.fx = cr
		r.irIndex = irIndex
	}

	if r.fx != nil {
		r.fx.SetWetDry(wet, 1.0)
	}

	return nil
}

func (r *convReverbChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if r.fx == nil {
		return
	}

	_ = r.fx.ProcessInPlace(block)
}

// ---------------------------------------------------------------------------
// vocoder
// ---------------------------------------------------------------------------

type vocoderChainRuntime struct {
	fx         *effects.Vocoder
	carrierBuf []float64 // reusable buffer for the carrier input
}

func (r *vocoderChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	err := r.fx.SetSampleRate(e.sampleRate)
	if err != nil {
		return err
	}

	err = r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 0.5), 0.01, 100))
	if err != nil {
		return err
	}

	err = r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 2.0), 0.01, 1000))
	if err != nil {
		return err
	}

	err = r.fx.SetInputLevel(clamp(getNodeNum(node, "inputLevel", 0), 0, 10))
	if err != nil {
		return err
	}

	err = r.fx.SetSynthLevel(clamp(getNodeNum(node, "synthLevel", 0), 0, 10))
	if err != nil {
		return err
	}

	return r.fx.SetVocoderLevel(clamp(getNodeNum(node, "vocoderLevel", 1), 0, 10))
}

func (r *vocoderChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	// Single-input fallback: use the same signal as both modulator and carrier.
	if len(r.carrierBuf) < len(block) {
		r.carrierBuf = make([]float64, len(block))
	}

	copy(r.carrierBuf, block)
	_ = r.fx.ProcessBlock(block, r.carrierBuf, block)
}

func (r *vocoderChainRuntime) processVocoder(modulator, carrier []float64) {
	_ = r.fx.ProcessBlock(modulator, carrier, modulator)
}
