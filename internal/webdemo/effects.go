package webdemo

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
func (e *Engine) SetLimiter(params LimiterParams) error {
	prevEnabled := e.limParams.Enabled
	params.Threshold = clamp(params.Threshold, -24, 0)
	params.Release = clamp(params.Release, 1, 5000)

	e.limParams = params
	if err := e.rebuildLimiter(); err != nil {
		return err
	}

	if prevEnabled && !params.Enabled {
		e.limiter.Reset()
	}

	return nil
}

// SetEffects updates effect settings.
func (e *Engine) SetEffects(params EffectsParams) error {
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

	params.ChorusMix = clamp(params.ChorusMix, 0, 1)
	params.ChorusDepth = clamp(params.ChorusDepth, 0, 0.01)

	params.ChorusSpeedHz = clamp(params.ChorusSpeedHz, 0.05, 5)
	if params.ChorusStages < 1 {
		params.ChorusStages = 1
	}

	if params.ChorusStages > 6 {
		params.ChorusStages = 6
	}

	params.FlangerRateHz = clamp(params.FlangerRateHz, 0.05, 5)
	params.FlangerDepth = clamp(params.FlangerDepth, 0, 0.0099)

	params.FlangerBaseDelay = clamp(params.FlangerBaseDelay, 0.0001, 0.01)
	if params.FlangerBaseDelay+params.FlangerDepth > 0.01 {
		params.FlangerDepth = 0.01 - params.FlangerBaseDelay
	}

	params.FlangerFeedback = clamp(params.FlangerFeedback, -0.99, 0.99)
	params.FlangerMix = clamp(params.FlangerMix, 0, 1)
	params.RingModCarrierHz = clamp(params.RingModCarrierHz, 1, e.sampleRate*0.49)
	params.RingModMix = clamp(params.RingModMix, 0, 1)

	params.BitCrusherBitDepth = clamp(params.BitCrusherBitDepth, 1, 32)
	if params.BitCrusherDownsample < 1 {
		params.BitCrusherDownsample = 1
	}

	if params.BitCrusherDownsample > 256 {
		params.BitCrusherDownsample = 256
	}

	params.BitCrusherMix = clamp(params.BitCrusherMix, 0, 1)
	params.WidenerWidth = clamp(params.WidenerWidth, 0, 4)
	params.WidenerMix = clamp(params.WidenerMix, 0, 1)
	params.PhaserRateHz = clamp(params.PhaserRateHz, 0.05, 5)
	params.PhaserMinFreqHz = clamp(params.PhaserMinFreqHz, 20, e.sampleRate*0.45)

	params.PhaserMaxFreqHz = clamp(params.PhaserMaxFreqHz, params.PhaserMinFreqHz+1, e.sampleRate*0.49)
	if params.PhaserStages < 1 {
		params.PhaserStages = 1
	}

	if params.PhaserStages > 12 {
		params.PhaserStages = 12
	}

	params.PhaserFeedback = clamp(params.PhaserFeedback, -0.99, 0.99)
	params.PhaserMix = clamp(params.PhaserMix, 0, 1)
	params.TremoloRateHz = clamp(params.TremoloRateHz, 0.05, 20)
	params.TremoloDepth = clamp(params.TremoloDepth, 0, 1)
	params.TremoloSmoothingMs = clamp(params.TremoloSmoothingMs, 0, 200)
	params.TremoloMix = clamp(params.TremoloMix, 0, 1)
	params.DelayTime = clamp(params.DelayTime, 0.001, 2.0)
	params.DelayFeedback = clamp(params.DelayFeedback, 0, 0.99)
	params.DelayMix = clamp(params.DelayMix, 0, 1)

	params.TimePitchSemitones = clamp(params.TimePitchSemitones, -24, 24)
	params.TimePitchSequence = clamp(params.TimePitchSequence, 20, 120)

	params.TimePitchOverlap = clamp(params.TimePitchOverlap, 4, 60)
	if params.TimePitchOverlap >= params.TimePitchSequence {
		params.TimePitchOverlap = params.TimePitchSequence - 1
	}

	params.TimePitchSearch = clamp(params.TimePitchSearch, 2, 40)

	params.SpectralPitchSemitones = clamp(params.SpectralPitchSemitones, -24, 24)

	params.SpectralPitchFrameSize = sanitizeSpectralPitchFrameSize(params.SpectralPitchFrameSize)
	if params.SpectralPitchHop < 1 || params.SpectralPitchHop >= params.SpectralPitchFrameSize {
		params.SpectralPitchHop = params.SpectralPitchFrameSize / 4
	}

	if params.SpectralPitchHop < 1 {
		params.SpectralPitchHop = 1
	}

	if params.ReverbModel != "fdn" && params.ReverbModel != "freeverb" {
		params.ReverbModel = "freeverb"
	}

	params.ReverbWet = clamp(params.ReverbWet, 0, 1.5)
	params.ReverbDry = clamp(params.ReverbDry, 0, 1.5)
	params.ReverbRoomSize = clamp(params.ReverbRoomSize, 0, 0.98)
	params.ReverbDamp = clamp(params.ReverbDamp, 0, 0.99)
	params.ReverbGain = clamp(params.ReverbGain, 0, 0.1)
	params.ReverbRT60 = clamp(params.ReverbRT60, 0.2, 8)
	params.ReverbPreDelay = clamp(params.ReverbPreDelay, 0, 0.1)
	params.ReverbModDepth = clamp(params.ReverbModDepth, 0, 0.01)
	params.ReverbModRate = clamp(params.ReverbModRate, 0, 1)

	params.HarmonicBassFrequency = clamp(params.HarmonicBassFrequency, 10, 500)
	params.HarmonicBassInputGain = clamp(params.HarmonicBassInputGain, 0, 2)
	params.HarmonicBassHighGain = clamp(params.HarmonicBassHighGain, 0, 2)
	params.HarmonicBassOriginal = clamp(params.HarmonicBassOriginal, 0, 2)
	params.HarmonicBassHarmonic = clamp(params.HarmonicBassHarmonic, 0, 2)
	params.HarmonicBassDecay = clamp(params.HarmonicBassDecay, -1, 1)

	params.HarmonicBassResponseMs = clamp(params.HarmonicBassResponseMs, 1, 200)
	if params.HarmonicBassHighpass < 0 {
		params.HarmonicBassHighpass = 0
	}

	if params.HarmonicBassHighpass > 2 {
		params.HarmonicBassHighpass = 2
	}

	graph, err := parseChainGraph(params.ChainGraphJSON)
	if err != nil {
		return err
	}

	e.effects = params
	if err := e.rebuildEffects(); err != nil {
		return err
	}

	if err := e.syncChainEffectNodes(graph); err != nil {
		return err
	}

	e.chainGraph = graph
	if prevChorusEnabled && !params.ChorusEnabled {
		e.chorus.Reset()
	}

	if prevFlangerEnabled && !params.FlangerEnabled {
		e.flanger.Reset()
	}

	if prevRingModEnabled && !params.RingModEnabled {
		e.ringMod.Reset()
	}

	if prevCrusherEnabled && !params.BitCrusherEnabled {
		e.crusher.Reset()
	}

	if prevWidenerEnabled && !params.WidenerEnabled {
		e.widener.Reset()
	}

	if prevPhaserEnabled && !params.PhaserEnabled {
		e.phaser.Reset()
	}

	if prevTremoloEnabled && !params.TremoloEnabled {
		e.tremolo.Reset()
	}

	if prevDelayEnabled && !params.DelayEnabled {
		e.delay.Reset()
	}

	if prevReverbEnabled && !params.ReverbEnabled {
		e.reverb.Reset()
		e.fdn.Reset()
	}

	if prevReverbModel != params.ReverbModel {
		e.reverb.Reset()
		e.fdn.Reset()
	}

	if prevBassEnabled && !params.HarmonicBassEnabled {
		e.bass.Reset()
	}

	if prevTimePitchEnabled && !params.TimePitchEnabled {
		e.tp.Reset()
	}

	if prevSpectralPitchEnabled && !params.SpectralPitchEnabled {
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
