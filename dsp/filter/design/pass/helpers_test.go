package pass

import (
	"math"
	"math/cmplx"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

const tol = 1e-9

func assertFiniteCoefficients(t *testing.T, c biquad.Coefficients) {
	t.Helper()
	v := []float64{c.B0, c.B1, c.B2, c.A1, c.A2}
	for i := range v {
		if math.IsNaN(v[i]) || math.IsInf(v[i], 0) {
			t.Fatalf("invalid coefficient[%d]=%v", i, v[i])
		}
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

func chainForTest(sections []biquad.Coefficients) *biquad.Chain {
	return biquad.NewChain(sections)
}
