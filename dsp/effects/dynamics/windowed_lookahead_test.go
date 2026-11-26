package dynamics

import (
	"math"
	"testing"
)

func limiterReferenceInput(frame, channel int) float32 {
	switch {
	case frame == 13+channel*17:
		return 0.125
	case frame >= 513 && frame < 1025:
		divisor := float32(104)
		if channel != 0 {
			divisor = 35
		}

		return float32((frame*(7+6*channel))%127-63) / divisor
	case frame >= 1923 && frame < 2759:
		signal := float32(-0.25)
		if (frame>>3)&1 != 0 {
			signal = 0.3
		}

		return signal * float32(channel+1)
	case frame == 3150+channel*401:
		return -3
	case frame >= 4096 && frame < 6464:
		return float32((frame*(3+4*channel))%11-5) / 128
	default:
		return 0
	}
}

func TestWindowedLookaheadReleasedDualMono(t *testing.T) {
	limiter, err := NewWindowedLookahead(1024)
	if err != nil {
		t.Fatal(err)
	}

	if err := limiter.Configure(48000, 64, 0, -0.01, 75, LookaheadParabolic); err != nil {
		t.Fatal(err)
	}
	// Channel zero of the original DVLL binary, 48 kHz, 64-sample blocks.
	want := map[int]float32{
		77:   0.124856174,
		600:  0.05762592703104019,
		900:  -0.509028971195221,
		2000: -0.24971234798431396,
		3214: -0.9988493919372559,
		4210: 0.010449826717376709,
	}

	for frame := range 4211 {
		delayed, gain := limiter.ProcessSample(limiterReferenceInput(frame, 0))
		if expected, ok := want[frame]; ok && math.Abs(float64(delayed*gain-expected)) > 2e-7 {
			t.Errorf("frame %d: got %g, want %g", frame, delayed*gain, expected)
		}
	}

	if allocs := testing.AllocsPerRun(100, func() { limiter.ProcessSample(0.75) }); allocs != 0 {
		t.Fatalf("allocations per sample = %g", allocs)
	}
}

// settleLookahead feeds a constant input and returns the final gain.
func settleLookahead(l *WindowedLookahead, input float32, frames int) float32 {
	var gain float32
	for range frames {
		_, gain = l.ProcessSample(input)
	}

	return gain
}

func TestWindowedLookaheadLinearFullWindow(t *testing.T) {
	limiter, err := NewWindowedLookahead(64)
	if err != nil {
		t.Fatal(err)
	}

	if err := limiter.Configure(48000, 64, 0, 0, 75, LookaheadLinear); err != nil {
		t.Fatal(err)
	}

	// A held peak of 2 needs a reduction of 0.5; the two half rings form one
	// 64-sample window, so the linear shape reaches it exactly.
	if gain := settleLookahead(limiter, 2, 256); math.Abs(float64(gain)-0.5) > 1e-6 {
		t.Fatalf("settled linear gain = %g, want 0.5", gain)
	}
}

func TestWindowedLookaheadShapeSwitch(t *testing.T) {
	for _, shapes := range [][2]LookaheadAttack{
		{LookaheadParabolic, LookaheadLinear},
		{LookaheadLinear, LookaheadParabolic},
	} {
		limiter, err := NewWindowedLookahead(64)
		if err != nil {
			t.Fatal(err)
		}

		if err := limiter.Configure(48000, 64, 0, 0, 1, shapes[0]); err != nil {
			t.Fatal(err)
		}

		settleLookahead(limiter, 2, 256)

		if err := limiter.Configure(48000, 64, 0, 0, 1, shapes[1]); err != nil {
			t.Fatal(err)
		}

		if gain := settleLookahead(limiter, 2, 1); math.Abs(float64(gain)-0.5) > 1e-6 {
			t.Errorf("%d->%d: gain after switch = %g, want 0.5", shapes[0], shapes[1], gain)
		}

		if gain := settleLookahead(limiter, 2, 256); math.Abs(float64(gain)-0.5) > 1e-6 {
			t.Errorf("%d->%d: settled gain = %g, want 0.5", shapes[0], shapes[1], gain)
		}

		if gain := settleLookahead(limiter, 0, 4800); math.Abs(float64(gain)-1) > 1e-6 {
			t.Errorf("%d->%d: released gain = %g, want 1", shapes[0], shapes[1], gain)
		}
	}
}

func TestWindowedLookaheadHoldExpiresWithDelayedPeak(t *testing.T) {
	limiter, err := NewWindowedLookahead(4)
	if err != nil {
		t.Fatal(err)
	}

	if err := limiter.Configure(48000, 4, 0, 0, 75, LookaheadParabolic); err != nil {
		t.Fatal(err)
	}

	// The rescan after the 4 expires finds the 2 in the slot being output
	// now, so it must be held for that sample only.
	inputs := []float32{4, 2, 0, 0, 0, 0, 0, 0}
	wantHold := []float32{4, 4, 4, 4, 4, 2, 0, 0}

	for frame, input := range inputs {
		limiter.ProcessSample(input)

		if limiter.holdValue != wantHold[frame] {
			t.Errorf("frame %d: held peak = %g, want %g", frame, limiter.holdValue, wantHold[frame])
		}
	}
}
