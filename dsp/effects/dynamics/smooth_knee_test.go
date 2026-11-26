package dynamics

import (
	"math"
	"testing"
)

func TestSmoothKneeCurve(t *testing.T) {
	for _, tc := range []struct {
		ratio, knee, inputDB float64
	}{
		{1, 0, 0},
		{4, 0, -24},
		{4, 0, 0},
		{4, 6, -36},
		{4, 6, -12},
		{4, 6, 0},
		{0.5, 3, -30},
		{0.01, 0, -18},
		{100, 10, 0},
	} {
		const threshold = -12

		var detector SmoothKneeDetector
		detector.Configure(48000, SmoothKneeSettings{
			AttackMS: 0.01, ReleaseMS: 0.05, ThresholdDB: threshold,
			Ratio: tc.ratio, KneeDB: tc.knee,
		})
		peak := math.Pow(10, tc.inputDB/20)
		temp := 0.5 * (1/tc.ratio - 1) * (tc.inputDB - threshold)
		want := math.Pow(10, (temp-math.Hypot(temp, tc.knee))/20)

		var got float64
		for range 24 {
			got = detector.Gain(peak)
		}

		if math.Abs(got-want) > max(0.0005*want, 2e-6) {
			t.Errorf("ratio %g knee %g input %g dB: gain %g, want %g",
				tc.ratio, tc.knee, tc.inputDB, got, want)
		}
	}
}

func TestSmoothKneeDetectorHalfLifeAndMakeup(t *testing.T) {
	for _, rate := range []float64{44100, 48000, 96000} {
		var detector SmoothKneeDetector

		settings := SmoothKneeSettings{
			AttackMS: 10, ReleaseMS: 20, ThresholdDB: -18, Ratio: 3,
			KneeDB: 4, MakeupDB: 6, AutoMakeup: true,
		}
		detector.Configure(rate, settings)

		for range int(rate / 100) {
			detector.Gain(1)
		}

		// The third-order exponent approximation slightly shifts the half-life.
		if math.Abs(detector.peak-0.5) > 0.001 {
			t.Errorf("rate %g: peak after attack half-life %g", rate, detector.peak)
		}

		for range int(rate / 50) {
			detector.Gain(0)
		}

		if math.Abs(detector.peak-0.25) > 0.002 {
			t.Errorf("rate %g: peak after release half-life %g", rate, detector.peak)
		}

		atZero := -settings.ThresholdDB * 0.5 * (1/settings.Ratio - 1)

		want := math.Pow(10, (math.Hypot(atZero, settings.KneeDB)-atZero)/20)
		if got := detector.MakeupGain(); math.Abs(got-want) > 2e-5 {
			t.Errorf("auto makeup %g, want %g", got, want)
		}

		settings.AutoMakeup = false
		detector.Configure(rate, settings)

		if got := detector.MakeupGain(); math.Abs(got-math.Pow(10, 0.3)) > 1e-14 {
			t.Errorf("manual makeup %g", got)
		}

		if detector.peak < 0.248 {
			t.Errorf("automation discarded detector history at rate %g", rate)
		}
	}
}

func TestSmoothKneeInstantAttack(t *testing.T) {
	var detector SmoothKneeDetector
	detector.Configure(48000, SmoothKneeSettings{ThresholdDB: -12, Ratio: 4})

	want := math.Pow(10, -0.75*12/20)
	if got := detector.Gain(1); math.Abs(got-want) > 0.0005*want {
		t.Fatalf("zero attack time: gain %g, want %g", got, want)
	}
}

func TestSmoothKneeSilenceAndClip(t *testing.T) {
	var detector SmoothKneeDetector
	detector.Configure(44100, SmoothKneeSettings{
		AttackMS: 0.01, ReleaseMS: 0.05, Ratio: 0.01, ThresholdDB: 10,
	})

	if got := detector.Gain(0); got != 0 {
		t.Fatalf("silent expanded key: got %g, want zero", got)
	}

	for _, x := range []float32{-1000, -3, -1, -0.1, 0, 0.1, 1, 3, 1000} {
		if got := SmoothKneeSoftClip(x); math.Abs(float64(got)-math.Tanh(float64(x))) > 3e-5 {
			t.Errorf("clip(%g) = %g, want %g", x, got, math.Tanh(float64(x)))
		}
	}
}
