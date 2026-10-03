package align

import (
	"fmt"
	"math"
)

// Defaults of [Lag] and [LagChannels]. They are the settings of
// AudioVisualizer's cmd/verifyrender at its 24 kHz analysis rate: a coarse
// search over ±480 samples in steps of 16, a ±16-sample fine search and
// correlation sums over every 16th sample.
const (
	// DefaultLagMaxLag is the default coarse search range in seconds (±20 ms).
	DefaultLagMaxLag = 0.02
	// DefaultLagCoarseStep is the default lag step of the coarse search in
	// samples.
	DefaultLagCoarseStep = 16
	// DefaultLagFineRadius is the default radius of the fine search around
	// the best coarse lag in samples.
	DefaultLagFineRadius = 16
	// DefaultLagStride is the default sample decimation of the correlation
	// sums.
	DefaultLagStride = 16
	// DefaultLagMinOverlap is the default share of the window's grid samples
	// that must have a partner in x for a lag to be a candidate. The default
	// 0 only requires one pair, as verifyrender does.
	DefaultLagMinOverlap = 0.0
)

// LagResult is the outcome of [Lag] and [LagChannels].
type LagResult struct {
	// Samples is the best integer lag in samples. Positive means x lags
	// behind the reference: x[i+Samples] lines up with ref[i].
	Samples int
	// Seconds is Samples divided by the sample rate.
	Seconds float64
	// RefinedSamples is a sub-sample estimate of the lag: the vertex of the
	// parabola through the correlations at Samples-1, Samples and Samples+1.
	// It equals Samples when the three points do not form a peak.
	RefinedSamples float64
	// Correlation is the normalized correlation Σab/sqrt(Σa²·Σb²) of
	// reference and x at lag Samples.
	Correlation float64
	// GainDB is the level of x relative to the reference at lag Samples,
	// 10·log10(Σb²/Σa²) in dB. Positive means x is louder.
	GainDB float64
}

// LagOption configures [Lag] and [LagChannels].
type LagOption func(*lagConfig) error

type lagConfig struct {
	maxLag     float64
	coarseStep int
	fineRadius int
	stride     int
	minOverlap float64
	start, end float64
	window     bool
}

// WithLagMaxLag sets the coarse search range to ±seconds (default 20 ms),
// rounded to whole samples. The fine search may extend up to the fine
// radius beyond it. Lags as long as the longer signal or longer are never
// evaluated: no samples overlap there, so they could not win anyway.
func WithLagMaxLag(seconds float64) LagOption {
	return func(c *lagConfig) error {
		if !(seconds >= 0) || math.IsInf(seconds, 0) {
			return fmt.Errorf("%w: max lag %v", ErrInvalidArgument, seconds)
		}

		c.maxLag = seconds

		return nil
	}
}

// WithLagCoarseStep sets the lag step of the coarse search in samples
// (default 16; 1 makes the coarse search exhaustive).
func WithLagCoarseStep(samples int) LagOption {
	return func(c *lagConfig) error {
		if samples < 1 {
			return fmt.Errorf("%w: coarse step %d", ErrInvalidArgument, samples)
		}

		c.coarseStep = samples

		return nil
	}
}

// WithLagFineRadius sets the radius in samples of the exhaustive fine
// search around the best coarse lag (default 16; 0 disables it).
func WithLagFineRadius(samples int) LagOption {
	return func(c *lagConfig) error {
		if samples < 0 {
			return fmt.Errorf("%w: fine radius %d", ErrInvalidArgument, samples)
		}

		c.fineRadius = samples

		return nil
	}
}

// WithLagStride evaluates the correlation sums on every n-th reference
// sample (default 16; 1 uses every sample).
func WithLagStride(n int) LagOption {
	return func(c *lagConfig) error {
		if n < 1 {
			return fmt.Errorf("%w: stride %d", ErrInvalidArgument, n)
		}

		c.stride = n

		return nil
	}
}

// WithLagMinOverlap sets the share of the window's grid samples (0 <= share
// <= 1, default 0) that must pair with samples of x for a lag to be
// considered. Lags near the ends of the signals keep only a few pairs, whose
// correlation can approach 1 by chance. With the default 20 ms range and
// windows much longer than that this cannot happen; when the lag range
// approaches the window length, set a share such as 0.5 so that those edge
// lags cannot win.
func WithLagMinOverlap(share float64) LagOption {
	return func(c *lagConfig) error {
		if !(share >= 0) || share > 1 {
			return fmt.Errorf("%w: minimum overlap %v", ErrInvalidArgument, share)
		}

		c.minOverlap = share

		return nil
	}
}

// WithLagWindow restricts the reference samples to the window [start, end)
// in seconds, converted to sample indices by truncation and clamped to the
// reference length. Samples of x outside the window are still used when the
// lag reaches them. By default the whole reference is used.
func WithLagWindow(start, end float64) LagOption {
	return func(c *lagConfig) error {
		if !(start >= 0) || math.IsInf(start, 0) || !(end > start) || math.IsInf(end, 0) {
			return fmt.Errorf("%w: window [%v, %v)", ErrInvalidArgument, start, end)
		}

		c.start, c.end, c.window = start, end, true

		return nil
	}
}

// Lag estimates the lag, correlation and gain of the mono signal x against
// the mono reference ref, both sampled at sampleRate. It is [LagChannels]
// with one channel.
func Lag(ref, x []float64, sampleRate float64, opts ...LagOption) (LagResult, error) {
	r := [1][]float64{ref}
	s := [1][]float64{x}

	return LagChannels(r[:], s[:], sampleRate, opts...)
}

// LagChannels estimates the lag of x against the reference ref (one slice
// per channel; x needs the same number of channels, and the channels of
// each signal must have equal non-zero lengths; x may be longer or shorter
// than ref).
//
// For a lag L, the correlation sums run over the reference samples i of the
// window on a stride grid (i = start, start+stride, ...), skip samples whose
// partner i+L lies outside x, and pool all channels:
//
//	Σab = Σ_i Σ_ch ref[ch][i]·x[ch][i+L],  Σa² and Σb² likewise.
//
// The search evaluates the normalized correlation for L = -maxLag,
// -maxLag+coarseStep, ..., up to +maxLag, then for every lag within the
// fine radius of the best coarse lag, keeping the first strict maximum.
// The result reports the best lag with its correlation and gain.
//
// This is the alignment check of AudioVisualizer's cmd/verifyrender, and
// with the default options it reproduces that code bit for bit. LagChannels
// does not allocate without options and allocates one small configuration
// copy with options. Non-finite samples propagate into the result. It
// returns an error wrapping [ErrInvalidArgument] when the inputs or options
// are invalid, the window is empty, or either signal has no energy at the
// best lag.
func LagChannels(ref, x [][]float64, sampleRate float64, opts ...LagOption) (LagResult, error) {
	cfg := lagConfig{
		maxLag:     DefaultLagMaxLag,
		coarseStep: DefaultLagCoarseStep,
		fineRadius: DefaultLagFineRadius,
		stride:     DefaultLagStride,
		minOverlap: DefaultLagMinOverlap,
	}

	if len(opts) > 0 {
		var err error

		cfg, err = applyLagOptions(cfg, opts)
		if err != nil {
			return LagResult{}, err
		}
	}

	err := validateLag(ref, x, sampleRate)
	if err != nil {
		return LagResult{}, err
	}

	lo, hi := 0, len(ref[0])
	if cfg.window {
		lo, hi = truncClamp(cfg.start*sampleRate, len(ref[0])), truncClamp(cfg.end*sampleRate, len(ref[0]))
	}

	if lo >= hi {
		return LagResult{}, fmt.Errorf("%w: empty window [%v, %v) s", ErrInvalidArgument, cfg.start, cfg.end)
	}

	s := lagSearch{ref: ref, x: x, lo: lo, hi: hi, stride: cfg.stride}

	// A candidate lag must pair at least minOverlap of the window's grid
	// samples with samples of x. Edge lags that keep only a handful of pairs
	// can reach a correlation near 1 by chance and must not win.
	gridSamples := (hi - lo + cfg.stride - 1) / cfg.stride
	s.minPairs = max(1, int(math.Ceil(cfg.minOverlap*float64(gridSamples))))

	// No reference sample has a partner in x at a lag of limit or more in
	// either direction, so those lags have NaN correlations and never win.
	// Skipping them keeps the result and bounds the search.
	limit := max(len(ref[0]), len(x[0]))
	first, last := coarseRange(math.Round(cfg.maxLag*sampleRate), cfg.coarseStep, limit)

	best, lag := -1.0, 0

	for offset := first; offset <= last; offset += cfg.coarseStep {
		c, _ := s.stats(offset)
		if c > best {
			best, lag = c, offset
		}

		if cfg.coarseStep > last-offset {
			break
		}
	}

	coarse := lag
	radius := min(cfg.fineRadius, 2*limit)

	for offset := max(coarse-radius, -limit); offset <= min(coarse+radius, limit); offset++ {
		c, _ := s.stats(offset)
		if c > best {
			best, lag = c, offset
		}
	}

	ab, aa, bb, _ := s.sums(lag)
	if !(aa > 0) || !(bb > 0) {
		return LagResult{}, fmt.Errorf("%w: no signal energy at lag %d", ErrInvalidArgument, lag)
	}

	c, gain := ab/math.Sqrt(aa*bb), 10*math.Log10(bb/aa)

	return LagResult{
		Samples:        lag,
		Seconds:        float64(lag) / sampleRate,
		RefinedSamples: float64(lag) + s.vertex(lag, c),
		Correlation:    c,
		GainDB:         gain,
	}, nil
}

// coarseRange returns the first and last lag of the coarse grid -maxLag,
// -maxLag+step, ... (up to +maxLag) that lie within ±limit. maxLag is a
// whole, non-negative number of samples. The grid keeps its alignment to
// -maxLag; maxLag is clamped in float, so huge values cannot overflow. The
// range is empty (first > last) when no grid lag lies within ±limit.
func coarseRange(maxLag float64, step, limit int) (int, int) {
	if maxLag <= float64(limit) {
		m := int(maxLag)

		return -m, m
	}

	// Advance from -maxLag to the first grid lag >= -limit.
	// The remainder is NaN for an infinite maxLag, which has no alignment.
	first := -limit
	if rem := math.Mod(maxLag-float64(limit), float64(step)); rem > 0 {
		first += step - int(rem)
	}

	return first, limit
}

// truncClamp returns x truncated to an integer and limited to [0, n],
// clamping before the integer conversion so that huge values cannot
// overflow.
func truncClamp(x float64, n int) int {
	switch {
	case x <= 0:
		return 0
	case x >= float64(n):
		return n
	default:
		return int(x)
	}
}

// lagSearch holds the inputs of one lag search.
type lagSearch struct {
	ref, x         [][]float64
	lo, hi, stride int
	minPairs       int
}

// sums returns Σab, Σa² and Σb² at lag over the stride grid of [lo, hi),
// skipping reference samples whose partner lies outside x, and the number of
// grid samples that had a partner. The iteration order (samples, then
// channels) is the one of verifyrender's stats.
func (s *lagSearch) sums(lag int) (float64, float64, float64, int) {
	var ab, aa, bb float64

	pairs := 0

	first, last := s.lo, min(s.hi, len(s.x[0])-lag)
	if first+lag < 0 {
		// Advance to the first grid sample whose partner is inside x.
		first += (-lag - first + s.stride - 1) / s.stride * s.stride
	}

	for i := first; i < last; i += s.stride {
		pairs++

		j := i + lag
		for ch := range s.ref {
			x, y := s.ref[ch][i], s.x[ch][j]
			ab += x * y
			aa += x * x
			bb += y * y
		}
	}

	return ab, aa, bb, pairs
}

// stats returns the normalized correlation and the gain in dB at lag, NaN
// when a sum is zero or fewer than minPairs grid samples overlap (NaN never
// wins a search).
func (s *lagSearch) stats(lag int) (float64, float64) {
	ab, aa, bb, pairs := s.sums(lag)
	if pairs < s.minPairs {
		return math.NaN(), math.NaN()
	}

	return ab / math.Sqrt(aa*bb), 10 * math.Log10(bb/aa)
}

// vertex returns the offset of the parabola vertex through the correlations
// at lag-1, lag and lag+1 (with c at lag), or 0 when they do not form a peak.
func (s *lagSearch) vertex(lag int, c float64) float64 {
	y0, _ := s.stats(lag - 1)
	y2, _ := s.stats(lag + 1)

	den := y0 - 2*c + y2
	if !(c >= y0) || !(c >= y2) || !(den < 0) {
		return 0
	}

	return 0.5 * (y0 - y2) / den
}

// applyLagOptions applies opts to a copy of cfg. It is separate from
// LagChannels so that only calls with options pay for the heap copy of the
// configuration that passing it to an option function requires.
func applyLagOptions(cfg lagConfig, opts []LagOption) (lagConfig, error) {
	for i, opt := range opts {
		if opt == nil {
			return lagConfig{}, fmt.Errorf("%w: lag option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return lagConfig{}, err
		}
	}

	return cfg, nil
}

func validateLag(ref, x [][]float64, sampleRate float64) error {
	if !(sampleRate > 0) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("%w: sample rate %v", ErrInvalidArgument, sampleRate)
	}

	if len(ref) == 0 || len(ref[0]) == 0 {
		return fmt.Errorf("%w: empty reference", ErrInvalidArgument)
	}

	if len(x) != len(ref) {
		return fmt.Errorf("%w: x has %d channels, reference %d", ErrInvalidArgument, len(x), len(ref))
	}

	if len(x[0]) == 0 {
		return fmt.Errorf("%w: empty signal", ErrInvalidArgument)
	}

	for ch := range ref {
		if len(ref[ch]) != len(ref[0]) {
			return fmt.Errorf("%w: reference channel %d has %d samples, want %d", ErrInvalidArgument, ch, len(ref[ch]), len(ref[0]))
		}

		if len(x[ch]) != len(x[0]) {
			return fmt.Errorf("%w: channel %d of x has %d samples, want %d", ErrInvalidArgument, ch, len(x[ch]), len(x[0]))
		}
	}

	return nil
}
