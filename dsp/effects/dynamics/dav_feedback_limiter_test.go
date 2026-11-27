package dynamics

import (
	"fmt"
	"math"
	"testing"
)

func TestDAVFeedbackLimiterGainAndTiming(t *testing.T) {
	// Bit vectors were independently derived using a cast-by-cast model
	// of the Win32 limiter recurrence, including active hard-knee timing.
	for _, tc := range []struct {
		name                      string
		threshold, knee, peak     float64
		attackCycle, releaseCycle uint32
		gain, release             uint32
		attack                    uint64
	}{
		{"quiet soft knee", -10, 2, 1e-24, 0xbab60b61, 0xb991a2b4, 0x3f7fbfbb, 0x3f800000, 0},
		{"loud soft knee", -18, 3, .5, 0xbbe38e39, 0xb9888889, 0x3e6d9ca7, 0x3f800000, 0},
		{"hard knee at 48 kHz", -18, 0, .5, 0xc0055555, 0xbed55555, 0x3e80ea07, 0x3f3fcba6, 0x3fe872f198000000},
		{"hard knee at 44.1 kHz", -30, 0, 1, 0xc0111fed, 0xbee83315, 0x3d0186fd, 0x3f3af309, 0x3fe95a8fa0000000},
		{"hard knee at 96 kHz", -90, 0, 32, 0xbf855555, 0xbe555555, 0x3584a317, 0x3f5d968c, 0x3fe0749df0000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			curve, err := NewDAVSoftKneeGain(tc.threshold, tc.knee)
			if err != nil {
				t.Fatal(err)
			}

			gain, attack, release := curve.FeedbackLimiterGainAndTiming(tc.peak,
				math.Float32frombits(tc.attackCycle), math.Float32frombits(tc.releaseCycle))
			if got := math.Float32bits(float32(gain)); got != tc.gain {
				t.Errorf("gain %08x, want %08x", got, tc.gain)
			}

			if got := math.Float64bits(attack); got != tc.attack {
				t.Errorf("attack %016x, want %016x", got, tc.attack)
			}

			if got := math.Float32bits(float32(release)); got != tc.release {
				t.Errorf("release %08x, want %08x", got, tc.release)
			}

			if want := curve.Gain(tc.peak); gain != want {
				t.Errorf("gain %.12g, want existing transfer %.12g", gain, want)
			}
		})
	}
}

func TestDAVFeedbackLimiterInvalidInputs(t *testing.T) {
	curve, err := NewDAVSoftKneeGain(-18, 3)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		peak            float64
		attack, release float32
	}{
		{0, -.01, -.005},
		{math.NaN(), -.01, -.005},
		{math.Inf(1), -.01, -.005},
		{.5, float32(math.Inf(-1)), -.005},
		{.5, -.01, .005},
		{.5, -.01, float32(math.NaN())},
	} {
		gain, attack, release := curve.FeedbackLimiterGainAndTiming(tc.peak, tc.attack, tc.release)
		if gain != curve.Gain(tc.peak) || attack != 0 || release != 1 {
			t.Errorf("input %+v: got gain %.9g attack %.9g release %.9g", tc, gain, attack, release)
		}
	}
}

func ExampleDAVSoftKneeGain_FeedbackLimiterGainAndTiming() {
	curve, _ := NewDAVSoftKneeGain(-18, 0)

	const rate = 48000

	attackCycle := float32(-1 / (.01 * .001 * rate))
	releaseCycle := float32(-1 / (.05 * .001 * rate))
	gain, attack, release := curve.FeedbackLimiterGainAndTiming(.5, attackCycle, releaseCycle)
	fmt.Printf("%.3f %.3f %.3f\n", gain, attack, release)
	// Output: 0.252 0.764 0.749
}
