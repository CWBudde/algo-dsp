package signal

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

const defaultSeed int64 = 1

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
		seed: defaultSeed,
	}
}

// NewGeneratorWithOptions creates a configured signal generator with signal-specific options.
func NewGeneratorWithOptions(coreOpts []core.ProcessorOption, opts ...Option) *Generator {
	g := &Generator{
		cfg:  core.ApplyProcessorOptions(coreOpts...),
		seed: defaultSeed,
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

// Seed returns the deterministic RNG seed used by stochastic generators.
func (g *Generator) Seed() int64 {
	return g.seed
}

// SetSeed sets the deterministic RNG seed used by stochastic generators.
func (g *Generator) SetSeed(seed int64) {
	g.seed = seed
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

// Multisine generates an equal-weighted sum of sine tones from freqsHz.
func (g *Generator) Multisine(freqsHz []float64, amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("multisine samples must be > 0: %d", samples)
	}

	if len(freqsHz) == 0 {
		return nil, errors.New("multisine frequencies must not be empty")
	}

	if g.cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("multisine sample rate must be > 0: %f", g.cfg.SampleRate)
	}

	out := make([]float64, samples)

	toneAmp := amplitude / float64(len(freqsHz))
	for _, freqHz := range freqsHz {
		step := 2 * math.Pi * freqHz / g.cfg.SampleRate
		for i := range out {
			out[i] += toneAmp * math.Sin(step*float64(i))
		}
	}

	return out, nil
}

// Impulse generates an impulse with amplitude at pos and zeros elsewhere.
func (g *Generator) Impulse(amplitude float64, samples, pos int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("impulse samples must be > 0: %d", samples)
	}

	if pos < 0 || pos >= samples {
		return nil, fmt.Errorf("impulse position out of range: pos=%d samples=%d", pos, samples)
	}

	out := make([]float64, samples)
	out[pos] = amplitude

	return out, nil
}

// LinearSweep generates a linear-frequency sine sweep between startHz and endHz.
func (g *Generator) LinearSweep(startHz, endHz, amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("linear sweep samples must be > 0: %d", samples)
	}

	if g.cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("linear sweep sample rate must be > 0: %f", g.cfg.SampleRate)
	}

	duration := float64(samples) / g.cfg.SampleRate
	k := (endHz - startHz) / duration

	out := make([]float64, samples)
	for i := range out {
		t := float64(i) / g.cfg.SampleRate
		phase := 2 * math.Pi * (startHz*t + 0.5*k*t*t)
		out[i] = amplitude * math.Sin(phase)
	}

	return out, nil
}

// LogSweep generates an exponential-frequency sine sweep between startHz and endHz.
func (g *Generator) LogSweep(startHz, endHz, amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("log sweep samples must be > 0: %d", samples)
	}

	if g.cfg.SampleRate <= 0 {
		return nil, fmt.Errorf("log sweep sample rate must be > 0: %f", g.cfg.SampleRate)
	}

	if startHz <= 0 || endHz <= 0 {
		return nil, fmt.Errorf("log sweep frequencies must be > 0: start=%f end=%f", startHz, endHz)
	}

	duration := float64(samples) / g.cfg.SampleRate
	k := math.Log(endHz/startHz) / duration

	out := make([]float64, samples)
	if k == 0 {
		return g.Sine(startHz, amplitude, samples)
	}

	for i := range out {
		t := float64(i) / g.cfg.SampleRate
		phase := 2 * math.Pi * startHz * ((math.Exp(k*t) - 1) / k)
		out[i] = amplitude * math.Sin(phase)
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

// PinkNoise generates deterministic pink noise (1/f spectrum) using the
// Voss-McCartney algorithm with 5 contribution bands. The output approximates
// a -3 dB/octave spectral slope.
func (g *Generator) PinkNoise(amplitude float64, samples int) ([]float64, error) {
	if samples <= 0 {
		return nil, fmt.Errorf("noise samples must be > 0: %d", samples)
	}

	if amplitude < 0 {
		return nil, fmt.Errorf("noise amplitude must be >= 0: %f", amplitude)
	}

	// Band weights and cumulative probability thresholds from Voss-McCartney.
	pA := [5]float64{0.23980, 0.18727, 0.16380, 0.194685, 0.214463}
	pSUM := [5]float64{0.00198, 0.01478, 0.06378, 0.23378, 0.91578}

	rng := rand.New(rand.NewSource(g.seed))

	var contributions [5]float64

	out := make([]float64, samples)

	for i := range out {
		ur1 := rng.Float64()
		ur2 := rng.Float64()
		val := ur2*2 - 1

		for b := range 5 {
			if ur1 <= pSUM[b] {
				contributions[b] = val * pA[b]
				break
			}
		}

		sum := 0.0
		for _, c := range contributions {
			sum += c
		}

		out[i] = sum * amplitude
	}

	return out, nil
}

// Normalize scales data to target peak amplitude and returns a new slice.
func Normalize(data []float64, targetPeak float64) ([]float64, error) {
	if targetPeak < 0 {
		return nil, fmt.Errorf("normalize target peak must be >= 0: %f", targetPeak)
	}

	if len(data) == 0 {
		return nil, errors.New("normalize input must not be empty")
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

// Clip limits all values to [minVal, maxVal] and returns a new slice.
func Clip(data []float64, minVal, maxVal float64) ([]float64, error) {
	if minVal > maxVal {
		return nil, fmt.Errorf("clip min must be <= max: min=%f max=%f", minVal, maxVal)
	}

	out := make([]float64, len(data))
	for i, v := range data {
		switch {
		case v < minVal:
			out[i] = minVal
		case v > maxVal:
			out[i] = maxVal
		default:
			out[i] = v
		}
	}

	return out, nil
}

// RemoveDC removes the mean from the input and returns a new slice.
func RemoveDC(data []float64) ([]float64, error) {
	if len(data) == 0 {
		return nil, errors.New("remove dc input must not be empty")
	}

	sum := 0.0
	for _, v := range data {
		sum += v
	}

	mean := sum / float64(len(data))

	out := make([]float64, len(data))
	for i, v := range data {
		out[i] = v - mean
	}

	return out, nil
}

// EnvelopeFollower computes a peak-style envelope from abs(input).
// attack and release must be in [0, 1], where larger values track faster.
func EnvelopeFollower(data []float64, attack, release float64) ([]float64, error) {
	if attack < 0 || attack > 1 {
		return nil, fmt.Errorf("attack must be in [0,1]: %f", attack)
	}

	if release < 0 || release > 1 {
		return nil, fmt.Errorf("release must be in [0,1]: %f", release)
	}

	out := make([]float64, len(data))
	env := 0.0

	for i, v := range data {
		target := math.Abs(v)

		coeff := release
		if target > env {
			coeff = attack
		}

		env += coeff * (target - env)
		out[i] = env
	}

	return out, nil
}
