package spatial

import (
	"fmt"
	"math"
)

const (
	defaultPanPosition = 0.0 // centre

	minPanPosition = -1.0 // hard left
	maxPanPosition = 1.0  // hard right

	defaultAutoPanRate = 0.0 // disabled by default

	minAutoPanRate = 0.0  // exclusive lower bound when enabled
	maxAutoPanRate = 20.0 // above ~20 Hz the sweep is heard as modulation, not movement

	defaultAutoPanDepth = 1.0

	minAutoPanDepth = 0.0
	maxAutoPanDepth = 1.0
)

// PanLaw selects the gain curve applied across the pan range. Each law trades
// off between preserving power (for uncorrelated material) and preserving
// summed amplitude (for mono-compatible material); the name of the law refers
// to the level at the centre position relative to a hard-panned channel.
type PanLaw int

const (
	// PanLawEqualPower is the constant-power (sine/cosine) law: the centre sits
	// 3 dB below a hard-panned channel and gL^2+gR^2 is 1 at every position.
	// This is the default and the usual choice for panning a mono source.
	PanLawEqualPower PanLaw = iota
	// PanLawLinear is the constant-amplitude law: the centre sits 6 dB below a
	// hard-panned channel and gL+gR is 1 at every position, which keeps a
	// mono fold-down of the panned signal at a constant level.
	PanLawLinear
	// PanLawCompromise is the geometric mean of the equal-power and linear
	// laws, placing the centre 4.5 dB down. It is a common console pan law
	// that trades a little of each invariant for a middle ground.
	PanLawCompromise
)

func (l PanLaw) valid() bool {
	return l == PanLawEqualPower || l == PanLawLinear || l == PanLawCompromise
}

// StereoPannerOption mutates stereo panner construction parameters.
type StereoPannerOption func(*stereoPannerConfig) error

type stereoPannerConfig struct {
	position     float64
	law          PanLaw
	autoPanRate  float64 // 0 means disabled
	autoPanDepth float64
}

func defaultStereoPannerConfig() stereoPannerConfig {
	return stereoPannerConfig{
		position:     defaultPanPosition,
		law:          PanLawEqualPower,
		autoPanRate:  defaultAutoPanRate,
		autoPanDepth: defaultAutoPanDepth,
	}
}

// WithPanPosition sets the pan position: -1 is hard left, 0 is centre and +1
// is hard right.
func WithPanPosition(position float64) StereoPannerOption {
	return func(cfg *stereoPannerConfig) error {
		if err := validatePanPosition(position); err != nil {
			return err
		}

		cfg.position = position

		return nil
	}
}

// WithPanLaw selects the pan law (default [PanLawEqualPower]).
func WithPanLaw(law PanLaw) StereoPannerOption {
	return func(cfg *stereoPannerConfig) error {
		if !law.valid() {
			return fmt.Errorf("stereo panner: invalid pan law: %d", law)
		}

		cfg.law = law

		return nil
	}
}

// WithAutoPanRate enables the auto-pan LFO at the given rate in Hz. Set to 0
// to disable (default). Valid range when enabled: (0, 20] Hz.
func WithAutoPanRate(hz float64) StereoPannerOption {
	return func(cfg *stereoPannerConfig) error {
		if err := validateAutoPanRate(hz); err != nil {
			return err
		}

		cfg.autoPanRate = hz

		return nil
	}
}

// WithAutoPanDepth sets how far the auto-pan LFO swings the position around
// the configured pan position. 0 disables the swing, 1 (default) sweeps the
// full pan range.
func WithAutoPanDepth(depth float64) StereoPannerOption {
	return func(cfg *stereoPannerConfig) error {
		if err := validateAutoPanDepth(depth); err != nil {
			return err
		}

		cfg.autoPanDepth = depth

		return nil
	}
}

func validatePanPosition(position float64) error {
	if position < minPanPosition || position > maxPanPosition ||
		math.IsNaN(position) || math.IsInf(position, 0) {
		return fmt.Errorf("stereo panner position must be in [%g, %g]: %f",
			minPanPosition, maxPanPosition, position)
	}

	return nil
}

func validateAutoPanRate(hz float64) error {
	if hz == 0 {
		return nil
	}

	if hz <= minAutoPanRate || hz > maxAutoPanRate || math.IsNaN(hz) || math.IsInf(hz, 0) {
		return fmt.Errorf("stereo panner auto-pan rate must be 0 (disabled) or in (%g, %g] Hz: %f",
			minAutoPanRate, maxAutoPanRate, hz)
	}

	return nil
}

func validateAutoPanDepth(depth float64) error {
	if depth < minAutoPanDepth || depth > maxAutoPanDepth ||
		math.IsNaN(depth) || math.IsInf(depth, 0) {
		return fmt.Errorf("stereo panner auto-pan depth must be in [%g, %g]: %f",
			minAutoPanDepth, maxAutoPanDepth, depth)
	}

	return nil
}

// StereoPanner places a signal in the stereo field using a selectable pan law.
//
// It offers two modes. The mono-pan mode ([StereoPanner.ProcessMono]) spreads a
// single input across a stereo pair using the raw law gains, so a centred
// source sits below unity by the law's centre level (3 dB for the default
// equal-power law). The balance mode ([StereoPanner.ProcessStereo]) rebalances
// an existing stereo pair and only ever attenuates: at the centre both channels
// pass through untouched, and moving off centre fades the opposite channel
// towards silence along the law's taper.
//
// An optional auto-pan LFO sweeps the position around the configured pan
// position. Every Process call advances the LFO by exactly one sample, so a
// caller must not invoke both a mono-pan and a balance method for the same
// sample.
//
// This processor is real-time safe and not thread-safe.
type StereoPanner struct {
	sampleRate   float64
	position     float64
	law          PanLaw
	autoPanRate  float64 // 0 means disabled
	autoPanDepth float64

	lfoPhase float64
	phaseInc float64

	// Cached gains for the static (auto-pan disabled) path.
	panL, panR         float64
	balanceL, balanceR float64
}

// NewStereoPanner creates a stereo panner with practical defaults and optional
// overrides.
func NewStereoPanner(sampleRate float64, opts ...StereoPannerOption) (*StereoPanner, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("stereo panner sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultStereoPannerConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	p := &StereoPanner{
		sampleRate:   sampleRate,
		position:     cfg.position,
		law:          cfg.law,
		autoPanRate:  cfg.autoPanRate,
		autoPanDepth: cfg.autoPanDepth,
	}

	p.rebuild()

	return p, nil
}

// panGains returns the raw law gains for a mono source panned to position.
func panGains(law PanLaw, position float64) (float64, float64) {
	// Map [-1, +1] onto [0, 1], where 0 is hard left and 1 is hard right.
	x := (position + 1) * 0.5

	switch law {
	case PanLawLinear:
		return 1 - x, x
	case PanLawCompromise:
		return math.Sqrt((1 - x) * math.Cos(x*math.Pi*0.5)),
			math.Sqrt(x * math.Sin(x*math.Pi*0.5))
	case PanLawEqualPower:
		fallthrough
	default:
		return math.Cos(x * math.Pi * 0.5), math.Sin(x * math.Pi * 0.5)
	}
}

// panTaper returns the far-channel fade used by the balance mode, normalised
// so that it is 1 at the centre and 0 when fully panned away. u is the
// magnitude of the pan position and is assumed to lie in [0, 1].
func panTaper(law PanLaw, u float64) float64 {
	switch law {
	case PanLawLinear:
		return 1 - u
	case PanLawCompromise:
		return math.Sqrt((1 - u) * math.Cos(u*math.Pi*0.5))
	case PanLawEqualPower:
		fallthrough
	default:
		return math.Cos(u * math.Pi * 0.5)
	}
}

// balanceGains returns the attenuate-only gains applied to an existing stereo
// pair: the near channel passes at unity and the far channel follows the law's
// taper.
func balanceGains(law PanLaw, position float64) (float64, float64) {
	if position >= 0 {
		return panTaper(law, position), 1
	}

	return 1, panTaper(law, -position)
}

// effectivePosition returns the pan position for the current LFO phase without
// advancing it.
func (p *StereoPanner) effectivePosition() float64 {
	if p.autoPanRate == 0 {
		return p.position
	}

	pos := p.position + p.autoPanDepth*math.Sin(p.lfoPhase)

	return math.Min(maxPanPosition, math.Max(minPanPosition, pos))
}

// advance moves the LFO on by one sample. It is only called when the auto-pan
// LFO is enabled.
func (p *StereoPanner) advance() {
	p.lfoPhase += p.phaseInc
	if p.lfoPhase >= 2*math.Pi {
		p.lfoPhase -= 2 * math.Pi
	}
}

// ProcessMono pans a single mono sample and returns the resulting left and
// right outputs.
func (p *StereoPanner) ProcessMono(sample float64) (float64, float64) {
	if p.autoPanRate == 0 {
		return sample * p.panL, sample * p.panR
	}

	gainL, gainR := panGains(p.law, p.effectivePosition())

	p.advance()

	return sample * gainL, sample * gainR
}

// ProcessMonoToStereo pans the mono input buffer into the supplied left and
// right buffers. All three buffers must have the same length.
func (p *StereoPanner) ProcessMonoToStereo(in, left, right []float64) error {
	if len(in) != len(left) || len(in) != len(right) {
		return fmt.Errorf("stereo panner: input and output buffers must have equal length: %d, %d, %d",
			len(in), len(left), len(right))
	}

	for i, v := range in {
		left[i], right[i] = p.ProcessMono(v)
	}

	return nil
}

// ProcessMonoToInterleaved pans the mono input buffer into an interleaved
// stereo output buffer (L, R, L, R, ...). The output length must be twice the
// input length.
func (p *StereoPanner) ProcessMonoToInterleaved(in, out []float64) error {
	if len(out) != 2*len(in) {
		return fmt.Errorf("stereo panner: interleaved output length must be twice the input length: %d != %d",
			len(out), 2*len(in))
	}

	for i, v := range in {
		out[2*i], out[2*i+1] = p.ProcessMono(v)
	}

	return nil
}

// ProcessStereo rebalances a single stereo sample pair and returns the
// resulting left and right outputs. Unlike [StereoPanner.ProcessMono] this
// never applies a gain above unity: a centred position passes the pair through
// unchanged.
func (p *StereoPanner) ProcessStereo(left, right float64) (float64, float64) {
	if p.autoPanRate == 0 {
		return left * p.balanceL, right * p.balanceR
	}

	gainL, gainR := balanceGains(p.law, p.effectivePosition())

	p.advance()

	return left * gainL, right * gainR
}

// ProcessStereoInPlace rebalances paired left/right buffers in place. Both
// buffers must have the same length.
func (p *StereoPanner) ProcessStereoInPlace(left, right []float64) error {
	if len(left) != len(right) {
		return fmt.Errorf("stereo panner: left and right buffers must have equal length: %d != %d",
			len(left), len(right))
	}

	for i := range left {
		left[i], right[i] = p.ProcessStereo(left[i], right[i])
	}

	return nil
}

// ProcessInterleavedInPlace rebalances an interleaved stereo buffer
// (L, R, L, R, ...) in place. The buffer length must be even.
func (p *StereoPanner) ProcessInterleavedInPlace(buf []float64) error {
	if len(buf)%2 != 0 {
		return fmt.Errorf("stereo panner: interleaved buffer length must be even: %d", len(buf))
	}

	for i := 0; i < len(buf); i += 2 {
		buf[i], buf[i+1] = p.ProcessStereo(buf[i], buf[i+1])
	}

	return nil
}

// Reset clears the auto-pan LFO phase.
func (p *StereoPanner) Reset() {
	p.lfoPhase = 0
}

// Gains returns the mono-pan gains currently applied to the left and right
// outputs. With auto-pan enabled these follow the LFO; reading them does not
// advance it.
func (p *StereoPanner) Gains() (float64, float64) {
	if p.autoPanRate == 0 {
		return p.panL, p.panR
	}

	return panGains(p.law, p.effectivePosition())
}

// BalanceGains returns the balance-mode gains currently applied to the left and
// right channels. With auto-pan enabled these follow the LFO; reading them does
// not advance it.
func (p *StereoPanner) BalanceGains() (float64, float64) {
	if p.autoPanRate == 0 {
		return p.balanceL, p.balanceR
	}

	return balanceGains(p.law, p.effectivePosition())
}

// SampleRate returns the sample rate in Hz.
func (p *StereoPanner) SampleRate() float64 { return p.sampleRate }

// Position returns the configured pan position in [-1, +1].
func (p *StereoPanner) Position() float64 { return p.position }

// Law returns the active pan law.
func (p *StereoPanner) Law() PanLaw { return p.law }

// AutoPanRate returns the auto-pan LFO rate in Hz, or 0 if disabled.
func (p *StereoPanner) AutoPanRate() float64 { return p.autoPanRate }

// AutoPanDepth returns the auto-pan LFO depth.
func (p *StereoPanner) AutoPanDepth() float64 { return p.autoPanDepth }

// SetSampleRate updates the sample rate and recomputes the LFO increment.
func (p *StereoPanner) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("stereo panner sample rate must be > 0 and finite: %f", sampleRate)
	}

	p.sampleRate = sampleRate

	p.rebuild()

	return nil
}

// SetPosition sets the pan position: -1 is hard left, 0 is centre and +1 is
// hard right.
func (p *StereoPanner) SetPosition(position float64) error {
	if err := validatePanPosition(position); err != nil {
		return err
	}

	p.position = position

	p.rebuild()

	return nil
}

// SetLaw selects the pan law.
func (p *StereoPanner) SetLaw(law PanLaw) error {
	if !law.valid() {
		return fmt.Errorf("stereo panner: invalid pan law: %d", law)
	}

	p.law = law

	p.rebuild()

	return nil
}

// SetAutoPanRate sets the auto-pan LFO rate in Hz. Set to 0 to disable.
func (p *StereoPanner) SetAutoPanRate(hz float64) error {
	if err := validateAutoPanRate(hz); err != nil {
		return err
	}

	p.autoPanRate = hz

	p.rebuild()

	return nil
}

// SetAutoPanDepth sets the auto-pan LFO depth.
func (p *StereoPanner) SetAutoPanDepth(depth float64) error {
	if err := validateAutoPanDepth(depth); err != nil {
		return err
	}

	p.autoPanDepth = depth

	p.rebuild()

	return nil
}

// rebuild refreshes the cached static gains and the LFO phase increment.
func (p *StereoPanner) rebuild() {
	p.panL, p.panR = panGains(p.law, p.position)
	p.balanceL, p.balanceR = balanceGains(p.law, p.position)
	p.phaseInc = 2 * math.Pi * p.autoPanRate / p.sampleRate
}
