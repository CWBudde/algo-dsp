package onset

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrInvalidArgument reports an invalid option value or input.
	ErrInvalidArgument = errors.New("onset: invalid argument")
	// ErrNilOption reports a nil option.
	ErrNilOption = errors.New("onset: nil option")
)

// Default detection parameters.
const (
	// DefaultRMSGate is the minimum frame RMS of a candidate.
	DefaultRMSGate = 0.001
	// DefaultThreshold is the minimum candidate flux relative to the 95th
	// percentile of the positive flux.
	DefaultThreshold = 0.12
	// DefaultLocalRadius is the half-width in frames of the local flux mean.
	DefaultLocalRadius = 25
	// DefaultLocalFactor is the minimum ratio of candidate flux to the local
	// flux mean.
	DefaultLocalFactor = 1.35
	// DefaultMinSpacing is the minimum distance between events in seconds.
	DefaultMinSpacing = 0.075
	// DefaultRefineWindow is the half-width in seconds of the attack search.
	DefaultRefineWindow = 0.05
	// DefaultAttackStep is the block length in seconds of the attack curve.
	DefaultAttackStep = 0.005

	referencePercentile = 0.95
)

// Kind is a drum label of an [Event].
type Kind int

const (
	// KindUnknown is an unlabelled onset.
	KindUnknown Kind = iota
	// KindKick is a low-band dominated hit.
	KindKick
	// KindSnare is a hit with body and noise (snare, clap).
	KindSnare
	// KindHat is a top-band dominated hit without body.
	KindHat
)

// String returns "unknown", "kick", "snare" or "hat".
func (k Kind) String() string {
	switch k {
	case KindUnknown:
		return "unknown"
	case KindKick:
		return "kick"
	case KindSnare:
		return "snare"
	case KindHat:
		return "hat"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// Event is a detected onset.
type Event struct {
	// Time is the onset time in seconds.
	Time float64
	// Strength is min(1, flux/p95) of the detecting frame, in (0, 1].
	Strength float64
	// Kind is the drum label set by [ClassifyDrums]; KindUnknown otherwise.
	Kind Kind
}

// Option configures [Detect].
type Option func(*config) error

type config struct {
	rmsGate      float64
	threshold    float64
	localRadius  int
	localFactor  float64
	minSpacing   float64
	refineWindow float64
	attackStep   float64
}

func defaultConfig() config {
	return config{
		rmsGate:      DefaultRMSGate,
		threshold:    DefaultThreshold,
		localRadius:  DefaultLocalRadius,
		localFactor:  DefaultLocalFactor,
		minSpacing:   DefaultMinSpacing,
		refineWindow: DefaultRefineWindow,
		attackStep:   DefaultAttackStep,
	}
}

func nonNegative(name string, v float64) error {
	if !(v >= 0) || math.IsInf(v, 0) {
		return fmt.Errorf("%w: %s %v", ErrInvalidArgument, name, v)
	}

	return nil
}

// WithRMSGate sets the minimum frame RMS of a candidate (default 0.001).
func WithRMSGate(rms float64) Option {
	return func(c *config) error {
		c.rmsGate = rms

		return nonNegative("RMS gate", rms)
	}
}

// WithThreshold sets the minimum flux of a candidate relative to the 95th
// percentile of the positive flux (default 0.12).
func WithThreshold(relative float64) Option {
	return func(c *config) error {
		c.threshold = relative

		return nonNegative("threshold", relative)
	}
}

// WithLocalMean sets the adaptive threshold: a candidate's flux must be at
// least factor times the mean flux over ±radius frames (defaults 25 and
// 1.35).
func WithLocalMean(radius int, factor float64) Option {
	return func(c *config) error {
		if radius < 0 {
			return fmt.Errorf("%w: local radius %d", ErrInvalidArgument, radius)
		}

		c.localRadius, c.localFactor = radius, factor

		return nonNegative("local factor", factor)
	}
}

// WithMinSpacing sets the minimum distance in seconds between two events
// (default 75 ms). Closer events are merged, keeping the stronger one.
func WithMinSpacing(seconds float64) Option {
	return func(c *config) error {
		c.minSpacing = seconds

		return nonNegative("min spacing", seconds)
	}
}

// WithRefinement sets the attack refinement: the event moves to the start
// of the attackStep-long block with the largest RMS rise within ±window
// seconds of the frame centre (defaults ±50 ms and 5 ms). A window of 0
// disables refinement and reports frame centre times.
func WithRefinement(window, attackStep float64) Option {
	return func(c *config) error {
		c.refineWindow, c.attackStep = window, attackStep

		err := nonNegative("refine window", window)
		if err != nil {
			return err
		}

		if !(attackStep > 0) || math.IsInf(attackStep, 0) {
			return fmt.Errorf("%w: attack step %v", ErrInvalidArgument, attackStep)
		}

		return nil
	}
}

func applyOptions(opts []Option) (config, error) {
	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return cfg, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

// Detect returns the onsets found in f, whose features were extracted from
// channels (the same audio, used for attack refinement). See the package
// documentation for the algorithm.
//
// It returns nil when the track has no positive flux at all, and a non-nil,
// possibly empty slice otherwise.
func Detect(channels [][]float64, f *features.Frames, opts ...Option) ([]Event, error) {
	cfg, err := applyOptions(opts)
	if err != nil {
		return nil, err
	}

	err = checkInput(channels, f)
	if err != nil {
		return nil, err
	}

	var (
		attack []float64
		step   int
	)

	if cfg.refineWindow > 0 {
		step = max(1, features.FloorSamples(cfg.attackStep*f.SampleRate))
		attack = attackCurve(channels, step)
	}

	flux := f.Flux

	active := make([]float64, 0, len(flux))
	for _, v := range flux {
		if v > 0 {
			active = append(active, v)
		}
	}

	hi := features.Percentile(active, referencePercentile)
	if hi <= 0 {
		return nil, nil
	}

	radius := 0
	if attack != nil {
		radius = int(math.Round(cfg.refineWindow * f.SampleRate / float64(step)))
	}

	candidates := []Event{}

	for i := 1; i < len(flux)-1; i++ {
		if f.RMS[i] < cfg.rmsGate || flux[i] < cfg.threshold*hi || flux[i] <= flux[i-1] || flux[i] < flux[i+1] {
			continue
		}

		lo, end := max(0, i-cfg.localRadius), min(len(flux), i+cfg.localRadius+1)

		avg := 0.0
		for j := lo; j < end; j++ {
			avg += flux[j]
		}

		avg /= float64(end - lo)
		if flux[i] < cfg.localFactor*avg {
			continue
		}

		position := float64(i) * float64(f.Hop) / f.SampleRate

		if attack != nil {
			centre := i * f.Hop / step
			best := 0.0

			for j := max(0, centre-radius); j < min(len(attack), centre+radius+1); j++ {
				if attack[j] > best {
					best = attack[j]
					position = float64(j*step) / f.SampleRate
				}
			}
		}

		candidates = append(candidates, Event{Time: position, Strength: math.Min(1, flux[i]/hi)})
	}

	return suppress(candidates, cfg.minSpacing), nil
}

func checkInput(channels [][]float64, f *features.Frames) error {
	if f == nil {
		return fmt.Errorf("%w: nil frames", ErrInvalidArgument)
	}

	err := f.Validate()
	if err != nil {
		return fmt.Errorf("onset: %w", err)
	}

	if len(f.RMS) != len(f.Flux) {
		return fmt.Errorf("%w: %d RMS values for %d flux values", ErrInvalidArgument, len(f.RMS), len(f.Flux))
	}

	if len(channels) == 0 || len(channels[0]) == 0 {
		return fmt.Errorf("%w: empty audio", ErrInvalidArgument)
	}

	for i, ch := range channels {
		if len(ch) != len(channels[0]) {
			return fmt.Errorf("%w: channel %d length %d != %d", ErrInvalidArgument, i, len(ch), len(channels[0]))
		}
	}

	return nil
}

// attackCurve returns the positive RMS rise of consecutive step-sample
// blocks, all channels pooled.
func attackCurve(channels [][]float64, step int) []float64 {
	n := len(channels[0])
	attack := make([]float64, (n+step-1)/step)
	prev := 0.0

	for i := range attack {
		e, count := 0.0, 0.0

		for j := i * step; j < min(n, (i+1)*step); j++ {
			for _, ch := range channels {
				e += ch[j] * ch[j]
				count++
			}
		}

		r := math.Sqrt(e / math.Max(1, count))
		attack[i] = math.Max(0, r-prev)
		prev = r
	}

	return attack
}

// suppress keeps the strongest of any events closer than spacing and
// returns the survivors in time order. The sort calls are part of the
// reference behaviour: ties in strength are resolved by sort.Slice.
func suppress(candidates []Event, spacing float64) []Event {
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Strength > candidates[j].Strength })

	events := []Event{}

	for _, e := range candidates {
		keep := true

		for _, existing := range events {
			if math.Abs(e.Time-existing.Time) < spacing {
				keep = false

				break
			}
		}

		if keep {
			events = append(events, e)
		}
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Time < events[j].Time })

	return events
}
