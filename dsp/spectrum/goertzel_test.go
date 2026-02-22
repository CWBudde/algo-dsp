package spectrum

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func TestGoertzel_Basic(t *testing.T) {
	fs := 48000.0
	f0 := 1000.0
	length := 1024
	sig := testutil.DeterministicSine(f0, fs, 1.0, length)

	g, err := NewGoertzel(f0, fs)
	if err != nil {
		t.Fatalf("NewGoertzel: %v", err)
	}

	g.ProcessBlock(sig)
	p := g.Power()

	// Compare with a direct DFT calculation at that exact frequency.
	var dft complex128
	for n, x := range sig {
		angle := -2 * math.Pi * f0 / fs * float64(n)
		dft += complex(x, 0) * cmplx.Exp(complex(0, angle))
	}
	wantP := real(dft)*real(dft) + imag(dft)*imag(dft)

	// Use a relative tolerance for power as it can grow large
	if math.Abs(p-wantP) > 1e-7*wantP {
		t.Errorf("Power mismatch: got %v, want %v (diff %v)", p, wantP, math.Abs(p-wantP))
	}

	mag := g.Magnitude()
	wantMag := cmplx.Abs(dft)
	if math.Abs(mag-wantMag) > 1e-7*wantMag {
		t.Errorf("Magnitude mismatch: got %v, want %v (diff %v)", mag, wantMag, math.Abs(mag-wantMag))
	}
}

func TestGoertzel_Reset(t *testing.T) {
	fs := 48000.0
	f0 := 1000.0
	g, _ := NewGoertzel(f0, fs)
	g.ProcessSample(1.0)
	if g.Power() == 0 {
		t.Error("Power should be non-zero after processing")
	}
	g.Reset()
	if g.Power() != 0 {
		t.Error("Power should be zero after reset")
	}
}

func TestGoertzel_Setters(t *testing.T) {
	g, _ := NewGoertzel(1000, 48000)
	if err := g.SetFrequency(2000); err != nil {
		t.Errorf("SetFrequency: %v", err)
	}
	if g.Frequency() != 2000 {
		t.Errorf("Frequency: got %v, want 2000", g.Frequency())
	}
	if err := g.SetSampleRate(44100); err != nil {
		t.Errorf("SetSampleRate: %v", err)
	}
	if g.SampleRate() != 44100 {
		t.Errorf("SampleRate: got %v, want 44100", g.SampleRate())
	}

	if err := g.SetFrequency(-1); err == nil {
		t.Error("SetFrequency should fail for negative frequency")
	}
	if err := g.SetFrequency(22051); err == nil {
		t.Error("SetFrequency should fail for frequency > fs/2")
	}
	if err := g.SetSampleRate(0); err == nil {
		t.Error("SetSampleRate should fail for 0 sample rate")
	}
}

func TestMultiGoertzel(t *testing.T) {
	fs := 48000.0
	freqs := []float64{100, 1000, 5000}
	mg, err := NewMultiGoertzel(freqs, fs)
	if err != nil {
		t.Fatalf("NewMultiGoertzel: %v", err)
	}

	sig := testutil.DeterministicSine(1000, fs, 1.0, 1024)
	mg.ProcessBlock(sig)
	powers := mg.Powers()

	if len(powers) != 3 {
		t.Fatalf("Expected 3 powers, got %d", len(powers))
	}

	// Power at 1000 Hz should be much higher than at 100 or 5000 Hz
	if powers[1] <= powers[0] || powers[1] <= powers[2] {
		t.Errorf("Expected peak at index 1, got %v", powers)
	}

	mg.Reset()
	powers = mg.Powers()
	for i, p := range powers {
		if p != 0 {
			t.Errorf("Power at index %d should be 0 after reset, got %v", i, p)
		}
	}
}

func TestGoertzel_EdgeCases(t *testing.T) {
	// DC
	g, _ := NewGoertzel(0, 48000)
	g.ProcessBlock(testutil.DC(1.0, 100))
	p := g.Power()
	// DFT sum for DC of 1.0 is 100. Power is 100^2 = 10000.
	if math.Abs(p-10000) > 1e-9 {
		t.Errorf("DC power mismatch: got %v, want 10000", p)
	}

	// Nyquist
	g, _ = NewGoertzel(24000, 48000)
	sig := make([]float64, 100)
	for i := range sig {
		if i%2 == 0 {
			sig[i] = 1.0
		} else {
			sig[i] = -1.0
		}
	}
	g.ProcessBlock(sig)
	p = g.Power()
	if math.Abs(p-10000) > 1e-9 {
		t.Errorf("Nyquist power mismatch: got %v, want 10000", p)
	}

	// dB Power
	g, _ = NewGoertzel(1000, 48000)
	if g.PowerDB() != -300 {
		t.Errorf("Expected -300 dB for zero power, got %v", g.PowerDB())
	}
}

func TestAnalyzeBlock(t *testing.T) {
	fs := 48000.0
	f0 := 1000.0
	sig := testutil.DeterministicSine(f0, fs, 1.0, 1024)
	p, err := AnalyzeBlock(sig, f0, fs)
	if err != nil {
		t.Fatalf("AnalyzeBlock: %v", err)
	}
	if p == 0 {
		t.Error("AnalyzeBlock should return non-zero power")
	}
}
