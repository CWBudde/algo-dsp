package features_test

import (
	"math"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

// musicFixture returns a deterministic stereo test signal at 24 kHz: a bed of
// low-level noise, a 440 Hz tone with different gains per channel, decaying
// kick-like 60 Hz hits every 0.5 s, decaying noise bursts on the off-beats,
// an exactly silent gap from 2.0 to 2.3 s and an anti-phase 1 kHz section.
func musicFixture(seconds float64) [][]float64 {
	n := int(seconds * SampleRate)
	left := testutil.DeterministicNoise(1, 0.003, n)
	right := testutil.DeterministicNoise(2, 0.003, n)
	burst := testutil.DeterministicNoise(3, 0.5, SampleRate/5)

	for i := range n {
		t := float64(i) / SampleRate
		tone := math.Sin(2 * math.Pi * 440 * t)
		left[i] += 0.2 * tone
		right[i] += 0.12 * tone

		beat := math.Mod(t, 0.5)
		kick := 0.7 * math.Sin(2*math.Pi*60*beat) * math.Exp(-beat/0.05)
		left[i] += kick
		right[i] += kick

		if off := math.Mod(t+0.25, 0.5); int(off*SampleRate) < len(burst) {
			v := burst[int(off*SampleRate)] * math.Exp(-off/0.02)
			left[i] += v
			right[i] -= 0.5 * v
		}

		if t >= 2.6 && t < 3.0 {
			a := 0.3 * math.Sin(2*math.Pi*1000*t)
			left[i] += a
			right[i] -= a
		}

		if t >= 2.0 && t < 2.3 {
			left[i], right[i] = 0, 0
		}
	}

	return [][]float64{left, right}
}
