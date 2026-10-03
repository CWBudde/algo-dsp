package fade_test

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/fade"
)

// Keep the pre-optimization formulas independent of the optimized dispatch.
func referenceEnvelope(x float64, shape fade.Shape) float64 {
	if x == 0 || x == 1 {
		return x
	}

	switch shape {
	case fade.EqualPower:
		return math.Sin(x * (math.Pi / 2))
	case fade.Logarithmic:
		return math.Log1p(9*x) / math.Log(10)
	case fade.SCurve:
		return x * x * (3 - 2*x)
	default:
		return x
	}
}

func TestFadeOptimizedBitsMatchReference(t *testing.T) {
	t.Parallel()

	positions := [][2]int64{
		{0, 1},
		{0, 3},
		{0, 257},
		{1234567, 28800000},
		{(1 << 53) - 257, 1 << 53},
		{(1 << 53) + 11, (1 << 53) + 1000},
		{math.MaxInt64 - 1000, math.MaxInt64},
	}
	for _, shape := range []fade.Shape{fade.Linear, fade.EqualPower, fade.Logarithmic, fade.SCurve} {
		for _, pos := range positions {
			start, total := pos[0], pos[1]
			length := int(min(257, total-start))

			left, right, got := make([]float32, length), make([]float32, length), make([]float32, length)
			for i := range left {
				left[i], right[i] = float32(math.Sin(float64(i))*.33), float32(math.Cos(float64(i))*2)
			}

			for _, rising := range []bool{true, false} {
				if err := fade.ApplyInto32(got, left, start, total, shape, rising); err != nil {
					t.Fatal(err)
				}

				for i, sample := range left {
					x := float64(start+int64(i)) / float64(max(total-1, 1))
					if !rising {
						x = 1 - x
					}

					if total == 1 {
						x = 0
					}

					want := float32(float64(sample) * referenceEnvelope(x, shape))
					if math.Float32bits(got[i]) != math.Float32bits(want) {
						t.Fatalf("%s rising%v pos%v index%d got%v want%v", shape, rising, pos, i, got[i], want)
					}
				}
			}

			if total < 2 {
				continue
			}

			if err := fade.CrossfadeInto32(got, left, right, start, total, shape); err != nil {
				t.Fatal(err)
			}

			for i := range got {
				x := float64(start+int64(i)) / float64(total-1)

				want := float32(float64(left[i])*referenceEnvelope(1-x, shape) + float64(right[i])*referenceEnvelope(x, shape))
				if math.Float32bits(got[i]) != math.Float32bits(want) {
					t.Fatalf("crossfade %s pos%v index%d got%v want%v", shape, pos, i, got[i], want)
				}
			}
		}
	}
}
