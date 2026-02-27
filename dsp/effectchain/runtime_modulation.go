package effectchain

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

type chorusRuntime struct {
	fx *modulation.Chorus
}

func (r *chorusRuntime) Configure(ctx Context, p Params) error {
	stages := min(max(int(math.Round(p.GetNum("stages", 3))), 1), 6)

	return configureChorus(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("mix", 0.18), 0, 1),
		core.Clamp(p.GetNum("depth", 0.003), 0, 0.01),
		core.Clamp(p.GetNum("speedHz", 0.35), 0.05, 5),
		stages,
	)
}

func (r *chorusRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type flangerRuntime struct {
	fx *modulation.Flanger
}

func (r *flangerRuntime) Configure(ctx Context, p Params) error {
	return configureFlanger(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("rateHz", 0.25), 0.05, 5),
		core.Clamp(p.GetNum("baseDelay", 0.001), 0.0001, 0.01),
		core.Clamp(p.GetNum("depth", 0.0015), 0, 0.0099),
		core.Clamp(p.GetNum("feedback", 0.25), -0.99, 0.99),
		core.Clamp(p.GetNum("mix", 0.5), 0, 1),
	)
}

func (r *flangerRuntime) Process(block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type ringModRuntime struct {
	fx *modulation.RingModulator
}

func (r *ringModRuntime) Configure(ctx Context, p Params) error {
	return configureRingMod(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("carrierHz", 440), 1, ctx.SampleRate*0.49),
		core.Clamp(p.GetNum("mix", 1), 0, 1),
	)
}

func (r *ringModRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type bitCrusherRuntime struct {
	fx *effects.BitCrusher
}

func (r *bitCrusherRuntime) Configure(ctx Context, p Params) error {
	ds := min(max(int(math.Round(p.GetNum("downsample", 4))), 1), 256)

	return configureBitCrusher(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("bitDepth", 8), 1, 32),
		ds,
		core.Clamp(p.GetNum("mix", 1), 0, 1),
	)
}

func (r *bitCrusherRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type distortionRuntime struct {
	fx *effects.Distortion
}

func (r *distortionRuntime) Configure(ctx Context, p Params) error {
	mode := normalizeDistortionMode(p.Str["mode"])
	approx := normalizeDistortionApproxMode(p.Str["approx"])

	return configureDistortion(
		r.fx,
		ctx.SampleRate,
		mode,
		approx,
		core.Clamp(p.GetNum("drive", 1.8), 0.01, 20),
		core.Clamp(p.GetNum("mix", 1.0), 0, 1),
		core.Clamp(p.GetNum("output", 1.0), 0, 4),
		core.Clamp(p.GetNum("clip", 1.0), 0.05, 1),
		core.Clamp(p.GetNum("shape", 0.5), 0, 1),
		core.Clamp(p.GetNum("bias", 0), -1, 1),
		3,
		effects.ChebyshevHarmonicAll,
		false,
		1.0,
		false,
	)
}

func (r *distortionRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type distChebRuntime struct {
	fx *effects.Distortion
}

func (r *distChebRuntime) Configure(ctx Context, p Params) error {
	approx := normalizeDistortionApproxMode(p.Str["approx"])
	chebMode := normalizeChebyshevHarmonicMode(p.Str["harmonic"])

	chebOrder := min(max(int(math.Round(p.GetNum("order", 3))), 1), 16)

	chebInvert := p.GetNum("invert", 0) >= 0.5
	chebDCBypass := p.GetNum("dcBypass", 0) >= 0.5

	err := configureDistortion(
		r.fx,
		ctx.SampleRate,
		effects.DistortionModeChebyshev,
		approx,
		core.Clamp(p.GetNum("drive", 1.0), 0.01, 20),
		core.Clamp(p.GetNum("mix", 1.0), 0, 1),
		core.Clamp(p.GetNum("output", 1.0), 0, 4),
		1.0,
		0.5,
		0.0,
		chebOrder,
		chebMode,
		chebInvert,
		core.Clamp(p.GetNum("gain", 1.0), 0, 4),
		chebDCBypass,
	)
	if err != nil {
		return err
	}

	weights := make([]float64, 16)
	for k := range 16 {
		weights[k] = p.GetNum(fmt.Sprintf("w%d", k+1), 0)
	}

	err = r.fx.SetChebyshevWeights(weights)
	if err != nil {
		return fmt.Errorf("effectchain: set chebyshev weights: %w", err)
	}

	return nil
}

func (r *distChebRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type transformerRuntime struct {
	fx *effects.TransformerSimulation
}

func normalizeTransformerOversampling(value int) int {
	switch {
	case value <= 3:
		return 2
	case value <= 6:
		return 4
	default:
		return 8
	}
}

func (r *transformerRuntime) Configure(ctx Context, p Params) error {
	quality := normalizeTransformerQuality(p.Str["quality"])

	oversampling := int(math.Round(p.GetNum("oversampling", 4)))
	switch oversampling {
	case 2, 4, 8:
	default:
		oversampling = normalizeTransformerOversampling(oversampling)
	}

	return configureTransformer(
		r.fx,
		ctx.SampleRate,
		quality,
		core.Clamp(p.GetNum("drive", 2.0), 0.1, 30),
		core.Clamp(p.GetNum("mix", 1.0), 0, 1),
		core.Clamp(p.GetNum("output", 1.0), 0, 4),
		core.Clamp(p.GetNum("highpassHz", 25), 5, ctx.SampleRate*0.45),
		core.Clamp(p.GetNum("dampingHz", 9000), 200, ctx.SampleRate*0.49),
		oversampling,
	)
}

func (r *transformerRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

// widenerRuntime applies the stereo widener to a mono signal using a
// short decorrelation delay to approximate a stereo side signal, then folds back to mono.
type widenerRuntime struct {
	fx         *spatial.StereoWidener
	sampleRate float64
	mix        float64
	scratchBuf []float64
}

func (r *widenerRuntime) Configure(ctx Context, p Params) error {
	r.sampleRate = ctx.SampleRate
	r.mix = core.Clamp(p.GetNum("mix", 0.5), 0, 1)

	return configureWidener(r.fx, ctx.SampleRate, core.Clamp(p.GetNum("width", 1), 0, 4))
}

func (r *widenerRuntime) Process(block []float64) {
	if len(block) == 0 || r.fx == nil {
		return
	}

	if len(r.scratchBuf) < len(block) {
		r.scratchBuf = make([]float64, len(block))
	}

	dry := r.scratchBuf[:len(block)]
	copy(dry, block)

	delaySamples := max(int(r.sampleRate*0.001), 1)

	for i := range block {
		left := dry[i]

		right := dry[i]
		if i >= delaySamples {
			right = dry[i-delaySamples]
		}

		l2, r2 := r.fx.ProcessStereo(left, right)
		wet := 0.5 * (l2 + r2)
		block[i] = dry[i]*(1-r.mix) + wet*r.mix
	}
}

type phaserRuntime struct {
	fx *modulation.Phaser
}

func (r *phaserRuntime) Configure(ctx Context, p Params) error {
	minHz := core.Clamp(p.GetNum("minFreqHz", 300), 20, ctx.SampleRate*0.45)
	maxHz := core.Clamp(p.GetNum("maxFreqHz", 1600), minHz+1, ctx.SampleRate*0.49)

	stages := min(max(int(math.Round(p.GetNum("stages", 6))), 1), 12)

	return configurePhaser(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("rateHz", 0.4), 0.05, 5),
		minHz,
		maxHz,
		stages,
		core.Clamp(p.GetNum("feedback", 0.2), -0.99, 0.99),
		core.Clamp(p.GetNum("mix", 0.5), 0, 1),
	)
}

func (r *phaserRuntime) Process(block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type tremoloRuntime struct {
	fx *modulation.Tremolo
}

func (r *tremoloRuntime) Configure(ctx Context, p Params) error {
	return configureTremolo(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("rateHz", 4), 0.05, 20),
		core.Clamp(p.GetNum("depth", 0.6), 0, 1),
		core.Clamp(p.GetNum("smoothingMs", 5), 0, 200),
		core.Clamp(p.GetNum("mix", 1), 0, 1),
	)
}

func (r *tremoloRuntime) Process(block []float64) {
	_ = r.fx.ProcessInPlace(block)
}

type delayRuntime struct {
	fx *effects.Delay
}

func (r *delayRuntime) Configure(ctx Context, p Params) error {
	return configureDelay(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("time", 0.25), 0.001, 2),
		core.Clamp(p.GetNum("feedback", 0.35), 0, 0.99),
		core.Clamp(p.GetNum("mix", 0.25), 0, 1),
	)
}

func (r *delayRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type simpleDelayRuntime struct {
	sampleRate   float64
	delayMs      float64
	delaySamples int
	write        int
	buf          []float64
}

func (r *simpleDelayRuntime) Configure(ctx Context, p Params) error {
	r.sampleRate = ctx.SampleRate
	r.delayMs = core.Clamp(p.GetNum("delayMs", 20), 0, 500)

	r.delaySamples = max(int(math.Round(r.delayMs*r.sampleRate/1000.0)), 0)

	size := max(r.delaySamples+1, 1)

	if len(r.buf) != size {
		r.buf = make([]float64, size)
		r.write = 0
	}

	return nil
}

func (r *simpleDelayRuntime) Process(block []float64) {
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
