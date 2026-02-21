package webdemo

import (
	"encoding/json"
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

type chainGraphNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Bypassed bool   `json:"bypassed"`
	Fixed    bool   `json:"fixed"`
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
}

type compiledChainGraph struct {
	Nodes    map[string]compiledChainNode
	Incoming map[string][]string
	Outgoing map[string][]string
	Order    []string
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
	if err := e.flanger.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
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
	if err := e.ringMod.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.ringMod.SetCarrierHz(e.effects.RingModCarrierHz); err != nil {
		return err
	}
	if err := e.ringMod.SetMix(e.effects.RingModMix); err != nil {
		return err
	}
	if err := e.crusher.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.crusher.SetBitDepth(e.effects.BitCrusherBitDepth); err != nil {
		return err
	}
	if err := e.crusher.SetDownsample(e.effects.BitCrusherDownsample); err != nil {
		return err
	}
	if err := e.crusher.SetMix(e.effects.BitCrusherMix); err != nil {
		return err
	}
	if err := e.widener.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.widener.SetWidth(e.effects.WidenerWidth); err != nil {
		return err
	}
	if err := e.widener.SetBassMonoFreq(0); err != nil {
		return err
	}
	if err := e.phaser.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
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
	if err := e.tremolo.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
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
	if err := e.delay.SetSampleRate(e.sampleRate); err != nil {
		return err
	}
	if err := e.delay.SetTime(e.effects.DelayTime); err != nil {
		return err
	}
	if err := e.delay.SetFeedback(e.effects.DelayFeedback); err != nil {
		return err
	}
	if err := e.delay.SetMix(e.effects.DelayMix); err != nil {
		return err
	}

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
		e.applyEffectByType(node.Type, dst)
	}

	out := buffers[outputID]
	if out == nil {
		return false
	}
	copy(block, out)
	return true
}

func (e *Engine) applyEffectByType(effectType string, block []float64) {
	switch effectType {
	case "chorus":
		e.chorus.ProcessInPlace(block)
	case "flanger":
		_ = e.flanger.ProcessInPlace(block)
	case "ringmod":
		e.ringMod.ProcessInPlace(block)
	case "bitcrusher":
		e.crusher.ProcessInPlace(block)
	case "widener":
		e.processWidenerMonoInPlace(block)
	case "phaser":
		_ = e.phaser.ProcessInPlace(block)
	case "tremolo":
		_ = e.tremolo.ProcessInPlace(block)
	case "delay":
		e.delay.ProcessInPlace(block)
	case "bass":
		e.bass.ProcessInPlace(block)
	case "pitch-time":
		e.tp.ProcessInPlace(block)
	case "pitch-spectral":
		e.sp.ProcessInPlace(block)
	case "reverb":
		if e.effects.ReverbModel == "fdn" {
			e.fdn.ProcessInPlace(block)
		} else {
			e.reverb.ProcessInPlace(block)
		}
	case "split", "sum":
		// Utility graph-only nodes.
	}
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
		nodes[n.ID] = compiledChainNode{
			ID:       n.ID,
			Type:     n.Type,
			Bypassed: n.Bypassed,
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
