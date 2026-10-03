package resample

import (
	"errors"
	"math"
	"math/bits"
)

var (
	// ErrInvalidRatio indicates an invalid up/down ratio.
	ErrInvalidRatio = errors.New("resample: invalid ratio")
	// ErrInvalidRate indicates an invalid input/output sample rate.
	ErrInvalidRate = errors.New("resample: invalid sample rate")
	// ErrShortDst indicates that the destination cannot hold the next output block.
	ErrShortDst = errors.New("resample: destination too short")
	// ErrOutputTooLarge indicates an output block whose length cannot fit in int.
	ErrOutputTooLarge = errors.New("resample: output length exceeds int capacity")
)

// Quality controls default anti-aliasing filter settings.
type Quality int

const (
	// QualityFast prioritizes lower CPU usage.
	QualityFast Quality = iota
	// QualityBalanced is the default quality/performance trade-off.
	QualityBalanced
	// QualityBest prioritizes stopband attenuation and passband flatness.
	QualityBest
)

// Profile exposes default filter parameters for each quality mode.
type Profile struct {
	TapsPerPhase      int
	CutoffScale       float64
	KaiserBeta        float64
	NominalStopbandDB float64
}

// QualityProfile returns the default profile used by quality mode q.
func QualityProfile(q Quality) Profile {
	switch q {
	case QualityFast:
		return Profile{TapsPerPhase: 16, CutoffScale: 0.88, KaiserBeta: 5.0, NominalStopbandDB: 55}
	case QualityBest:
		return Profile{TapsPerPhase: 64, CutoffScale: 0.96, KaiserBeta: 9.0, NominalStopbandDB: 90}
	default:
		return Profile{TapsPerPhase: 32, CutoffScale: 0.92, KaiserBeta: 7.5, NominalStopbandDB: 75}
	}
}

type config struct {
	quality      Quality
	tapsPerPhase int
	cutoffScale  float64
	kaiserBeta   float64
	maxDen       int
}

// Option configures the resampler.
type Option func(*config)

// WithQuality selects a predefined anti-aliasing quality mode.
func WithQuality(q Quality) Option {
	return func(cfg *config) {
		cfg.quality = q
	}
}

// WithTapsPerPhase overrides taps per polyphase branch.
func WithTapsPerPhase(n int) Option {
	return func(cfg *config) {
		if n > 0 {
			cfg.tapsPerPhase = n
		}
	}
}

// WithCutoffScale overrides normalized cutoff scaling in range (0, 1].
// 1.0 equals the theoretical anti-aliasing cutoff.
func WithCutoffScale(v float64) Option {
	return func(cfg *config) {
		if v > 0 && v <= 1 {
			cfg.cutoffScale = v
		}
	}
}

// WithKaiserBeta overrides the Kaiser window beta parameter.
func WithKaiserBeta(beta float64) Option {
	return func(cfg *config) {
		if beta >= 0 {
			cfg.kaiserBeta = beta
		}
	}
}

// WithMaxDenominator caps denominator size for rate-ratio approximation.
func WithMaxDenominator(n int) Option {
	return func(cfg *config) {
		if n > 0 {
			cfg.maxDen = n
		}
	}
}

func defaultConfig() config {
	return config{
		quality: QualityBalanced,
		maxDen:  4096,
	}
}

func (c config) finalized() config {
	p := QualityProfile(c.quality)
	if c.tapsPerPhase <= 0 {
		c.tapsPerPhase = p.TapsPerPhase
	}

	if c.cutoffScale <= 0 || c.cutoffScale > 1 {
		c.cutoffScale = p.CutoffScale
	}

	if c.kaiserBeta < 0 {
		c.kaiserBeta = p.KaiserBeta
	}

	if c.kaiserBeta == 0 {
		c.kaiserBeta = p.KaiserBeta
	}

	if c.maxDen <= 0 {
		c.maxDen = 4096
	}

	return c
}

// Resampler performs rational sample-rate conversion using a polyphase FIR.
type Resampler struct {
	up                 int
	down               int
	advance, remainder int

	quality Quality
	profile Profile

	taps       []float64
	phases     [][]float64
	maxPhaseLn int

	phase      int
	inputIndex int
	history    []float64
}

// NewRational creates a resampler for ratio up/down.
func NewRational(up, down int, opts ...Option) (*Resampler, error) {
	if up <= 0 || down <= 0 {
		return nil, ErrInvalidRatio
	}

	g := gcd(up, down)
	up /= g
	down /= g

	cfg := defaultConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	cfg = cfg.finalized()

	taps, phases, maxPhaseLn, err := designPolyphaseFIR(up, down, cfg)
	if err != nil {
		return nil, err
	}

	return &Resampler{
		up:         up,
		down:       down,
		advance:    down / up,
		remainder:  down % up,
		quality:    cfg.quality,
		profile:    QualityProfile(cfg.quality),
		taps:       taps,
		phases:     phases,
		maxPhaseLn: maxPhaseLn,
		history:    make([]float64, 0, maxInt(0, maxPhaseLn-1)),
	}, nil
}

// NewForRates creates a resampler by approximating outRate/inRate as a ratio.
func NewForRates(inRate, outRate float64, opts ...Option) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 || math.IsNaN(inRate) || math.IsNaN(outRate) {
		return nil, ErrInvalidRate
	}

	cfg := defaultConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	cfg = cfg.finalized()

	up, down := approximateRatio(outRate/inRate, cfg.maxDen)

	return NewRational(up, down, opts...)
}

// Upsample2x is a convenience wrapper for 2:1 conversion.
func Upsample2x(input []float64, opts ...Option) ([]float64, error) {
	r, err := NewRational(2, 1, opts...)
	if err != nil {
		return nil, err
	}

	return r.Process(input), nil
}

// Downsample2x is a convenience wrapper for 1:2 conversion.
func Downsample2x(input []float64, opts ...Option) ([]float64, error) {
	r, err := NewRational(1, 2, opts...)
	if err != nil {
		return nil, err
	}

	return r.Process(input), nil
}

// Resample converts input using ratio up/down as a one-shot helper.
func Resample(input []float64, up, down int, opts ...Option) ([]float64, error) {
	r, err := NewRational(up, down, opts...)
	if err != nil {
		return nil, err
	}

	return r.Process(input), nil
}

// Reset clears internal filter state.
func (r *Resampler) Reset() {
	r.phase = 0
	r.inputIndex = 0
	r.history = r.history[:0]
}

// Clone creates a fresh, reset mono stream with the same ratio and filter.
// It shares immutable coefficients with r but allocates independent history.
// It does not copy r's streaming position or samples. Each channel can use a
// clone without redesigning the same FIR; distinct clones may run concurrently.
func (r *Resampler) Clone() *Resampler {
	return &Resampler{
		up: r.up, down: r.down, advance: r.advance, remainder: r.remainder, quality: r.quality, profile: r.profile,
		taps: r.taps, phases: r.phases, maxPhaseLn: r.maxPhaseLn,
		history: make([]float64, 0, r.maxPhaseLn-1),
	}
}

// Process converts an input block and preserves internal state for streaming.
func (r *Resampler) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}

	out := make([]float64, r.PredictOutputLen(len(input)))

	n, err := r.ProcessInto(out, input)
	if err != nil {
		panic(err)
	}

	return out[:n]
}

// ProcessInto converts an entire mono input block into dst, preserving filter
// state across calls. It writes exactly PredictOutputLen(len(input)) samples
// without allocating. Input and dst must not overlap. Empty input does nothing.
//
// ErrShortDst and ErrOutputTooLarge leave both dst and filter state unchanged.
// The FIR is causal: initial output includes its group delay. To flush a finite
// signal's tail, explicitly process zeros; no tail is added implicitly. Repeated
// input loops remain continuous when processed without Reset.
func (r *Resampler) ProcessInto(dst, input []float64) (int, error) {
	count, fits := r.outputLen(len(input))
	if !fits {
		return 0, ErrOutputTooLarge
	}

	if len(dst) < count {
		return 0, ErrShortDst
	}

	if len(input) == 0 {
		return 0, nil
	}

	if count == 0 {
		r.inputIndex -= len(input)
	} else {
		r.processInto(dst[:count], input)
	}

	r.retainHistory(input)

	return count, nil
}

func (r *Resampler) processInto(dst, input []float64) {
	phase, inputIndex := r.phase, r.inputIndex
	advance, remainder, up := r.advance, r.remainder, r.up
	phases, history := r.phases, r.history

	for outputIndex := range dst {
		var value float64

		coefficients := phases[phase]
		if inputIndex >= len(coefficients)-1 {
			// Most output windows lie entirely in the current block. Establish
			// one slice bound, then keep the original newest-to-oldest order.
			length := len(coefficients)
			window := input[inputIndex-length+1 : inputIndex+1]

			i := 0
			for ; i+3 < length; i += 4 {
				value += coefficients[i] * window[length-1-i]
				value += coefficients[i+1] * window[length-2-i]
				value += coefficients[i+2] * window[length-3-i]
				value += coefficients[i+3] * window[length-4-i]
			}

			for ; i < length; i++ {
				value += coefficients[i] * window[length-1-i]
			}
		} else {
			for tapIndex, coefficient := range coefficients {
				index := inputIndex - tapIndex

				var sample float64
				if index >= 0 {
					sample = input[index]
				} else if index >= -len(history) {
					sample = history[len(history)+index]
				} else {
					continue
				}

				value += coefficient * sample
			}
		}

		dst[outputIndex] = value

		step := advance

		if phase >= up-remainder {
			phase -= up - remainder
			step++
		} else {
			phase += remainder
		}

		if outputIndex == len(dst)-1 {
			// Rebase before adding, so neither the offset nor an absolute
			// stream counter can overflow on 32-bit/WASM targets.
			inputIndex = step - (len(input) - inputIndex)
		} else {
			inputIndex += step
		}
	}

	r.phase, r.inputIndex = phase, inputIndex
}

func (r *Resampler) retainHistory(input []float64) {
	keep := r.maxPhaseLn - 1
	if len(input) >= keep {
		r.history = r.history[:keep]
		copy(r.history, input[len(input)-keep:])

		return
	}

	retained := min(len(r.history), keep-len(input))
	copy(r.history[:retained], r.history[len(r.history)-retained:])
	r.history = r.history[:retained+len(input)]
	copy(r.history[retained:], input)
}

// PredictOutputLen returns the exact output length for the next Process or
// ProcessInto call without changing state. Nonpositive lengths return zero.
// If the output length cannot fit in int, it returns the maximum int value;
// ProcessInto then rejects the input with ErrOutputTooLarge.
func (r *Resampler) PredictOutputLen(inputLen int) int {
	count, _ := r.outputLen(inputLen)

	return count
}

func (r *Resampler) outputLen(inputLen int) (int, bool) {
	if inputLen <= r.inputIndex {
		return 0, true
	}
	// ceil(((inputLen-inputIndex)*up-phase)/down), using a wide product
	// so prediction is safe even for lengths near the platform's int limit.
	high, low := bits.Mul64(uint64(inputLen-r.inputIndex), uint64(r.up))
	low, borrow := bits.Sub64(low, uint64(r.phase), 0)
	high -= borrow
	maxLength := int(^uint(0) >> 1)

	limitHigh, limitLow := bits.Mul64(uint64(maxLength), uint64(r.down))
	if high > limitHigh || (high == limitHigh && low > limitLow) {
		return maxLength, false
	}

	quotient, remainder := bits.Div64(high, low, uint64(r.down))
	if remainder != 0 {
		quotient++
	}

	return int(quotient), true
}

// GroupDelayInput returns the linear-phase FIR delay in input sample frames.
// It can be fractional. Feeding zeros explicitly flushes the delayed tail.
func (r *Resampler) GroupDelayInput() float64 {
	return float64(len(r.taps)-1) / (2 * float64(r.up))
}

// GroupDelayOutput returns the linear-phase FIR delay in output sample frames.
// It can be fractional; a transport can discard its ceiling for integer-frame
// alignment, then trim the flushed output to the desired signal duration.
func (r *Resampler) GroupDelayOutput() float64 {
	return float64(len(r.taps)-1) / (2 * float64(r.down))
}

// Ratio returns reduced up/down conversion factors.
func (r *Resampler) Ratio() (up, down int) {
	return r.up, r.down
}

// Quality returns the configured quality mode.
func (r *Resampler) Quality() Quality {
	return r.quality
}

// TapsPerPhase returns taps in each polyphase branch for phase 0.
func (r *Resampler) TapsPerPhase() int {
	if len(r.phases) == 0 {
		return 0
	}

	return len(r.phases[0])
}

// Prototype returns a copy of the underlying prototype FIR taps.
func (r *Resampler) Prototype() []float64 {
	out := make([]float64, len(r.taps))
	copy(out, r.taps)

	return out
}
