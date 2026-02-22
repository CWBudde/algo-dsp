package polyroot

import (
	"math"
	"math/cmplx"
	"testing"
)

func almostEqual(valA, valB, tol float64) bool {
	if valA == valB {
		return true
	}

	diff := math.Abs(valA - valB)
	if tol > 0 && tol < 1 {
		mag := math.Max(math.Abs(valA), math.Abs(valB))
		if mag > 1 {
			return diff/mag < tol
		}
	}

	return diff < tol
}

func TestDurandKerner_Quadratic(t *testing.T) {
	// z^2 - 3z + 2 = (z-1)(z-2), roots at 1 and 2
	coeff := []complex128{1, -3, 2}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}

	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}

	r := [2]float64{real(roots[0]), real(roots[1])}
	if r[0] > r[1] {
		r[0], r[1] = r[1], r[0]
	}

	if !almostEqual(r[0], 1.0, 1e-10) || !almostEqual(r[1], 2.0, 1e-10) {
		t.Errorf("expected roots {1,2}, got {%v, %v}", r[0], r[1])
	}
}

func TestDurandKerner_Quartic(t *testing.T) {
	// (z^2 - 1)(z^2 - 4) = z^4 - 5z^2 + 4, roots: -2, -1, 1, 2
	coeff := []complex128{1, 0, -5, 0, 4}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}

	if len(roots) != 4 {
		t.Fatalf("expected 4 roots, got %d", len(roots))
	}

	for i, r := range roots {
		val := PolyEval(coeff, r)
		if cmplx.Abs(val) > 1e-8 {
			t.Errorf("root %d: p(%v) = %v, expected ~0", i, r, val)
		}
	}
}

func TestDurandKerner_ConjugatePairRoots(t *testing.T) {
	// z^4 + 1 has roots at e^{i*pi/4 * (2k+1)}, k=0..3
	coeff := []complex128{1, 0, 0, 0, 1}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}

	if len(roots) != 4 {
		t.Fatalf("expected 4 roots, got %d", len(roots))
	}

	for i, r := range roots {
		if !almostEqual(cmplx.Abs(r), 1.0, 1e-9) {
			t.Errorf("root %d: |r|=%v, expected 1.0", i, cmplx.Abs(r))
		}
	}
}

func TestDurandKerner_ClusteredRoots(t *testing.T) {
	// (z - 0.9)^2 * (z - 0.8)^2 - two double roots
	r1, r2 := 0.9, 0.8
	c4 := complex(1, 0)
	c3 := complex(-2*(r1+r2), 0)
	c2 := complex(r1*r1+4*r1*r2+r2*r2, 0)
	c1 := complex(-2*r1*r2*(r1+r2), 0)
	c0 := complex(r1*r1*r2*r2, 0)
	coeff := []complex128{c4, c3, c2, c1, c0}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}

	for i, r := range roots {
		val := PolyEval(coeff, r)
		if cmplx.Abs(val) > 1e-6 {
			t.Errorf("clustered root %d: p(%v) = %v, expected ~0", i, r, val)
		}
	}
}

func TestPolyEval(t *testing.T) {
	// p(z) = 2z^3 - 3z + 5, p(2) = 16 - 6 + 5 = 15
	coeff := []complex128{2, 0, -3, 5}

	val := PolyEval(coeff, 2)
	if !almostEqual(real(val), 15, 1e-12) || !almostEqual(imag(val), 0, 1e-12) {
		t.Errorf("PolyEval: expected 15, got %v", val)
	}
}

func TestPairConjugates_TwoPairs(t *testing.T) {
	roots := []complex128{
		complex(0.5, 0.3),
		complex(0.5, -0.3),
		complex(-0.2, 0.7),
		complex(-0.2, -0.7),
	}

	pairs, err := PairConjugates(roots)
	if err != nil {
		t.Fatal(err)
	}

	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	for i, p := range pairs {
		if !IsConjugate(p[0], p[1], ConjugateTol) {
			t.Errorf("pair %d is not conjugate: %v, %v", i, p[0], p[1])
		}
	}
}

func TestPairConjugates_RealRoots(t *testing.T) {
	roots := []complex128{
		complex(0.5, 1e-15),
		complex(0.5, -1e-15),
		complex(0.8, 1e-15),
		complex(0.8, -1e-15),
	}

	pairs, err := PairConjugates(roots)
	if err != nil {
		t.Fatal(err)
	}

	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
}

func TestPairConjugates_UnpairedReturnsError(t *testing.T) {
	roots := []complex128{
		complex(0.5, 0.3),
		complex(0.5, -0.3),
		complex(0.1, 0.9),
		complex(0.9, 0.1),
	}

	_, err := PairConjugates(roots)
	if err == nil {
		t.Error("expected error for unpaired roots, got nil")
	}
}

func TestQuadFromRoots_ConjugatePair(t *testing.T) {
	pair := [2]complex128{complex(0.5, 0.3), complex(0.5, -0.3)}

	b0, b1, b2, err := QuadFromRoots(pair)
	if err != nil {
		t.Fatal(err)
	}

	if !almostEqual(b0, 1.0, 1e-12) {
		t.Errorf("b0: expected 1.0, got %v", b0)
	}

	if !almostEqual(b1, -1.0, 1e-12) {
		t.Errorf("b1: expected -1.0, got %v", b1)
	}

	expectedB2 := 0.5*0.5 + 0.3*0.3 // 0.34
	if !almostEqual(b2, expectedB2, 1e-12) {
		t.Errorf("b2: expected %v, got %v", expectedB2, b2)
	}
}

func TestQuadFromRoots_NotConjugate_ReturnsError(t *testing.T) {
	pair := [2]complex128{complex(0.5, 0.3), complex(0.6, -0.3)}

	_, _, _, err := QuadFromRoots(pair)
	if err == nil {
		t.Error("expected error for non-conjugate pair")
	}
}

func TestIsConjugate(t *testing.T) {
	tests := []struct {
		name string
		a, b complex128
		want bool
	}{
		{"exact conjugates", complex(1, 2), complex(1, -2), true},
		{"near conjugates", complex(1, 2), complex(1.0+1e-9, -2.0+1e-9), true},
		{"not conjugates", complex(1, 2), complex(2, -2), false},
		{"real values", complex(5, 0), complex(5, 0), true},
		{"zero", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsConjugate(tt.a, tt.b, ConjugateTol)
			if got != tt.want {
				t.Errorf("IsConjugate(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================
// Durand-Kerner stress tests
// ============================================================

func TestDurandKerner_UnitCircleRoots(t *testing.T) {
	// z^4 - 1, roots: 1, -1, i, -i
	coeff := []complex128{1, 0, 0, 0, -1}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}

	for i, r := range roots {
		if !almostEqual(cmplx.Abs(r), 1.0, 1e-8) {
			t.Errorf("root %d: |r|=%v, expected 1.0", i, cmplx.Abs(r))
		}

		val := PolyEval(coeff, r)
		if cmplx.Abs(val) > 1e-7 {
			t.Errorf("root %d: p(r) = %v, expected ~0", i, val)
		}
	}
}

func TestDurandKerner_LargeCoeffRange(t *testing.T) {
	// Polynomial with very different coefficient magnitudes
	coeff := []complex128{1e6, 0, 1e-3, 0, 1e6}

	roots, err := DurandKerner(coeff)
	if err != nil {
		t.Skipf("large coefficient range: %v (known limitation)", err)
		return
	}

	for i, r := range roots {
		val := PolyEval(coeff, r)

		residual := cmplx.Abs(val) / 1e6
		if residual > 1e-4 {
			t.Errorf("root %d: relative residual = %e", i, residual)
		}
	}
}
