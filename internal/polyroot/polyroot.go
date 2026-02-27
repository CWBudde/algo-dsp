// Package polyroot provides polynomial root-finding and fourth-order section
// factorisation utilities shared by filter design packages.
package polyroot

import (
	"errors"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ErrDegeneratePolynomial is returned when a polynomial has degenerate
// coefficients (leading coefficient zero, convergence failure, etc.).
var ErrDegeneratePolynomial = errors.New("polyroot: degenerate polynomial")

// SplitFourthOrder factors a fourth-order digital section (5-tap numerator b
// and denominator a, both in ascending power order) into two cascaded biquad
// sections. It finds the roots of both polynomials, pairs them as conjugates,
// and reconstructs two second-order sections. The leading coefficient b[0] is
// applied as gain to the first biquad.
func SplitFourthOrder(b, a [5]float64) ([]biquad.Coefficients, error) {
	if a[0] == 0 || b[0] == 0 {
		return nil, ErrDegeneratePolynomial
	}

	numRoots, err := rootsFromPolyAsc(b)
	if err != nil {
		return nil, err
	}

	denRoots, err := rootsFromPolyAsc(a)
	if err != nil {
		return nil, err
	}

	numPairs, err := PairConjugates(numRoots)
	if err != nil {
		return nil, err
	}

	denPairs, err := PairConjugates(denRoots)
	if err != nil {
		return nil, err
	}

	sections := make([]biquad.Coefficients, 2)
	scale := b[0]

	for i := range 2 {
		b0, b1, b2, err := QuadFromRoots(numPairs[i])
		if err != nil {
			return nil, err
		}

		a0, a1, a2, err := QuadFromRoots(denPairs[i])
		if err != nil {
			return nil, err
		}

		if i == 0 {
			b0 *= scale
			b1 *= scale
			b2 *= scale
		}

		if a0 == 0 {
			return nil, ErrDegeneratePolynomial
		}

		sections[i] = biquad.Coefficients{
			B0: b0 / a0,
			B1: b1 / a0,
			B2: b2 / a0,
			A1: a1 / a0,
			A2: a2 / a0,
		}
	}

	return sections, nil
}

// rootsFromPolyAsc finds the roots of a polynomial given in ascending power
// order (c[0] + c[1]*z + c[2]*z^2 + ...). It reverses the coefficients to
// descending order for the Durand-Kerner solver, then inverts the roots to
// map from the reciprocal polynomial back to the original variable.
func rootsFromPolyAsc(c [5]float64) ([]complex128, error) {
	coeff := []complex128{
		complex(c[4], 0),
		complex(c[3], 0),
		complex(c[2], 0),
		complex(c[1], 0),
		complex(c[0], 0),
	}

	roots, err := DurandKerner(coeff)
	if err != nil {
		return nil, err
	}

	out := make([]complex128, len(roots))
	for i, x := range roots {
		if x == 0 {
			return nil, ErrDegeneratePolynomial
		}

		out[i] = 1 / x
	}

	return out, nil
}

// QuadFromRoots expands a conjugate root pair into monic second-order
// polynomial coefficients. Given roots (a+jb) and (a-jb), it returns the
// coefficients of z^2 - 2a*z + (a^2 + b^2) as (1, -2a, a^2+b^2).
func QuadFromRoots(pair [2]complex128) (float64, float64, float64, error) {
	root1 := pair[0]
	root2 := pair[1]

	if !IsConjugate(root1, root2, ConjugateTol) {
		return 0, 0, 0, ErrDegeneratePolynomial
	}

	a := real(root1)
	b := math.Abs(imag(root1))

	return 1.0, -2 * a, a*a + b*b, nil
}

// PairConjugates groups a slice of complex roots into conjugate pairs. For
// each unused root, it finds the closest match to the expected conjugate and
// validates the pairing within ConjugateTol.
func PairConjugates(roots []complex128) ([][2]complex128, error) {
	used := make([]bool, len(roots))
	pairs := make([][2]complex128, 0, len(roots)/2)

	for i := range roots {
		if used[i] {
			continue
		}

		root := roots[i]
		conj := complex(real(root), -imag(root))
		best := -1
		bestDist := math.MaxFloat64

		for j := range roots {
			if i == j || used[j] {
				continue
			}

			d := cmplx.Abs(roots[j] - conj)
			if d < bestDist {
				bestDist = d
				best = j
			}
		}

		if best == -1 || !IsConjugate(root, roots[best], ConjugateTol) {
			return nil, ErrDegeneratePolynomial
		}

		used[i] = true
		used[best] = true
		pairs = append(pairs, [2]complex128{root, roots[best]})
	}

	return pairs, nil
}

// DurandKerner finds all roots of a polynomial using the Durand-Kerner
// (Weierstrass) simultaneous iteration method. Coefficients are in descending
// power order: coeff[0]*z^n + coeff[1]*z^(n-1) + ... + coeff[n].
//
//nolint:cyclop
func DurandKerner(coeff []complex128) ([]complex128, error) {
	if len(coeff) < 2 {
		return nil, ErrDegeneratePolynomial
	}

	lead := coeff[0]
	if lead == 0 {
		return nil, ErrDegeneratePolynomial
	}

	n := len(coeff) - 1

	norm := make([]complex128, len(coeff))
	for i := range coeff {
		norm[i] = coeff[i] / lead
	}

	radius := 0.0
	for i := 1; i <= n; i++ {
		if r := cmplx.Abs(norm[i]); r > radius {
			radius = r
		}
	}

	if radius < 1 {
		radius = 1
	}

	roots := make([]complex128, n)
	for i := range n {
		angle := 2*math.Pi*float64(i)/float64(n) + 0.3
		r := radius * (1 + 0.1*float64(i)/float64(n))
		roots[i] = complex(r*math.Cos(angle), r*math.Sin(angle))
	}

	const (
		maxIter = 500
		tol     = 1e-12
	)

	for range maxIter {
		maxDelta := 0.0

		for i := range n {
			den := complex(1, 0)

			for j := range n {
				if i == j {
					continue
				}

				den *= roots[i] - roots[j]
			}

			if cmplx.Abs(den) == 0 {
				roots[i] += complex(1e-10, 1e-10)
				continue
			}

			f := PolyEval(norm, roots[i])
			delta := f / den

			roots[i] -= delta
			if d := cmplx.Abs(delta); d > maxDelta {
				maxDelta = d
			}
		}

		if maxDelta < tol {
			return roots, nil
		}
	}

	maxResidual := 0.0

	for _, r := range roots {
		res := cmplx.Abs(PolyEval(norm, r))
		if res > maxResidual {
			maxResidual = res
		}
	}

	if maxResidual < 1e-6 {
		return roots, nil
	}

	return nil, ErrDegeneratePolynomial
}

// PolyEval evaluates a polynomial at x using Horner's method. Coefficients
// are in descending power order: coeff[0]*x^n + ... + coeff[n].
func PolyEval(coeff []complex128, x complex128) complex128 {
	v := coeff[0]
	for i := 1; i < len(coeff); i++ {
		v = v*x + coeff[i]
	}

	return v
}

// ConjugateTol is the relative tolerance for conjugate pair matching.
const ConjugateTol = 1e-7

// IsConjugate checks whether a and b are complex conjugates within tolerance.
func IsConjugate(a, b complex128, tol float64) bool {
	if math.Abs(real(a)-real(b)) > tol*math.Max(1, math.Abs(real(a))) {
		return false
	}

	if math.Abs(imag(a)+imag(b)) > tol*math.Max(1, math.Abs(imag(a))) {
		return false
	}

	return true
}
