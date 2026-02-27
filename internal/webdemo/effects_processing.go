package webdemo

// processEffectsInPlace routes to graph-based or legacy serial processing.
func (e *Engine) processEffectsInPlace(block []float64) {
	if len(block) == 0 {
		return
	}

	if e.chain != nil && e.chain.Process(block) {
		return
	}

	e.processEffectsLegacyInPlace(block)
}

// processEffectsLegacyInPlace applies the fixed-order serial effect chain.
func (e *Engine) processEffectsLegacyInPlace(block []float64) {
	e.processLegacyPreDynamicsInPlace(block)
	e.processLegacyModulationInPlace(block)
	e.processLegacyPitchInPlace(block)
	e.processLegacyReverbInPlace(block)
}

func (e *Engine) processLegacyPreDynamicsInPlace(block []float64) {
	if e.effects.HarmonicBassEnabled {
		e.bass.ProcessInPlace(block)
	}

	if e.effects.DelayEnabled {
		e.delay.ProcessInPlace(block)
	}
}

func (e *Engine) processLegacyModulationInPlace(block []float64) {
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
}

func (e *Engine) processLegacyPitchInPlace(block []float64) {
	if e.effects.TimePitchEnabled {
		e.tp.ProcessInPlace(block)
	}

	if e.effects.SpectralPitchEnabled {
		e.sp.ProcessInPlace(block)
	}
}

func (e *Engine) processLegacyReverbInPlace(block []float64) {
	if !e.effects.ReverbEnabled {
		return
	}

	if e.effects.ReverbModel == reverbModelFDN {
		e.fdn.ProcessInPlace(block)

		return
	}

	e.reverb.ProcessInPlace(block)
}

// processWidenerMonoInPlace applies the stereo widener to a mono signal using a
// short decorrelation delay to approximate a stereo side signal, then folds back to mono.
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
	delaySamples := max(
		// 1 ms
		int(e.sampleRate*0.001), 1)

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
