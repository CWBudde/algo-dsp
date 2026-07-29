package webdemo

import (
	"math"
	"testing"
)

// The two functions below are the demo's own mono fold-down approximations for
// effects that are inherently stereo. They are real DSP design decisions that
// live in this package rather than in dsp/effects, and until now nothing
// covered them at all.

func testTone(n int, freqHz float64) []float64 {
	block := make([]float64, n)
	for i := range block {
		block[i] = 0.5 * math.Sin(2*math.Pi*freqHz*float64(i)/testSampleRate)
	}

	return block
}

func assertFinite(t *testing.T, label string, block []float64) {
	t.Helper()

	for i, v := range block {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("%s: sample %d is not finite: %v", label, i, v)
		}
	}
}

func TestProcessWidenerMonoInPlace(t *testing.T) {
	t.Parallel()

	t.Run("full dry mix is an identity", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.effects.WidenerMix = 0

		block := testTone(512, 440)
		want := append([]float64(nil), block...)

		e.processWidenerMonoInPlace(block)

		for i := range want {
			if block[i] != want[i] {
				t.Fatalf("sample %d changed to %v with a zero mix, want %v", i, block[i], want[i])
			}
		}
	})

	t.Run("wet mix stays finite and bounded", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.effects.WidenerMix = 1
		e.effects.WidenerWidth = 2

		block := testTone(1024, 440)
		e.processWidenerMonoInPlace(block)

		assertFinite(t, "widener", block)

		for i, v := range block {
			if math.Abs(v) > 4 {
				t.Fatalf("sample %d = %v, want the fold-down to stay bounded", i, v)
			}
		}
	})

	t.Run("empty block is a no-op", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.processWidenerMonoInPlace(nil)
		e.processWidenerMonoInPlace([]float64{})
	})

	t.Run("scratch buffer grows to fit", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.effects.WidenerMix = 0.5
		e.chainBuf = nil

		for _, n := range []int{64, 4096, 128} {
			block := testTone(n, 220)
			e.processWidenerMonoInPlace(block)

			if len(block) != n {
				t.Fatalf("block length changed from %d to %d", n, len(block))
			}

			assertFinite(t, "widener resize", block)
		}
	})
}

func TestProcessRotaryInPlace(t *testing.T) {
	t.Parallel()

	t.Run("produces finite audio and preserves length", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)

		block := testTone(2048, 440)
		e.processRotaryInPlace(block)

		if len(block) != 2048 {
			t.Fatalf("block length changed to %d", len(block))
		}

		assertFinite(t, "rotary", block)
	})

	t.Run("silence in, silence out", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)

		block := make([]float64, 1024)
		e.processRotaryInPlace(block)

		for i, v := range block {
			if v != 0 {
				t.Fatalf("sample %d = %v, want silence", i, v)
			}
		}
	})

	t.Run("empty block is a no-op", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.processRotaryInPlace(nil)
		e.processRotaryInPlace([]float64{})
	})
}

// TestProcessEffectsInPlacePrefersTheGraph documents the routing rule: when a
// graph is loaded it handles the block, and the legacy serial chain is the
// fallback for when it is not.
func TestProcessEffectsInPlaceRouting(t *testing.T) {
	t.Parallel()

	t.Run("empty block is a no-op", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)
		e.processEffectsInPlace(nil)
	})

	t.Run("falls back to the legacy chain with no graph", func(t *testing.T) {
		t.Parallel()

		e := newTestEngine(t)

		// No graph loaded, one legacy effect enabled.
		e.effects.ChorusEnabled = true
		e.effects.ChorusMix = 1

		block := testTone(1024, 440)
		before := append([]float64(nil), block...)

		e.processEffectsInPlace(block)

		assertFinite(t, "legacy chain", block)

		changed := false

		for i := range block {
			if block[i] != before[i] {
				changed = true
				break
			}
		}

		if !changed {
			t.Error("the legacy chain left the block untouched with chorus enabled")
		}
	})
}

// TestLegacyChainStaysFiniteWithEverythingOn is a blunt but useful guard: turn
// every legacy effect on at once and confirm the serial chain does not produce
// NaN, Inf, or runaway gain.
func TestLegacyChainStaysFiniteWithEverythingOn(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	err := e.SetEffects(EffectsParams{
		ChorusEnabled:          true,
		ChorusMix:              0.5,
		ChorusDepth:            0.003,
		ChorusSpeedHz:          0.35,
		ChorusStages:           3,
		FlangerEnabled:         true,
		FlangerRateHz:          0.3,
		FlangerDepth:           0.002,
		FlangerBaseDelay:       0.001,
		FlangerFeedback:        0.5,
		FlangerMix:             0.5,
		RingModEnabled:         true,
		RingModCarrierHz:       220,
		RingModMix:             0.5,
		BitCrusherEnabled:      true,
		BitCrusherBitDepth:     8,
		BitCrusherDownsample:   2,
		BitCrusherMix:          0.5,
		WidenerEnabled:         true,
		WidenerWidth:           1.5,
		WidenerMix:             0.5,
		PhaserEnabled:          true,
		PhaserRateHz:           0.4,
		PhaserMinFreqHz:        200,
		PhaserMaxFreqHz:        2000,
		PhaserStages:           4,
		PhaserFeedback:         0.4,
		PhaserMix:              0.5,
		TremoloEnabled:         true,
		TremoloRateHz:          4,
		TremoloDepth:           0.5,
		TremoloSmoothingMs:     5,
		TremoloMix:             0.5,
		RotarySpeakerEnabled:   true,
		RotaryMix:              0.5,
		RotaryDrive:            1,
		RotaryStereoWidth:      1,
		RotaryCrossoverHz:      800,
		DelayEnabled:           true,
		DelayTime:              0.25,
		DelayFeedback:          0.4,
		DelayMix:               0.4,
		ReverbEnabled:          true,
		ReverbModel:            "freeverb",
		ReverbWet:              0.4,
		ReverbDry:              0.6,
		ReverbRoomSize:         0.7,
		ReverbDamp:             0.4,
		ReverbGain:             0.5,
		HarmonicBassEnabled:    true,
		HarmonicBassFrequency:  100,
		HarmonicBassInputGain:  1,
		HarmonicBassHighGain:   1,
		HarmonicBassOriginal:   0.5,
		HarmonicBassHarmonic:   0.5,
		HarmonicBassDecay:      0.5,
		HarmonicBassResponseMs: 20,
	})
	if err != nil {
		t.Fatalf("SetEffects: %v", err)
	}

	peak := 0.0

	// Several blocks, so feedback paths have time to build up.
	for range 20 {
		block := testTone(1024, 440)
		e.processEffectsLegacyInPlace(block)
		assertFinite(t, "legacy chain", block)

		for _, v := range block {
			peak = math.Max(peak, math.Abs(v))
		}
	}

	if peak > 100 {
		t.Errorf("legacy chain peaked at %v, which suggests runaway feedback", peak)
	}
}
