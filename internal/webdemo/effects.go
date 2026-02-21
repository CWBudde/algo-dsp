package webdemo

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

type chainGraphNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Bypassed bool   `json:"bypassed"`
	Fixed    bool   `json:"fixed"`
	Params   any    `json:"params"`
}

type chainGraphConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type chainGraphState struct {
	Nodes       []chainGraphNode       `json:"nodes"`
	Connections []chainGraphConnection `json:"connections"`
}

type compiledChainNode struct {
	ID       string
	Type     string
	Bypassed bool
	Num      map[string]float64
	Str      map[string]string
}

type compiledChainGraph struct {
	Nodes    map[string]compiledChainNode
	Incoming map[string][]string
	Outgoing map[string][]string
	Order    []string
}

type chainEffectNode struct {
	effectType string
	chorus     *modulation.Chorus
	flanger    *modulation.Flanger
	ringMod    *modulation.RingModulator
	crusher    *effects.BitCrusher
	widener    *spatial.StereoWidener
	phaser     *modulation.Phaser
	tremolo    *modulation.Tremolo
	delay      *effects.Delay
	bass       *effects.HarmonicBass
	tp         *pitch.PitchShifter
	sp         *pitch.SpectralPitchShifter
	reverb     *reverb.Reverb
	fdn        *reverb.FDNReverb
}

// SetCompressor updates compressor parameters.
func (e *Engine) SetCompressor(p CompressorParams) error {
	prevEnabled := e.compParams.Enabled
	p.ThresholdDB = clamp(p.ThresholdDB, -60, 0)
	p.Ratio = clamp(p.Ratio, 1, 100)
	p.KneeDB = clamp(p.KneeDB, 0, 24)
	p.AttackMs = clamp(p.AttackMs, 0.1, 1000)
	p.ReleaseMs = clamp(p.ReleaseMs, 1, 5000)
	p.MakeupGainDB = clamp(p.MakeupGainDB, 0, 24)

	e.compParams = p
	if err := e.rebuildCompressor(); err != nil {
		return err
	}
	if prevEnabled && !p.Enabled {
		e.compressor.Reset()
	}
	return nil
}

// SetLimiter updates limiter parameters.
func (e *Engine) SetLimiter(p LimiterParams) error {
	prevEnabled := e.limParams.Enabled
	p.Threshold = clamp(p.Threshold, -24, 0)
	p.Release = clamp(p.Release, 1, 5000)

	e.limParams = p
	if err := e.rebuildLimiter(); err != nil {
		return err
	}
	if prevEnabled && !p.Enabled {
		e.limiter.Reset()
	}
	return nil
}

// SetEffects updates effect settings.
func (e *Engine) SetEffects(p EffectsParams) error {
	prevChorusEnabled := e.effects.ChorusEnabled
	prevFlangerEnabled := e.effects.FlangerEnabled
	prevRingModEnabled := e.effects.RingModEnabled
	prevCrusherEnabled := e.effects.BitCrusherEnabled
	prevWidenerEnabled := e.effects.WidenerEnabled
	prevPhaserEnabled := e.effects.PhaserEnabled
	prevTremoloEnabled := e.effects.TremoloEnabled
	prevDelayEnabled := e.effects.DelayEnabled
	prevReverbEnabled := e.effects.ReverbEnabled
	prevReverbModel := e.effects.ReverbModel
	prevBassEnabled := e.effects.HarmonicBassEnabled
	prevTimePitchEnabled := e.effects.TimePitchEnabled
	prevSpectralPitchEnabled := e.effects.SpectralPitchEnabled

	p.ChorusMix = clamp(p.ChorusMix, 0, 1)
	p.ChorusDepth = clamp(p.ChorusDepth, 0, 0.01)
	p.ChorusSpeedHz = clamp(p.ChorusSpeedHz, 0.05, 5)
	if p.ChorusStages < 1 {
		p.ChorusStages = 1
	}
	if p.ChorusStages > 6 {
		p.ChorusStages = 6
	}
	p.FlangerRateHz = clamp(p.FlangerRateHz, 0.05, 5)
	p.FlangerDepth = clamp(p.FlangerDepth, 0, 0.0099)
	p.FlangerBaseDelay = clamp(p.FlangerBaseDelay, 0.0001, 0.01)
	if p.FlangerBaseDelay+p.FlangerDepth > 0.01 {
		p.FlangerDepth = 0.01 - p.FlangerBaseDelay
	}
	p.FlangerFeedback = clamp(p.FlangerFeedback, -0.99, 0.99)
	p.FlangerMix = clamp(p.FlangerMix, 0, 1)
	p.RingModCarrierHz = clamp(p.RingModCarrierHz, 1, e.sampleRate*0.49)
	p.RingModMix = clamp(p.RingModMix, 0, 1)
	p.BitCrusherBitDepth = clamp(p.BitCrusherBitDepth, 1, 32)
	if p.BitCrusherDownsample < 1 {
		p.BitCrusherDownsample = 1
	}
	if p.BitCrusherDownsample > 256 {
		p.BitCrusherDownsample = 256
	}
	p.BitCrusherMix = clamp(p.BitCrusherMix, 0, 1)
	p.WidenerWidth = clamp(p.WidenerWidth, 0, 4)
	p.WidenerMix = clamp(p.WidenerMix, 0, 1)
	p.PhaserRateHz = clamp(p.PhaserRateHz, 0.05, 5)
	p.PhaserMinFreqHz = clamp(p.PhaserMinFreqHz, 20, e.sampleRate*0.45)
	p.PhaserMaxFreqHz = clamp(p.PhaserMaxFreqHz, p.PhaserMinFreqHz+1, e.sampleRate*0.49)
	if p.PhaserStages < 1 {
		p.PhaserStages = 1
	}
	if p.PhaserStages > 12 {
		p.PhaserStages = 12
	}
	p.PhaserFeedback = clamp(p.PhaserFeedback, -0.99, 0.99)
	p.PhaserMix = clamp(p.PhaserMix, 0, 1)
	p.TremoloRateHz = clamp(p.TremoloRateHz, 0.05, 20)
	p.TremoloDepth = clamp(p.TremoloDepth, 0, 1)
	p.TremoloSmoothingMs = clamp(p.TremoloSmoothingMs, 0, 200)
	p.TremoloMix = clamp(p.TremoloMix, 0, 1)
	p.DelayTime = clamp(p.DelayTime, 0.001, 2.0)
	p.DelayFeedback = clamp(p.DelayFeedback, 0, 0.99)
	p.DelayMix = clamp(p.DelayMix, 0, 1)

	p.TimePitchSemitones = clamp(p.TimePitchSemitones, -24, 24)
	p.TimePitchSequence = clamp(p.TimePitchSequence, 20, 120)
	p.TimePitchOverlap = clamp(p.TimePitchOverlap, 4, 60)
	if p.TimePitchOverlap >= p.TimePitchSequence {
		p.TimePitchOverlap = p.TimePitchSequence - 1
	}
	p.TimePitchSearch = clamp(p.TimePitchSearch, 2, 40)

	p.SpectralPitchSemitones = clamp(p.SpectralPitchSemitones, -24, 24)
	p.SpectralPitchFrameSize = sanitizeSpectralPitchFrameSize(p.SpectralPitchFrameSize)
	if p.SpectralPitchHop < 1 || p.SpectralPitchHop >= p.SpectralPitchFrameSize {
		p.SpectralPitchHop = p.SpectralPitchFrameSize / 4
	}
	if p.SpectralPitchHop < 1 {
		p.SpectralPitchHop = 1
	}

	if p.ReverbModel != "fdn" && p.ReverbModel != "freeverb" {
		p.ReverbModel = "freeverb"
	}
	p.ReverbWet = clamp(p.ReverbWet, 0, 1.5)
	p.ReverbDry = clamp(p.ReverbDry, 0, 1.5)
	p.ReverbRoomSize = clamp(p.ReverbRoomSize, 0, 0.98)
	p.ReverbDamp = clamp(p.ReverbDamp, 0, 0.99)
	p.ReverbGain = clamp(p.ReverbGain, 0, 0.1)
	p.ReverbRT60 = clamp(p.ReverbRT60, 0.2, 8)
	p.ReverbPreDelay = clamp(p.ReverbPreDelay, 0, 0.1)
	p.ReverbModDepth = clamp(p.ReverbModDepth, 0, 0.01)
	p.ReverbModRate = clamp(p.ReverbModRate, 0, 1)

	p.HarmonicBassFrequency = clamp(p.HarmonicBassFrequency, 10, 500)
	p.HarmonicBassInputGain = clamp(p.HarmonicBassInputGain, 0, 2)
	p.HarmonicBassHighGain = clamp(p.HarmonicBassHighGain, 0, 2)
	p.HarmonicBassOriginal = clamp(p.HarmonicBassOriginal, 0, 2)
	p.HarmonicBassHarmonic = clamp(p.HarmonicBassHarmonic, 0, 2)
	p.HarmonicBassDecay = clamp(p.HarmonicBassDecay, -1, 1)
	p.HarmonicBassResponseMs = clamp(p.HarmonicBassResponseMs, 1, 200)
	if p.HarmonicBassHighpass < 0 {
		p.HarmonicBassHighpass = 0
	}
	if p.HarmonicBassHighpass > 2 {
		p.HarmonicBassHighpass = 2
	}

	graph, err := parseChainGraph(p.ChainGraphJSON)
	if err != nil {
		return err
	}

	e.effects = p
	if err := e.rebuildEffects(); err != nil {
		return err
	}
	if err := e.syncChainEffectNodes(graph); err != nil {
		return err
	}
	e.chainGraph = graph
	if prevChorusEnabled && !p.ChorusEnabled {
		e.chorus.Reset()
	}
	if prevFlangerEnabled && !p.FlangerEnabled {
		e.flanger.Reset()
	}
	if prevRingModEnabled && !p.RingModEnabled {
		e.ringMod.Reset()
	}
	if prevCrusherEnabled && !p.BitCrusherEnabled {
		e.crusher.Reset()
	}
	if prevWidenerEnabled && !p.WidenerEnabled {
		e.widener.Reset()
	}
	if prevPhaserEnabled && !p.PhaserEnabled {
		e.phaser.Reset()
	}
	if prevTremoloEnabled && !p.TremoloEnabled {
		e.tremolo.Reset()
	}
	if prevDelayEnabled && !p.DelayEnabled {
		e.delay.Reset()
	}
	if prevReverbEnabled && !p.ReverbEnabled {
		e.reverb.Reset()
		e.fdn.Reset()
	}
	if prevReverbModel != p.ReverbModel {
		e.reverb.Reset()
		e.fdn.Reset()
	}
	if prevBassEnabled && !p.HarmonicBassEnabled {
		e.bass.Reset()
	}
	if prevTimePitchEnabled && !p.TimePitchEnabled {
		e.tp.Reset()
	}
	if prevSpectralPitchEnabled && !p.SpectralPitchEnabled {
		e.sp.Reset()
	}
	return nil
}

func (e *Engine) rebuildCompressor() error {
	if err := e.compressor.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.compressor.SetThreshold(e.compParams.ThresholdDB); err != nil {
		return err
	}
	if err := e.compressor.SetRatio(e.compParams.Ratio); err != nil {
		return err
	}
	if err := e.compressor.SetKnee(e.compParams.KneeDB); err != nil {
		return err
	}
	if err := e.compressor.SetAttack(e.compParams.AttackMs); err != nil {
		return err
	}
	if err := e.compressor.SetRelease(e.compParams.ReleaseMs); err != nil {
		return err
	}
	if e.compParams.AutoMakeup {
		if err := e.compressor.SetAutoMakeup(true); err != nil {
			return err
		}
	} else {
		if err := e.compressor.SetMakeupGain(e.compParams.MakeupGainDB); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) rebuildLimiter() error {
	if err := e.limiter.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.limiter.SetThreshold(e.limParams.Threshold); err != nil {
		return err
	}
	if err := e.limiter.SetRelease(e.limParams.Release); err != nil {
		return err
	}
	return nil
}

func (e *Engine) rebuildEffects() error {
	if err := e.chorus.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.ChorusEnabled {
		if err := e.chorus.SetMix(e.effects.ChorusMix); err != nil {
			return err
		}
		if err := e.chorus.SetDepth(e.effects.ChorusDepth); err != nil {
			return err
		}
		if err := e.chorus.SetSpeedHz(e.effects.ChorusSpeedHz); err != nil {
			return err
		}
		if err := e.chorus.SetStages(e.effects.ChorusStages); err != nil {
			return err
		}
	}
	if err := e.flanger.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.FlangerEnabled {
		if err := e.flanger.SetRateHz(e.effects.FlangerRateHz); err != nil {
			return err
		}
		// Apply timing in a transition-safe order to avoid invalid intermediate
		// base+depth combinations during whole-graph parameter updates.
		if err := e.flanger.SetDepthSeconds(0); err != nil {
			return err
		}
		if err := e.flanger.SetBaseDelaySeconds(e.effects.FlangerBaseDelay); err != nil {
			return err
		}
		if err := e.flanger.SetDepthSeconds(e.effects.FlangerDepth); err != nil {
			return err
		}
		if err := e.flanger.SetFeedback(e.effects.FlangerFeedback); err != nil {
			return err
		}
		if err := e.flanger.SetMix(e.effects.FlangerMix); err != nil {
			return err
		}
	}
	if err := e.ringMod.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.RingModEnabled {
		if err := e.ringMod.SetCarrierHz(e.effects.RingModCarrierHz); err != nil {
			return err
		}
		if err := e.ringMod.SetMix(e.effects.RingModMix); err != nil {
			return err
		}
	}
	if err := e.crusher.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.BitCrusherEnabled {
		if err := e.crusher.SetBitDepth(e.effects.BitCrusherBitDepth); err != nil {
			return err
		}
		if err := e.crusher.SetDownsample(e.effects.BitCrusherDownsample); err != nil {
			return err
		}
		if err := e.crusher.SetMix(e.effects.BitCrusherMix); err != nil {
			return err
		}
	}
	if err := e.widener.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.WidenerEnabled {
		if err := e.widener.SetWidth(e.effects.WidenerWidth); err != nil {
			return err
		}
		if err := e.widener.SetBassMonoFreq(0); err != nil {
			return err
		}
	}
	if err := e.phaser.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.PhaserEnabled {
		if err := e.phaser.SetRateHz(e.effects.PhaserRateHz); err != nil {
			return err
		}
		if err := e.phaser.SetFrequencyRangeHz(e.effects.PhaserMinFreqHz, e.effects.PhaserMaxFreqHz); err != nil {
			return err
		}
		if err := e.phaser.SetStages(e.effects.PhaserStages); err != nil {
			return err
		}
		if err := e.phaser.SetFeedback(e.effects.PhaserFeedback); err != nil {
			return err
		}
		if err := e.phaser.SetMix(e.effects.PhaserMix); err != nil {
			return err
		}
	}
	if err := e.tremolo.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.TremoloEnabled {
		if err := e.tremolo.SetRateHz(e.effects.TremoloRateHz); err != nil {
			return err
		}
		if err := e.tremolo.SetDepth(e.effects.TremoloDepth); err != nil {
			return err
		}
		if err := e.tremolo.SetSmoothingMs(e.effects.TremoloSmoothingMs); err != nil {
			return err
		}
		if err := e.tremolo.SetMix(e.effects.TremoloMix); err != nil {
			return err
		}
	}
	if err := e.delay.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if e.effects.DelayEnabled {
		if err := e.delay.SetTime(e.effects.DelayTime); err != nil {
			return err
		}
		if err := e.delay.SetFeedback(e.effects.DelayFeedback); err != nil {
			return err
		}
		if err := e.delay.SetMix(e.effects.DelayMix); err != nil {
			return err
		}
	}

	if e.effects.TimePitchEnabled {
		if err := e.tp.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := e.tp.SetPitchSemitones(e.effects.TimePitchSemitones); err != nil {
			return err
		}
		if err := e.tp.SetSequence(e.effects.TimePitchSequence); err != nil {
			return err
		}
		if err := e.tp.SetOverlap(e.effects.TimePitchOverlap); err != nil {
			return err
		}
		if err := e.tp.SetSearch(e.effects.TimePitchSearch); err != nil {
			return err
		}
	}

	if e.effects.SpectralPitchEnabled {
		if err := e.sp.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := e.sp.SetPitchSemitones(e.effects.SpectralPitchSemitones); err != nil {
			return err
		}
		if err := e.sp.SetFrameSize(e.effects.SpectralPitchFrameSize); err != nil {
			return err
		}
		if err := e.sp.SetAnalysisHop(e.effects.SpectralPitchHop); err != nil {
			return err
		}
	}

	if e.effects.ReverbEnabled {
		if e.effects.ReverbModel == "fdn" {
			if err := e.fdn.SetSampleRate(e.sampleRate); err != nil {
				return err
			}
			if err := e.fdn.SetWet(e.effects.ReverbWet); err != nil {
				return err
			}
			if err := e.fdn.SetDry(e.effects.ReverbDry); err != nil {
				return err
			}
			if err := e.fdn.SetRT60(e.effects.ReverbRT60); err != nil {
				return err
			}
			if err := e.fdn.SetPreDelay(e.effects.ReverbPreDelay); err != nil {
				return err
			}
			if err := e.fdn.SetDamp(e.effects.ReverbDamp); err != nil {
				return err
			}
			if err := e.fdn.SetModDepth(e.effects.ReverbModDepth); err != nil {
				return err
			}
			if err := e.fdn.SetModRate(e.effects.ReverbModRate); err != nil {
				return err
			}
		} else {
			e.reverb.SetWet(e.effects.ReverbWet)
			e.reverb.SetDry(e.effects.ReverbDry)
			e.reverb.SetRoomSize(e.effects.ReverbRoomSize)
			e.reverb.SetDamp(e.effects.ReverbDamp)
			e.reverb.SetGain(e.effects.ReverbGain)
		}
	}

	if e.effects.HarmonicBassEnabled {
		if err := e.bass.SetSampleRate(e.sampleRate); err != nil {
			return err
		}
		if err := e.bass.SetFrequency(e.effects.HarmonicBassFrequency); err != nil {
			return err
		}
		if err := e.bass.SetInputLevel(e.effects.HarmonicBassInputGain); err != nil {
			return err
		}
		if err := e.bass.SetHighFrequencyLevel(e.effects.HarmonicBassHighGain); err != nil {
			return err
		}
		if err := e.bass.SetOriginalBassLevel(e.effects.HarmonicBassOriginal); err != nil {
			return err
		}
		if err := e.bass.SetHarmonicBassLevel(e.effects.HarmonicBassHarmonic); err != nil {
			return err
		}
		if err := e.bass.SetDecay(e.effects.HarmonicBassDecay); err != nil {
			return err
		}
		if err := e.bass.SetResponse(e.effects.HarmonicBassResponseMs); err != nil {
			return err
		}
		if err := e.bass.SetHighpassMode(effects.HighpassSelect(e.effects.HarmonicBassHighpass)); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeSpectralPitchFrameSize(n int) int {
	if n < 256 {
		return 256
	}
	if n > 4096 {
		return 4096
	}
	if n > 0 && (n&(n-1)) == 0 {
		return n
	}

	lower := 256
	for lower < n {
		lower <<= 1
	}
	upper := lower
	lower >>= 1
	if lower < 256 {
		lower = 256
	}
	if upper > 4096 {
		upper = 4096
	}
	if n-lower <= upper-n {
		return lower
	}
	return upper
}

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

func (e *Engine) processEffectsByGraphInPlace(block []float64, g *compiledChainGraph) bool {
	if g == nil {
		return false
	}
	const inputID = "_input"
	const outputID = "_output"
	if _, ok := g.Nodes[inputID]; !ok {
		return false
	}
	if _, ok := g.Nodes[outputID]; !ok {
		return false
	}

	if e.chainOutBuf == nil {
		e.chainOutBuf = make(map[string][]float64, len(g.Nodes))
	}
	buffers := e.chainOutBuf
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
		if len(parents) == 0 {
			for i := range dst {
				dst[i] = 0
			}
		} else if len(parents) == 1 {
			copy(dst, buffers[parents[0]])
		} else {
			for i := range mixBuf {
				mixBuf[i] = 0
			}
			for _, p := range parents {
				src := buffers[p]
				for i := range mixBuf {
					mixBuf[i] += src[i]
				}
			}
			scale := 1.0 / float64(len(parents))
			for i := range mixBuf {
				dst[i] = mixBuf[i] * scale
			}
		}

		if id == outputID || node.Bypassed {
			continue
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

func (e *Engine) applyCompiledNode(node compiledChainNode, block []float64) {
	if node.Type == "split" || node.Type == "sum" || node.Type == "_input" || node.Type == "_output" {
		return
	}
	rt := e.chainNodes[node.ID]
	if rt == nil {
		return
	}
	switch node.Type {
	case "chorus":
		rt.chorus.ProcessInPlace(block)
	case "flanger":
		_ = rt.flanger.ProcessInPlace(block)
	case "ringmod":
		rt.ringMod.ProcessInPlace(block)
	case "bitcrusher":
		rt.crusher.ProcessInPlace(block)
	case "widener":
		e.processNodeWidenerMonoInPlace(block, rt, clamp(getNodeNum(node, "mix", 0.5), 0, 1))
	case "phaser":
		_ = rt.phaser.ProcessInPlace(block)
	case "tremolo":
		_ = rt.tremolo.ProcessInPlace(block)
	case "delay":
		rt.delay.ProcessInPlace(block)
	case "bass":
		rt.bass.ProcessInPlace(block)
	case "pitch-time":
		rt.tp.ProcessInPlace(block)
	case "pitch-spectral":
		rt.sp.ProcessInPlace(block)
	case "reverb":
		model := node.Str["model"]
		if model == "fdn" {
			rt.fdn.ProcessInPlace(block)
		} else {
			rt.reverb.ProcessInPlace(block)
		}
	}
}

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
	case "bass":
		rt.bass, err = effects.NewHarmonicBass(e.sampleRate)
	case "pitch-time":
		rt.tp, err = pitch.NewPitchShifter(e.sampleRate)
	case "pitch-spectral":
		rt.sp, err = pitch.NewSpectralPitchShifter(e.sampleRate)
	case "reverb":
		rt.reverb = reverb.NewReverb()
		rt.fdn, err = reverb.NewFDNReverb(e.sampleRate)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rt, nil
}

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
	}
	return nil
}

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

func (e *Engine) processNodeWidenerMonoInPlace(block []float64, rt *chainEffectNode, mix float64) {
	if len(block) == 0 || rt == nil || rt.widener == nil {
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
		l2, r2 := rt.widener.ProcessStereo(left, right)
		wet := 0.5 * (l2 + r2)
		block[i] = dry[i]*(1-mix) + wet*mix
	}
}

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
