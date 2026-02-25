package webdemo

import "github.com/cwbudde/algo-dsp/dsp/effects/reverb"

// SetCompressor updates compressor parameters.
func (e *Engine) SetCompressor(param CompressorParams) error {
	prevEnabled := e.compParams.Enabled
	param.ThresholdDB = clamp(param.ThresholdDB, -60, 0)
	param.Ratio = clamp(param.Ratio, 1, 100)
	param.KneeDB = clamp(param.KneeDB, 0, 24)
	param.AttackMs = clamp(param.AttackMs, 0.1, 1000)
	param.ReleaseMs = clamp(param.ReleaseMs, 1, 5000)
	param.MakeupGainDB = clamp(param.MakeupGainDB, 0, 24)

	e.compParams = param
	if err := e.rebuildCompressor(); err != nil {
		return err
	}

	if prevEnabled && !param.Enabled {
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

	p.ConvReverbWet = clamp(p.ConvReverbWet, 0, 1.5)

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

	if (prevConvReverbEnabled && !p.ConvReverbEnabled) || prevConvReverbIRIndex != p.ConvReverbIRIndex {
		if e.convReverb != nil {
			e.convReverb.Reset()
		}
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
	steps := []func() error{
		e.rebuildChorusEffect,
		e.rebuildFlangerEffect,
		e.rebuildRingModEffect,
		e.rebuildBitCrusherEffect,
		e.rebuildWidenerEffect,
		e.rebuildPhaserEffect,
		e.rebuildTremoloEffect,
		e.rebuildDelayEffect,
		e.rebuildTimePitchEffect,
		e.rebuildSpectralPitchEffect,
		e.rebuildReverbEffect,
		e.rebuildHarmonicBassEffect,
		e.rebuildConvReverbEffect,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) rebuildChorusEffect() error {
	if !e.effects.ChorusEnabled {
		return nil
	}

	return configureChorus(
		e.chorus,
		e.sampleRate,
		e.effects.ChorusMix,
		e.effects.ChorusDepth,
		e.effects.ChorusSpeedHz,
		e.effects.ChorusStages,
	)
}

func (e *Engine) rebuildFlangerEffect() error {
	if !e.effects.FlangerEnabled {
		return nil
	}

	return configureFlanger(
		e.flanger,
		e.sampleRate,
		e.effects.FlangerRateHz,
		e.effects.FlangerBaseDelay,
		e.effects.FlangerDepth,
		e.effects.FlangerFeedback,
		e.effects.FlangerMix,
	)
}

func (e *Engine) rebuildRingModEffect() error {
	if !e.effects.RingModEnabled {
		return nil
	}

	return configureRingMod(e.ringMod, e.sampleRate, e.effects.RingModCarrierHz, e.effects.RingModMix)
}

func (e *Engine) rebuildBitCrusherEffect() error {
	if !e.effects.BitCrusherEnabled {
		return nil
	}

	return configureBitCrusher(
		e.crusher,
		e.sampleRate,
		e.effects.BitCrusherBitDepth,
		e.effects.BitCrusherDownsample,
		e.effects.BitCrusherMix,
	)
}

func (e *Engine) rebuildWidenerEffect() error {
	if !e.effects.WidenerEnabled {
		return nil
	}

	return configureWidener(e.widener, e.sampleRate, e.effects.WidenerWidth)
}

func (e *Engine) rebuildPhaserEffect() error {
	if !e.effects.PhaserEnabled {
		return nil
	}

	return configurePhaser(
		e.phaser,
		e.sampleRate,
		e.effects.PhaserRateHz,
		e.effects.PhaserMinFreqHz,
		e.effects.PhaserMaxFreqHz,
		e.effects.PhaserStages,
		e.effects.PhaserFeedback,
		e.effects.PhaserMix,
	)
}

func (e *Engine) rebuildTremoloEffect() error {
	if !e.effects.TremoloEnabled {
		return nil
	}

	return configureTremolo(
		e.tremolo,
		e.sampleRate,
		e.effects.TremoloRateHz,
		e.effects.TremoloDepth,
		e.effects.TremoloSmoothingMs,
		e.effects.TremoloMix,
	)
}

func (e *Engine) rebuildDelayEffect() error {
	if !e.effects.DelayEnabled {
		return nil
	}

	return configureDelay(e.delay, e.sampleRate, e.effects.DelayTime, e.effects.DelayFeedback, e.effects.DelayMix)
}

func (e *Engine) rebuildTimePitchEffect() error {
	if !e.effects.TimePitchEnabled {
		return nil
	}

	return configureTimePitch(
		e.tp,
		e.sampleRate,
		e.effects.TimePitchSemitones,
		e.effects.TimePitchSequence,
		e.effects.TimePitchOverlap,
		e.effects.TimePitchSearch,
	)
}

func (e *Engine) rebuildSpectralPitchEffect() error {
	if !e.effects.SpectralPitchEnabled {
		return nil
	}

	return configureSpectralPitch(
		e.sp,
		e.sampleRate,
		e.effects.SpectralPitchSemitones,
		e.effects.SpectralPitchFrameSize,
		e.effects.SpectralPitchHop,
	)
}

func (e *Engine) rebuildReverbEffect() error {
	if !e.effects.ReverbEnabled {
		return nil
	}

	if e.effects.ReverbModel == "fdn" {
		return configureFDNReverb(
			e.fdn,
			e.sampleRate,
			e.effects.ReverbWet,
			e.effects.ReverbDry,
			e.effects.ReverbRT60,
			e.effects.ReverbPreDelay,
			e.effects.ReverbDamp,
			e.effects.ReverbModDepth,
			e.effects.ReverbModRate,
		)
	}

	configureFreeverb(e.reverb, e.effects.ReverbWet, e.effects.ReverbDry, e.effects.ReverbRoomSize, e.effects.ReverbDamp, e.effects.ReverbGain)

	return nil
}

func (e *Engine) rebuildHarmonicBassEffect() error {
	if !e.effects.HarmonicBassEnabled {
		return nil
	}

	return configureHarmonicBass(
		e.bass,
		e.sampleRate,
		e.effects.HarmonicBassFrequency,
		e.effects.HarmonicBassInputGain,
		e.effects.HarmonicBassHighGain,
		e.effects.HarmonicBassOriginal,
		e.effects.HarmonicBassHarmonic,
		e.effects.HarmonicBassDecay,
		e.effects.HarmonicBassResponseMs,
		e.effects.HarmonicBassHighpass,
	)
}

func (e *Engine) rebuildConvReverbEffect() error {
	if !e.effects.ConvReverbEnabled || e.irLib == nil {
		return nil
	}

	ir := e.irLib.GetIR(e.effects.ConvReverbIRIndex)
	if ir == nil || len(ir.Samples) == 0 {
		return nil
	}

	// Mix stereo channels to mono for the convolution kernel.
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

	// Only rebuild the engine if the IR changed.
	if e.convReverb == nil || e.convReverbIRIndex != e.effects.ConvReverbIRIndex {
		cr, err := reverb.NewConvolutionReverb(kernel, 7) // 128-sample latency
		if err != nil {
			return err
		}

		e.convReverb = cr
		e.convReverbIRIndex = e.effects.ConvReverbIRIndex
	}

	e.convReverb.SetWetDry(e.effects.ConvReverbWet, 1.0)

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
