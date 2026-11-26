package dynamics

import (
	"math"
	"testing"
)

func TestDAVSoftKneeCompressorGain(t *testing.T) {
	for _, tc := range []struct {
		peak, threshold, knee, ratio, want float64
	}{
		{.125, -10, 2, 4, 0x1.ddbc699c3cf33p-1},
		{.5, -10, 2, 4, 0x1.438a05f264c44p-1},
		{1, -10, 2, 4, 0x1.97b362678a34cp-2},
		{.5, -10, 2, 1, 0x1.96bc3f62d6f88p-1},
		{.03125, -10, 2, .5, 0x1.8bbaa86ffc702p-4},
		{1e-10, -10, 4, .01, 0},
		{0, -10, 2, 4, 1},
		{0, -10, 2, .5, 0},
		{math.Inf(1), -10, 2, 4, 0},
		{math.Inf(1), -10, 2, .5, 1},
	} {
		c, err := NewDAVSoftKneeCompressorGain(tc.threshold, tc.knee, tc.ratio)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Gain(tc.peak); got != tc.want {
			t.Errorf("peak %g, threshold %g, knee %g, ratio %g: got %x, want %x",
				tc.peak, tc.threshold, tc.knee, tc.ratio, got, tc.want)
		}
	}
	if _, err := NewDAVSoftKneeCompressorGain(-10, 2, 0); err == nil {
		t.Error("accepted zero ratio")
	}
}

func TestDAVCompressorTiming(t *testing.T) {
	for _, tc := range []struct {
		attackMS, releaseMS, rate, wantAttack, wantRelease float64
	}{
		{15, 75, 88200, 0x1.12a8p-11, 0x1.fff244p-1},
		{1, 20, 96000, 0x1.d792p-8, 0x1.ffd0aep-1},
	} {
		attack, release, err := DAVCompressorTiming(tc.attackMS, tc.releaseMS, tc.rate)
		if err != nil || attack != tc.wantAttack || release != tc.wantRelease {
			t.Errorf("got (%x, %x, %v), want (%x, %x, nil)", attack, release, err, tc.wantAttack, tc.wantRelease)
		}
	}
	if _, _, err := DAVCompressorTiming(1, 20, 0); err == nil {
		t.Error("accepted zero sample rate")
	}
}
