package orfanidis

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const tol = 1e-6

func TestPeaking_InvalidParams(t *testing.T) {
	if _, err := Peaking(0, 1, 1, 1, 1, 1); err == nil {
		m := "expected error for G0 <= 0"
		t.Fatal(m)
	}
	if _, err := Peaking(1, 1, 1, 1, 0, 1); err == nil {
		t.Fatal("expected error for w0 <= 0")
	}
	if _, err := Peaking(1, 1, 1, 1, math.Pi, 1); err == nil {
		t.Fatal("expected error for w0 >= pi")
	}
	if _, err := Peaking(1, 1, 1, 1, 1, 0); err == nil {
		t.Fatal("expected error for dw <= 0")
	}
	if _, err := Peaking(1, 1, 1, 1, 1, math.Pi); err == nil {
		t.Fatal("expected error for dw >= pi")
	}
}

func TestPeakingFromFreqQGain_InvalidParams(t *testing.T) {
	if _, err := PeakingFromFreqQGain(0, 1000, 0.707, 6); err == nil {
		t.Fatal("expected error for invalid sample rate")
	}
	if _, err := PeakingFromFreqQGain(48000, 0, 0.707, 6); err == nil {
		t.Fatal("expected error for invalid frequency")
	}
	if _, err := PeakingFromFreqQGain(48000, 1000, 0, 6); err == nil {
		t.Fatal("expected error for invalid Q")
	}
}

func TestPeakingFromFreqQGain_ResponseSanity(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 6.0

	c, err := PeakingFromFreqQGain(sr, f0, q, gainDB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestPeakingCascade_ResponseAtF0(t *testing.T) {
	sr := 48000.0
	f0 := 1000.0
	q := 0.707
	gainDB := 9.0
	sections := 3

	coeffs, err := PeakingCascade(sr, f0, q, gainDB, sections)
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

func TestPeakingCascade_InvalidParams(t *testing.T) {
	if _, err := PeakingCascade(48000, 1000, 0.707, 6, 0); err == nil {
		t.Fatal("expected error for sections <= 0")
	}
}

func assertStableSection(t *testing.T, c biquad.Coefficients) {
	t.Helper()
	r1, r2 := sectionRoots(c)
	if cmplx.Abs(r1) >= 1+tol || cmplx.Abs(r2) >= 1+tol {
		t.Fatalf("unstable poles: |r1|=%v |r2|=%v coeff=%#v", cmplx.Abs(r1), cmplx.Abs(r2), c)
	}
}

func sectionRoots(c biquad.Coefficients) (complex128, complex128) {
	disc := complex(c.A1*c.A1-4*c.A2, 0)
	sqrtDisc := cmplx.Sqrt(disc)
	r1 := (-complex(c.A1, 0) + sqrtDisc) / 2
	r2 := (-complex(c.A1, 0) - sqrtDisc) / 2
	return r1, r2
}
