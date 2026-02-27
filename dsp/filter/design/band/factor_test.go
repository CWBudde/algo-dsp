package band

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/internal/polyroot"
)

func TestSplitFOSection_KnownFactorization(t *testing.T) {
	// Build a 4th-order polynomial from two known biquads and verify round-trip.
	nb := [3]float64{1, 0.5, 0.2}
	na := [3]float64{1, -0.3, 0.1}
	mb := [3]float64{1, -0.4, 0.3}
	ma := [3]float64{1, 0.2, 0.05}

	var B, A [5]float64

	for i := range 3 {
		for j := range 3 {
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

			numRoots, err := polyroot.DurandKerner(numCoeff)
			if err != nil {
				t.Logf("  NUM root-finding FAILED: %v", err)
			} else {
				t.Logf("  NUM roots found:")

				for i, r := range numRoots {
					res := polyroot.PolyEval(numCoeff, r)
					t.Logf("    root[%d] = (%.10f, %.10f)  |r|=%.10f  residual=%.2e",
						i, real(r), imag(r), cmplx.Abs(r), cmplx.Abs(res))
				}
			}

			denRoots, err := polyroot.DurandKerner(denCoeff)
			if err != nil {
				t.Logf("  DEN root-finding FAILED: %v", err)
			} else {
				t.Logf("  DEN roots found:")

				for i, r := range denRoots {
					res := polyroot.PolyEval(denCoeff, r)
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
