package design

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func TestPeakRaw_InvalidParams(t *testing.T) {
	_, err := PeakRaw(0, 1, 1, 1, 1, 1)
	if err == nil {
		t.Fatal("expected error for G0 <= 0")
	}

	_, err = PeakRaw(1, 1, 1, 1, 0, 1)
	if err == nil {
		t.Fatal("expected error for w0 <= 0")
	}

	_, err = PeakRaw(1, 1, 1, 1, math.Pi, 1)
	if err == nil {
		t.Fatal("expected error for w0 >= pi")
	}

	_, err = PeakRaw(1, 1, 1, 1, 1, 0)
	if err == nil {
		t.Fatal("expected error for dw <= 0")
	}

	_, err = PeakRaw(1, 1, 1, 1, 1, math.Pi)
	if err == nil {
		t.Fatal("expected error for dw >= pi")
	}
}

func TestPeak_WithOrfanidisOptions_ResponseSanity(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 6.0

	c := Peak(f0, gainDB, q, sr, WithDCGain(1.0), WithNyquistGain(1.0))
	if c == (biquad.Coefficients{}) {
		t.Fatal("expected non-zero coefficients")
	}

	G := math.Pow(10, gainDB/20.0)

	magAtF0 := cmplx.Abs(c.Response(f0, sr))
	if math.Abs(magAtF0-G) > 1e-2 {
		t.Fatalf("peak magnitude=%.6f, want %.6f", magAtF0, G)
	}

	magDC := cmplx.Abs(c.Response(0, sr))
	if math.Abs(magDC-1) > 1e-2 {
		t.Fatalf("DC magnitude=%.6f, want ~1", magDC)
	}

	magNyq := cmplx.Abs(c.Response(sr/2, sr))
	if math.Abs(magNyq-1) > 1e-2 {
		t.Fatalf("Nyquist magnitude=%.6f, want ~1", magNyq)
	}

	assertStableSection(t, c)
}

func TestPeak_WithoutOptions_MatchesRBJ(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 6.0

	withOpts := Peak(f0, gainDB, q, sr)
	rbj := peakRBJ(f0, gainDB, q, sr)

	if !almostEqual(withOpts.B0, rbj.B0, 1e-12) ||
		!almostEqual(withOpts.B1, rbj.B1, 1e-12) ||
		!almostEqual(withOpts.B2, rbj.B2, 1e-12) ||
		!almostEqual(withOpts.A1, rbj.A1, 1e-12) ||
		!almostEqual(withOpts.A2, rbj.A2, 1e-12) {
		t.Fatalf("Peak() without options should match RBJ\ngot:  %+v\nwant: %+v", withOpts, rbj)
	}
}

func TestPeakCascade_ResponseAtF0(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 9.0
	sections := 3

	coeffs, err := PeakCascade(sr, f0, q, gainDB, sections, WithDCGain(1.0), WithNyquistGain(1.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain := biquad.NewChain(coeffs)

	G := math.Pow(10, gainDB/20.0)

	magAtF0 := cmplx.Abs(chain.Response(f0, sr))
	if math.Abs(magAtF0-G) > 2e-2 {
		t.Fatalf("cascade magnitude=%.6f, want %.6f", magAtF0, G)
	}
}

func TestPeakCascade_InvalidParams(t *testing.T) {
	_, err := PeakCascade(48000, 1000, 0.707, 6, 0)
	if err == nil {
		t.Fatal("expected error for sections <= 0")
	}
}

func TestPeakCascade_WithoutOptions(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 9.0

	coeffs, err := PeakCascade(sr, f0, q, gainDB, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(coeffs) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(coeffs))
	}

	for _, c := range coeffs {
		assertStableSection(t, c)
	}
}
