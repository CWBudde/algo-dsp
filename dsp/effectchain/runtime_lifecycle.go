package effectchain

func (r *chorusRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *flangerRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *ringModRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *bitCrusherRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *distortionRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *distChebRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *transformerRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *phaserRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *tremoloRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *delayRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *rotaryRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *bassRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *timePitchRuntime) Reset() {
	if r.stream != nil {
		r.stream.Reset()
	}
}

func (r *spectralPitchRuntime) Reset() {
	if r.stream != nil {
		r.stream.Reset()
	}
}

func (r *spectralFreezeRuntime) Reset() {
	if r.stream != nil {
		r.stream.Reset()
	}
}

func (r *granularRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *freeverbRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *fdnReverbRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *compressorRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *limiterRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *lookaheadLimiterRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *gateRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *expanderRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *deesserRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *transientShaperRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *multibandRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *vocoderRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *equalizerRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *dynamicEQRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *autoWahRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *frequencyShifterRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *widenerRuntime) Reset() {
	r.fx.Reset()
	clear(r.monoDelay)
	r.monoWrite = 0
	clear(r.scratchBuf)
}
func (r *simpleDelayRuntime) Reset() { clear(r.buf); r.write = 0 }
func (r *convReverbRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}
}

func (r *filterRuntime) Reset() {
	if r.fx != nil {
		r.fx.Reset()
	}

	if r.moogLP != nil {
		r.moogLP.Reset()
	}
}
func (r *reverbRuntime) Reset() { r.freeverb.Reset(); r.fdn.Reset() }
func (r *spatialRuntime) Reset() {
	if r.panner != nil {
		r.panner.Reset()
	}

	if r.haas != nil {
		r.haas.Reset()
	}

	if r.cross != nil {
		r.cross.Reset()
	}
}

func (r *widenerRuntime) Prepare(n int) error {
	if len(r.scratchBuf) < n {
		r.scratchBuf = make([]float64, n)
	}

	return nil
}

func (r *rotaryRuntime) Prepare(n int) error {
	if len(r.scratchBuf) < n {
		r.scratchBuf = make([]float64, n)
	}

	return nil
}

func (r *vocoderRuntime) Prepare(n int) error {
	if len(r.carrierBuf) < n {
		r.carrierBuf = make([]float64, n)
	}

	return nil
}
func (r *convReverbRuntime) Prepare(n int) error { return r.fx.Prepare(n) }

func (r *multibandRuntime) Prepare(n int) error { return r.fx.Prepare(n) }
