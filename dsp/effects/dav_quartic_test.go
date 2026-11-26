package effects

import (
	"math"
	"testing"
)

func TestDAVQuarticSaturationExactValues(t *testing.T) {
	tests := []struct {
		input float64
		bits  uint64
	}{
		{0.125, 0x3fafff4bffbc7fe7},
		{0.25, 0x3fbffa2a83c61929},
		{0.5, 0x3fcfcfe7f3f9fcfe},
		{1, 0x3fde9bd37a6f4dea},
		{2, 0x3fe9249249249249},
		{4, 0x3feec0d4c77b0353},
		{8, 0x3fefdff551c5eca4},
	}
	for _, test := range tests {
		got := math.Float64bits(DAVQuarticSaturation(test.input))
		if got != test.bits {
			t.Errorf("input %g: got %#x, want %#x", test.input, got, test.bits)
		}
		got = math.Float64bits(DAVQuarticSaturation(-test.input))
		if want := test.bits | 1<<63; got != want {
			t.Errorf("input %g: got %#x, want %#x", -test.input, got, want)
		}
	}
}

func TestDAVQuarticSaturationBoundaries(t *testing.T) {
	for _, input := range []float64{65535, 65536, 1e80, math.MaxFloat64, math.Inf(1)} {
		if positive, negative := DAVQuarticSaturation(input), DAVQuarticSaturation(-input); positive != 1 || negative != -1 {
			t.Errorf("input magnitude %g: got %g and %g, want 1 and -1", input, positive, negative)
		}
	}
	for _, zero := range []float64{0, math.Copysign(0, -1)} {
		if got := math.Float64bits(DAVQuarticSaturation(zero)); got != 0 {
			t.Errorf("input zero: got bits %#x, want positive zero", got)
		}
	}
	if !math.IsNaN(DAVQuarticSaturation(math.NaN())) {
		t.Error("expected NaN propagation")
	}
}
