package webdemo

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

type chorusChainRuntime struct {
	fx *modulation.Chorus
}

func (r *chorusChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	stages := int(math.Round(getNodeNum(node, "stages", 3)))
	if stages < 1 {
		stages = 1
	}

	if stages > 6 {
		stages = 6
	}

	return configureChorus(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "mix", 0.18), 0, 1),
		clamp(getNodeNum(node, "depth", 0.003), 0, 0.01),
		clamp(getNodeNum(node, "speedHz", 0.35), 0.05, 5),
		stages,
	)
}

func (r *chorusChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type flangerChainRuntime struct {
	fx *modulation.Flanger
}

func (r *flangerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureFlanger(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "rateHz", 0.25), 0.05, 5),
		clamp(getNodeNum(node, "baseDelay", 0.001), 0.0001, 0.01),
		clamp(getNodeNum(node, "depth", 0.0015), 0, 0.0099),
		clamp(getNodeNum(node, "feedback", 0.25), -0.99, 0.99),
		clamp(getNodeNum(node, "mix", 0.5), 0, 1),
	)
}

func (r *flangerChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type ringModChainRuntime struct {
	fx *modulation.RingModulator
}

func (r *ringModChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureRingMod(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "carrierHz", 440), 1, e.sampleRate*0.49),
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *ringModChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type bitCrusherChainRuntime struct {
	fx *effects.BitCrusher
}

func (r *bitCrusherChainRuntime) Configure(eng *Engine, node compiledChainNode) error {
	downsample := int(math.Round(getNodeNum(node, "downsample", 4)))
	if downsample < 1 {
		downsample = 1
	}

	if downsample > 256 {
		downsample = 256
	}

	return configureBitCrusher(
		r.fx,
		eng.sampleRate,
		clamp(getNodeNum(node, "bitDepth", 8), 1, 32),
		downsample,
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *bitCrusherChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type distortionChainRuntime struct {
	fx *effects.Distortion
}

func (r *distortionChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	mode := normalizeDistortionMode(node.Str["mode"])
	approx := normalizeDistortionApproxMode(node.Str["approx"])
	chebMode := normalizeChebyshevHarmonicMode(node.Str["chebHarmonic"])

	chebOrder := int(math.Round(getNodeNum(node, "chebOrder", 3)))
	if chebOrder < 1 {
		chebOrder = 1
	}

	if chebOrder > 16 {
		chebOrder = 16
	}

	chebInvert := getNodeNum(node, "chebInvert", 0) >= 0.5
	chebDCBypass := getNodeNum(node, "chebDCBypass", 0) >= 0.5

	return configureDistortion(
		r.fx,
		e.sampleRate,
		mode,
		approx,
		clamp(getNodeNum(node, "drive", 1.8), 0.01, 20),
		clamp(getNodeNum(node, "mix", 1.0), 0, 1),
		clamp(getNodeNum(node, "output", 1.0), 0, 4),
		clamp(getNodeNum(node, "clip", 1.0), 0.05, 1),
		clamp(getNodeNum(node, "shape", 0.5), 0, 1),
		clamp(getNodeNum(node, "bias", 0), -1, 1),
		chebOrder,
		chebMode,
		chebInvert,
		clamp(getNodeNum(node, "chebGain", 1.0), 0, 4),
		chebDCBypass,
	)
}

func (r *distortionChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type transformerChainRuntime struct {
	fx *effects.TransformerSimulation
}

func (r *transformerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	quality := normalizeTransformerQuality(node.Str["quality"])

	oversampling := int(math.Round(getNodeNum(node, "oversampling", 4)))
	switch oversampling {
	case 2, 4, 8:
	default:
		if oversampling <= 3 {
			oversampling = 2
		} else if oversampling <= 6 {
			oversampling = 4
		} else {
			oversampling = 8
		}
	}

	return configureTransformer(
		r.fx,
		e.sampleRate,
		quality,
		clamp(getNodeNum(node, "drive", 2.0), 0.1, 30),
		clamp(getNodeNum(node, "mix", 1.0), 0, 1),
		clamp(getNodeNum(node, "output", 1.0), 0, 4),
		clamp(getNodeNum(node, "highpassHz", 25), 5, e.sampleRate*0.45),
		clamp(getNodeNum(node, "dampingHz", 9000), 200, e.sampleRate*0.49),
		oversampling,
	)
}

func (r *transformerChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type widenerChainRuntime struct {
	fx *spatial.StereoWidener
}

func (r *widenerChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureWidener(r.fx, e.sampleRate, clamp(getNodeNum(node, "width", 1), 0, 4))
}

func (r *widenerChainRuntime) Process(e *Engine, node compiledChainNode, block []float64) {
	e.processNodeWidenerMonoInPlace(block, r.fx, clamp(getNodeNum(node, "mix", 0.5), 0, 1))
}

type phaserChainRuntime struct {
	fx *modulation.Phaser
}

func (r *phaserChainRuntime) Configure(eng *Engine, node compiledChainNode) error {
	minHz := clamp(getNodeNum(node, "minFreqHz", 300), 20, eng.sampleRate*0.45)
	maxHz := clamp(getNodeNum(node, "maxFreqHz", 1600), minHz+1, eng.sampleRate*0.49)

	stages := int(math.Round(getNodeNum(node, "stages", 6)))
	if stages < 1 {
		stages = 1
	}

	if stages > 12 {
		stages = 12
	}

	return configurePhaser(
		r.fx,
		eng.sampleRate,
		clamp(getNodeNum(node, "rateHz", 0.4), 0.05, 5),
		minHz,
		maxHz,
		stages,
		clamp(getNodeNum(node, "feedback", 0.2), -0.99, 0.99),
		clamp(getNodeNum(node, "mix", 0.5), 0, 1),
	)
}

func (r *phaserChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type tremoloChainRuntime struct {
	fx *modulation.Tremolo
}

func (r *tremoloChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureTremolo(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "rateHz", 4), 0.05, 20),
		clamp(getNodeNum(node, "depth", 0.6), 0, 1),
		clamp(getNodeNum(node, "smoothingMs", 5), 0, 200),
		clamp(getNodeNum(node, "mix", 1), 0, 1),
	)
}

func (r *tremoloChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type delayChainRuntime struct {
	fx *effects.Delay
}

func (r *delayChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	return configureDelay(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "time", 0.25), 0.001, 2),
		clamp(getNodeNum(node, "feedback", 0.35), 0, 0.99),
		clamp(getNodeNum(node, "mix", 0.25), 0, 1),
	)
}

func (r *delayChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type simpleDelayChainRuntime struct {
	sampleRate   float64
	delayMs      float64
	delaySamples int
	write        int
	buf          []float64
}

func (r *simpleDelayChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	r.sampleRate = e.sampleRate
	r.delayMs = clamp(getNodeNum(node, "delayMs", 20), 0, 500)

	r.delaySamples = int(math.Round(r.delayMs * r.sampleRate / 1000.0))
	if r.delaySamples < 0 {
		r.delaySamples = 0
	}

	size := r.delaySamples + 1
	if size < 1 {
		size = 1
	}

	if len(r.buf) != size {
		r.buf = make([]float64, size)
		r.write = 0
	}

	return nil
}

func (r *simpleDelayChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if len(r.buf) <= 1 {
		return
	}

	for i := range block {
		r.buf[r.write] = block[i]

		readPos := r.write + 1
		if readPos >= len(r.buf) {
			readPos = 0
		}

		block[i] = r.buf[readPos]
		r.write = readPos
	}
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
	highpass := int(math.Round(getNodeNum(node, "highpass", 0)))
	if highpass < 0 {
		highpass = 0
	}

	if highpass > 2 {
		highpass = 2
	}

	return configureHarmonicBass(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "frequency", 80), 10, 500),
		clamp(getNodeNum(node, "inputGain", 1), 0, 2),
		clamp(getNodeNum(node, "highGain", 1), 0, 2),
		clamp(getNodeNum(node, "original", 1), 0, 2),
		clamp(getNodeNum(node, "harmonic", 0), 0, 2),
		clamp(getNodeNum(node, "decay", 0), -1, 1),
		clamp(getNodeNum(node, "responseMs", 20), 1, 200),
		highpass,
	)
}

func (r *bassChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type timePitchChainRuntime struct {
	fx *pitch.PitchShifter
}

func (r *timePitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	seq := clamp(getNodeNum(node, "sequence", 40), 20, 120)

	overlap := clamp(getNodeNum(node, "overlap", 10), 4, 60)
	if overlap >= seq {
		overlap = seq - 1
	}

	return configureTimePitch(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "semitones", 0), -24, 24),
		seq,
		overlap,
		clamp(getNodeNum(node, "search", 15), 2, 40),
	)
}

func (r *timePitchChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralPitchChainRuntime struct {
	fx *pitch.SpectralPitchShifter
}

func (r *spectralPitchChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(getNodeNum(node, "frameSize", 1024))))

	hop := int(math.Round(float64(frame) * clamp(getNodeNum(node, "hopRatio", 0.25), 0.01, 0.99)))
	if hop < 1 {
		hop = 1
	}

	if hop >= frame {
		hop = frame - 1
	}

	return configureSpectralPitch(
		r.fx,
		e.sampleRate,
		clamp(getNodeNum(node, "semitones", 0), -24, 24),
		frame,
		hop,
	)
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
		return configureFDNReverb(
			r.fdn,
			e.sampleRate,
			clamp(getNodeNum(node, "wet", 0.22), 0, 1.5),
			clamp(getNodeNum(node, "dry", 1), 0, 1.5),
			clamp(getNodeNum(node, "rt60", 1.8), 0.2, 8),
			clamp(getNodeNum(node, "preDelay", 0.01), 0, 0.1),
			clamp(getNodeNum(node, "damp", 0.45), 0, 0.99),
			clamp(getNodeNum(node, "modDepth", 0.002), 0, 0.01),
			clamp(getNodeNum(node, "modRate", 0.1), 0, 1),
		)
	}

	configureFreeverb(
		r.freeverb,
		clamp(getNodeNum(node, "wet", 0.22), 0, 1.5),
		clamp(getNodeNum(node, "dry", 1), 0, 1.5),
		clamp(getNodeNum(node, "roomSize", 0.72), 0, 0.98),
		clamp(getNodeNum(node, "damp", 0.45), 0, 0.99),
		clamp(getNodeNum(node, "gain", 0.015), 0, 0.1),
	)

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

type lookaheadLimiterChainRuntime struct {
	fx *dynamics.LookaheadLimiter
}

func (r *lookaheadLimiterChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}

	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -1), -24, 0)); err != nil {
		return err
	}

	if err := r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
		return err
	}

	return r.fx.SetLookahead(clamp(getNodeNum(node, "lookaheadMs", 3), 0, 200))
}

func (r *lookaheadLimiterChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

func (r *lookaheadLimiterChainRuntime) ProcessWithSidechain(program, sidechain []float64) {
	r.fx.ProcessInPlaceSidechain(program, sidechain)
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

type expanderChainRuntime struct {
	fx *dynamics.Expander
}

func (r *expanderChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}

	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -35), -80, 0)); err != nil {
		return err
	}

	if err := r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 2), 1, 100)); err != nil {
		return err
	}

	if err := r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 6), 0, 24)); err != nil {
		return err
	}

	if err := r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 1), 0.1, 1000)); err != nil {
		return err
	}

	if err := r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 100), 1, 5000)); err != nil {
		return err
	}

	if err := r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -60), -120, 0)); err != nil {
		return err
	}

	if err := r.fx.SetTopology(normalizeDynamicsTopology(node.Str["topology"])); err != nil {
		return err
	}

	if err := r.fx.SetDetectorMode(normalizeDynamicsDetectorMode(node.Str["detector"])); err != nil {
		return err
	}

	return r.fx.SetRMSWindow(clamp(getNodeNum(node, "rmsWindowMs", 30), 1, 1000))
}

func (r *expanderChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type deesserChainRuntime struct {
	fx *dynamics.DeEsser
}

func (r *deesserChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}

	if err := r.fx.SetFrequency(clamp(getNodeNum(node, "freqHz", 6000), 1000, e.sampleRate*0.49)); err != nil {
		return err
	}

	if err := r.fx.SetQ(clamp(getNodeNum(node, "q", 1.5), 0.1, 10)); err != nil {
		return err
	}

	if err := r.fx.SetThreshold(clamp(getNodeNum(node, "thresholdDB", -20), -80, 0)); err != nil {
		return err
	}

	if err := r.fx.SetRatio(clamp(getNodeNum(node, "ratio", 4), 1, 100)); err != nil {
		return err
	}

	if err := r.fx.SetKnee(clamp(getNodeNum(node, "kneeDB", 3), 0, 12)); err != nil {
		return err
	}

	if err := r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 0.5), 0.01, 50)); err != nil {
		return err
	}

	if err := r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 20), 1, 500)); err != nil {
		return err
	}

	if err := r.fx.SetRange(clamp(getNodeNum(node, "rangeDB", -24), -60, 0)); err != nil {
		return err
	}

	if err := r.fx.SetMode(normalizeDeesserMode(node.Str["mode"])); err != nil {
		return err
	}

	if err := r.fx.SetDetector(normalizeDeesserDetector(node.Str["detector"])); err != nil {
		return err
	}

	order := int(math.Round(getNodeNum(node, "filterOrder", 2)))
	if order < 1 {
		order = 1
	}

	if order > 4 {
		order = 4
	}

	if err := r.fx.SetFilterOrder(order); err != nil {
		return err
	}

	r.fx.SetListen(getNodeNum(node, "listen", 0) >= 0.5)

	return nil
}

func (r *deesserChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type transientShaperChainRuntime struct {
	fx *dynamics.TransientShaper
}

func (r *transientShaperChainRuntime) Configure(e *Engine, node compiledChainNode) error {
	if err := r.fx.SetSampleRate(e.sampleRate); err != nil {
		return err
	}

	if err := r.fx.SetAttackAmount(clamp(getNodeNum(node, "attack", 0), -1, 1)); err != nil {
		return err
	}

	if err := r.fx.SetSustainAmount(clamp(getNodeNum(node, "sustain", 0), -1, 1)); err != nil {
		return err
	}

	if err := r.fx.SetAttack(clamp(getNodeNum(node, "attackMs", 10), 0.1, 200)); err != nil {
		return err
	}

	return r.fx.SetRelease(clamp(getNodeNum(node, "releaseMs", 120), 1, 2000))
}

func (r *transientShaperChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	r.fx.ProcessInPlace(block)
}

type multibandChainRuntime struct {
	fx        *dynamics.MultibandCompressor
	lastBands int
	lastOrder int
	lastC1    float64
	lastC2    float64
	lastSR    float64
}

func (r *multibandChainRuntime) Configure(eng *Engine, node compiledChainNode) error {
	bands := int(math.Round(getNodeNum(node, "bands", 3)))
	if bands < 2 {
		bands = 2
	}

	if bands > 3 {
		bands = 3
	}

	order := int(math.Round(getNodeNum(node, "order", 4)))
	if order < 2 {
		order = 2
	}

	if order > 24 {
		order = 24
	}

	if order%2 != 0 {
		order++
	}

	cross1 := clamp(getNodeNum(node, "cross1Hz", 250), 40, eng.sampleRate*0.2)
	cross2 := clamp(getNodeNum(node, "cross2Hz", 3000), cross1+100, eng.sampleRate*0.45)

	rebuild := r.fx == nil ||
		r.lastBands != bands ||
		r.lastOrder != order ||
		math.Abs(r.lastC1-cross1) > 1e-9 ||
		math.Abs(r.lastC2-cross2) > 1e-9 ||
		math.Abs(r.lastSR-eng.sampleRate) > 1e-9

	if rebuild {
		freqs := []float64{cross1}
		if bands == 3 {
			freqs = append(freqs, cross2)
		}

		fx, err := dynamics.NewMultibandCompressor(freqs, order, eng.sampleRate)
		if err != nil {
			return err
		}

		r.fx = fx
		r.lastBands = bands
		r.lastOrder = order
		r.lastC1 = cross1
		r.lastC2 = cross2
		r.lastSR = eng.sampleRate
	}

	// Band 1 (low)
	if err := r.fx.SetBandThreshold(0, clamp(getNodeNum(node, "lowThresholdDB", -20), -80, 0)); err != nil {
		return err
	}

	if err := r.fx.SetBandRatio(0, clamp(getNodeNum(node, "lowRatio", 2.5), 1, 20)); err != nil {
		return err
	}

	// Band 2 (mid / high for 2-band)
	if err := r.fx.SetBandThreshold(1, clamp(getNodeNum(node, "midThresholdDB", -18), -80, 0)); err != nil {
		return err
	}

	if err := r.fx.SetBandRatio(1, clamp(getNodeNum(node, "midRatio", 3.0), 1, 20)); err != nil {
		return err
	}

	// Optional band 3 (high)
	if bands == 3 {
		if err := r.fx.SetBandThreshold(2, clamp(getNodeNum(node, "highThresholdDB", -14), -80, 0)); err != nil {
			return err
		}

		if err := r.fx.SetBandRatio(2, clamp(getNodeNum(node, "highRatio", 4.0), 1, 20)); err != nil {
			return err
		}
	}

	attack := clamp(getNodeNum(node, "attackMs", 8), 0.1, 1000)
	release := clamp(getNodeNum(node, "releaseMs", 120), 1, 5000)
	knee := clamp(getNodeNum(node, "kneeDB", 6), 0, 24)
	makeup := clamp(getNodeNum(node, "makeupGainDB", 0), 0, 24)
	autoMakeup := getNodeNum(node, "autoMakeup", 0) >= 0.5

	for band := 0; band < r.fx.NumBands(); band++ {
		if err := r.fx.SetBandAttack(band, attack); err != nil {
			return err
		}

		if err := r.fx.SetBandRelease(band, release); err != nil {
			return err
		}

		if err := r.fx.SetBandKnee(band, knee); err != nil {
			return err
		}

		if err := r.fx.SetBandAutoMakeup(band, autoMakeup); err != nil {
			return err
		}

		if !autoMakeup {
			if err := r.fx.SetBandMakeupGain(band, makeup); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *multibandChainRuntime) Process(_ *Engine, _ compiledChainNode, block []float64) {
	if r.fx == nil {
		return
	}

	r.fx.ProcessInPlace(block)
}
