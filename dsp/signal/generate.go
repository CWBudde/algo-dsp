package signal

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

// Generator creates deterministic signals from a shared configuration.
type Generator struct {
	cfg  core.ProcessorConfig
	seed int64
}

// Option configures a Generator.
type Option func(*Generator)

// WithSeed sets deterministic random seed for noise generation.
func WithSeed(seed int64) Option {
	return func(g *Generator) {
		g.seed = seed
	}
}

// NewGenerator creates a configured signal generator.
func NewGenerator(opts ...core.ProcessorOption) *Generator {
	return &Generator{
		cfg:  core.ApplyProcessorOptions(opts...),
		seed: 1,
	}
}

// NewGeneratorWithOptions creates a configured signal generator with signal-specific options.
func NewGeneratorWithOptions(coreOpts []core.ProcessorOption, opts ...Option) *Generator {
	g := &Generator{
		cfg:  core.ApplyProcessorOptions(coreOpts...),
		seed: 1,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

// Config returns the generator processor configuration.
func (g *Generator) Config() core.ProcessorConfig {
	return g.cfg
}

// Sine generates a sine wave.
func (g *Generator) Sine(freqHz, amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("sine samples must be > 0: %d", samples)
	}
	if g.cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("sine sample rate must be > 0: %f", g.cfg.SampleRate)
	}
	out := make([]float64, samples)
	step := 2 * math.Pi * freqHz / g.cfg.SampleRate
	for i := range out {
		out[i] = amplitude * math.Sin(step*float64(i))
	}
	return out, nil
}

// WhiteNoise generates deterministic white noise in [-amplitude, amplitude].
func (g *Generator) WhiteNoise(amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("noise samples must be > 0: %d", samples)
	}
	if amplitude < 0 {
		return nil, fmt.Errorf("noise amplitude must be >= 0: %f", amplitude)
	}
	out := make([]float64, samples)
	rng := rand.New(rand.NewSource(g.seed))
	for i := range out {
		out[i] = (rng.Float64()*2 - 1) * amplitude
	}
	return out, nil
}

// Normalize scales data to target peak amplitude and returns a new slice.
func Normalize(data []float64, targetPeak float64) ([]float64, error) {
	if targetPeak < 0 {
		return nil, fmt.Errorf("normalize target peak must be >= 0: %f", targetPeak)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("normalize input must not be empty")
	}

	maxAbs := 0.0
	for _, v := range data {
		av := math.Abs(v)
		if av > maxAbs {
			maxAbs = av
		}
	}

	out := make([]float64, len(data))
	if maxAbs == 0 || targetPeak == 0 {
		return out, nil
	}

	scale := targetPeak / maxAbs
	for i, v := range data {
		out[i] = v * scale
	}
	return out, nil
}
