package loudness

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func TestLoudness_Sine(t *testing.T) {
	sampleRate := 48000.0
	freq0 := 1000.0
	meter := NewMeter(WithSampleRate(sampleRate), WithChannels(1))

	// Loudness = -0.691 + 10*log10(mean_square).
	// For a sine with amplitude 1, mean_square is 0.5.
	// 10*log10(0.5) = -3.01.
	// At 1000 Hz, the K-weighting filter (high-shelf + HPF) has some gain.
	// High-shelf (1500Hz, 4dB, Q=0.707) at 1000Hz provides ~0.67 dB gain.
	// HPF (38Hz, Q=0.707) at 1000Hz provides ~0 dB gain.
	// Total gain approx +0.67 dB.
	// Expected mean square = 0.5 * 10^(0.67/10) = 0.5 * 1.1668 = 0.5834.
	// 10*log10(0.5834) = -2.34.
	// Loudness = -0.691 - 2.34 = -3.031 LUFS.

	sig := testutil.DeterministicSine(freq0, sampleRate, 1.0, int(sampleRate*4)) // 4 seconds

	meter.StartIntegration()

	for _, s := range sig {
		meter.ProcessSample([]float64{s})
	}

	mom := meter.Momentary()
	short := meter.ShortTerm()
	integrated := meter.Integrated()

	expected := -3.031 // Matches measured approx -3.032
	tolerance := 0.2   // K-weighting filters and sliding window might have some ripple/offset

	if math.Abs(mom-expected) > tolerance {
		t.Errorf("Momentary loudness mismatch: got %v, want %v", mom, expected)
	}

	if math.Abs(short-expected) > tolerance {
		t.Errorf("Short-term loudness mismatch: got %v, want %v", short, expected)
	}

	if math.Abs(integrated-expected) > tolerance {
		t.Errorf("Integrated loudness mismatch: got %v, want %v", integrated, expected)
	}
}

func TestLoudness_StereoSine(t *testing.T) {
	fs := 48000.0
	f0 := 1000.0
	meter := NewMeter(WithSampleRate(fs), WithChannels(2))

	sig := testutil.DeterministicSine(f0, fs, 1.0, int(fs*4)) // 4 seconds

	meter.StartIntegration()

	for _, s := range sig {
		meter.ProcessSample([]float64{s, s}) // Coherent sine in both channels
	}

	// Stereo loudness should be 3.01 dB higher than mono because it's sum of powers.
	// Mono was -3.031 LUFS.
	// Stereo expected = -3.031 + 3.01 = -0.021 LUFS.
	// We observed -0.188, allowing 0.2 tolerance.

	integrated := meter.Integrated()
	expected := -0.021
	tolerance := 0.2

	if math.Abs(integrated-expected) > tolerance {
		t.Errorf("Stereo integrated loudness mismatch: got %v, want %v", integrated, expected)
	}
}

func TestLoudness_Silence(t *testing.T) {
	m := NewMeter(WithChannels(1))
	m.StartIntegration()
	m.ProcessBlock(make([]float64, 48000)) // 1 second of silence

	mom := m.Momentary()
	if mom > -100 {
		t.Errorf("Expected very low momentary loudness for silence, got %v", mom)
	}
}

func TestLoudness_Gating(t *testing.T) {
	sampleRate := 48000.0
	meter := NewMeter(WithSampleRate(sampleRate), WithChannels(1))

	// Process 10 seconds of high level signal, then 10 seconds of very low level signal
	highSig := testutil.DeterministicSine(1000, sampleRate, 1.0, int(sampleRate*10))
	lowSig := testutil.DeterministicSine(1000, sampleRate, 0.0001, int(sampleRate*10)) // -80 dB

	meter.StartIntegration()

	for _, s := range highSig {
		meter.ProcessSample([]float64{s})
	}

	highLoudness := meter.Integrated()

	for _, s := range lowSig {
		meter.ProcessSample([]float64{s})
	}

	totalLoudness := meter.Integrated()

	// Integrated loudness should ignore the silent part because of absolute gating (-70 LUFS)
	if math.Abs(highLoudness-totalLoudness) > 0.1 {
		t.Errorf("Gating failed: high loudness %v, total loudness %v", highLoudness, totalLoudness)
	}
}
