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

func TestConvenienceWrappersMatchProcess(t *testing.T) {
	input := sine(1000, 48000, 257)

	tests := []struct {
		name     string
		up, down int
		process  func([]float64, ...Option) ([]float64, error)
	}{
		{"up", 2, 1, Upsample2x},
		{"down", 1, 2, Downsample2x},
		{"rational", 3, 2, func(input []float64, opts ...Option) ([]float64, error) {
			return Resample(input, 3, 2, opts...)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := []Option{WithQuality(QualityBest), WithCutoffScale(0.8), WithKaiserBeta(6)}
			r := mustResampler(t, tc.up, tc.down, opts...)

			got, err := tc.process(input, opts...)
			if err != nil {
				t.Fatal(err)
			}

			checkOutputBits(t, got, r.Process(input))
		})
	}

	if _, err := Resample(input, 0, 1); err != ErrInvalidRatio {
		t.Fatalf("invalid ratio error = %v", err)
	}
	// These deliberately invalid design parameters must propagate errors
	// through the allocating convenience APIs, not return partial samples.
	invalid := WithCutoffScale(math.SmallestNonzeroFloat64)
	if _, err := Upsample2x(input, invalid); err == nil {
		t.Fatal("Upsample2x accepted invalid filter")
	}

	if _, err := Downsample2x(input, invalid); err == nil {
		t.Fatal("Downsample2x accepted invalid filter")
	}
}

func TestOptionsAndEmptyCompatibility(t *testing.T) {
	r := mustResampler(t, 3, 2, nil, WithQuality(QualityFast),
		WithTapsPerPhase(12), WithCutoffScale(0.8), WithKaiserBeta(4))
	if r.Quality() != QualityFast || r.TapsPerPhase() != 12 {
		t.Fatalf("quality/taps = %v/%d", r.Quality(), r.TapsPerPhase())
	}

	if got := r.Process(nil); got != nil {
		t.Fatalf("empty Process = %v, want nil", got)
	}

	if (&Resampler{}).TapsPerPhase() != 0 {
		t.Fatal("empty resampler reports nonzero taps")
	}

	for _, rates := range [][2]float64{{0, 48000}, {48000, -1}, {math.NaN(), 48000}, {48000, math.NaN()}} {
		if _, err := NewForRates(rates[0], rates[1]); err != ErrInvalidRate {
			t.Fatalf("invalid rates %v error = %v", rates, err)
		}
	}

	approx, err := NewForRates(44100, 48000, WithMaxDenominator(8))
	if err != nil {
		t.Fatal(err)
	}

	if _, down := approx.Ratio(); down > 8 {
		t.Fatalf("denominator = %d, exceeds configured cap", down)
	}
	// Out-of-range option values must retain the documented defaults.
	defaultStream := mustResampler(t, 3, 2)
	ignored := mustResampler(t, 3, 2, WithTapsPerPhase(0), WithCutoffScale(2),
		WithKaiserBeta(-1), WithMaxDenominator(0))
	checkOutputBits(t, ignored.Prototype(), defaultStream.Prototype())
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

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
