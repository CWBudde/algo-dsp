package effectchain

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
	"github.com/cwbudde/algo-dsp/dsp/effects/dynamics"
	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/effects/reverb"
	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

type registryConfig struct {
	irProvider     IRProvider
	filterDesigner FilterDesigner
}

func wrapRuntimeInitErr(effectType string, err error) error {
	return fmt.Errorf("effectchain: create %s runtime: %w", effectType, err)
}

// RegistryOption configures the default registry.
type RegistryOption func(*registryConfig)

// WithIRProvider sets the impulse response provider for convolution reverb.
func WithIRProvider(p IRProvider) RegistryOption {
	return func(c *registryConfig) { c.irProvider = p }
}

// WithFilterDesigner sets the filter designer for biquad filter chain building.
func WithFilterDesigner(d FilterDesigner) RegistryOption {
	return func(c *registryConfig) { c.filterDesigner = d }
}

// DefaultRegistry returns a Registry pre-populated with all built-in effect runtimes.
//
//nolint:funlen
func DefaultRegistry(opts ...RegistryOption) *Registry {
	cfg := &registryConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	r := NewRegistry()

	r.MustRegister("chorus", func(_ Context) (Runtime, error) {
		fx, err := modulation.NewChorus()
		if err != nil {
			return nil, wrapRuntimeInitErr("chorus", err)
		}

		return &chorusRuntime{fx: fx}, nil
	})
	r.MustRegister("flanger", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewFlanger(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("flanger", err)
		}

		return &flangerRuntime{fx: fx}, nil
	})
	r.MustRegister("ringmod", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewRingModulator(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("ringmod", err)
		}

		return &ringModRuntime{fx: fx}, nil
	})
	r.MustRegister("bitcrusher", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewBitCrusher(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("bitcrusher", err)
		}

		return &bitCrusherRuntime{fx: fx}, nil
	})
	r.MustRegister("distortion", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewDistortion(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("distortion", err)
		}

		return &distortionRuntime{fx: fx}, nil
	})
	r.MustRegister("dist-cheb", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewDistortion(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dist-cheb", err)
		}

		return &distChebRuntime{fx: fx}, nil
	})
	r.MustRegister("transformer", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewTransformerSimulation(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("transformer", err)
		}

		return &transformerRuntime{fx: fx}, nil
	})
	r.MustRegister("widener", func(ctx Context) (Runtime, error) {
		fx, err := spatial.NewStereoWidener(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("widener", err)
		}

		return &widenerRuntime{fx: fx}, nil
	})
	r.MustRegister("phaser", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewPhaser(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("phaser", err)
		}

		return &phaserRuntime{fx: fx}, nil
	})
	r.MustRegister("tremolo", func(ctx Context) (Runtime, error) {
		fx, err := modulation.NewTremolo(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("tremolo", err)
		}

		return &tremoloRuntime{fx: fx}, nil
	})
	r.MustRegister("delay", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewDelay(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("delay", err)
		}

		return &delayRuntime{fx: fx}, nil
	})
	r.MustRegister("delay-simple", func(_ Context) (Runtime, error) {
		return &simpleDelayRuntime{}, nil
	})

	// Register all filter variants.
	filterNodeTypes := []string{
		"filter",
		"filter-lowpass",
		"filter-highpass",
		"filter-bandpass",
		"filter-notch",
		"filter-allpass",
		"filter-peak",
		"filter-lowshelf",
		"filter-highshelf",
		"filter-moog",
	}
	for _, effectType := range filterNodeTypes {
		t := effectType
		r.MustRegister(t, func(_ Context) (Runtime, error) {
			return &filterRuntime{
				fx:       biquad.NewChain([]biquad.Coefficients{{B0: 1}}),
				designer: cfg.filterDesigner,
			}, nil
		})
	}

	r.MustRegister("bass", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewHarmonicBass(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("bass", err)
		}

		return &bassRuntime{fx: fx}, nil
	})
	r.MustRegister("pitch-time", func(ctx Context) (Runtime, error) {
		fx, err := pitch.NewPitchShifter(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("pitch-time", err)
		}

		return &timePitchRuntime{fx: fx}, nil
	})
	r.MustRegister("pitch-spectral", func(ctx Context) (Runtime, error) {
		fx, err := pitch.NewSpectralPitchShifter(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("pitch-spectral", err)
		}

		return &spectralPitchRuntime{fx: fx}, nil
	})
	r.MustRegister("spectral-freeze", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewSpectralFreeze(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("spectral-freeze", err)
		}

		return &spectralFreezeRuntime{fx: fx}, nil
	})
	r.MustRegister("granular", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewGranular(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("granular", err)
		}

		return &granularRuntime{fx: fx}, nil
	})
	r.MustRegister("reverb", func(ctx Context) (Runtime, error) {
		fdn, err := reverb.NewFDNReverb(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("reverb", err)
		}

		return &reverbRuntime{
			freeverb: &freeverbRuntime{fx: reverb.NewReverb()},
			fdn:      &fdnReverbRuntime{fx: fdn},
		}, nil
	})
	r.MustRegister("reverb-freeverb", func(_ Context) (Runtime, error) {
		return &freeverbRuntime{fx: reverb.NewReverb()}, nil
	})
	r.MustRegister("reverb-fdn", func(ctx Context) (Runtime, error) {
		fdn, err := reverb.NewFDNReverb(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("reverb-fdn", err)
		}

		return &fdnReverbRuntime{fx: fdn}, nil
	})
	r.MustRegister("reverb-conv", func(_ Context) (Runtime, error) {
		return &convReverbRuntime{irIndex: -1, irProvider: cfg.irProvider}, nil
	})

	// Dynamics processors.
	r.MustRegister("dyn-compressor", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewCompressor(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-compressor", err)
		}

		return &compressorRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-limiter", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewLimiter(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-limiter", err)
		}

		return &limiterRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-lookahead", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewLookaheadLimiter(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-lookahead", err)
		}

		return &lookaheadLimiterRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-gate", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewGate(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-gate", err)
		}

		return &gateRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-expander", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewExpander(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-expander", err)
		}

		return &expanderRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-deesser", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewDeEsser(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-deesser", err)
		}

		return &deesserRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-transient", func(ctx Context) (Runtime, error) {
		fx, err := dynamics.NewTransientShaper(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("dyn-transient", err)
		}

		return &transientShaperRuntime{fx: fx}, nil
	})
	r.MustRegister("dyn-multiband", func(_ Context) (Runtime, error) {
		return &multibandRuntime{}, nil
	})
	r.MustRegister("vocoder", func(ctx Context) (Runtime, error) {
		fx, err := effects.NewVocoder(ctx.SampleRate)
		if err != nil {
			return nil, wrapRuntimeInitErr("vocoder", err)
		}

		return &vocoderRuntime{fx: fx}, nil
	})

	return r
}
