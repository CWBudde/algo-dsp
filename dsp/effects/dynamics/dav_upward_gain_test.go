package dynamics

import (
	"math"
	"testing"
)

func TestDAVSoftKneeUpwardGain(t *testing.T) {
	for _, tc := range []struct {
		peak, threshold, knee, ratio, want float64
	}{
		{.125, -10, 2, .5, 0x1.55ca1d8665aa9p+1},
		{.5, -10, 2, 5, 0x1.3d10f03492ee7p+5},
		{1, -10, 2, 4, 0x1.f7d8d2a04af84p+9},
		{.5, -10, 2, 1, 0x1.42508a1c204fep+0},
		{.03125, -10, 2, .5, 0x1.4b4585ccd210ap+3},
		{.9, -20, 0, 100, math.MaxFloat32},
		{1e-24, -10, 2, .01, math.MaxFloat32},
		{math.NaN(), -10, 2, 5, 1},
		{math.Inf(1), -10, 2, .5, 1},
		{math.Inf(1), -10, 2, 5, math.MaxFloat32},
		{math.Inf(1), -10, 2, 1, 0x1.42508a1c204fep+0},
	} {
		c, err := NewDAVSoftKneeUpwardGain(tc.threshold, tc.knee, tc.ratio)
		if err != nil {
			t.Fatal(err)
		}

		if got := c.Gain(tc.peak); got != tc.want {
			t.Errorf("peak %g, threshold %g, knee %g, ratio %g: got %x, want %x",
				tc.peak, tc.threshold, tc.knee, tc.ratio, got, tc.want)
		}
	}

	if _, err := NewDAVSoftKneeUpwardGain(-10, 2, 0); err == nil {
		t.Error("accepted zero ratio")
	}
}
