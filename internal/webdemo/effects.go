package webdemo

import "github.com/cwbudde/algo-dsp/dsp/effects"

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

// SetEffects updates chorus/reverb/harmonic bass settings.
func (e *Engine) SetEffects(p EffectsParams) error {
	prevChorusEnabled := e.effects.ChorusEnabled
	prevReverbEnabled := e.effects.ReverbEnabled
	prevReverbModel := e.effects.ReverbModel
	prevBassEnabled := e.effects.HarmonicBassEnabled

	p.ChorusMix = clamp(p.ChorusMix, 0, 1)
	p.ChorusDepth = clamp(p.ChorusDepth, 0, 0.01)
	p.ChorusSpeedHz = clamp(p.ChorusSpeedHz, 0.05, 5)
	if p.ChorusStages < 1 {
		p.ChorusStages = 1
	}
	if p.ChorusStages > 6 {
		p.ChorusStages = 6
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

	e.effects = p
	if err := e.rebuildEffects(); err != nil {
		return err
	}
	if prevChorusEnabled && !p.ChorusEnabled {
		e.chorus.Reset()
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
