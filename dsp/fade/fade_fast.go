package fade

import "math"

// Integer positions through 2^53 are exactly representable. Convert the int64
// block offset once, and add small loop indices in float64; this avoids the
// repeated int64-to-float conversion helpers on the WASM target. A dispatch
// outside the loop also avoids a string switch/function call per sample.
func applyFast32(dst, src []float32, start, total int64, shape Shape, rising bool) error {
	_ = dst[len(src)-1 : len(src)]

	base, denominator := float64(start), float64(max(total-1, 1))
	if total == 1 {
		for i, sample := range src {
			dst[i] = float32(float64(sample) * 0)
		}

		return nil
	}

	switch shape {
	case EqualPower:
		for i, sample := range src {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			gain := math.Sin(x * (math.Pi / 2))
			if x == 0 || x == 1 {
				gain = x
			}

			dst[i] = float32(float64(sample) * gain)
		}
	case Logarithmic:
		for i, sample := range src {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			gain := math.Log1p(9*x) / math.Ln10
			if x == 0 || x == 1 {
				gain = x
			}

			dst[i] = float32(float64(sample) * gain)
		}
	case SCurve:
		for i, sample := range src {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			dst[i] = float32(float64(sample) * (x * x * (3 - 2*x)))
		}
	default:
		for i, sample := range src {
			x := (base + float64(i)) / denominator
			if !rising {
				x = 1 - x
			}

			dst[i] = float32(float64(sample) * x)
		}
	}

	return nil
}

func crossfadeFast32(dst, left, right []float32, start, total int64, shape Shape) error {
	_ = left[len(dst)-1 : len(dst)]
	_ = right[len(dst)-1 : len(dst)]
	base, denominator := float64(start), float64(total-1)

	switch shape {
	case EqualPower:
		for i := range dst {
			x := (base + float64(i)) / denominator

			falling, rising := math.Sin((1-x)*(math.Pi/2)), math.Sin(x*(math.Pi/2))
			if x == 0 || x == 1 {
				falling, rising = 1-x, x
			}

			dst[i] = float32(float64(left[i])*falling + float64(right[i])*rising)
		}
	case Logarithmic:
		for i := range dst {
			x := (base + float64(i)) / denominator

			falling, rising := math.Log1p(9*(1-x))/math.Ln10, math.Log1p(9*x)/math.Ln10
			if x == 0 || x == 1 {
				falling, rising = 1-x, x
			}

			dst[i] = float32(float64(left[i])*falling + float64(right[i])*rising)
		}
	case SCurve:
		for i := range dst {
			x := (base + float64(i)) / denominator
			y := 1 - x
			falling, rising := y*y*(3-2*y), x*x*(3-2*x)
			dst[i] = float32(float64(left[i])*falling + float64(right[i])*rising)
		}
	default:
		for i := range dst {
			x := (base + float64(i)) / denominator
			dst[i] = float32(float64(left[i])*(1-x) + float64(right[i])*x)
		}
	}

	return nil
}
