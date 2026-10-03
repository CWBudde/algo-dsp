package structure

import (
	"fmt"
	"math"
)

// Boundary is a structural boundary found by [Peaks]: a novelty peak just
// before row Index.
type Boundary struct {
	// Index is the row (novelty index) the boundary precedes.
	Index int
	// Novelty is the novelty value at Index.
	Novelty float64
	// Position is Index mapped by [WithPositions] (for example the beat
	// start time in seconds); float64(Index) without it.
	Position float64
	// Mark is the index of the reference mark nearest to Position (see
	// [WithMarks]), the earliest one on a tie; -1 without marks.
	Mark int
	// MarkOffset is Position minus the nearest mark; 0 without marks.
	MarkOffset float64
}

// PeakOption configures [Peaks]. Options return an error wrapping
// [ErrInvalidArgument] for invalid values.
type PeakOption func(*peakConfig) error

type peakConfig struct {
	marks    []float64
	position func(int) float64
}

// WithMarks sets reference marks, for example hand-set cue starts, in the
// unit of the boundary positions (see [WithPositions]). Each boundary then
// reports its nearest mark and the offset to it. The marks must be finite;
// the slice is read, not copied, during the call.
func WithMarks(marks []float64) PeakOption {
	return func(cfg *peakConfig) error {
		for i, m := range marks {
			if math.IsNaN(m) || math.IsInf(m, 0) {
				return fmt.Errorf("%w: mark %d is %g", ErrInvalidArgument, i, m)
			}
		}

		cfg.marks = marks

		return nil
	}
}

// WithPositions maps a novelty index to a position, for example a beat
// index to its start time in seconds. [Boundary.Position] and the mark
// offsets use it. Without it the position of index t is float64(t).
func WithPositions(position func(index int) float64) PeakOption {
	return func(cfg *peakConfig) error {
		if position == nil {
			return fmt.Errorf("%w: nil position function", ErrInvalidArgument)
		}

		cfg.position = position

		return nil
	}
}

// Peaks picks structural boundaries from a novelty curve (see
// [FooteNovelty]). Index t is a boundary if novelty[t] exceeds
// mean + sigma·sd of the curve (population standard deviation) and is the
// maximum within ±radius indices; of equal values in a window only the
// earliest counts. Boundaries are returned in index order.
//
// radius must be at least 0 ([DefaultPeakRadius] is 4) and sigma finite
// ([DefaultPeakSigma] is 1). A constant curve has no boundaries.
func Peaks(novelty []float64, radius int, sigma float64, opts ...PeakOption) ([]Boundary, error) {
	if radius < 0 {
		return nil, fmt.Errorf("%w: peak radius must be >= 0, got %d", ErrInvalidArgument, radius)
	}

	if math.IsNaN(sigma) || math.IsInf(sigma, 0) {
		return nil, fmt.Errorf("%w: peak sigma %g", ErrInvalidArgument, sigma)
	}

	var cfg peakConfig

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	n := float64(len(novelty))
	mean, sq := 0.0, 0.0

	for _, v := range novelty {
		mean += v / n
	}

	for _, v := range novelty {
		sq += (v - mean) * (v - mean) / n
	}

	threshold := mean + sigma*math.Sqrt(sq)

	var out []Boundary

	for t, v := range novelty {
		if v <= threshold || !isPeak(novelty, t, radius) {
			continue
		}

		b := Boundary{Index: t, Novelty: v, Position: float64(t), Mark: -1}
		if cfg.position != nil {
			b.Position = cfg.position(t)
		}

		best := math.Inf(1)

		for k, m := range cfg.marks {
			if d := b.Position - m; math.Abs(d) < math.Abs(best) {
				best, b.Mark = d, k
			}
		}

		if b.Mark >= 0 {
			b.MarkOffset = best
		}

		out = append(out, b)
	}

	return out, nil
}

// isPeak reports whether novelty[t] is the maximum within ±radius, the
// earliest of equal values.
func isPeak(novelty []float64, t, radius int) bool {
	v := novelty[t]

	for k := max(0, t-radius); k <= min(len(novelty)-1, t+radius); k++ {
		if novelty[k] > v || (novelty[k] == v && k < t) {
			return false
		}
	}

	return true
}
