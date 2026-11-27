package dynamics

import (
	"fmt"
	"math"
	"testing"
)

func TestDAVFeedbackLikeGainAndTiming(t *testing.T) {
	// Golden bits come from an independent model of the Win32 Smooth
	// Multiband gain and dynamic time-factor recurrence.
	for _, tc := range []struct {
		threshold, knee, ratio, peak float64
		attackCycle, releaseCycle    uint32
		gain, attack, release        uint64
	}{
		{-10, 3, 4, .5, 0xbb888889, 0xb9da740e, 0x3fe2531aa74454f1, 0x3f59d090fbe07800, 0x3feffeb555db7bcc},
		{-23, 2, 8, .045, 0xbbf7adf5, 0xba071911, 0x3feccb2e15a6bd02, 0x3f494dbbee971c00, 0x3fefff918b45b006},
		{-30, .2, 100, .25, 0xc0055556, 0xbed55555, 0x3fc08665d3284a18, 0x3fe8500af6f9209d, 0x3fe80f7ebe6f3f91},
		{-30, .2, 100, .0315, 0xbf855556, 0xbe555555, 0x3fef545d63ce77c4, 0x3f8ae2cbec05f140, 0x3fefea6064246ba0},
		{-18, 0, 1, .5, 0xbb888889, 0xb991a2b4, 0x3ff0000000000000, 0x3f67a1468e3e0a00, 0x3feffe6c2abb11ff},
		{-30, .2, 100, 1e-24, 0xc0055556, 0xbed55555, 0x3fefffea3c4e9cfa, 0x3f8d5dc2ef273f40, 0x3fefe85eb9408f0e},
	} {
		curve, err := NewDAVSoftKneeCompressorGain(tc.threshold, tc.knee, tc.ratio)
		if err != nil {
			t.Fatal(err)
		}

		gain, attack, release := curve.FeedbackLikeGainAndTiming(tc.peak,
			math.Float32frombits(tc.attackCycle), math.Float32frombits(tc.releaseCycle))
		if bits := math.Float64bits(gain); bits != tc.gain || gain != curve.Gain(tc.peak) {
			t.Errorf("ratio %g peak %g: gain %016x, want %016x", tc.ratio, tc.peak, bits, tc.gain)
		}

		if bits := math.Float64bits(attack); bits != tc.attack {
			t.Errorf("ratio %g peak %g: attack %016x, want %016x", tc.ratio, tc.peak, bits, tc.attack)
		}

		if bits := math.Float64bits(release); bits != tc.release {
			t.Errorf("ratio %g peak %g: release %016x, want %016x", tc.ratio, tc.peak, bits, tc.release)
		}
	}
}

func TestDAVFeedbackLikeTimingInvalidInputs(t *testing.T) {
	curve, err := NewDAVSoftKneeCompressorGain(-30, .2, 100)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		peak            float64
		attack, release float32
	}{
		{0, -.05, -.01},
		{math.NaN(), -.05, -.01},
		{math.Inf(1), -.05, -.01},
		{.5, float32(math.NaN()), -.01},
		{.5, -.05, float32(math.Inf(-1))},
		{.5, .05, -.01},
	} {
		gain, attack, release := curve.FeedbackLikeGainAndTiming(tc.peak, tc.attack, tc.release)
		if gain != curve.Gain(tc.peak) || attack != 0 || release != 1 {
			t.Errorf("input %+v: got gain %g attack %g release %g", tc, gain, attack, release)
		}
	}
}

func TestDAVFeedbackLikeTimingExtremeRatios(t *testing.T) {
	const attackCycle, releaseCycle = float32(-.05), float32(-.01)

	for _, tc := range []struct {
		threshold, ratio, peak float64
		attack, release        float64
	}{
		// A hard knee far below the peak keeps the unit time scale.
		{-math.MaxFloat32, math.MaxFloat32, .5,
			1 - davPower2Extended(float64(attackCycle)), davPower2Extended(float64(releaseCycle))},
		// Expansion with a vanishing ratio reacts instantly.
		{-math.MaxFloat32, 1e-30, .5, 1, 0},
	} {
		curve, err := NewDAVSoftKneeCompressorGain(tc.threshold, 0, tc.ratio)
		if err != nil {
			t.Fatal(err)
		}

		_, attack, release := curve.FeedbackLikeGainAndTiming(tc.peak, attackCycle, releaseCycle)
		if math.Abs(attack-tc.attack) > 1e-12 || math.Abs(release-tc.release) > 1e-12 {
			t.Errorf("ratio %g: attack %g release %g, want %g %g", tc.ratio, attack, release, tc.attack, tc.release)
		}
	}
}

func ExampleDAVSoftKneeCompressorGain_FeedbackLikeGainAndTiming() {
	curve, _ := NewDAVSoftKneeCompressorGain(-10, 3, 4)

	const sampleRate = 48000

	attackCycle := float32(-1 / (5 * .001 * sampleRate))
	releaseCycle := float32(-1 / (50 * .001 * sampleRate))
	gain, attack, release := curve.FeedbackLikeGainAndTiming(.5, attackCycle, releaseCycle)
	fmt.Printf("%.3f %.6f %.6f\n", gain, attack, release)
	// Output: 0.573 0.001576 0.999842
}
