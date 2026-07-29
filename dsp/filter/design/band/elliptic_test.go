package band

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/orfanidis"
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
	sections := []orfanidis.Section{{B0: 2.5, A0: 1}}
	w0 := 2 * math.Pi * 1000 / testSR

	fo, err := orfanidis.BandpassBLT(sections, w0)
	if err != nil {
		t.Fatalf("BandpassBLT: %v", err)
	}

	if len(fo) != 1 {
		t.Fatalf("expected 1 section, got %d", len(fo))
	}

	if !almostEqual(fo[0].B[0], 2.5, 1e-12) {
		t.Errorf("gain section b[0] = %v, expected 2.5", fo[0].B[0])
	}

	if !almostEqual(fo[0].A[0], 1.0, 1e-12) {
		t.Errorf("gain section a[0] = %v, expected 1.0", fo[0].A[0])
	}
}

func TestBlt_AllSectionsProcessed(t *testing.T) {
	sections := make([]orfanidis.Section, 5)
	for i := range sections {
		v := float64(i + 1)
		sections[i] = orfanidis.Section{
			B0: v, B1: v * 0.1, B2: v * 0.01,
			A0: 1, A1: 0.2 * v, A2: 0.03 * v,
		}
	}

	w0 := 2 * math.Pi * 1000 / testSR

	fo, err := orfanidis.BandpassBLT(sections, w0)
	if err != nil {
		t.Fatalf("BandpassBLT: %v", err)
	}

	if len(fo) != 5 {
		t.Fatalf("expected 5 output sections, got %d", len(fo))
	}

	for i, s := range fo {
		allZero := true

		for j := range 5 {
			if !isZero(s.B[j]) || !isZero(s.A[j]) {
				allZero = false
				break
			}
		}

		if allZero {
			t.Errorf("section %d has all-zero coefficients; blt did not process it", i)
		}
	}
}
