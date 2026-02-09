package bank

import (
	"math"
	"testing"
)

func TestAnalyzer_PeakNearTone(t *testing.T) {
	const sr = 48000
	an, err := NewOctaveAnalyzer(3, sr, WithAnalyzerFrequencyRange(100, 10000), WithAnalyzerEnvelopeHz(200))
	if err != nil {
		t.Fatalf("NewOctaveAnalyzer error: %v", err)
	}

	freq := 1000.0
	n := 4096
	input := make([]float64, n)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}
	peaks := an.ProcessBlock(input)
	if len(peaks) == 0 {
		t.Fatal("expected non-empty peaks")
	}

	bands := an.Bands()
	if len(bands) != len(peaks) {
		t.Fatalf("bands/peaks mismatch: %d != %d", len(bands), len(peaks))
	}

	maxIdx := 0
	for i := 1; i < len(peaks); i++ {
		if peaks[i] > peaks[maxIdx] {
			maxIdx = i
		}
	}

	center := bands[maxIdx].CenterHz
	if center < 800 || center > 1250 {
		t.Fatalf("peak band center %.1f Hz not near tone %.1f Hz", center, freq)
	}
}
