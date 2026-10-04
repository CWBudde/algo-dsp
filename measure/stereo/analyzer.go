// Package stereo provides bounded phase correlation and mid/side display points.
package stereo

import (
	"fmt"
	"math"
)

// Point is an orthonormal mid/side projection: Mid=(L+R)/sqrt(2),
// Side=(L-R)/sqrt(2). Values retain the original signal amplitude.
type Point struct{ Mid, Side float64 }

// Analyzer computes correlation over a fixed sliding window. It retains
// evenly decimated display points and never retains caller input storage.
type Analyzer struct {
	left, right                                         []float64
	points                                              []Point
	write, count, pointWrite, pointCount, stride, clock int
}

// NewAnalyzer reserves a sliding window and 1..4096 display points. Window frames
// must be 1..1048576. Processing, reset and snapshot copying allocate nothing.
func NewAnalyzer(windowFrames, points int) (*Analyzer, error) {
	if windowFrames < 1 || windowFrames > 1<<20 || points < 1 || points > 4096 {
		return nil, fmt.Errorf("stereo analyzer: invalid window or point capacity")
	}

	return &Analyzer{left: make([]float64, windowFrames), right: make([]float64, windowFrames), points: make([]Point, points), stride: max(1, windowFrames/points)}, nil
}

// ProcessInterleaved32 measures the first stereo pair; mono is treated as L=R.
// All supplied channels are validated atomically, including unused channels.
func (a *Analyzer) ProcessInterleaved32(src []float32, channels int) error {
	if a == nil || len(a.left) == 0 || channels < 1 || channels > 32 || len(src)%channels != 0 {
		return fmt.Errorf("stereo analyzer: invalid channel layout or unconfigured")
	}

	for _, v := range src {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("stereo analyzer: nonfinite input")
		}
	}

	for frame := 0; frame < len(src)/channels; frame++ {
		l := float64(src[frame*channels])

		r := l
		if channels > 1 {
			r = float64(src[frame*channels+1])
		}

		a.left[a.write] = l
		a.right[a.write] = r
		a.write = (a.write + 1) % len(a.left)

		a.count = min(a.count+1, len(a.left))
		if a.clock%a.stride == 0 {
			a.points[a.pointWrite] = Point{Mid: (l + r) * math.Sqrt(.5), Side: (l - r) * math.Sqrt(.5)}
			a.pointWrite = (a.pointWrite + 1) % len(a.points)
			a.pointCount = min(a.pointCount+1, len(a.points))
		}

		a.clock = (a.clock + 1) % a.stride
	}

	return nil
}

// Correlation returns [-1,+1]; silence or a silent side yields zero.
func (a *Analyzer) Correlation() float64 {
	if a == nil {
		return 0
	}
	// Rebuild positive energies from the retained window. Subtracting expired
	// energies loses quiet samples after large signals and cannot recover.
	var ll, rr, lr float64

	for i, l := range a.left {
		r := a.right[i]
		ll += l * l
		rr += r * r
		lr += l * r
	}

	if ll == 0 || rr == 0 {
		return 0
	}

	return math.Max(-1, math.Min(1, lr/math.Sqrt(ll*rr)))
}

// PointsInto copies the most recent display points chronologically. A short
// destination gets the most recent subset. The return value is points written.
func (a *Analyzer) PointsInto(dst []Point) int {
	if a == nil {
		return 0
	}

	n := min(len(dst), a.pointCount)

	start := (a.pointWrite - n + len(a.points)) % len(a.points)
	for i := range n {
		dst[i] = a.points[(start+i)%len(a.points)]
	}

	return n
}

// Reset clears signal history without reallocating.
func (a *Analyzer) Reset() {
	if a == nil {
		return
	}

	clear(a.left)
	clear(a.right)
	clear(a.points)
	a.write = 0
	a.count = 0
	a.pointWrite = 0
	a.pointCount = 0
	a.clock = 0
}
