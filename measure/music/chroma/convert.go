package chroma

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// Profile returns the mean of every pitch class over the frames
// [start, end), the input of harmony.EstimateKey. The range is clipped to
// [0, n), n being the length of the shortest row; an empty range gives all
// zeros.
func Profile(c [12][]float64, start, end int) [12]float64 {
	n := len(c[0])
	for _, row := range c[1:] {
		n = min(n, len(row))
	}

	start, end = max(start, 0), min(end, n)

	var p [12]float64

	if end <= start {
		return p
	}

	for pc, row := range c {
		sum := 0.0
		for _, v := range row[start:end] {
			sum += v
		}

		p[pc] = sum / float64(end-start)
	}

	return p
}

// BeatSync averages frame chroma per beat of a grid, the input of
// motif.FindChromaMotifs. Beat i spans [grid.BeatStart(i),
// grid.BeatStart(i+1)), the span FindChromaMotifs assumes, and covers the
// frames j with ceil(start·frameRate) ≤ j < ceil(end·frameRate), clipped to
// the signal: the rule harmony.Windows applies to its spans. So a frame
// belongs to the beat that contains its centre j/frameRate, a frame exactly
// on a beat boundary to the later beat.
//
// The result has one vector per beat, beats in all (pass len(grid.Beats())
// to cover the grid's duration). Each vector is the mean of the beat's
// frames per class; beats without frames, such as beats before the first
// frame (a negative grid origin) or after the last, are all zero. Note that
// beat times always follow the grid's constant tempo, not measured beat
// times given with rhythm.WithBeats.
//
// BeatSync returns an error wrapping [ErrInvalidArgument] for a grid not
// built by rhythm.NewGrid, a frame rate that is not positive and finite, a
// negative beat count, or rows of unequal length.
func BeatSync(c [12][]float64, frameRate float64, grid rhythm.Grid, beats int) ([][12]float64, error) {
	err := grid.Validate()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	if !finitePositive(frameRate) {
		return nil, fmt.Errorf("%w: frame rate %g", ErrInvalidArgument, frameRate)
	}

	if beats < 0 {
		return nil, fmt.Errorf("%w: beat count %d", ErrInvalidArgument, beats)
	}

	n := len(c[0])
	for pc, row := range c {
		if len(row) != n {
			return nil, fmt.Errorf("%w: chroma row %d has %d frames, row 0 has %d", ErrInvalidArgument, pc, len(row), n)
		}
	}

	out := make([][12]float64, beats)

	for i := range out {
		lo := ceilClamp(grid.BeatStart(i)*frameRate, n)
		hi := max(lo, ceilClamp(grid.BeatStart(i+1)*frameRate, n))
		out[i] = Profile(c, lo, hi)
	}

	return out, nil
}

// ceilClamp returns ceil(x) limited to [0, n], clamping before the integer
// conversion so that huge values cannot overflow (as harmony does).
func ceilClamp(x float64, n int) int {
	v := math.Ceil(x)

	switch {
	case v <= 0:
		return 0
	case v >= float64(n):
		return n
	default:
		return int(v)
	}
}
