package modulation

import (
	"fmt"
	"math"
)

const (
	defaultFlangerRateHz           = 0.25
	defaultFlangerDepthSeconds     = 0.0015
	defaultFlangerBaseDelaySeconds = 0.001
	defaultFlangerFeedback         = 0.25
	defaultFlangerMix              = 0.5

	minFlangerDelaySeconds = 0.0001 // 0.1 ms
	maxFlangerDelaySeconds = 0.0100 // 10 ms
)

// FlangerOption mutates flanger construction parameters.
type FlangerOption func(*flangerConfig) error

type flangerConfig struct {
	rateHz       float64
	depthSeconds float64
	baseDelay    float64
	feedback     float64
	mix          float64
}

func defaultFlangerConfig() flangerConfig {
	return flangerConfig{
		rateHz:       defaultFlangerRateHz,
		depthSeconds: defaultFlangerDepthSeconds,
		baseDelay:    defaultFlangerBaseDelaySeconds,
		feedback:     defaultFlangerFeedback,
		mix:          defaultFlangerMix,
	}
}

// WithFlangerRateHz sets modulation speed in Hz.
func WithFlangerRateHz(rateHz float64) FlangerOption {
	return func(cfg *flangerConfig) error {
		if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
			return fmt.Errorf("flanger rate must be > 0 and finite: %f", rateHz)
		}

		cfg.rateHz = rateHz

		return nil
	}
}

// WithFlangerDepthSeconds sets modulation depth in seconds.
func WithFlangerDepthSeconds(depth float64) FlangerOption {
	return func(cfg *flangerConfig) error {
		if depth < 0 || math.IsNaN(depth) || math.IsInf(depth, 0) {
			return fmt.Errorf("flanger depth must be >= 0 and finite: %f", depth)
		}

		cfg.depthSeconds = depth

		return nil
	}
}

// WithFlangerBaseDelaySeconds sets base delay in seconds.
func WithFlangerBaseDelaySeconds(baseDelay float64) FlangerOption {
	return func(cfg *flangerConfig) error {
		if baseDelay < minFlangerDelaySeconds || baseDelay > maxFlangerDelaySeconds ||
			math.IsNaN(baseDelay) || math.IsInf(baseDelay, 0) {
			return fmt.Errorf("flanger base delay must be in [%f, %f]: %f",
				minFlangerDelaySeconds, maxFlangerDelaySeconds, baseDelay)
		}

		cfg.baseDelay = baseDelay

		return nil
	}
}

// WithFlangerFeedback sets feedback amount in [-0.99, 0.99].
func WithFlangerFeedback(feedback float64) FlangerOption {
	return func(cfg *flangerConfig) error {
		if feedback < -0.99 || feedback > 0.99 || math.IsNaN(feedback) || math.IsInf(feedback, 0) {
			return fmt.Errorf("flanger feedback must be in [-0.99, 0.99]: %f", feedback)
		}

		cfg.feedback = feedback

		return nil
	}
}

// WithFlangerMix sets wet amount in [0, 1].
func WithFlangerMix(mix float64) FlangerOption {
	return func(cfg *flangerConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("flanger mix must be in [0, 1]: %f", mix)
		}

		cfg.mix = mix

		return nil
	}
}

// Flanger is a short modulated-delay effect with feedback and wet/dry mix.
type Flanger struct {
	sampleRate float64
	rateHz     float64
	depth      float64
	baseDelay  float64
	feedback   float64
	mix        float64

	lfoPhase float64

	delayLine []float64
	write     int
	maxDelay  int
}

// NewFlanger creates a flanger with practical defaults and optional overrides.
func NewFlanger(sampleRate float64, opts ...FlangerOption) (*Flanger, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("flanger sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultFlangerConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	f := &Flanger{
		sampleRate: sampleRate,
		rateHz:     cfg.rateHz,
		depth:      cfg.depthSeconds,
		baseDelay:  cfg.baseDelay,
		feedback:   cfg.feedback,
		mix:        cfg.mix,
	}

	err := f.validateParams()
	if err != nil {
		return nil, err
	}

	err = f.reconfigureDelayLine()
	if err != nil {
		return nil, err
	}

	return f, nil
}

// SetSampleRate updates sample rate.
func (f *Flanger) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("flanger sample rate must be > 0 and finite: %f", sampleRate)
	}

	f.sampleRate = sampleRate

	return f.reconfigureDelayLine()
}

// SetRateHz sets modulation speed in Hz.
func (f *Flanger) SetRateHz(rateHz float64) error {
	if rateHz <= 0 || math.IsNaN(rateHz) || math.IsInf(rateHz, 0) {
		return fmt.Errorf("flanger rate must be > 0 and finite: %f", rateHz)
	}

	f.rateHz = rateHz

	return nil
}

// SetDepthSeconds sets modulation depth in seconds.
func (f *Flanger) SetDepthSeconds(depth float64) error {
	if depth < 0 || math.IsNaN(depth) || math.IsInf(depth, 0) {
		return fmt.Errorf("flanger depth must be >= 0 and finite: %f", depth)
	}

	prev := f.depth

	f.depth = depth

	err := f.reconfigureDelayLine()
	if err != nil {
		f.depth = prev
		return err
	}

	return nil
}

// SetBaseDelaySeconds sets base delay in seconds.
func (f *Flanger) SetBaseDelaySeconds(baseDelay float64) error {
	if baseDelay < minFlangerDelaySeconds || baseDelay > maxFlangerDelaySeconds ||
		math.IsNaN(baseDelay) || math.IsInf(baseDelay, 0) {
		return fmt.Errorf("flanger base delay must be in [%f, %f]: %f",
			minFlangerDelaySeconds, maxFlangerDelaySeconds, baseDelay)
	}

	prev := f.baseDelay

	f.baseDelay = baseDelay

	err := f.reconfigureDelayLine()
	if err != nil {
		f.baseDelay = prev
		return err
	}

	return nil
}

// SetFeedback sets feedback amount in [-0.99, 0.99].
func (f *Flanger) SetFeedback(feedback float64) error {
	if feedback < -0.99 || feedback > 0.99 || math.IsNaN(feedback) || math.IsInf(feedback, 0) {
		return fmt.Errorf("flanger feedback must be in [-0.99, 0.99]: %f", feedback)
	}

	f.feedback = feedback

	return nil
}

// SetMix sets wet amount in [0, 1].
func (f *Flanger) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("flanger mix must be in [0, 1]: %f", mix)
	}

	f.mix = mix

	return nil
}

// Reset clears delay and LFO state.
func (f *Flanger) Reset() {
	for i := range f.delayLine {
		f.delayLine[i] = 0
	}

	f.write = 0
	f.lfoPhase = 0
}

// Process processes one sample.
func (f *Flanger) Process(sample float64) float64 {
	mod := 0.5 * (1 + math.Sin(f.lfoPhase))

	delaySamples := (f.baseDelay + f.depth*mod) * f.sampleRate
	if delaySamples < 1 {
		delaySamples = 1
	}

	delayed := f.sampleFractionalDelay(delaySamples)

	f.delayLine[f.write] = sample + delayed*f.feedback

	f.write++
	if f.write >= len(f.delayLine) {
		f.write = 0
	}

	f.lfoPhase += 2 * math.Pi * f.rateHz / f.sampleRate
	if f.lfoPhase >= 2*math.Pi {
		f.lfoPhase -= 2 * math.Pi
	}

	return sample*(1-f.mix) + delayed*f.mix
}

// ProcessSample is an alias for Process.
func (f *Flanger) ProcessSample(sample float64) float64 {
	return f.Process(sample)
}

// ProcessInPlace applies flanging to buf in place.
func (f *Flanger) ProcessInPlace(buf []float64) error {
	for i := range buf {
		buf[i] = f.Process(buf[i])
	}

	return nil
}

// SampleRate returns sample rate in Hz.
func (f *Flanger) SampleRate() float64 { return f.sampleRate }

// RateHz returns LFO speed in Hz.
func (f *Flanger) RateHz() float64 { return f.rateHz }

// DepthSeconds returns modulation depth in seconds.
func (f *Flanger) DepthSeconds() float64 { return f.depth }

// BaseDelaySeconds returns base delay in seconds.
func (f *Flanger) BaseDelaySeconds() float64 { return f.baseDelay }

// Feedback returns feedback amount in [-0.99, 0.99].
func (f *Flanger) Feedback() float64 { return f.feedback }

// Mix returns wet amount in [0, 1].
func (f *Flanger) Mix() float64 { return f.mix }

//nolint:cyclop
func (f *Flanger) validateParams() error {
	if f.sampleRate <= 0 || math.IsNaN(f.sampleRate) || math.IsInf(f.sampleRate, 0) {
		return fmt.Errorf("flanger sample rate must be > 0 and finite: %f", f.sampleRate)
	}

	if f.rateHz <= 0 || math.IsNaN(f.rateHz) || math.IsInf(f.rateHz, 0) {
		return fmt.Errorf("flanger rate must be > 0 and finite: %f", f.rateHz)
	}

	if f.depth < 0 || math.IsNaN(f.depth) || math.IsInf(f.depth, 0) {
		return fmt.Errorf("flanger depth must be >= 0 and finite: %f", f.depth)
	}

	if f.baseDelay < minFlangerDelaySeconds || f.baseDelay > maxFlangerDelaySeconds ||
		math.IsNaN(f.baseDelay) || math.IsInf(f.baseDelay, 0) {
		return fmt.Errorf("flanger base delay must be in [%f, %f]: %f",
			minFlangerDelaySeconds, maxFlangerDelaySeconds, f.baseDelay)
	}

	if f.baseDelay+f.depth > maxFlangerDelaySeconds {
		return fmt.Errorf("flanger max delay exceeds %f seconds: base=%f depth=%f",
			maxFlangerDelaySeconds, f.baseDelay, f.depth)
	}

	if f.feedback < -0.99 || f.feedback > 0.99 || math.IsNaN(f.feedback) || math.IsInf(f.feedback, 0) {
		return fmt.Errorf("flanger feedback must be in [-0.99, 0.99]: %f", f.feedback)
	}

	if f.mix < 0 || f.mix > 1 || math.IsNaN(f.mix) || math.IsInf(f.mix, 0) {
		return fmt.Errorf("flanger mix must be in [0, 1]: %f", f.mix)
	}

	return nil
}

func (f *Flanger) reconfigureDelayLine() error {
	err := f.validateParams()
	if err != nil {
		return err
	}

	needed := max(int(math.Ceil((f.baseDelay+f.depth)*f.sampleRate))+3, 4)

	if needed == len(f.delayLine) {
		f.maxDelay = needed - 3
		return nil
	}

	old := f.delayLine
	oldWrite := f.write
	f.delayLine = make([]float64, needed)
	f.write = 0
	f.maxDelay = needed - 3

	if len(old) > 0 {
		copyCount := min(len(old), len(f.delayLine))

		for i := range copyCount {
			src := oldWrite - 1 - i
			if src < 0 {
				src += len(old)
			}

			dst := f.write - 1 - i
			if dst < 0 {
				dst += len(f.delayLine)
			}

			f.delayLine[dst] = old[src]
		}
	}

	return nil
}

func (f *Flanger) sampleFractionalDelay(delay float64) float64 {
	if delay < 0 {
		delay = 0
	}

	maxDelay := float64(f.maxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	p := int(math.Floor(delay))
	t := delay - float64(p)

	xm1 := f.sampleDelayInt(maxInt(0, p-1))
	x0 := f.sampleDelayInt(p)
	x1 := f.sampleDelayInt(p + 1)
	x2 := f.sampleDelayInt(p + 2)

	return hermite4(t, xm1, x0, x1, x2)
}

func (f *Flanger) sampleDelayInt(delay int) float64 {
	if delay < 0 || delay >= len(f.delayLine) {
		return 0
	}

	idx := f.write - delay
	if idx < 0 {
		idx += len(f.delayLine)
	}

	return f.delayLine[idx]
}
