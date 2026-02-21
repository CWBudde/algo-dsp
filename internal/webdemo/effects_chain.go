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
		e.chainNodes = map[string]*chainEffectNode{}
	}
	seen := map[string]struct{}{}
	for _, node := range graph.Nodes {
		if node.Type == "_input" || node.Type == "_output" || node.Type == "split" || node.Type == "sum" {
			continue
		}
		seen[node.ID] = struct{}{}
		rt := e.chainNodes[node.ID]
		if rt == nil || rt.effectType != node.Type {
			var err error
			rt, err = e.newChainEffectNode(node.Type)
			if err != nil {
				return err
			}
			if rt == nil {
				continue
			}
			e.chainNodes[node.ID] = rt
		}
		if err := e.configureChainEffectNode(rt, node); err != nil {
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

// newChainEffectNode is a factory that creates and initialises a new effect instance by type string.
func (e *Engine) newChainEffectNode(effectType string) (*chainEffectNode, error) {
	rt := &chainEffectNode{effectType: effectType}
	var err error
	switch effectType {
	case "chorus":
		rt.chorus, err = modulation.NewChorus()
	case "flanger":
		rt.flanger, err = modulation.NewFlanger(e.sampleRate)
	case "ringmod":
		rt.ringMod, err = modulation.NewRingModulator(e.sampleRate)
	case "bitcrusher":
		rt.crusher, err = effects.NewBitCrusher(e.sampleRate)
	case "widener":
		rt.widener, err = spatial.NewStereoWidener(e.sampleRate)
	case "phaser":
		rt.phaser, err = modulation.NewPhaser(e.sampleRate)
	case "tremolo":
		rt.tremolo, err = modulation.NewTremolo(e.sampleRate)
	case "delay":
		rt.delay, err = effects.NewDelay(e.sampleRate)
	case "filter":
		rt.filter = biquad.NewChain([]biquad.Coefficients{{B0: 1}})
	case "bass":
		rt.bass, err = effects.NewHarmonicBass(e.sampleRate)
	case "pitch-time":
		rt.tp, err = pitch.NewPitchShifter(e.sampleRate)
	case "pitch-spectral":
		rt.sp, err = pitch.NewSpectralPitchShifter(e.sampleRate)
	case "reverb":
		rt.reverb = reverb.NewReverb()
		rt.fdn, err = reverb.NewFDNReverb(e.sampleRate)
	case "dyn-compressor":
		rt.comp, err = dynamics.NewCompressor(e.sampleRate)
	case "dyn-limiter":
		rt.limiter, err = dynamics.NewLimiter(e.sampleRate)
	case "dyn-gate":
		rt.gate, err = dynamics.NewGate(e.sampleRate)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rt, nil
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

// configureChainEffectNode applies all effect parameters to the given runtime node instance.
func (e *Engine) configureChainEffectNode(rt *chainEffectNode, node compiledChainNode) error {
	switch node.Type {
	case "chorus":
		if err := rt.chorus.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.chorus.SetMix(clamp(getNodeNum(node, "mix", 0.18), 0, 1)); err != nil {
			return err
		}
		if err := rt.chorus.SetDepth(clamp(getNodeNum(node, "depth", 0.003), 0, 0.01)); err != nil {
			return err
		}
		if err := rt.chorus.SetSpeedHz(clamp(getNodeNum(node, "speedHz", 0.35), 0.05, 5)); err != nil {
			return err
		}
		stages := int(math.Round(getNodeNum(node, "stages", 3)))
		if stages < 1 {
			stages = 1
		}
		if stages > 6 {
			stages = 6
		}
		return rt.chorus.SetStages(stages)
	case "flanger":
		if err := rt.flanger.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.flanger.SetRateHz(clamp(getNodeNum(node, "rateHz", 0.25), 0.05, 5)); err != nil {
			return err
		}
		if err := rt.flanger.SetDepthSeconds(0); err != nil {
			return err
		}
		if err := rt.flanger.SetBaseDelaySeconds(clamp(getNodeNum(node, "baseDelay", 0.001), 0.0001, 0.01)); err != nil {
			return err
		}
		depth := clamp(getNodeNum(node, "depth", 0.0015), 0, 0.0099)
		if err := rt.flanger.SetDepthSeconds(depth); err != nil {
			return err
		}
		if err := rt.flanger.SetFeedback(clamp(getNodeNum(node, "feedback", 0.25), -0.99, 0.99)); err != nil {
			return err
		}
		return rt.flanger.SetMix(clamp(getNodeNum(node, "mix", 0.5), 0, 1))
	case "ringmod":
		if err := rt.ringMod.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.ringMod.SetCarrierHz(clamp(getNodeNum(node, "carrierHz", 440), 1, e.sampleRate*0.49)); err != nil {
			return err
		}
		return rt.ringMod.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
	case "bitcrusher":
		if err := rt.crusher.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.crusher.SetBitDepth(clamp(getNodeNum(node, "bitDepth", 8), 1, 32)); err != nil {
			return err
		}
		ds := int(math.Round(getNodeNum(node, "downsample", 4)))
		if ds < 1 {
			ds = 1
		}
		if ds > 256 {
			ds = 256
		}
		if err := rt.crusher.SetDownsample(ds); err != nil {
			return err
		}
		return rt.crusher.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
	case "widener":
		if err := rt.widener.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.widener.SetWidth(clamp(getNodeNum(node, "width", 1), 0, 4)); err != nil {
			return err
		}
		return rt.widener.SetBassMonoFreq(0)
	case "phaser":
		if err := rt.phaser.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.phaser.SetRateHz(clamp(getNodeNum(node, "rateHz", 0.4), 0.05, 5)); err != nil {
			return err
		}
		minHz := clamp(getNodeNum(node, "minFreqHz", 300), 20, e.sampleRate*0.45)
		maxHz := clamp(getNodeNum(node, "maxFreqHz", 1600), minHz+1, e.sampleRate*0.49)
		if err := rt.phaser.SetFrequencyRangeHz(minHz, maxHz); err != nil {
			return err
		}
		stages := int(math.Round(getNodeNum(node, "stages", 6)))
		if stages < 1 {
			stages = 1
		}
		if stages > 12 {
			stages = 12
		}
		if err := rt.phaser.SetStages(stages); err != nil {
			return err
		}
		if err := rt.phaser.SetFeedback(clamp(getNodeNum(node, "feedback", 0.2), -0.99, 0.99)); err != nil {
			return err
		}
		return rt.phaser.SetMix(clamp(getNodeNum(node, "mix", 0.5), 0, 1))
	case "tremolo":
		if err := rt.tremolo.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.tremolo.SetRateHz(clamp(getNodeNum(node, "rateHz", 4), 0.05, 20)); err != nil {
			return err
		}
		if err := rt.tremolo.SetDepth(clamp(getNodeNum(node, "depth", 0.6), 0, 1)); err != nil {
			return err
		}
		if err := rt.tremolo.SetSmoothingMs(clamp(getNodeNum(node, "smoothingMs", 5), 0, 200)); err != nil {
			return err
		}
		return rt.tremolo.SetMix(clamp(getNodeNum(node, "mix", 1), 0, 1))
	case "delay":
		if err := rt.delay.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.delay.SetTime(clamp(getNodeNum(node, "time", 0.25), 0.001, 2)); err != nil {
			return err
		}
		if err := rt.delay.SetFeedback(clamp(getNodeNum(node, "feedback", 0.35), 0, 0.99)); err != nil {
			return err
		}
		return rt.delay.SetMix(clamp(getNodeNum(node, "mix", 0.25), 0, 1))
	case "filter":
		family := normalizeEQFamily(node.Str["family"])
		kind := normalizeEQType("mid", node.Str["kind"])
		family = normalizeEQFamilyForType(kind, family)
		order := normalizeEQOrder(kind, family, int(math.Round(getNodeNum(node, "order", 2))))
		freq := clamp(getNodeNum(node, "freq", 1200), 20, e.sampleRate*0.49)
		gainDB := clamp(getNodeNum(node, "gain", 0), -24, 24)
		shape := clampEQShape(kind, family, freq, e.sampleRate, getNodeNum(node, "q", 0.707))
		rt.filter = buildEQChain(family, kind, order, freq, gainDB, shape, e.sampleRate)
		return nil
	case "bass":
		if err := rt.bass.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.bass.SetFrequency(clamp(getNodeNum(node, "frequency", 80), 10, 500)); err != nil {
			return err
		}
		if err := rt.bass.SetInputLevel(clamp(getNodeNum(node, "inputGain", 1), 0, 2)); err != nil {
			return err
		}
		if err := rt.bass.SetHighFrequencyLevel(clamp(getNodeNum(node, "highGain", 1), 0, 2)); err != nil {
			return err
		}
		if err := rt.bass.SetOriginalBassLevel(clamp(getNodeNum(node, "original", 1), 0, 2)); err != nil {
			return err
		}
		if err := rt.bass.SetHarmonicBassLevel(clamp(getNodeNum(node, "harmonic", 0), 0, 2)); err != nil {
			return err
		}
		if err := rt.bass.SetDecay(clamp(getNodeNum(node, "decay", 0), -1, 1)); err != nil {
			return err
		}
		if err := rt.bass.SetResponse(clamp(getNodeNum(node, "responseMs", 20), 1, 200)); err != nil {
			return err
		}
		hp := int(math.Round(getNodeNum(node, "highpass", 0)))
		if hp < 0 {
			hp = 0
		}
		if hp > 2 {
			hp = 2
		}
		return rt.bass.SetHighpassMode(effects.HighpassSelect(hp))
	case "pitch-time":
		if err := rt.tp.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.tp.SetPitchSemitones(clamp(getNodeNum(node, "semitones", 0), -24, 24)); err != nil {
			return err
		}
		seq := clamp(getNodeNum(node, "sequence", 40), 20, 120)
		if err := rt.tp.SetSequence(seq); err != nil {
			return err
		}
		ov := clamp(getNodeNum(node, "overlap", 10), 4, 60)
		if ov >= seq {
			ov = seq - 1
		}
		if err := rt.tp.SetOverlap(ov); err != nil {
			return err
		}
		return rt.tp.SetSearch(clamp(getNodeNum(node, "search", 15), 2, 40))
	case "pitch-spectral":
		if err := rt.sp.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.sp.SetPitchSemitones(clamp(getNodeNum(node, "semitones", 0), -24, 24)); err != nil {
			return err
		}
		frame := sanitizeSpectralPitchFrameSize(int(math.Round(getNodeNum(node, "frameSize", 1024))))
		if err := rt.sp.SetFrameSize(frame); err != nil {
			return err
		}
		hop := int(math.Round(float64(frame) * clamp(getNodeNum(node, "hopRatio", 0.25), 0.01, 0.99)))
		if hop < 1 {
			hop = 1
		}
		if hop >= frame {
			hop = frame - 1
		}
		return rt.sp.SetAnalysisHop(hop)
	case "reverb":
		model := node.Str["model"]
		if model != "fdn" && model != "freeverb" {
			model = "freeverb"
		}
		if model == "fdn" {
			if err := rt.fdn.SetSampleRate(e.sampleRate); err != nil {
				return err
			}
			if err := rt.fdn.SetWet(clamp(getNodeNum(node, "wet", 0.22), 0, 1.5)); err != nil {
				return err
			}
			if err := rt.fdn.SetDry(clamp(getNodeNum(node, "dry", 1), 0, 1.5)); err != nil {
				return err
			}
			if err := rt.fdn.SetRT60(clamp(getNodeNum(node, "rt60", 1.8), 0.2, 8)); err != nil {
				return err
			}
			if err := rt.fdn.SetPreDelay(clamp(getNodeNum(node, "preDelay", 0.01), 0, 0.1)); err != nil {
				return err
			}
			if err := rt.fdn.SetDamp(clamp(getNodeNum(node, "damp", 0.45), 0, 0.99)); err != nil {
				return err
			}
			if err := rt.fdn.SetModDepth(clamp(getNodeNum(node, "modDepth", 0.002), 0, 0.01)); err != nil {
				return err
			}
			return rt.fdn.SetModRate(clamp(getNodeNum(node, "modRate", 0.1), 0, 1))
		}
		rt.reverb.SetWet(clamp(getNodeNum(node, "wet", 0.22), 0, 1.5))
		rt.reverb.SetDry(clamp(getNodeNum(node, "dry", 1), 0, 1.5))
		rt.reverb.SetRoomSize(clamp(getNodeNum(node, "roomSize", 0.72), 0, 0.98))
		rt.reverb.SetDamp(clamp(getNodeNum(node, "damp", 0.45), 0, 0.99))
		rt.reverb.SetGain(clamp(getNodeNum(node, "gain", 0.015), 0, 0.1))
	case "dyn-compressor":
		if err := rt.comp.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.comp.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -20), -60, 0)); err != nil {
			return err
		}
		if err := rt.comp.SetRatio(clamp(getNodeNum(node, "ratio", 4), 1, 100)); err != nil {
			return err
		}
		if err := rt.comp.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24)); err != nil {
			return err
		}
		if err := rt.comp.SetAttack(clamp(getNodeNum(node, "attackMs", 10), 0.1, 1000)); err != nil {
			return err
		}
		if err := rt.comp.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
			return err
		}
		if err := rt.comp.SetAutoMakeup(false); err != nil {
			return err
		}
		return rt.comp.SetMakeupGain(clamp(getNodeNum(node, "makeupGainDB", 0), 0, 24))
	case "dyn-limiter":
		if err := rt.limiter.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.limiter.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -0.1), -24, 0)); err != nil {
			return err
		}
		return rt.limiter.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000))
	case "dyn-gate":
		if err := rt.gate.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := rt.gate.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -40), -80, 0)); err != nil {
			return err
		}
		if err := rt.gate.SetRatio(clamp(getNodeNum(node, "ratio", 10), 1, 100)); err != nil {
			return err
		}
		if err := rt.gate.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24)); err != nil {
			return err
		}
		if err := rt.gate.SetAttack(clamp(getNodeNum(node, "attackMs", 0.1), 0.1, 1000)); err != nil {
			return err
		}
		if err := rt.gate.SetHold(clamp(getNodeNum(node, "holdMs", 50), 0, 5000)); err != nil {
			return err
		}
		if err := rt.gate.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
			return err
		}
		return rt.gate.SetRange(clamp(getNodeNum(node, "rangeDB", -80), -120, 0))
	}
	return nil
}
