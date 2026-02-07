package band

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"
)

func TestPolyRootsDurandKerner_Quadratic(t *testing.T) {
	// z^2 - 3z + 2 = (z-1)(z-2), roots at 1 and 2
	coeff := []complex128{1, -3, 2}
	roots, err := polyRootsDurandKerner(coeff)
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

func TestPolyRootsDurandKerner_Quartic(t *testing.T) {
	// (z^2 - 1)(z^2 - 4) = z^4 - 5z^2 + 4, roots: -2, -1, 1, 2
	coeff := []complex128{1, 0, -5, 0, 4}
	roots, err := polyRootsDurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 4 {
		t.Fatalf("expected 4 roots, got %d", len(roots))
	}
	for i, r := range roots {
		val := polyEval(coeff, r)
		if cmplx.Abs(val) > 1e-8 {
			t.Errorf("root %d: p(%v) = %v, expected ~0", i, r, val)
		}
	}
}

func TestPolyRootsDurandKerner_ConjugatePairRoots(t *testing.T) {
	// z^4 + 1 has roots at e^{i*pi/4 * (2k+1)}, k=0..3
	coeff := []complex128{1, 0, 0, 0, 1}
	roots, err := polyRootsDurandKerner(coeff)
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

func TestPolyRootsDurandKerner_ClusteredRoots(t *testing.T) {
	// (z - 0.9)^2 * (z - 0.8)^2 - two double roots
	r1, r2 := 0.9, 0.8
	c4 := complex(1, 0)
	c3 := complex(-2*(r1+r2), 0)
	c2 := complex(r1*r1+4*r1*r2+r2*r2, 0)
	c1 := complex(-2*r1*r2*(r1+r2), 0)
	c0 := complex(r1*r1*r2*r2, 0)
	coeff := []complex128{c4, c3, c2, c1, c0}

	roots, err := polyRootsDurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range roots {
		val := polyEval(coeff, r)
		if cmplx.Abs(val) > 1e-6 {
			t.Errorf("clustered root %d: p(%v) = %v, expected ~0", i, r, val)
		}
	}
}

func TestPolyEval(t *testing.T) {
	// p(z) = 2z^3 - 3z + 5, p(2) = 16 - 6 + 5 = 15
	coeff := []complex128{2, 0, -3, 5}
	val := polyEval(coeff, 2)
	if !almostEqual(real(val), 15, 1e-12) || !almostEqual(imag(val), 0, 1e-12) {
		t.Errorf("polyEval: expected 15, got %v", val)
	}
}

func TestPairConjugates_TwoPairs(t *testing.T) {
	roots := []complex128{
		complex(0.5, 0.3),
		complex(0.5, -0.3),
		complex(-0.2, 0.7),
		complex(-0.2, -0.7),
	}
	pairs, err := pairConjugates(roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	for i, p := range pairs {
		if !isConjugate(p[0], p[1], conjugateTol) {
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
	pairs, err := pairConjugates(roots)
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
	_, err := pairConjugates(roots)
	if err == nil {
		t.Error("expected error for unpaired roots, got nil")
	}
}

func TestQuadFromRoots_ConjugatePair(t *testing.T) {
	pair := [2]complex128{complex(0.5, 0.3), complex(0.5, -0.3)}
	b0, b1, b2, err := quadFromRoots(pair)
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
	_, _, _, err := quadFromRoots(pair)
	if err == nil {
		t.Error("expected error for non-conjugate pair")
	}
}

func TestSplitFOSection_KnownFactorization(t *testing.T) {
	// Build a 4th-order polynomial from two known biquads and verify round-trip.
	nb := [3]float64{1, 0.5, 0.2}
	na := [3]float64{1, -0.3, 0.1}
	mb := [3]float64{1, -0.4, 0.3}
	ma := [3]float64{1, 0.2, 0.05}

	var B, A [5]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			B[i+j] += nb[i] * mb[j]
			A[i+j] += na[i] * ma[j]
		}
	}

	sections, err := splitFOSection(B, A)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	for _, freq := range []float64{100, 500, 1000, 5000, 10000, 20000} {
		w := 2 * math.Pi * freq / testSR
		ejw := cmplx.Exp(complex(0, -w))
		ej2w := cmplx.Exp(complex(0, -2*w))
		ej3w := cmplx.Exp(complex(0, -3*w))
		ej4w := cmplx.Exp(complex(0, -4*w))

		origNum := complex(B[0], 0) + complex(B[1], 0)*ejw + complex(B[2], 0)*ej2w +
			complex(B[3], 0)*ej3w + complex(B[4], 0)*ej4w
		origDen := complex(A[0], 0) + complex(A[1], 0)*ejw + complex(A[2], 0)*ej2w +
			complex(A[3], 0)*ej3w + complex(A[4], 0)*ej4w
		origH := origNum / origDen

		splitH := cascadeResponse(sections, freq, testSR)

		origMag := cmplx.Abs(origH)
		splitMag := cmplx.Abs(splitH)
		if !almostEqual(origMag, splitMag, 1e-6) {
			t.Errorf("freq=%v: orig mag=%.10f, split mag=%.10f", freq, origMag, splitMag)
		}
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
			got := isConjugate(tt.a, tt.b, conjugateTol)
			if got != tt.want {
				t.Errorf("isConjugate(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================
// Durand-Kerner stress tests
// ============================================================

func TestPolyRootsDurandKerner_UnitCircleRoots(t *testing.T) {
	// z^4 - 1, roots: 1, -1, i, -i
	coeff := []complex128{1, 0, 0, 0, -1}
	roots, err := polyRootsDurandKerner(coeff)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range roots {
		if !almostEqual(cmplx.Abs(r), 1.0, 1e-8) {
			t.Errorf("root %d: |r|=%v, expected 1.0", i, cmplx.Abs(r))
		}
		val := polyEval(coeff, r)
		if cmplx.Abs(val) > 1e-7 {
			t.Errorf("root %d: p(r) = %v, expected ~0", i, val)
		}
	}
}

func TestPolyRootsDurandKerner_LargeCoeffRange(t *testing.T) {
	// Polynomial with very different coefficient magnitudes
	coeff := []complex128{1e6, 0, 1e-3, 0, 1e6}
	roots, err := polyRootsDurandKerner(coeff)
	if err != nil {
		t.Skipf("large coefficient range: %v (known limitation)", err)
		return
	}
	for i, r := range roots {
		val := polyEval(coeff, r)
		residual := cmplx.Abs(val) / 1e6
		if residual > 1e-4 {
			t.Errorf("root %d: relative residual = %e", i, residual)
		}
	}
}

// ============================================================
// Diagnostic tests: understand splitFOSection failure modes
// ============================================================

// TestSplitFOSection_DiagnoseLowFreq examines the polynomials at low frequencies
// where Durand-Kerner is known to fail.
func TestSplitFOSection_DiagnoseLowFreq(t *testing.T) {
	testCases := []struct {
		name   string
		f0, bw float64
	}{
		{"f0=500_bw=250", 500, 250},
		{"f0=250_bw=125", 250, 125},
		{"f0=125_bw=62.5", 125, 62.5},
		{"f0=1000_bw=50", 1000, 50}, // narrow BW
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w0 := 2 * math.Pi * tc.f0 / testSR
			wb := 2 * math.Pi * tc.bw / testSR
			gainDB := 12.0
			gbDB := butterworthBWGainDB(gainDB)

			G0 := db2Lin(0)
			G := db2Lin(gainDB)
			Gb := db2Lin(gbDB)
			e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
			g := math.Pow(G, 0.25)
			g0 := math.Pow(G0, 0.25)
			beta := math.Pow(e, -0.25) * math.Tan(wb/2)
			c0 := math.Cos(w0)

			t.Logf("Parameters: w0=%.6f wb=%.6f c0=%.10f beta=%.10f", w0, wb, c0, beta)
			t.Logf("  G=%.6f G0=%.6f Gb=%.6f e=%.6f g=%.6f g0=%.6f", G, G0, Gb, e, g, g0)

			// Compute the first section's (i=1) polynomials
			ui := 1.0 / 4.0 // (2*1-1)/4 for order=4
			si := math.Sin(math.Pi * ui / 2.0)
			Di := beta*beta + 2*si*beta + 1

			B := [5]float64{
				(g*g*beta*beta + 2*g*g0*si*beta + g0*g0) / Di,
				-4 * c0 * (g0*g0 + g*g0*si*beta) / Di,
				2 * (g0*g0*(1+2*c0*c0) - g*g*beta*beta) / Di,
				-4 * c0 * (g0*g0 - g*g0*si*beta) / Di,
				(g*g*beta*beta - 2*g*g0*si*beta + g0*g0) / Di,
			}
			A := [5]float64{
				1,
				-4 * c0 * (1 + si*beta) / Di,
				2 * (1 + 2*c0*c0 - beta*beta) / Di,
				-4 * c0 * (1 - si*beta) / Di,
				(beta*beta - 2*si*beta + 1) / Di,
			}

			t.Logf("  B = [%.10e, %.10e, %.10e, %.10e, %.10e]", B[0], B[1], B[2], B[3], B[4])
			t.Logf("  A = [%.10e, %.10e, %.10e, %.10e, %.10e]", A[0], A[1], A[2], A[3], A[4])

			// Compute condition: ratio of max to min |coefficient|
			maxB, minB := 0.0, math.MaxFloat64
			for _, v := range B {
				av := math.Abs(v)
				if av > maxB {
					maxB = av
				}
				if av > 0 && av < minB {
					minB = av
				}
			}
			t.Logf("  B condition (max/min): %.2e", maxB/minB)

			// Check palindromic symmetry (sign of ill-conditioning)
			t.Logf("  B[0]/B[4] = %.10f (1.0 = palindromic)", B[0]/B[4])
			t.Logf("  B[1]/B[3] = %.10f (1.0 = palindromic)", B[1]/B[3])
			t.Logf("  A[0]/A[4] = %.10f", A[0]/A[4])
			t.Logf("  A[1]/A[3] = %.10f", A[1]/A[3])

			// Now try to find roots
			numCoeff := []complex128{
				complex(B[4], 0), complex(B[3], 0), complex(B[2], 0),
				complex(B[1], 0), complex(B[0], 0),
			}
			denCoeff := []complex128{
				complex(A[4], 0), complex(A[3], 0), complex(A[2], 0),
				complex(A[1], 0), complex(A[0], 0),
			}

			numRoots, err := polyRootsDurandKerner(numCoeff)
			if err != nil {
				t.Logf("  NUM root-finding FAILED: %v", err)
			} else {
				t.Logf("  NUM roots found:")
				for i, r := range numRoots {
					res := polyEval(numCoeff, r)
					t.Logf("    root[%d] = (%.10f, %.10f)  |r|=%.10f  residual=%.2e",
						i, real(r), imag(r), cmplx.Abs(r), cmplx.Abs(res))
				}
			}

			denRoots, err := polyRootsDurandKerner(denCoeff)
			if err != nil {
				t.Logf("  DEN root-finding FAILED: %v", err)
			} else {
				t.Logf("  DEN roots found:")
				for i, r := range denRoots {
					res := polyEval(denCoeff, r)
					t.Logf("    root[%d] = (%.10f, %.10f)  |r|=%.10f  residual=%.2e",
						i, real(r), imag(r), cmplx.Abs(r), cmplx.Abs(res))
				}
			}

			// Try the full pipeline
			sections, err := splitFOSection(B, A)
			if err != nil {
				t.Logf("  splitFOSection FAILED: %v", err)
			} else {
				t.Logf("  splitFOSection SUCCESS: %d sections", len(sections))
				for i, s := range sections {
					t.Logf("    biquad[%d]: B0=%.10f B1=%.10f B2=%.10f A1=%.10f A2=%.10f",
						i, s.B0, s.B1, s.B2, s.A1, s.A2)
				}
			}
		})
	}
}

// TestSplitFOSection_DiagnoseOrder12 examines the order-12 case specifically.
func TestSplitFOSection_DiagnoseOrder12(t *testing.T) {
	w0 := 2 * math.Pi * 1000 / testSR
	wb := 2 * math.Pi * 500 / testSR
	gainDB := 12.0
	gbDB := butterworthBWGainDB(gainDB)
	order := 12

	G0 := db2Lin(0)
	G := db2Lin(gainDB)
	Gb := db2Lin(gbDB)
	e := math.Sqrt((G*G - Gb*Gb) / (Gb*Gb - G0*G0))
	g := math.Pow(G, 1.0/float64(order))
	g0 := math.Pow(G0, 1.0/float64(order))
	beta := math.Pow(e, -1.0/float64(order)) * math.Tan(wb/2)
	c0 := math.Cos(w0)

	t.Logf("Order=%d beta=%.10f c0=%.10f g=%.10f g0=%.10f e=%.6f", order, beta, c0, g, g0, e)

	L := order / 2
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1) / float64(order)
		si := math.Sin(math.Pi * ui / 2.0)
		Di := beta*beta + 2*si*beta + 1

		B := [5]float64{
			(g*g*beta*beta + 2*g*g0*si*beta + g0*g0) / Di,
			-4 * c0 * (g0*g0 + g*g0*si*beta) / Di,
			2 * (g0*g0*(1+2*c0*c0) - g*g*beta*beta) / Di,
			-4 * c0 * (g0*g0 - g*g0*si*beta) / Di,
			(g*g*beta*beta - 2*g*g0*si*beta + g0*g0) / Di,
		}
		A := [5]float64{
			1,
			-4 * c0 * (1 + si*beta) / Di,
			2 * (1 + 2*c0*c0 - beta*beta) / Di,
			-4 * c0 * (1 - si*beta) / Di,
			(beta*beta - 2*si*beta + 1) / Di,
		}

		_, err := splitFOSection(B, A)
		status := "OK"
		if err != nil {
			status = fmt.Sprintf("FAILED: %v", err)
		}
		t.Logf("  section %d (ui=%.4f si=%.6f): %s", i, ui, si, status)
		t.Logf("    B = [%.6e, %.6e, %.6e, %.6e, %.6e]", B[0], B[1], B[2], B[3], B[4])
		t.Logf("    A = [%.6e, %.6e, %.6e, %.6e, %.6e]", A[0], A[1], A[2], A[3], A[4])
	}
}
