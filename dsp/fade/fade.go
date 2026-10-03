// Package fade applies position-aware fades and crossfades without allocating.
// Positions are relative to the complete fade, so arbitrary block partitions
// produce the same samples as one contiguous call.
package fade

import (
	"fmt"
	"math"
)

// Shape selects the rising envelope. Falling envelopes evaluate it backwards.
type Shape string

const (
	// Linear interpolates amplitude linearly.
	Linear Shape = "linear"
	// EqualPower uses a sine envelope; complementary squared gains sum to one.
	EqualPower Shape = "equal-power"
	// Logarithmic uses log(1+9*x)/log(10).
	Logarithmic Shape = "logarithmic"
	// SCurve uses the smoothstep polynomial x*x*(3-2*x).
	SCurve Shape = "s-curve"
)

func validate(start, total int64, size int, shape Shape) error {
	if total <= 0 || start < 0 || start > total || int64(size) > total-start {
		return fmt.Errorf("fade: block outside fade: start=%d total=%d size=%d", start, total, size)
	}

	switch shape {
	case Linear, EqualPower, Logarithmic, SCurve:
		return nil
	default:
		return fmt.Errorf("fade: unknown shape %q", shape)
	}
}

func envelope(x float64, shape Shape) float64 {
	if x == 0 || x == 1 {
		return x
	}

	switch shape {
	case EqualPower:
		return math.Sin(x * (math.Pi / 2))
	case Logarithmic:
		return math.Log1p(9*x) / math.Log(10)
	case SCurve:
		return x * x * (3 - 2*x)
	default:
		return x
	}
}

// ApplyInto32 multiplies src by a rising or falling envelope into equally sized
// dst. start is the first frame's offset in the complete total-frame fade.
// Both endpoints are exact: an N>1 rising fade runs from zero to one, and a
// falling fade from one to zero. A single-frame fade has zero gain in either direction.
// Exact src/dst aliasing is supported; partial overlap is not. Input arithmetic
// follows IEEE-754 without clipping. Invalid arguments leave dst unchanged.
func ApplyInto32(dst, src []float32, start, total int64, shape Shape, fadeIn bool) error {
	if len(dst) != len(src) {
		return fmt.Errorf("fade: input and output lengths differ")
	}

	if err := validate(start, total, len(src), shape); err != nil {
		return err
	}

	if len(src) == 0 {
		return nil
	}

	if total <= 1<<53 {
		return applyFast32(dst, src, start, total, shape, fadeIn)
	}

	denominator := float64(max(total-1, 1))
	for i, sample := range src {
		x := float64(start+int64(i)) / denominator
		if !fadeIn {
			x = 1 - x
		}

		if total == 1 {
			x = 0
		}

		dst[i] = float32(float64(sample) * envelope(x, shape))
	}

	return nil
}

// CrossfadeInto32 combines left's falling envelope and right's rising envelope
// into equally sized dst. start and total have the same meaning as ApplyInto32.
// A crossfade requires at least two frames. Exact aliasing with either input is
// supported; partial overlap is not. No clipping is applied. Invalid arguments
// leave dst unchanged.
func CrossfadeInto32(dst, left, right []float32, start, total int64, shape Shape) error {
	if len(dst) != len(left) || len(dst) != len(right) || total < 2 {
		return fmt.Errorf("crossfade: input and output lengths differ")
	}

	if err := validate(start, total, len(dst), shape); err != nil {
		return fmt.Errorf("crossfade: %w", err)
	}

	if len(dst) == 0 {
		return nil
	}

	if total <= 1<<53 {
		return crossfadeFast32(dst, left, right, start, total, shape)
	}

	denominator := float64(max(total-1, 1))
	for i := range dst {
		x := float64(start+int64(i)) / denominator
		dst[i] = float32(float64(left[i])*envelope(1-x, shape) + float64(right[i])*envelope(x, shape))
	}

	return nil
}
