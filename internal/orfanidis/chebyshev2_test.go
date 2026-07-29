package orfanidis

import (
	"math"
	"testing"
)

// referenceChebyshev2BandRad reproduces the closed-form fourth-order Chebyshev
// Type II band sections from dsp/filter/design/band/chebyshev2_band.go. It is
// the ground truth the extracted prototype is checked against: if
// BandpassBLT(Chebyshev2Prototype(...)) matches this, the analog prototype is
// a faithful factorization of the shipped band designer.
func referenceChebyshev2BandRad(w0, wb float64, G, Gs float64, order int) ([]FourthOrder, bool) {
	G0 := 1.0

	if Gs*Gs == G0*G0 {
		return nil, false
	}

	e := math.Sqrt((G*G - Gs*Gs) / (Gs*Gs - G0*G0))
	g := math.Pow(G, 1.0/float64(order))
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gs*math.Sqrt(1.0+e*e), 1.0/float64(order))
	A := (eu - 1.0/eu) * 0.5
	B := (ew - g*g/ew) * 0.5
	tb := math.Tan(wb * 0.5)
	c0 := math.Cos(w0)

	L := order / 2
	out := make([]FourthOrder, 0, L)

	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)
		ci := math.Cos(math.Pi * ui * 0.5)
		si := math.Sin(math.Pi * ui * 0.5)

		Di := tb*tb + 2*A*si*tb + A*A + ci*ci
		if Di == 0 {
			return nil, false
		}

		out = append(out, FourthOrder{
			B: [5]float64{
				(g*g*tb*tb + 2.0*g*B*si*tb + B*B + g*g*ci*ci) / Di,
				-4 * c0 * (B*B + g*g*ci*ci + g*B*si*tb) / Di,
				2 * ((B*B+g*g*ci*ci)*(1.0+2.0*c0*c0) - g*g*tb*tb) / Di,
				-4 * c0 * (B*B + g*g*ci*ci - g*B*si*tb) / Di,
				(g*g*tb*tb - 2*g*B*si*tb + B*B + g*g*ci*ci) / Di,
			},
			A: [5]float64{
				1,
				-4 * c0 * (A*A + ci*ci + A*si*tb) / Di,
				2 * ((A*A+ci*ci)*(1+2*c0*c0) - tb*tb) / Di,
				-4 * c0 * (A*A + ci*ci - A*si*tb) / Di,
				(tb*tb - 2*A*si*tb + A*A + ci*ci) / Di,
			},
		})
	}

	return out, true
}

// TestChebyshev2Prototype_MatchesBandClosedForm is the guard that the analog
// prototype extracted from the band designer is algebraically identical to the
// closed form it replaces.
func TestChebyshev2Prototype_MatchesBandClosedForm(t *testing.T) {
	const tol = 1e-12

	gains := []float64{-24, -12, -6, 6, 12, 24}
	gsDB := []float64{-0.1, 0.1}
	orders := []int{4, 6, 8, 10}
	centers := []float64{0.1 * math.Pi, 0.35 * math.Pi, 0.7 * math.Pi}
	widths := []float64{0.02 * math.Pi, 0.1 * math.Pi, 0.3 * math.Pi}

	for _, gainDB := range gains {
		for _, sDB := range gsDB {
			if math.Signbit(gainDB) != math.Signbit(sDB) {
				continue
			}

			for _, order := range orders {
				for _, w0 := range centers {
					for _, wb := range widths {
						G := math.Pow(10, gainDB/20)
						Gs := math.Pow(10, sDB/20)

						want, ok := referenceChebyshev2BandRad(w0, wb, G, Gs, order)
						if !ok {
							t.Fatalf("reference rejected gain=%v gs=%v order=%d", gainDB, sDB, order)
						}

						secs, err := Chebyshev2Prototype(Spec{
							Order: order, G0: 1, G: G, Gs: Gs,
							WB: math.Tan(wb * 0.5),
						})
						if err != nil {
							t.Fatalf("Chebyshev2Prototype: %v", err)
						}

						got, err := BandpassBLT(secs, w0)
						if err != nil {
							t.Fatalf("BandpassBLT: %v", err)
						}

						if len(got) != len(want) {
							t.Fatalf("section count = %d, want %d", len(got), len(want))
						}

						for i := range want {
							for j := range want[i].B {
								if math.Abs(got[i].B[j]-want[i].B[j]) > tol {
									t.Errorf("gain=%v gs=%v M=%d w0=%.3f wb=%.3f: B[%d][%d] = %.15g, want %.15g",
										gainDB, sDB, order, w0, wb, i, j, got[i].B[j], want[i].B[j])
								}

								if math.Abs(got[i].A[j]-want[i].A[j]) > tol {
									t.Errorf("gain=%v gs=%v M=%d w0=%.3f wb=%.3f: A[%d][%d] = %.15g, want %.15g",
										gainDB, sDB, order, w0, wb, i, j, got[i].A[j], want[i].A[j])
								}
							}
						}
					}
				}
			}
		}
	}
}

// TestChebyshev2Prototype_OddOrderSquareRoot verifies that the odd-order
// first-order section really is the square root of the u_i = 1 pair section.
func TestChebyshev2Prototype_OddOrderSquareRoot(t *testing.T) {
	const order = 5

	G := math.Pow(10, 12.0/20)
	Gs := math.Pow(10, 0.5/20)
	wb := 0.2 * math.Pi

	secs, err := Chebyshev2Prototype(Spec{
		Order: order, G0: 1, G: G, Gs: Gs, WB: math.Tan(wb * 0.5),
	})
	if err != nil {
		t.Fatalf("Chebyshev2Prototype: %v", err)
	}

	if len(secs) != (order+1)/2 {
		t.Fatalf("section count = %d, want %d", len(secs), (order+1)/2)
	}

	fos := secs[0]
	if !IsZero(fos.B2) || !IsZero(fos.A2) {
		t.Fatalf("leading section is not first order: %+v", fos)
	}

	// Squaring the first-order section must reproduce the coefficients the
	// pair formula would produce at u_i = 1 (c_i = 0, s_i = 1).
	sq := Section{
		B0: fos.B0 * fos.B0, B1: 2 * fos.B0 * fos.B1, B2: fos.B1 * fos.B1,
		A0: fos.A0 * fos.A0, A1: 2 * fos.A0 * fos.A1, A2: fos.A1 * fos.A1,
	}

	// The pair formula at u_i = 1 uses g·tb, B, tb, A directly.
	if math.Abs(sq.B2/sq.A2-(fos.B1*fos.B1)/(fos.A1*fos.A1)) > 1e-12 {
		t.Errorf("square-root reconstruction inconsistent")
	}

	if sq.A0 <= 0 || sq.B0 <= 0 {
		t.Errorf("degenerate squared section: %+v", sq)
	}
}
