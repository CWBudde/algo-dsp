package dynamics

import (
	"math"
	"testing"
)

func TestDAVSpectralGateGain(t *testing.T) {
	// Binary32 outputs from a separate extended-precision scalar replay
	// of the published Win32 filter. Include raw signed and wrapped
	// exponent cases as well as ordinary compressor attenuation.
	for _, tc := range []struct {
		peak, threshold, knee, ratio float32
		word                         uint32
	}{
		{.125, -10, 2, 4, 0x3f6ede35},
		{.03125, -10, 2, .5, 0x3dc5dd54},
		{.5, -10, 2, 1, 0x3f4b5e20},
		{.3, 0, 0, .01, 0},
		{.165, 0, 0, .01, 0xbecae43f},
		{.167, 0, 0, .01, 0},
		{.17, 0, 0, .01, 0},
		{1e-10, -10, 4, .01, 0x25529bee},
		{.12, 0, 0, .01, 0xa8111818},
		{.19, 0, 0, .01, 0},
	} {
		c, err := NewDAVSoftKneeCompressorGain(float64(tc.threshold), float64(tc.knee), float64(tc.ratio))
		if err != nil {
			t.Fatal(err)
		}

		got := math.Float32bits(c.SpectralGateGain(float64(tc.peak)))
		if got != tc.word {
			t.Errorf("peak %g threshold %g knee %g ratio %g: gain bits %08x, want %08x",
				tc.peak, tc.threshold, tc.knee, tc.ratio, got, tc.word)
		}
	}
}

func TestDAVSpectralGateGainRejectsNonfinite(t *testing.T) {
	c, err := NewDAVSoftKneeCompressorGain(0, 0, .01)
	if err != nil {
		t.Fatal(err)
	}

	for _, peak := range []float64{math.Inf(-1), -1, 0, math.NaN(), math.Inf(1)} {
		if got := c.SpectralGateGain(peak); got != 0 {
			t.Errorf("invalid peak %g produced %g", peak, got)
		}
	}

	if got := (DAVSoftKneeCompressorGain{}).SpectralGateGain(.25); got != 0 {
		t.Errorf("uninitialized transfer produced %g", got)
	}
}
