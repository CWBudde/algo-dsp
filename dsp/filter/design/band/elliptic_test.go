package band

import (
	"math"
	"testing"
)

func TestIsZero(t *testing.T) {
	if !isZero(0) {
		t.Error("isZero(0) should be true")
	}

	if !isZero(1e-13) {
		t.Error("isZero(1e-13) should be true")
	}

	if isZero(1e-11) {
		t.Error("isZero(1e-11) should be false")
	}
}

func TestBlt_GainOnlySection(t *testing.T) {
	sections := []soSection{{b0: 2.5, a0: 1, b1: 0, b2: 0, a1: 0, a2: 0}}
	w0 := 2 * math.Pi * 1000 / testSR

	fo := blt(sections, w0)
	if len(fo) != 1 {
		t.Fatalf("expected 1 section, got %d", len(fo))
	}

	if !almostEqual(fo[0].b[0], 2.5, 1e-12) {
		t.Errorf("gain section b[0] = %v, expected 2.5", fo[0].b[0])
	}

	if !almostEqual(fo[0].a[0], 1.0, 1e-12) {
		t.Errorf("gain section a[0] = %v, expected 1.0", fo[0].a[0])
	}
}

func TestBlt_AllSectionsProcessed(t *testing.T) {
	sections := make([]soSection, 5)
	for i := range sections {
		v := float64(i + 1)
		sections[i] = soSection{
			b0: v, b1: v * 0.1, b2: v * 0.01,
			a0: 1, a1: 0.2 * v, a2: 0.03 * v,
		}
	}

	w0 := 2 * math.Pi * 1000 / testSR

	fo := blt(sections, w0)
	if len(fo) != 5 {
		t.Fatalf("expected 5 output sections, got %d", len(fo))
	}

	for i, s := range fo {
		allZero := true

		for j := range 5 {
			if !isZero(s.b[j]) || !isZero(s.a[j]) {
				allZero = false
				break
			}
		}

		if allZero {
			t.Errorf("section %d has all-zero coefficients; blt did not process it", i)
		}
	}
}
