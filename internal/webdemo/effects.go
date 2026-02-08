package webdemo

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

// SetEffects updates chorus/reverb settings.
func (e *Engine) SetEffects(p EffectsParams) error {
	prevChorusEnabled := e.effects.ChorusEnabled
	prevReverbEnabled := e.effects.ReverbEnabled

	p.ChorusMix = clamp(p.ChorusMix, 0, 1)
	p.ChorusDepth = clamp(p.ChorusDepth, 0, 0.01)
	p.ChorusSpeedHz = clamp(p.ChorusSpeedHz, 0.05, 5)
	if p.ChorusStages < 1 {
		p.ChorusStages = 1
	}
	if p.ChorusStages > 6 {
		p.ChorusStages = 6
	}

	p.ReverbWet = clamp(p.ReverbWet, 0, 1.5)
	p.ReverbDry = clamp(p.ReverbDry, 0, 1.5)
	p.ReverbRoomSize = clamp(p.ReverbRoomSize, 0, 0.98)
	p.ReverbDamp = clamp(p.ReverbDamp, 0, 0.99)
	p.ReverbGain = clamp(p.ReverbGain, 0, 0.1)

	e.effects = p
	if err := e.rebuildEffects(); err != nil {
		return err
	}
	if prevChorusEnabled && !p.ChorusEnabled {
		e.chorus.Reset()
	}
	if prevReverbEnabled && !p.ReverbEnabled {
		e.reverb.Reset()
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

	e.reverb.SetWet(e.effects.ReverbWet)
	e.reverb.SetDry(e.effects.ReverbDry)
	e.reverb.SetRoomSize(e.effects.ReverbRoomSize)
	e.reverb.SetDamp(e.effects.ReverbDamp)
	e.reverb.SetGain(e.effects.ReverbGain)
	return nil
}
