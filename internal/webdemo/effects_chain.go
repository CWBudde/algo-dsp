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
	From string `json:"from"`
	To   string `json:"to"`
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
	Incoming map[string][]string
	Outgoing map[string][]string
	Order    []string
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
	registerChainEffectFactory("dyn-gate", func(e *Engine) (chainEffectRuntime, error) {
		fx, err := dynamics.NewGate(e.sampleRate)
		if err != nil {
			return nil, err
		}
		return &gateChainRuntime{fx: fx}, nil
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

	incoming := make(map[string][]string, len(nodes))
	outgoing := make(map[string][]string, len(nodes))
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
		outgoing[c.From] = append(outgoing[c.From], c.To)
		incoming[c.To] = append(incoming[c.To], c.From)
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
		for _, to := range outgoing[id] {
			indegree[to]--
			if indegree[to] == 0 {
				queue = append(queue, to)
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
func (e *Engine) syncChainEffectNodes(graph *compiledChainGraph) error {
	if graph == nil {
		e.chainNodes = nil
		return nil
	}
	if e.chainNodes == nil {
		e.chainNodes = map[string]*chainNodeRuntime{}
	}
	seen := map[string]struct{}{}
	for _, node := range graph.Nodes {
		if node.Type == "_input" || node.Type == "_output" || node.Type == "split" || node.Type == "sum" {
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
		if err := rt.effect.Configure(e, node); err != nil {
			return err
		}
	}
	for id := range e.chainNodes {
		if _, ok := seen[id]; !ok {
			delete(e.chainNodes, id)
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

type chorusChainRuntime struct {
	fx *modulation.Chorus
}

func (r *chorusChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetMix(clamp(getNodeNum(node, "mix", 0.18), 0, 1)); err != nil {
		return err
	}
	if err := r.fx.SetDepth(clamp(getNodeNum(node, "depth", 0.003), 0, 0.01)); err != nil {
		return err
	}
	if err := r.fx.SetSpeedHz(clamp(getNodeNum(node, "speedHz", 0.35), 0.05, 5)); err != nil {
		return err
	}
	stages := int(math.Round(getNodeNum(node, "stages", 3)))
	if stages < 1 {
		stages = 1
	}
	if stages > 6 {
		stages = 6
	}
	return r.fx.SetStages(stages)
}

func (r *chorusChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type flangerChainRuntime struct {
	fx *modulation.Flanger
}

func (r *flangerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetRateHz(clamp(getNodeNum(node, "rateHz", 0.25), 0.05, 5)); err != nil {
		return err
	}
	if err := r.fx.SetDepthSeconds(0); err != nil {
		return err
	}
	if err := r.fx.SetBaseDelaySeconds(clamp(getNodeNum(node, "baseDelay", 0.001), 0.0001, 0.01)); err != nil {
		return err
	}
	depth := clamp(getNodeNum(node, "depth", 0.0015), 0, 0.0099)
	if err := r.fx.SetDepthSeconds(depth); err != nil {
		return err
	}
	if err := r.fx.SetFeedback(clamp(getNodeNum(node, "feedback", 0.25), -0.99, 0.99)); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 0.5), 0, 1))
}

func (r *flangerChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type ringModChainRuntime struct {
	fx *modulation.RingModulator
}

func (r *ringModChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetCarrierHz(clamp(getNodeNum(node, "carrierHz", 440), 1, e.sampleRate*0.49)); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
}

func (r *ringModChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type bitCrusherChainRuntime struct {
	fx *effects.BitCrusher
}

func (r *bitCrusherChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetBitDepth(clamp(getNodeNum(node, "bitDepth", 8), 1, 32)); err != nil {
		return err
	}
	ds := int(math.Round(getNodeNum(node, "downsample", 4)))
	if ds < 1 {
		ds = 1
	}
	if ds > 256 {
		ds = 256
	}
	if err := r.fx.SetDownsample(ds); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
}

func (r *bitCrusherChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type widenerChainRuntime struct {
	fx *spatial.StereoWidener
}

func (r *widenerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetWidth(clamp(getNodeNum(node, "width", 1), 0, 4)); err != nil {
		return err
	}
	return r.fx.SetBassMonoFreq(0)
}

func (r *widenerChainRuntime) Process(e *Engine, node compiledChainNode, block []float64) {
	e.processNodeWidenerMonoInPlace(block, r.fx, clamp(getNodeNum(node, "mix", 0.5), 0, 1))
}

type phaserChainRuntime struct {
	fx *modulation.Phaser
}

func (r *phaserChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetRateHz(clamp(getNodeNum(node, "rateHz", 0.4), 0.05, 5)); err != nil {
		return err
	}
	minHz := clamp(getNodeNum(node, "minFreqHz", 300), 20, e.sampleRate*0.45)
	maxHz := clamp(getNodeNum(node, "maxFreqHz", 1600), minHz+1, e.sampleRate*0.49)
	if err := r.fx.SetFrequencyRangeHz(minHz, maxHz); err != nil {
		return err
	}
	stages := int(math.Round(getNodeNum(node, "stages", 6)))
	if stages < 1 {
		stages = 1
	}
	if stages > 12 {
		stages = 12
	}
	if err := r.fx.SetStages(stages); err != nil {
		return err
	}
	if err := r.fx.SetFeedback(clamp(getNodeNum(node, "feedback", 0.2), -0.99, 0.99)); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 0.5), 0, 1))
}

func (r *phaserChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type tremoloChainRuntime struct {
	fx *modulation.Tremolo
}

func (r *tremoloChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetRateHz(clamp(getNodeNum(node, "rateHz", 4), 0.05, 20)); err != nil {
		return err
	}
	if err := r.fx.SetDepth(clamp(getNodeNum(node, "depth", 0.6), 0, 1)); err != nil {
		return err
	}
	if err := r.fx.SetSmoothingMs(clamp(getNodeNum(node, "smoothingMs", 5), 0, 200)); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
}

func (r *tremoloChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type delayChainRuntime struct {
	fx *effects.Delay
}

func (r *delayChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetTime(clamp(getNodeNum(node, "time", 0.25), 0.001, 2)); err != nil {
		return err
	}
	if err := r.fx.SetFeedback(clamp(getNodeNum(node, "feedback", 0.35), 0, 0.99)); err != nil {
		return err
	}
	return r.fx.SetMix(clamp(getNodeNum(node, "mix", 0.25), 0, 1))
}

func (r *delayChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type filterChainRuntime struct {
	fx *biquad.Chain
}

func (r *filterChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	family := normalizeEQFamily(node.Str["family"])
	kind := normalizeEQType("mid", node.Str["kind"])
	family = normalizeEQFamilyForType(kind, family)
	order := normalizeEQOrder(kind, family, int(math.Round(getNodeNum(node, "order", 2))))
	freq := clamp(getNodeNum(node, "freq", 1200), 20, e.sampleRate*0.49)
	gainDB := clamp(getNodeNum(node, "gain", 0), -24, 24)
	shape := clampEQShape(kind, family, freq, e.sampleRate, getNodeNum(node, "q", 0.707))
	r.fx = buildEQChain(family, kind, order, freq, gainDB, shape, e.sampleRate)
	return nil
}

func (r *filterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if r.fx != nil {
		r.fx.ProcessBlock(block)
	}
}

type bassChainRuntime struct {
	fx *effects.HarmonicBass
}

func (r *bassChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetFrequency(clamp(getNodeNum(node, "frequency", 80), 10, 500)); err != nil {
		return err
	}
	if err := r.fx.SetInputLevel(clamp(getNodeNum(node, "inputGain", 1), 0, 2)); err != nil {
		return err
	}
	if err := r.fx.SetHighFrequencyLevel(clamp(getNodeNum(node, "highGain", 1), 0, 2)); err != nil {
		return err
	}
	if err := r.fx.SetOriginalBassLevel(clamp(getNodeNum(node, "original", 1), 0, 2)); err != nil {
		return err
	}
	if err := r.fx.SetHarmonicBassLevel(clamp(getNodeNum(node, "harmonic", 0), 0, 2)); err != nil {
		return err
	}
	if err := r.fx.SetDecay(clamp(getNodeNum(node, "decay", 0), -1, 1)); err != nil {
		return err
	}
	if err := r.fx.SetResponse(clamp(getNodeNum(node, "responseMs", 20), 1, 200)); err != nil {
		return err
	}
	hp := int(math.Round(getNodeNum(node, "highpass", 0)))
	if hp < 0 {
		hp = 0
	}
	if hp > 2 {
		hp = 2
	}
	return r.fx.SetHighpassMode(effects.HighpassSelect(hp))
}

func (r *bassChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type timePitchChainRuntime struct {
	fx *pitch.PitchShifter
}

func (r *timePitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetPitchSemitones(clamp(getNodeNum(node, "semitones", 0), -24, 24)); err != nil {
		return err
	}
	seq := clamp(getNodeNum(node, "sequence", 40), 20, 120)
	if err := r.fx.SetSequence(seq); err != nil {
		return err
	}
	ov := clamp(getNodeNum(node, "overlap", 10), 4, 60)
	if ov >= seq {
		ov = seq - 1
	}
	if err := r.fx.SetOverlap(ov); err != nil {
		return err
	}
	return r.fx.SetSearch(clamp(getNodeNum(node, "search", 15), 2, 40))
}

func (r *timePitchChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralPitchChainRuntime struct {
	fx *pitch.SpectralPitchShifter
}

func (r *spectralPitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetPitchSemitones(clamp(getNodeNum(node, "semitones", 0), -24, 24)); err != nil {
		return err
	}
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(getNodeNum(node, "frameSize", 1024))))
	if err := r.fx.SetFrameSize(frame); err != nil {
		return err
	}
	hop := int(math.Round(float64(frame) * clamp(getNodeNum(node, "hopRatio", 0.25), 0.01, 0.99)))
	if hop < 1 {
		hop = 1
	}
	if hop >= frame {
		hop = frame - 1
	}
	return r.fx.SetAnalysisHop(hop)
}

func (r *spectralPitchChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type reverbChainRuntime struct {
	freeverb *reverb.Reverb
	fdn      *reverb.FDNReverb
}

func (r *reverbChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	model := node.Str["model"]
	if model != "fdn" && model != "freeverb" {
		model = "freeverb"
	}
	if model == "fdn" {
		if err := r.fdn.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := r.fdn.SetWet(clamp(getNodeNum(node, "wet", 0.22), 0, 1.5)); err != nil {
			return err
		}
		if err := r.fdn.SetDry(clamp(getNodeNum(node, "dry", 1), 0, 1.5)); err != nil {
			return err
		}
		if err := r.fdn.SetRT60(clamp(getNodeNum(node, "rt60", 1.8), 0.2, 8)); err != nil {
			return err
		}
		if err := r.fdn.SetPreDelay(clamp(getNodeNum(node, "preDelay", 0.01), 0, 0.1)); err != nil {
			return err
		}
		if err := r.fdn.SetDamp(clamp(getNodeNum(node, "damp", 0.45), 0, 0.99)); err != nil {
			return err
		}
		if err := r.fdn.SetModDepth(clamp(getNodeNum(node, "modDepth", 0.002), 0, 0.01)); err != nil {
			return err
		}
		return r.fdn.SetModRate(clamp(getNodeNum(node, "modRate", 0.1), 0, 1))
	}
	r.freeverb.SetWet(clamp(getNodeNum(node, "wet", 0.22), 0, 1.5))
	r.freeverb.SetDry(clamp(getNodeNum(node, "dry", 1), 0, 1.5))
	r.freeverb.SetRoomSize(clamp(getNodeNum(node, "roomSize", 0.72), 0, 0.98))
	r.freeverb.SetDamp(clamp(getNodeNum(node, "damp", 0.45), 0, 0.99))
	r.freeverb.SetGain(clamp(getNodeNum(node, "gain", 0.015), 0, 0.1))
	return nil
}

func (r *reverbChainRuntime) Process(_ *Engine, node compiledChainNode, block []float64) {
	if node.Str["model"] == "fdn" {
		r.fdn.ProcessInPlace(block)
		return
	}
	r.freeverb.ProcessInPlace(block)
}

type compressorChainRuntime struct {
	fx *dynamics.Compressor
}

func (r *compressorChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -20), -60, 0)); err != nil {
		return err
	}
	if err := r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 4), 1, 100)); err != nil {
		return err
	}
	if err := r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24)); err != nil {
		return err
	}
	if err := r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 10), 0.1, 1000)); err != nil {
		return err
	}
	if err := r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
		return err
	}
	if err := r.fx.SetAutoMakeup(false); err != nil {
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
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -0.1), -24, 0)); err != nil {
		return err
	}
	return r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
}

func (r *limiterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type gateChainRuntime struct {
	fx *dynamics.Gate
}

func (r *gateChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -40), -80, 0)); err != nil {
		return err
	}
	if err := r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 10), 1, 100)); err != nil {
		return err
	}
	if err := r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24)); err != nil {
		return err
	}
	if err := r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 0.1), 0.1, 1000)); err != nil {
		return err
	}
	if err := r.fx.SetHold(clamp(getNodeNum(node, "holdMs", 50), 0, 5000)); err != nil {
		return err
	}
	if err := r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
		return err
	}
	return r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -80), -120, 0))
}

func (r *gateChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}
