package dynamics

import (
	"math"
	"testing"
)

func TestDAVLevelingGain(t *testing.T) {
	for _, tc := range []struct {
		peak, knee, ratio, gain, legacy float64
	}{
		{0, .5, 10, 1, 1},
		{.4, .5, 10, 0x1.d1d468p-1, 0x1.d1cc3470148c6p-1},
		{2.6, .5, 10, 0x1.712596p-2, 0x1.71234451d7ef0p-2},
		{.2, .67, 49, 0x1.ea16dcp-1, 0x1.ea0e27bf19bb5p-1},
		{19, .67, 49, 0x1.bfa86ap-5, 0x1.bfa811a70ec3bp-5},
		{.05, 1, 10, 0x1.01f356p+0, 0x1.01f51c2513cbfp+0},
		{1, .05, 1, 1, 1},
	} {
		c, err := NewDAVLevelingGain(tc.knee, tc.ratio)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Gain(tc.peak); got != tc.gain {
			t.Errorf("peak %g, knee %g, ratio %g: got %x, want %x", tc.peak, tc.knee, tc.ratio, got, tc.gain)
		}
		if got := c.LegacyGain(tc.peak); math.Abs(got-tc.legacy) > 2e-16 {
			t.Errorf("legacy peak %g, knee %g, ratio %g: got %x, want %x", tc.peak, tc.knee, tc.ratio, got, tc.legacy)
		}
	}
	if _, err := NewDAVLevelingGain(1.01, 1); err == nil {
		t.Error("accepted knee above one")
	}
	if _, err := NewDAVLevelingGain(.5, .99); err == nil {
		t.Error("accepted ratio below one")
	}
}
