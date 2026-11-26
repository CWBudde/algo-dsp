package dynamics

import (
	"math"
	"testing"
)

func TestDAVSoftKneeGain(t *testing.T) {
	// Single-width boundary taken from the retained DAV gain calculation,
	// including the release polynomial coefficients and square-root seed.
	for _, tc := range []struct {
		threshold, knee, peak float64
		bits                  uint32
	}{
		{0, 6, 1e-24, 0x3f7dcc62},
		{0, 6, 0.125, 0x3f4fd41d},
		{0, 6, 0.5, 0x3f2729e6},
		{0, 6, 1, 0x3f004dd3},
		{0, 6, 2, 0x3ea729e6},
		{0, 6, 4, 0x3e4071db},
		{-12, 3, 0.1, 0x3f642b17},
		{-12, 3, 0.25, 0x3f35a5ca},
		{-12, 3, 0.4, 0x3f05c7a7},
		{-12, 0, 1.3, 0x3e45dcbf},
		{0, 0, 0.7, 0x3f800000},
	} {
		curve, err := NewDAVSoftKneeGain(tc.threshold, tc.knee)
		if err != nil {
			t.Fatal(err)
		}
		if got := math.Float32bits(float32(curve.Gain(tc.peak))); got != tc.bits {
			t.Errorf("threshold=%g knee=%g peak=%g: gain bits %08x, want %08x", tc.threshold, tc.knee, tc.peak, got, tc.bits)
		}
	}
}

func TestDAVSoftKneeGainBoundaries(t *testing.T) {
	for _, settings := range [][2]float64{{0, -1}, {math.Inf(1), 6}, {0, math.NaN()}, {0, math.MaxFloat64}} {
		if _, err := NewDAVSoftKneeGain(settings[0], settings[1]); err == nil {
			t.Errorf("accepted invalid threshold/knee %v", settings)
		}
	}
	curve, err := NewDAVSoftKneeGain(0, 6)
	if err != nil {
		t.Fatal(err)
	}
	for _, peak := range []float64{0, -1, math.NaN()} {
		if gain := curve.Gain(peak); gain != 1 {
			t.Errorf("peak %g: gain %g, want unity", peak, gain)
		}
	}
	if gain := curve.Gain(math.Inf(1)); gain != 0 {
		t.Errorf("infinite peak: gain %g, want zero", gain)
	}
}
