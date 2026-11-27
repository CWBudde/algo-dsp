package effects

import (
	"math"
	"testing"
)

func TestDAVContinuousTanh4(t *testing.T) {
	for _, test := range []struct{ input, want float32 }{
		{0, 0},
		{1, .7615947723388672},
		{.5, .4621184170246124},
		{-.5, -.462118536233902},
		{1000, 1},
		{-1000, -1},
	} {
		if got := DAVContinuousTanh4(test.input); math.Abs(float64(got-test.want)) > 6e-8 {
			t.Fatalf("input %g gave %g, want %g", test.input, got, test.want)
		}
	}

	for i := -4096; i <= 4096; i++ {
		input := float32(i) / 512
		if got := DAVContinuousTanh4(input); math.Abs(float64(got)-math.Tanh(float64(input))) > 6e-6 {
			t.Fatalf("input %g exceeds approximation tolerance with %g", input, got)
		}
	}

	if !math.IsNaN(float64(DAVContinuousTanh4(float32(math.NaN())))) {
		t.Fatal("NaN must propagate")
	}
}
