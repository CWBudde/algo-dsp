package fade

import (
	"fmt"
	"math"
)

// EnvelopeInto64 computes unrounded gains for one complete fade's block.
// Its positions, endpoints and single-frame behavior match ApplyInto32.
// The caller can reuse one envelope across linked channels without repeating
// trigonometric/logarithmic work or rounding gains to float32. Invalid arguments
// leave dst unchanged; the successful path does not allocate.
func EnvelopeInto64(dst []float64, start, total int64, shape Shape, rising bool) error {
	if err := validate(start, total, len(dst), shape); err != nil {
		return err
	}

	if total == 1 {
		clear(dst)
		return nil
	}

	denominator := float64(total - 1)
	if total > 1<<53 {
		for i := range dst {
			x := float64(start+int64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			dst[i] = envelope(x, shape)
		}

		return nil
	}

	base := float64(start)

	switch shape {
	case EqualPower:
		for i := range dst {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			gain := math.Sin(x * (math.Pi / 2))
			if x == 0 || x == 1 {
				gain = x
			}

			dst[i] = gain
		}
	case Logarithmic:
		for i := range dst {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			gain := math.Log1p(9*x) / math.Ln10
			if x == 0 || x == 1 {
				gain = x
			}

			dst[i] = gain
		}
	case SCurve:
		for i := range dst {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			dst[i] = x * x * (3 - 2*x)
		}
	default:
		for i := range dst {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			dst[i] = x
		}
	}

	return nil
}

// ApplyEnvelopeInto32 multiplies src by a shared unrounded envelope. All slices
// must have equal lengths. Exact dst/src aliasing is supported; partial overlap
// is not. IEEE-754 nonfinite input/gain propagates without clipping or rescanning.
// Invalid lengths leave dst unchanged. The successful path does not allocate.
func ApplyEnvelopeInto32(dst, src []float32, envelope []float64) error {
	if len(dst) != len(src) || len(dst) != len(envelope) {
		return fmt.Errorf("fade.apply-envelope: input and output lengths differ")
	}

	if len(dst) == 0 {
		return nil
	}

	_ = src[len(dst)-1]

	_ = envelope[len(dst)-1]
	for i := range dst {
		dst[i] = float32(float64(src[i]) * envelope[i])
	}

	return nil
}

// CrossfadeEnvelopeInto32 combines left/right with shared unrounded falling and
// rising envelopes. All slices must have equal lengths. Exact dst aliasing with
// either source is supported; partial overlap is not. Arithmetic follows
// IEEE-754 without clipping. Invalid lengths leave dst unchanged. The successful
// path does not allocate.
func CrossfadeEnvelopeInto32(dst, left, right []float32, falling, rising []float64) error {
	if len(dst) != len(left) || len(dst) != len(right) || len(dst) != len(falling) || len(dst) != len(rising) {
		return fmt.Errorf("fade.crossfade-envelope: input and output lengths differ")
	}

	if len(dst) == 0 {
		return nil
	}

	_ = left[len(dst)-1]
	_ = right[len(dst)-1]
	_ = falling[len(dst)-1]

	_ = rising[len(dst)-1]
	for i := range dst {
		dst[i] = float32(float64(left[i])*falling[i] + float64(right[i])*rising[i])
	}

	return nil
}
