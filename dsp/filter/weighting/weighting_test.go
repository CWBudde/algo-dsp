package weighting

import (
	"math"
	"testing"
)

// IEC 61672 Table 3: A-weighting relative response levels.
var aWeightingRef = []struct {
	freq float64
	dB   float64
}{
	{10, -70.4},
	{12.5, -63.4},
	{16, -56.7},
	{20, -50.5},
	{25, -44.7},
	{31.5, -39.4},
	{40, -34.6},
	{50, -30.2},
	{63, -26.2},
	{80, -22.5},
	{100, -19.1},
	{125, -16.1},
	{160, -13.4},
	{200, -10.9},
	{250, -8.6},
	{315, -6.6},
	{400, -4.8},
	{500, -3.2},
	{630, -1.9},
	{800, -0.8},
	{1000, 0.0},
	{1250, 0.6},
	{1600, 1.0},
	{2000, 1.2},
	{2500, 1.3},
	{3150, 1.2},
	{4000, 1.0},
	{5000, 0.5},
	{6300, -0.1},
	{8000, -1.1},
	{10000, -2.5},
	{12500, -4.3},
	{16000, -6.6},
	{20000, -9.3},
}

// B-weighting relative response levels.
// Computed from the canonical analog transfer function:
//
//	H_B(s) = K_B * s^3 / ((s+ω1)^2 * (s+ω3) * (s+ω5)^2)
//
// B-weighting shares the double LP pole at f5=12194 Hz with C-weighting,
// so HF rolloff is similar. Values above 5 kHz differ from some published
// tables that use a non-standard single-pole variant.
var bWeightingRef = []struct {
	freq float64
	dB   float64
}{
	{10, -38.2},
	{12.5, -33.2},
	{16, -28.5},
	{20, -24.2},
	{25, -20.4},
	{31.5, -17.1},
	{40, -14.2},
	{50, -11.6},
	{63, -9.3},
	{80, -7.4},
	{100, -5.6},
	{125, -4.2},
	{160, -3.0},
	{200, -2.0},
	{250, -1.3},
	{315, -0.8},
	{400, -0.5},
	{500, -0.3},
	{630, -0.1},
	{800, 0.0},
	{1000, 0.0},
	{1250, 0.0},
	{1600, 0.0},
	{2000, -0.1},
	{2500, -0.3},
	{3150, -0.5},
	{4000, -0.8},
	{5000, -1.2},
	{6300, -1.9},
	{8000, -2.9},
	{10000, -4.3},
	{12500, -6.1},
	{16000, -8.5},
	{20000, -11.2},
}

// IEC 61672: C-weighting relative response levels.
var cWeightingRef = []struct {
	freq float64
	dB   float64
}{
	{10, -14.3},
	{12.5, -11.2},
	{16, -8.5},
	{20, -6.2},
	{25, -4.4},
	{31.5, -3.0},
	{40, -2.0},
	{50, -1.3},
	{63, -0.8},
	{80, -0.5},
	{100, -0.3},
	{125, -0.2},
	{160, -0.1},
	{200, 0.0},
	{250, 0.0},
	{315, 0.0},
	{400, 0.0},
	{500, 0.0},
	{630, 0.0},
	{800, 0.0},
	{1000, 0.0},
	{1250, 0.0},
	{1600, -0.1},
	{2000, -0.2},
	{2500, -0.3},
	{3150, -0.5},
	{4000, -0.8},
	{5000, -1.3},
	{6300, -2.0},
	{8000, -3.0},
	{10000, -4.4},
	{12500, -6.2},
	{16000, -8.5},
	{20000, -11.2},
}

// bltTolerance returns the acceptable deviation between the analog reference
// value and the bilinear-transformed digital filter at a given frequency
// and sample rate. The bilinear transform compresses frequencies near Nyquist,
// causing increasing deviation. At sr >= 96 kHz the error is negligible
// across the audio band.
//
// The base tolerance of 0.5 dB covers both the analog-to-digital conversion
// error and the ±0.05 dB rounding in the IEC 61672 reference table values.
func bltTolerance(freq, sr float64) float64 {
	ratio := freq / sr
	switch {
	case ratio > 0.4: // > 80% of Nyquist
		return 25.0
	case ratio > 0.3: // 60-80% of Nyquist
		return 5.0
	case ratio > 0.2: // 40-60% of Nyquist
		return 1.5
	case ratio > 0.1: // 20-40% of Nyquist
		return 1.0
	default: // < 20% of Nyquist
		return 0.5
	}
}

func TestAWeighting_IEC61672(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000} {
		chain := New(TypeA, sr)
		for _, ref := range aWeightingRef {
			if ref.freq >= sr/2 {
				continue
			}
			got := chain.MagnitudeDB(ref.freq, sr)
			diff := math.Abs(got - ref.dB)
			tol := bltTolerance(ref.freq, sr)
			if diff > tol {
				t.Errorf("A-weighting @ %g Hz (sr=%g): got %.2f dB, want %.1f dB (diff %.2f, tol %.1f)",
					ref.freq, sr, got, ref.dB, diff, tol)
			}
		}
	}
}

func TestBWeighting_IEC61672(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000} {
		chain := New(TypeB, sr)
		for _, ref := range bWeightingRef {
			if ref.freq >= sr/2 {
				continue
			}
			got := chain.MagnitudeDB(ref.freq, sr)
			diff := math.Abs(got - ref.dB)
			tol := bltTolerance(ref.freq, sr)
			if diff > tol {
				t.Errorf("B-weighting @ %g Hz (sr=%g): got %.2f dB, want %.1f dB (diff %.2f, tol %.1f)",
					ref.freq, sr, got, ref.dB, diff, tol)
			}
		}
	}
}

func TestCWeighting_IEC61672(t *testing.T) {
	for _, sr := range []float64{44100, 48000, 96000} {
		chain := New(TypeC, sr)
		for _, ref := range cWeightingRef {
			if ref.freq >= sr/2 {
				continue
			}
			got := chain.MagnitudeDB(ref.freq, sr)
			diff := math.Abs(got - ref.dB)
			tol := bltTolerance(ref.freq, sr)
			if diff > tol {
				t.Errorf("C-weighting @ %g Hz (sr=%g): got %.2f dB, want %.1f dB (diff %.2f, tol %.1f)",
					ref.freq, sr, got, ref.dB, diff, tol)
			}
		}
	}
}

func TestZWeighting_Unity(t *testing.T) {
	chain := New(TypeZ, 48000)
	for _, freq := range []float64{100, 1000, 10000, 20000} {
		got := chain.MagnitudeDB(freq, 48000)
		if math.Abs(got) > 1e-10 {
			t.Errorf("Z-weighting @ %g Hz: got %.6f dB, want 0 dB", freq, got)
		}
	}
}

func TestWeighting_1kHzNormalization(t *testing.T) {
	for _, typ := range []Type{TypeA, TypeB, TypeC, TypeZ} {
		chain := New(typ, 48000)
		got := chain.MagnitudeDB(1000, 48000)
		if math.Abs(got) > 0.01 {
			t.Errorf("%s-weighting: 1 kHz magnitude = %.4f dB, want 0 dB", typ, got)
		}
	}
}

func TestWeighting_ProcessSample(t *testing.T) {
	chain := New(TypeA, 48000)
	// Feed a 1 kHz sine and verify non-zero output.
	var maxOut float64
	for i := range 4800 {
		x := math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
		y := chain.ProcessSample(x)
		if a := math.Abs(y); a > maxOut {
			maxOut = a
		}
	}
	if maxOut < 0.5 {
		t.Errorf("A-weighting 1 kHz sine: max output %.4f, expected near 1.0", maxOut)
	}
}

func TestWeighting_ProcessBlock(t *testing.T) {
	chain := New(TypeC, 48000)
	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / 48000)
	}
	chain.ProcessBlock(buf)
	// After processing, the block should not be all zeros.
	var sum float64
	for _, v := range buf {
		sum += v * v
	}
	if sum < 1e-10 {
		t.Error("ProcessBlock output is all zeros")
	}
}

func TestWeighting_Reset(t *testing.T) {
	chain := New(TypeA, 48000)
	// Process some samples to build up state.
	for range 100 {
		chain.ProcessSample(1.0)
	}
	chain.Reset()
	// After reset, processing zero should yield zero.
	y := chain.ProcessSample(0)
	if y != 0 {
		t.Errorf("after Reset, ProcessSample(0) = %g, want 0", y)
	}
}

func TestWeighting_String(t *testing.T) {
	tests := []struct {
		typ  Type
		want string
	}{
		{TypeA, "A"},
		{TypeB, "B"},
		{TypeC, "C"},
		{TypeZ, "Z"},
		{Type(99), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.want {
			t.Errorf("Type(%d).String() = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestWeighting_PanicOnInvalidSampleRate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-positive sample rate")
		}
	}()
	New(TypeA, 0)
}

func TestWeighting_PanicOnUnknownType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown type")
		}
	}()
	New(Type(99), 48000)
}

func TestWeighting_OrderAndSections(t *testing.T) {
	tests := []struct {
		typ      Type
		sections int
		order    int
	}{
		{TypeA, 5, 10}, // 1 second-order HP + 2 first-order LP + 2 first-order HP = 5 sections
		{TypeB, 4, 8},  // 1 second-order HP + 2 first-order LP + 1 first-order HP = 4 sections
		{TypeC, 3, 6},  // 1 second-order HP + 2 first-order LP = 3 sections
		{TypeZ, 1, 2}, // 1 unity section
	}
	for _, tt := range tests {
		chain := New(tt.typ, 48000)
		if got := chain.NumSections(); got != tt.sections {
			t.Errorf("%s-weighting: NumSections() = %d, want %d", tt.typ, got, tt.sections)
		}
		if got := chain.Order(); got != tt.order {
			t.Errorf("%s-weighting: Order() = %d, want %d", tt.typ, got, tt.order)
		}
	}
}
