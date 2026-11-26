package dynamics

import (
	"math"
	"testing"
)

func TestSimpleFeedbackTapPrecedesMakeup(t *testing.T) {
	settings := SimpleFeedbackSettings{
		AttackMS: 0.0625, ReleaseMS: 40, ThresholdDB: -40, Ratio: 4, AutoMakeup: true,
	}

	var compressor SimpleFeedbackCompressor
	compressor.Configure(48000, settings)

	compressionThreshold := 0.01
	makeup := math.Pow(compressionThreshold, 0.25-1)

	first := compressor.ProcessSample(1)
	if math.Abs(first-makeup) > 1e-13 {
		t.Fatalf("first sample %g, want instantaneous %g", first, makeup)
	}

	previousCompressed := 1.0
	sampleCount := 3.0 * settings.Ratio
	coefficient := 1 - math.Pow(0.5, 1/sampleCount)
	peak := previousCompressed * coefficient

	want := makeup * compressionThreshold / peak * (compressionThreshold / peak) * (compressionThreshold / peak)
	if got := compressor.ProcessSample(-1); math.Abs(got+want) > 1e-13 {
		t.Fatalf("second sample %g, want %g using the pre-makeup tap", got, -want)
	}
}

func TestSimpleFeedbackSteadyStateAndSilentTail(t *testing.T) {
	const rate = 48000

	settings := SimpleFeedbackSettings{
		AttackMS: 5, ReleaseMS: 40, ThresholdDB: -18, Ratio: 4, AutoMakeup: true,
	}

	var compressor SimpleFeedbackCompressor
	compressor.Configure(rate, settings)

	const level = -0.5

	var got float64
	for range rate {
		got = compressor.ProcessSample(level)
	}

	want := -math.Pow(math.Abs(level), 1/settings.Ratio)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("steady-state result %g, want %g", got, want)
	}

	peak := compressor.peak
	if got := compressor.ProcessSample(0); got != 0 {
		t.Fatalf("first silent sample %g", got)
	}

	for range rate {
		if got := compressor.ProcessSample(0); got != 0 {
			t.Fatalf("silent tail sample %g", got)
		}
	}

	wantPeak := peak * math.Pow(0.5, 1/(0.001*settings.ReleaseMS*settings.Ratio))
	if math.Abs(compressor.peak-wantPeak) > 1e-12 {
		t.Fatalf("released peak %g, want %g", compressor.peak, wantPeak)
	}

	settings.AutoMakeup = false
	compressor.Configure(rate, settings)

	if compressor.peak <= 0 {
		t.Fatal("setting a parameter reset the feedback history")
	}

	for range rate {
		got = compressor.ProcessSample(level)
	}

	want *= math.Pow(10, settings.ThresholdDB*0.05*(1-1/settings.Ratio))
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("steady-state result with makeup disabled %g, want %g", got, want)
	}
}

func TestSimpleFeedbackExtremeThresholdAndRatio(t *testing.T) {
	var compressor SimpleFeedbackCompressor
	compressor.Configure(96000, SimpleFeedbackSettings{
		AttackMS: 0.03125, ReleaseMS: 5, ThresholdDB: -96, Ratio: 100,
	})

	const aboveThreshold = 1.01

	compressor.peak = compressor.threshold * aboveThreshold
	compressor.previous = compressor.peak

	input := compressor.peak
	got := compressor.ProcessSample(input)

	want := input * math.Pow(aboveThreshold, -99)
	if math.IsNaN(got) || math.Abs(got-want) > 1e-17 {
		t.Fatalf("gain at minimum threshold and max ratio = %g, want %g", got, want)
	}
}
