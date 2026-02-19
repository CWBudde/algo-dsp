package biquad

import (
	"math/cmplx"
	"testing"
)

func TestCoefficientsPoleZeroPair_SecondOrder(t *testing.T) {
	p1 := complex(0.72, 0.19)
	p2 := cmplx.Conj(p1)
	z1 := complex(0.31, 0.44)
	z2 := cmplx.Conj(z1)

	b0 := 2.3
	c := Coefficients{
		B0: b0,
		B1: -b0 * real(z1+z2),
		B2: b0 * real(z1*z2),
		A1: -real(p1 + p2),
		A2: real(p1 * p2),
	}

	pair := c.PoleZeroPair()
	if !unorderedRootsClose(pair.Poles, p1, p2, 1e-12) {
		t.Fatalf("unexpected poles: got=%v want={%v,%v}", pair.Poles, p1, p2)
	}
	if !unorderedRootsClose(pair.Zeros, z1, z2, 1e-12) {
		t.Fatalf("unexpected zeros: got=%v want={%v,%v}", pair.Zeros, z1, z2)
	}
}

func TestCoefficientsPoleZeroPair_FirstOrder(t *testing.T) {
	c := Coefficients{
		B0: 1.0,
		B1: -0.3,
		B2: 0.0,
		A1: -0.8,
		A2: 0.0,
	}

	pair := c.PoleZeroPair()
	if !unorderedRootsClose(pair.Poles, complex(0.8, 0), complex(0, 0), 1e-12) {
		t.Fatalf("unexpected first-order poles: %v", pair.Poles)
	}
	if !unorderedRootsClose(pair.Zeros, complex(0.3, 0), complex(0, 0), 1e-12) {
		t.Fatalf("unexpected first-order zeros: %v", pair.Zeros)
	}
}

func TestPoleZeroPairs_ChainAndSliceAgree(t *testing.T) {
	coeffs := []Coefficients{
		{B0: 1, B1: -0.4, B2: 0.1, A1: -1.2, A2: 0.45},
		{B0: 0.9, B1: 0.2, B2: 0.05, A1: -0.3, A2: 0.08},
	}

	fromSlice := PoleZeroPairs(coeffs)
	fromChain := NewChain(coeffs).PoleZeroPairs()

	if len(fromSlice) != len(coeffs) {
		t.Fatalf("slice pair count=%d, want=%d", len(fromSlice), len(coeffs))
	}
	if len(fromChain) != len(coeffs) {
		t.Fatalf("chain pair count=%d, want=%d", len(fromChain), len(coeffs))
	}

	for i := range coeffs {
		if !sameRootSet(fromSlice[i].Poles, fromChain[i].Poles, 1e-12) {
			t.Fatalf("section %d poles differ: slice=%v chain=%v", i, fromSlice[i].Poles, fromChain[i].Poles)
		}
		if !sameRootSet(fromSlice[i].Zeros, fromChain[i].Zeros, 1e-12) {
			t.Fatalf("section %d zeros differ: slice=%v chain=%v", i, fromSlice[i].Zeros, fromChain[i].Zeros)
		}
	}
}

func unorderedRootsClose(got [2]complex128, want1, want2 complex128, tol float64) bool {
	return (rootsClose(got[0], want1, tol) && rootsClose(got[1], want2, tol)) ||
		(rootsClose(got[0], want2, tol) && rootsClose(got[1], want1, tol))
}

func sameRootSet(a, b [2]complex128, tol float64) bool {
	return unorderedRootsClose(a, b[0], b[1], tol)
}

func rootsClose(a, b complex128, tol float64) bool {
	return cmplx.Abs(a-b) <= tol
}
