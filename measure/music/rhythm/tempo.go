package rhythm

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
	ErrInvalidArgument = errors.New("rhythm: invalid argument")
	// ErrNilOption reports a nil [Option].
	ErrNilOption = errors.New("rhythm: nil option")
)

// Default tempo-estimation parameters.
const (
	// DefaultNoveltyRadius is the moving-mean half-width of [Novelty] in
	// frames used by the AudioVisualizer.
	DefaultNoveltyRadius = 30
	// DefaultMinBPM is the lower end of the broad tempo scan.
	DefaultMinBPM = 60.0
	// DefaultMaxBPM is the upper end of the broad tempo scan.
	DefaultMaxBPM = 180.0
	// DefaultScanStep is the BPM step of the broad tempo scan.
	DefaultScanStep = 0.5
	// DefaultMergeDistance is the BPM distance below which scan candidates
	// are merged into the better one.
	DefaultMergeDistance = 2.0
	// DefaultCandidates is the number of scan candidates reported.
	DefaultCandidates = 6
	// DefaultRefineHalfWidth is the half-width in BPM of the refinement.
	DefaultRefineHalfWidth = 3.0
	// DefaultRefineStep is the BPM step of the refinement.
	DefaultRefineStep = 0.01
)

// lagMultiples are the beat-period multiples correlated by TempoScore.
var lagMultiples = [...]float64{1, 2, 4, 8}

// Novelty returns the flux minus its mean over frames i-radius..i+radius
// (clipped at the edges), half-wave rectified. A negative radius is treated
// as 0.
func Novelty(flux []float64, radius int) []float64 {
	radius = max(0, radius)
	y := make([]float64, len(flux))

	for i, v := range flux {
		lo, hi := max(0, i-radius), min(len(flux), i+radius+1)

		mean := 0.0
		for j := lo; j < hi; j++ {
			mean += flux[j]
		}

		mean /= float64(hi - lo)
		y[i] = math.Max(0, v-mean)
	}

	return y
}

// TempoScore returns the normalized autocorrelation of the novelty curve at
// 1, 2, 4 and 8 beat periods of bpm, pooled over the four lags:
// Σ x[i]·x(i-lag) / sqrt(Σ x[i]² · Σ x(i-lag)²), with x(i-lag) linearly
// interpolated. It returns 0 for a non-positive bpm, an invalid timing or
// a silent curve.
func TempoScore(novelty []float64, timing features.Timing, bpm float64) float64 {
	if !(bpm > 0) || timing.Validate() != nil {
		return 0
	}

	x := novelty
	lag := 60 * timing.SampleRate / (bpm * float64(timing.Hop))
	s, aa, bb := 0.0, 0.0, 0.0

	for _, multiple := range lagMultiples {
		shift := lag * multiple
		for i := int(math.Ceil(shift)); i < len(x); i++ {
			position := float64(i) - shift
			j := int(position)
			fraction := position - float64(j)
			v := x[j]*(1-fraction) + x[min(j+1, len(x)-1)]*fraction
			s += x[i] * v
			aa += x[i] * x[i]
			bb += v * v
		}
	}

	if aa*bb <= 1e-15 {
		return 0
	}

	return s / math.Sqrt(aa*bb)
}

// Candidate is a tempo hypothesis and its [TempoScore].
type Candidate struct {
	// BPM is the tempo in beats per minute.
	BPM float64
	// Correlation is the TempoScore of BPM.
	Correlation float64
}

// Tempo is the result of [EstimateTempo].
type Tempo struct {
	// BPM is the refined tempo, or 0 when the novelty has no rhythmic
	// structure (refined score 0).
	BPM float64
	// Correlation is the TempoScore of BPM.
	Correlation float64
	// Candidates are the best broad-scan tempos, at least the merge
	// distance apart, in order of decreasing score.
	Candidates []Candidate
}

// Option configures [EstimateTempo].
type Option func(*config) error

type config struct {
	minBPM, maxBPM, scanStep float64
	mergeDistance            float64
	candidates               int
	prior                    float64
	hasPrior                 bool
	refineHalfWidth          float64
	refineStep               float64
}

func defaultConfig() config {
	return config{
		minBPM:          DefaultMinBPM,
		maxBPM:          DefaultMaxBPM,
		scanStep:        DefaultScanStep,
		mergeDistance:   DefaultMergeDistance,
		candidates:      DefaultCandidates,
		refineHalfWidth: DefaultRefineHalfWidth,
		refineStep:      DefaultRefineStep,
	}
}

func positive(v float64) bool { return v > 0 && !math.IsInf(v, 0) }

// WithRange sets the broad scan to minBPM..maxBPM in steps of step BPM
// (default 60..180 in 0.5 steps).
func WithRange(minBPM, maxBPM, step float64) Option {
	return func(c *config) error {
		if !positive(minBPM) || !positive(maxBPM) || maxBPM < minBPM || !positive(step) {
			return fmt.Errorf("%w: scan range %v..%v step %v", ErrInvalidArgument, minBPM, maxBPM, step)
		}

		c.minBPM, c.maxBPM, c.scanStep = minBPM, maxBPM, step

		return nil
	}
}

// WithCandidates sets how many broad-scan candidates are kept (default 6)
// and the BPM distance below which a weaker candidate is dropped in favour
// of a stronger one (default 2).
func WithCandidates(n int, mergeDistance float64) Option {
	return func(c *config) error {
		if n < 1 || !(mergeDistance >= 0) || math.IsInf(mergeDistance, 0) {
			return fmt.Errorf("%w: %d candidates, merge distance %v", ErrInvalidArgument, n, mergeDistance)
		}

		c.candidates, c.mergeDistance = n, mergeDistance

		return nil
	}
}

// WithPrior centres the refinement on an explicit tempo hypothesis instead
// of the best broad-scan candidate.
func WithPrior(bpm float64) Option {
	return func(c *config) error {
		if !positive(bpm) {
			return fmt.Errorf("%w: prior %v BPM", ErrInvalidArgument, bpm)
		}

		c.prior, c.hasPrior = bpm, true

		return nil
	}
}

// WithRefinement sets the refinement to centre ± halfWidth BPM in steps of
// step BPM (default ±3 in 0.01 steps). A halfWidth of 0 only scores the
// centre.
func WithRefinement(halfWidth, step float64) Option {
	return func(c *config) error {
		if !(halfWidth >= 0) || math.IsInf(halfWidth, 0) || !positive(step) {
			return fmt.Errorf("%w: refinement ±%v step %v", ErrInvalidArgument, halfWidth, step)
		}

		c.refineHalfWidth, c.refineStep = halfWidth, step

		return nil
	}
}

// EstimateTempo scans the novelty curve for the best tempo; see the package
// documentation. A curve without rhythmic structure (silence) yields a
// Tempo with BPM 0 and no error.
func EstimateTempo(novelty []float64, timing features.Timing, opts ...Option) (Tempo, error) {
	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return Tempo{}, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return Tempo{}, err
		}
	}

	err := timing.Validate()
	if err != nil {
		return Tempo{}, fmt.Errorf("rhythm: %w", err)
	}

	x := novelty

	all := []Candidate{}
	for bpm := cfg.minBPM; bpm <= cfg.maxBPM; bpm += cfg.scanStep {
		all = append(all, Candidate{bpm, TempoScore(x, timing, bpm)})
	}

	// sort.Slice (not a stable sort) is part of the reference behaviour:
	// equal scores keep the order it produces.
	sort.Slice(all, func(i, j int) bool { return all[i].Correlation > all[j].Correlation })

	candidates := []Candidate{}

	for _, c := range all {
		near := false

		for _, p := range candidates {
			if math.Abs(c.BPM-p.BPM) < cfg.mergeDistance {
				near = true
			}
		}

		if !near {
			candidates = append(candidates, c)
		}

		if len(candidates) == cfg.candidates {
			break
		}
	}

	centre := cfg.prior
	if !cfg.hasPrior {
		centre = candidates[0].BPM
	}

	best := Candidate{BPM: centre}
	for bpm := centre - cfg.refineHalfWidth; bpm <= centre+cfg.refineHalfWidth; bpm += cfg.refineStep {
		score := TempoScore(x, timing, bpm)
		if score > best.Correlation {
			best = Candidate{bpm, score}
		}
	}

	if best.Correlation == 0 {
		return Tempo{Candidates: candidates}, nil
	}

	return Tempo{BPM: best.BPM, Correlation: best.Correlation, Candidates: candidates}, nil
}
