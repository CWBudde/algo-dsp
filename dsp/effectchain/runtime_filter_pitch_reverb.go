package effectchain

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/core"
	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/moog"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

// FilterDesigner builds biquad filter chains from parameters.
// This interface decouples the filter runtime from the EQ design logic
// that lives in the webdemo package.
type FilterDesigner interface {
	NormalizeFamily(family string) string
	NormalizeFamilyForType(kind, family string) string
	NormalizeOrder(kind, family string, order int) int
	ClampShape(kind, family string, freq, sampleRate, value float64) float64
	BuildChain(family, kind string, order int, freq, gainDB, q, sampleRate float64) *biquad.Chain
}

type filterRuntime struct {
	fx     *biquad.Chain
	moogLP *moog.Filter

	designer       FilterDesigner
	hasConfig      bool
	lastFamily     string
	lastKind       string
	lastOrder      int
	lastFreq       float64
	lastGainDB     float64
	lastShape      float64
	lastSampleRate float64
}

//nolint:cyclop
func (r *filterRuntime) Configure(ctx Context, p Params) error {
	family := normalizeFilterFamily(p.Str["family"], p.Type)
	kind := normalizeFilterKind(p.Type, p.Str["kind"])
	freq := core.Clamp(p.GetNum("freq", 1200), 20, ctx.SampleRate*0.49)
	gainDB := core.Clamp(p.GetNum("gain", 0), -24, 24)
	shape := core.Clamp(p.GetNum("q", 0.707), 0.2, 8)

	if family == familyMoog {
		order := int(math.Round(p.GetNum("order", 8)))
		oversampling := moogOversamplingFromOrder(order)
		resonance := core.Clamp(shape, 0, 4)

		drive := core.Clamp(math.Pow(10, gainDB/20), 0.1, 24)
		if r.hasConfig &&
			r.lastFamily == family &&
			r.lastKind == kind &&
			r.lastOrder == order &&
			floatEq(r.lastFreq, freq) &&
			floatEq(r.lastGainDB, gainDB) &&
			floatEq(r.lastShape, shape) &&
			floatEq(r.lastSampleRate, ctx.SampleRate) {
			return nil
		}

		if r.moogLP == nil {
			fx, err := moog.New(
				ctx.SampleRate,
				moog.WithVariant(moog.VariantHuovilainen),
				moog.WithOversampling(oversampling),
				moog.WithCutoffHz(freq),
				moog.WithResonance(resonance),
				moog.WithDrive(drive),
				moog.WithInputGain(1),
				moog.WithOutputGain(1),
				moog.WithNormalizeOutput(true),
			)
			if err != nil {
				return fmt.Errorf("effectchain: create moog filter: %w", err)
			}

			r.moogLP = fx
		} else {
			err := r.moogLP.SetSampleRate(ctx.SampleRate)
			if err != nil {
				return fmt.Errorf("effectchain: set moog sample rate: %w", err)
			}

			err = r.moogLP.SetOversampling(oversampling)
			if err != nil {
				return fmt.Errorf("effectchain: set moog oversampling: %w", err)
			}

			err = r.moogLP.SetCutoffHz(freq)
			if err != nil {
				return fmt.Errorf("effectchain: set moog cutoff: %w", err)
			}

			err = r.moogLP.SetResonance(resonance)
			if err != nil {
				return fmt.Errorf("effectchain: set moog resonance: %w", err)
			}

			err = r.moogLP.SetDrive(drive)
			if err != nil {
				return fmt.Errorf("effectchain: set moog drive: %w", err)
			}
		}

		r.fx = nil
		r.hasConfig = true
		r.lastFamily = family
		r.lastKind = kind
		r.lastOrder = order
		r.lastFreq = freq
		r.lastGainDB = gainDB
		r.lastShape = shape
		r.lastSampleRate = ctx.SampleRate

		return nil
	}

	r.moogLP = nil

	// Use the injected designer for biquad filter chain building.
	if r.designer != nil {
		family = r.designer.NormalizeFamily(family)
		family = r.designer.NormalizeFamilyForType(kind, family)
		order := r.designer.NormalizeOrder(kind, family, int(math.Round(p.GetNum("order", 2))))
		shape = r.designer.ClampShape(kind, family, freq, ctx.SampleRate, shape)

		if r.hasConfig &&
			r.lastFamily == family &&
			r.lastKind == kind &&
			r.lastOrder == order &&
			floatEq(r.lastFreq, freq) &&
			floatEq(r.lastGainDB, gainDB) &&
			floatEq(r.lastShape, shape) &&
			floatEq(r.lastSampleRate, ctx.SampleRate) {
			return nil
		}

		next := r.designer.BuildChain(family, kind, order, freq, gainDB, shape, ctx.SampleRate)
		switch {
		case r.fx == nil:
			r.fx = next
		case r.fx.NumSections() == next.NumSections():
			r.fx.SetGain(next.Gain())

			for i := range r.fx.NumSections() {
				r.fx.Section(i).Coefficients = next.Section(i).Coefficients
			}
		default:
			oldState := r.fx.State()
			newState := make([][2]float64, next.NumSections())
			copy(newState, oldState)
			next.SetState(newState)
			r.fx = next
		}

		r.hasConfig = true
		r.lastFamily = family
		r.lastKind = kind
		r.lastOrder = order
		r.lastFreq = freq
		r.lastGainDB = gainDB
		r.lastShape = shape
		r.lastSampleRate = ctx.SampleRate

		return nil
	}

	// Fallback: passthrough chain if no designer is provided.
	if r.fx == nil {
		r.fx = biquad.NewChain([]biquad.Coefficients{{B0: 1}})
	}

	return nil
}

func floatEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-12
}

func (r *filterRuntime) Process(block []float64) {
	if r.moogLP != nil {
		r.moogLP.ProcessInPlace(block)
		return
	}

	if r.fx != nil {
		r.fx.ProcessBlock(block)
	}
}

type bassRuntime struct {
	fx *effects.HarmonicBass
}

func (r *bassRuntime) Configure(ctx Context, p Params) error {
	hp := min(max(int(math.Round(p.GetNum("highpass", 0))), 0), 2)

	return configureHarmonicBass(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("frequency", 80), 10, 500),
		core.Clamp(p.GetNum("inputGain", 1), 0, 2),
		core.Clamp(p.GetNum("highGain", 1), 0, 2),
		core.Clamp(p.GetNum("original", 1), 0, 2),
		core.Clamp(p.GetNum("harmonic", 0), 0, 2),
		core.Clamp(p.GetNum("decay", 0), -1, 1),
		core.Clamp(p.GetNum("responseMs", 20), 1, 200),
		hp,
	)
}

func (r *bassRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type timePitchRuntime struct {
	fx *pitch.PitchShifter
}

func (r *timePitchRuntime) Configure(ctx Context, p Params) error {
	seq := core.Clamp(p.GetNum("sequence", 40), 20, 120)

	ov := core.Clamp(p.GetNum("overlap", 10), 4, 60)
	if ov >= seq {
		ov = seq - 1
	}

	return configureTimePitch(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("semitones", 0), -24, 24),
		seq,
		ov,
		core.Clamp(p.GetNum("search", 15), 2, 40),
	)
}

func (r *timePitchRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralPitchRuntime struct {
	fx *pitch.SpectralPitchShifter
}

func (r *spectralPitchRuntime) Configure(ctx Context, p Params) error {
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(p.GetNum("frameSize", 1024))))

	hop := max(int(math.Round(float64(frame)*core.Clamp(p.GetNum("hopRatio", 0.25), 0.01, 0.99))), 1)

	if hop >= frame {
		hop = frame - 1
	}

	return configureSpectralPitch(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("semitones", 0), -24, 24),
		frame,
		hop,
	)
}

func (r *spectralPitchRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type spectralFreezeRuntime struct {
	fx *effects.SpectralFreeze
}

func (r *spectralFreezeRuntime) Configure(ctx Context, p Params) error {
	frame := sanitizeSpectralPitchFrameSize(int(math.Round(p.GetNum("frameSize", 1024))))

	hop := max(int(math.Round(float64(frame)*core.Clamp(p.GetNum("hopRatio", 0.25), 0.01, 0.99))), 1)

	if hop >= frame {
		hop = frame - 1
	}

	frozen := p.GetNum("frozen", 1) >= 0.5

	return configureSpectralFreeze(
		r.fx,
		ctx.SampleRate,
		frame,
		hop,
		core.Clamp(p.GetNum("mix", 1), 0, 1),
		normalizeSpectralFreezePhaseMode(p.Str["phaseMode"]),
		frozen,
		window.TypeHann,
	)
}

func (r *spectralFreezeRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type granularRuntime struct {
	fx *effects.Granular
}

func (r *granularRuntime) Configure(ctx Context, p Params) error {
	return configureGranular(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("grainSeconds", 0.08), 0.005, 0.5),
		core.Clamp(p.GetNum("overlap", 0.5), 0, 0.95),
		core.Clamp(p.GetNum("pitch", 1), 0.25, 4),
		core.Clamp(p.GetNum("spray", 0.1), 0, 1),
		core.Clamp(p.GetNum("baseDelay", 0.08), 0, 2),
		core.Clamp(p.GetNum("mix", 1), 0, 1),
	)
}

func (r *granularRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type freeverbRuntime struct {
	fx *reverb.Reverb
}

func (r *freeverbRuntime) Configure(_ Context, p Params) error {
	configureFreeverb(
		r.fx,
		core.Clamp(p.GetNum("wet", 0.22), 0, 1.5),
		core.Clamp(p.GetNum("dry", 1), 0, 1.5),
		core.Clamp(p.GetNum("roomSize", 0.72), 0, 0.98),
		core.Clamp(p.GetNum("damp", 0.45), 0, 0.99),
		core.Clamp(p.GetNum("gain", 0.015), 0, 0.1),
	)

	return nil
}

func (r *freeverbRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

type fdnReverbRuntime struct {
	fx *reverb.FDNReverb
}

func (r *fdnReverbRuntime) Configure(ctx Context, p Params) error {
	return configureFDNReverb(
		r.fx,
		ctx.SampleRate,
		core.Clamp(p.GetNum("wet", 0.22), 0, 1.5),
		core.Clamp(p.GetNum("dry", 1), 0, 1.5),
		core.Clamp(p.GetNum("rt60", 1.8), 0.2, 8),
		core.Clamp(p.GetNum("preDelay", 0.01), 0, 0.1),
		core.Clamp(p.GetNum("damp", 0.45), 0, 0.99),
		core.Clamp(p.GetNum("modDepth", 0.002), 0, 0.01),
		core.Clamp(p.GetNum("modRate", 0.1), 0, 1),
	)
}

func (r *fdnReverbRuntime) Process(block []float64) {
	r.fx.ProcessInPlace(block)
}

const reverbModelFDN = "fdn"

// reverbRuntime delegates to either freeverb or FDN based on the model
// parameter stored during Configure.
type reverbRuntime struct {
	freeverb *freeverbRuntime
	fdn      *fdnReverbRuntime
	model    string
}

func (r *reverbRuntime) Configure(ctx Context, p Params) error {
	r.model = p.Str["model"]
	if r.model == reverbModelFDN {
		return r.fdn.Configure(ctx, p)
	}

	return r.freeverb.Configure(ctx, p)
}

func (r *reverbRuntime) Process(block []float64) {
	if r.model == reverbModelFDN {
		r.fdn.Process(block)
		return
	}

	r.freeverb.Process(block)
}
