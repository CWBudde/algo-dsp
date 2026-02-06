package resample

import (
	"math"
	"testing"
)

func TestNewRationalValidation(t *testing.T) {
	if _, err := NewRational(0, 1); err == nil {
		t.Fatal("expected error for up=0")
	}
	if _, err := NewRational(1, 0); err == nil {
		t.Fatal("expected error for down=0")
	}
}

func TestRatioReduction(t *testing.T) {
	r, err := NewRational(320, 294)
	if err != nil {
		t.Fatalf("NewRational() error = %v", err)
	}
	up, down := r.Ratio()
	if up != 160 || down != 147 {
		t.Fatalf("ratio = %d/%d, want 160/147", up, down)
	}
}

func TestNewForRatesCommon(t *testing.T) {
	r, err := NewForRates(44100, 48000)
	if err != nil {
		t.Fatalf("NewForRates() error = %v", err)
	}
	up, down := r.Ratio()
	if up != 160 || down != 147 {
		t.Fatalf("ratio = %d/%d, want 160/147", up, down)
	}
}

func TestPredictOutputLenMatchesProcess(t *testing.T) {
	r, err := NewRational(3, 2)
	if err != nil {
		t.Fatalf("NewRational() error = %v", err)
	}
	in := make([]float64, 257)
	for i := range in {
		in[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
	}
	want := r.PredictOutputLen(len(in))
	got := len(r.Process(in))
	if got != want {
		t.Fatalf("len(out) = %d, want %d", got, want)
	}
}

func TestStandardRatios_Length(t *testing.T) {
	tests := []struct {
		inRate  float64
		outRate float64
	}{
		{44100, 48000},
		{48000, 44100},
		{48000, 96000},
		{96000, 48000},
	}
	for _, tc := range tests {
		r, err := NewForRates(tc.inRate, tc.outRate, WithQuality(QualityBalanced))
		if err != nil {
			t.Fatalf("NewForRates(%v,%v) error = %v", tc.inRate, tc.outRate, err)
		}
		in := make([]float64, 4096)
		for i := range in {
			in[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / tc.inRate)
		}
		out := r.Process(in)
		expected := int(math.Round(float64(len(in)) * tc.outRate / tc.inRate))
		if d := absInt(len(out) - expected); d > 1 {
			t.Fatalf("%v->%v len=%d expected~%d", tc.inRate, tc.outRate, len(out), expected)
		}
	}
}

func TestQualityModes_PassbandAndStopband(t *testing.T) {
	tests := []struct {
		name          string
		quality       Quality
		maxPassbandDB float64
		minStopbandDB float64
	}{
		{name: "fast", quality: QualityFast, maxPassbandDB: 0.7, minStopbandDB: 20},
		{name: "balanced", quality: QualityBalanced, maxPassbandDB: 0.35, minStopbandDB: 35},
		{name: "best", quality: QualityBest, maxPassbandDB: 0.2, minStopbandDB: 50},
	}

	for _, tc := range tests {
		rPass, err := NewRational(1, 2, WithQuality(tc.quality))
		if err != nil {
			t.Fatalf("%s: NewRational passband error = %v", tc.name, err)
		}
		rStop, err := NewRational(1, 2, WithQuality(tc.quality))
		if err != nil {
			t.Fatalf("%s: NewRational stopband error = %v", tc.name, err)
		}

		inPass := sine(2000, 48000, 32768)
		inStop := sine(17000, 48000, 32768)

		outPass := rPass.Process(inPass)
		outStop := rStop.Process(inStop)

		inPassRMS := rms(inPass[4096:])
		outPassRMS := rms(outPass[2048:])
		passbandDB := math.Abs(dbRatio(outPassRMS, inPassRMS))
		if passbandDB > tc.maxPassbandDB {
			t.Fatalf("%s: passband droop %.2f dB > %.2f dB", tc.name, passbandDB, tc.maxPassbandDB)
		}

		inStopRMS := rms(inStop[4096:])
		outStopRMS := rms(outStop[2048:])
		stopAttenDB := -dbRatio(outStopRMS, inStopRMS)
		if stopAttenDB < tc.minStopbandDB {
			t.Fatalf("%s: stopband attenuation %.2f dB < %.2f dB", tc.name, stopAttenDB, tc.minStopbandDB)
		}
	}
}

func TestStreamingConsistency(t *testing.T) {
	r1, err := NewRational(160, 147, WithQuality(QualityBalanced))
	if err != nil {
		t.Fatalf("NewRational() error = %v", err)
	}
	r2, err := NewRational(160, 147, WithQuality(QualityBalanced))
	if err != nil {
		t.Fatalf("NewRational() error = %v", err)
	}

	in := sine(1000, 44100, 8192)
	whole := r1.Process(in)

	var chunked []float64
	for i := 0; i < len(in); i += 257 {
		end := min(len(in), i+257)
		chunked = append(chunked, r2.Process(in[i:end])...)
	}

	if len(chunked) != len(whole) {
		t.Fatalf("chunked len=%d whole len=%d", len(chunked), len(whole))
	}
	for i := range whole {
		if diff := math.Abs(whole[i] - chunked[i]); diff > 1e-12 {
			t.Fatalf("sample %d diff=%g", i, diff)
		}
	}
}

func sine(freq, sampleRate float64, n int) []float64 {
	out := make([]float64, n)
	for i := range n {
		out[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}
	return out
}

func rms(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	var s float64
	for _, v := range x {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)))
}

func dbRatio(out, in float64) float64 {
	if in == 0 || out == 0 {
		return -300
	}
	return 20 * math.Log10(out/in)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
