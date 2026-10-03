package motif

import (
	"fmt"
	"math"
	"slices"
)

// Default parameters. They are AudioVisualizer's DefaultMotifParams, so the
// functions of this package reproduce its story analysis bit for bit.
const (
	// DefaultGridMinNotes is the default fewest notes a grid window needs to
	// become a candidate.
	DefaultGridMinNotes = 4
	// DefaultGridMinSimilarity is the default lowest Dice similarity (after
	// the transposition penalty) of two matching grid windows.
	DefaultGridMinSimilarity = 0.5
	// DefaultGridTranspositionPenalty is the default amount subtracted from
	// the similarity of two grid windows that match only under a
	// transposition other than a whole number of octaves.
	DefaultGridTranspositionPenalty = 0.1
	// DefaultMaxSpanSlots is the default longest n-gram in grid slots, from
	// the first note's start to the last note's end.
	DefaultMaxSpanSlots = 32
	// DefaultMaxGapSlots is the default longest rest in slots between two
	// consecutive notes of an n-gram.
	DefaultMaxGapSlots = 4
	// DefaultMinSimilarity is the default lowest similarity of two matching
	// n-grams.
	DefaultMinSimilarity = 0.8
	// DefaultMinOccurrences is the default fewest occurrences of a motif.
	DefaultMinOccurrences = 3
	// DefaultIntervalWeight is the default weight of the interval distance
	// in the n-gram distance.
	DefaultIntervalWeight = 0.7
	// DefaultIOIWeight is the default weight of the inter-onset-ratio
	// distance in the n-gram distance.
	DefaultIOIWeight = 0.3
	// DefaultShiftOverlap is the default share of its units a candidate must
	// share with an accepted occurrence to be withdrawn (rotation and shift
	// suppression).
	DefaultShiftOverlap = 0.5
	// DefaultCoveredShare is the default share of a candidate's notes (or
	// beats) already covered by earlier motifs at which it is skipped.
	DefaultCoveredShare = 0.75
	// DefaultChromaMinSimilarity is the default lowest mean per-beat cosine
	// of two matching chroma windows under the best transposition.
	DefaultChromaMinSimilarity = 0.85
	// DefaultConfirmShare is the default share of a note motif's occurrences
	// that must coincide with one chroma motif for [Corroborate].
	DefaultConfirmShare = 0.6
	// DefaultOstinatoShare is the default share of occurrences that must lie
	// within a bar of another occurrence for the ostinato role.
	DefaultOstinatoShare = 0.6
	// DefaultLeitmotifs is the default most leitmotifs [Rank] picks.
	DefaultLeitmotifs = 4
	// DefaultLeitmotifOverlap is the default share of its occurrence time a
	// leitmotif may share with leitmotifs picked before it, exclusive.
	DefaultLeitmotifOverlap = 0.5
	// DefaultLeitmotifMinRatio is the default lowest salience of a
	// leitmotif relative to the best note motif.
	DefaultLeitmotifMinRatio = 0.5
	// DefaultLeitmotifsPerSource is the default most leitmotifs per source.
	DefaultLeitmotifsPerSource = 2

	// MaxSize is the largest value the size options accept:
	// [WithGridWindows], [WithLengths], [WithMaxSpanSlots] and
	// [WithChromaWindows]. It bounds the buffers and loops the sizes feed.
	MaxSize = 1 << 16

	// maxUnitTable bounds the inter-onset values tabulated for the n-gram
	// distance; larger values are computed directly.
	maxUnitTable = 256
)

// DefaultGridWindows returns the default grid window widths in slots: a bar
// and half a bar of sixteenths in 4/4.
func DefaultGridWindows() []int { return []int{16, 8} }

// DefaultLengths returns the default n-gram lengths in notes, longest first.
func DefaultLengths() []int { return []int{8, 6, 4} }

// DefaultChromaWindows returns the default chroma window widths in beats,
// longest first.
func DefaultChromaWindows() []int { return []int{8, 4} }

// Option configures the functions of this package. All functions share the
// option type and read only the options that concern them; each function's
// documentation lists them. Options return an error wrapping
// [ErrInvalidOption] for out-of-range values; a nil option gives an error
// wrapping [ErrNilOption] and [ErrInvalidOption].
//
// The options map one to one onto the fields of AudioVisualizer's
// MotifParams, in order: [WithGridWindows], [WithGridMinNotes],
// [WithGridMinSimilarity], [WithGridTranspositionPenalty], [WithLengths],
// [WithMaxSpanSlots], [WithMaxGapSlots], [WithMinSimilarity],
// [WithMinOccurrences], [WithDistanceWeights] (two fields),
// [WithShiftOverlap], [WithCoveredShare], [WithChromaWindows],
// [WithChromaMinSimilarity], [WithConfirmShare], [WithOstinatoShare],
// [WithLeitmotifs], [WithLeitmotifOverlap], [WithLeitmotifMinRatio] and
// [WithLeitmotifsPerSource].
type Option func(*config) error

type config struct {
	gridWindows         []int
	gridMinNotes        int
	gridMinSimilarity   float64
	gridTransposeCost   float64
	lengths             []int
	maxSpanSlots        int
	maxGapSlots         int
	minSimilarity       float64
	minOccurrences      int
	intervalWeight      float64
	ioiWeight           float64
	shiftOverlap        float64
	coveredShare        float64
	chromaWindows       []int
	chromaMinSimilarity float64
	confirmShare        float64
	ostinatoShare       float64
	leitmotifs          int
	leitmotifOverlap    float64
	leitmotifMinRatio   float64
	leitmotifsPerSource int
}

func defaultConfig() config {
	return config{
		gridWindows:         DefaultGridWindows(),
		gridMinNotes:        DefaultGridMinNotes,
		gridMinSimilarity:   DefaultGridMinSimilarity,
		gridTransposeCost:   DefaultGridTranspositionPenalty,
		lengths:             DefaultLengths(),
		maxSpanSlots:        DefaultMaxSpanSlots,
		maxGapSlots:         DefaultMaxGapSlots,
		minSimilarity:       DefaultMinSimilarity,
		minOccurrences:      DefaultMinOccurrences,
		intervalWeight:      DefaultIntervalWeight,
		ioiWeight:           DefaultIOIWeight,
		shiftOverlap:        DefaultShiftOverlap,
		coveredShare:        DefaultCoveredShare,
		chromaWindows:       DefaultChromaWindows(),
		chromaMinSimilarity: DefaultChromaMinSimilarity,
		confirmShare:        DefaultConfirmShare,
		ostinatoShare:       DefaultOstinatoShare,
		leitmotifs:          DefaultLeitmotifs,
		leitmotifOverlap:    DefaultLeitmotifOverlap,
		leitmotifMinRatio:   DefaultLeitmotifMinRatio,
		leitmotifsPerSource: DefaultLeitmotifsPerSource,
	}
}

func newConfig(opts []Option) (config, error) {
	cfg := defaultConfig()

	for i, opt := range opts {
		if opt == nil {
			return config{}, fmt.Errorf("%w: %w: option %d is nil", ErrNilOption, ErrInvalidOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return config{}, err
		}
	}

	return cfg, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func unitInterval(v float64) bool { return finite(v) && v >= 0 && v <= 1 }

func shareOption(name string, v float64, set func(*config)) Option {
	return func(cfg *config) error {
		if !unitInterval(v) {
			return fmt.Errorf("%w: %s must be in [0, 1], got %g", ErrInvalidOption, name, v)
		}

		set(cfg)

		return nil
	}
}

// checkSize requires minimum <= n <= [MaxSize].
func checkSize(name string, minimum, n int) error {
	if n < minimum || n > MaxSize {
		return fmt.Errorf("%w: %s must be in [%d, %d], got %d", ErrInvalidOption, name, minimum, MaxSize, n)
	}

	return nil
}

func sizesOption(name string, minimum int, sizes []int, set func(*config, []int)) Option {
	sizes = slices.Clone(sizes)

	return func(cfg *config) error {
		for _, n := range sizes {
			err := checkSize(name, minimum, n)
			if err != nil {
				return err
			}
		}

		set(cfg, sizes)

		return nil
	}
}

// WithGridWindows sets the widths in slots of the grid windows that
// [FindNoteMotifs] compares by position and pitch class, each from 2 to
// [MaxSize], in the order they are searched (default [DefaultGridWindows]: 16
// and 8). Windows start every half width from the start of bar 0. No widths
// disable the grid pass.
func WithGridWindows(slots ...int) Option {
	return sizesOption("grid window", 2, slots, func(cfg *config, v []int) { cfg.gridWindows = v })
}

// WithGridMinNotes sets the fewest notes a grid window needs, >= 1 (default
// [DefaultGridMinNotes]).
func WithGridMinNotes(n int) Option {
	return func(cfg *config) error {
		if n < 1 {
			return fmt.Errorf("%w: grid minimum notes must be >= 1, got %d", ErrInvalidOption, n)
		}

		cfg.gridMinNotes = n

		return nil
	}
}

// WithGridMinSimilarity sets the lowest similarity in [0, 1] of two matching
// grid windows (default [DefaultGridMinSimilarity]).
func WithGridMinSimilarity(v float64) Option {
	return shareOption("grid minimum similarity", v, func(cfg *config) { cfg.gridMinSimilarity = v })
}

// WithGridTranspositionPenalty sets the penalty in [0, 1] subtracted from the
// similarity of two grid windows that match only under a transposition
// other than a whole number of octaves (default
// [DefaultGridTranspositionPenalty]).
func WithGridTranspositionPenalty(v float64) Option {
	return shareOption("grid transposition penalty", v, func(cfg *config) { cfg.gridTransposeCost = v })
}

// WithLengths sets the n-gram lengths in notes, each from 2 to [MaxSize], in
// the order they are searched (default [DefaultLengths]: 8, 6 and 4). No
// lengths disable the n-gram pass.
func WithLengths(notes ...int) Option {
	return sizesOption("n-gram length", 2, notes, func(cfg *config, v []int) { cfg.lengths = v })
}

// WithMaxSpanSlots sets the longest n-gram in slots, from 1 to [MaxSize],
// from the first note's start to the last note's end (default
// [DefaultMaxSpanSlots]).
func WithMaxSpanSlots(n int) Option {
	return func(cfg *config) error {
		err := checkSize("maximum span in slots", 1, n)
		if err != nil {
			return err
		}

		cfg.maxSpanSlots = n

		return nil
	}
}

// WithMaxGapSlots sets the longest rest in slots, >= 0, between the end of a
// note and the start of the next one inside an n-gram (default
// [DefaultMaxGapSlots]).
func WithMaxGapSlots(n int) Option {
	return func(cfg *config) error {
		if n < 0 {
			return fmt.Errorf("%w: maximum gap must be >= 0 slots, got %d", ErrInvalidOption, n)
		}

		cfg.maxGapSlots = n

		return nil
	}
}

// WithMinSimilarity sets the lowest similarity in [0, 1] of two matching
// n-grams (default [DefaultMinSimilarity]).
func WithMinSimilarity(v float64) Option {
	return shareOption("minimum similarity", v, func(cfg *config) { cfg.minSimilarity = v })
}

// WithMinOccurrences sets the fewest occurrences of a motif, >= 2 (default
// [DefaultMinOccurrences]). It applies to note and chroma motifs.
func WithMinOccurrences(n int) Option {
	return func(cfg *config) error {
		if n < 2 {
			return fmt.Errorf("%w: minimum occurrences must be >= 2, got %d", ErrInvalidOption, n)
		}

		cfg.minOccurrences = n

		return nil
	}
}

// WithDistanceWeights sets the weights, each finite and >= 0, of the
// interval and inter-onset-ratio terms of the n-gram distance (defaults
// [DefaultIntervalWeight] and [DefaultIOIWeight]). With weights summing to 1
// the similarity stays in [0, 1].
func WithDistanceWeights(interval, ioi float64) Option {
	return func(cfg *config) error {
		if !finite(interval) || interval < 0 || !finite(ioi) || ioi < 0 {
			return fmt.Errorf("%w: distance weights must be finite and >= 0, got %g and %g", ErrInvalidOption, interval, ioi)
		}

		cfg.intervalWeight, cfg.ioiWeight = interval, ioi

		return nil
	}
}

// WithShiftOverlap sets the share in [0, 1] of its units (notes, slots or
// beats) a candidate must share with an accepted occurrence to be withdrawn
// from later clusters (default [DefaultShiftOverlap]). This keeps the
// rotations and shifts of a repeating figure from forming motifs of their
// own.
func WithShiftOverlap(v float64) Option {
	return shareOption("shift overlap", v, func(cfg *config) { cfg.shiftOverlap = v })
}

// WithCoveredShare sets the share in [0, 1] of a candidate's notes (or
// beats) covered by earlier, longer motifs at which the candidate is skipped
// (default [DefaultCoveredShare]).
func WithCoveredShare(v float64) Option {
	return shareOption("covered share", v, func(cfg *config) { cfg.coveredShare = v })
}

// WithChromaWindows sets the widths in beats of the chroma windows of
// [FindChromaMotifs], each from 1 to [MaxSize], in the order they are
// searched (default [DefaultChromaWindows]: 8 and 4). No widths disable the
// chroma pass.
func WithChromaWindows(beats ...int) Option {
	return sizesOption("chroma window", 1, beats, func(cfg *config, v []int) { cfg.chromaWindows = v })
}

// WithChromaMinSimilarity sets the lowest mean per-beat cosine in [0, 1] of
// two matching chroma windows (default [DefaultChromaMinSimilarity]).
func WithChromaMinSimilarity(v float64) Option {
	return shareOption("chroma minimum similarity", v, func(cfg *config) { cfg.chromaMinSimilarity = v })
}

// WithConfirmShare sets the share in [0, 1] of a note motif's occurrences
// that must coincide with occurrences of one chroma motif for [Corroborate]
// (default [DefaultConfirmShare]).
func WithConfirmShare(v float64) Option {
	return shareOption("confirm share", v, func(cfg *config) { cfg.confirmShare = v })
}

// WithOstinatoShare sets the share in [0, 1] of occurrences that must lie
// within a bar of another occurrence, exclusive, for [Score] to give a motif
// the [RoleOstinato] role (default [DefaultOstinatoShare]).
func WithOstinatoShare(v float64) Option {
	return shareOption("ostinato share", v, func(cfg *config) { cfg.ostinatoShare = v })
}

// WithLeitmotifs sets the most leitmotifs [Rank] picks, >= 0 (default
// [DefaultLeitmotifs]).
func WithLeitmotifs(n int) Option {
	return func(cfg *config) error {
		if n < 0 {
			return fmt.Errorf("%w: leitmotifs must be >= 0, got %d", ErrInvalidOption, n)
		}

		cfg.leitmotifs = n

		return nil
	}
}

// WithLeitmotifOverlap sets the share, finite and >= 0, of its occurrence
// time a leitmotif may share with the leitmotifs picked before it,
// exclusive (default [DefaultLeitmotifOverlap]). With 0 only motifs that
// share no occurrence time with an earlier pick qualify.
func WithLeitmotifOverlap(v float64) Option {
	return func(cfg *config) error {
		if !finite(v) || v < 0 {
			return fmt.Errorf("%w: leitmotif overlap must be finite and >= 0, got %g", ErrInvalidOption, v)
		}

		cfg.leitmotifOverlap = v

		return nil
	}
}

// WithLeitmotifMinRatio sets the lowest salience of a leitmotif relative to
// the most salient note motif, finite and >= 0 (default
// [DefaultLeitmotifMinRatio]).
func WithLeitmotifMinRatio(v float64) Option {
	return func(cfg *config) error {
		if !finite(v) || v < 0 {
			return fmt.Errorf("%w: leitmotif salience ratio must be finite and >= 0, got %g", ErrInvalidOption, v)
		}

		cfg.leitmotifMinRatio = v

		return nil
	}
}

// WithLeitmotifsPerSource sets the most leitmotifs [Rank] picks from one
// source, >= 0 (default [DefaultLeitmotifsPerSource]).
func WithLeitmotifsPerSource(n int) Option {
	return func(cfg *config) error {
		if n < 0 {
			return fmt.Errorf("%w: leitmotifs per source must be >= 0, got %d", ErrInvalidOption, n)
		}

		cfg.leitmotifsPerSource = n

		return nil
	}
}
