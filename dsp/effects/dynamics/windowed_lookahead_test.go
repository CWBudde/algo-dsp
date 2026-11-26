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
