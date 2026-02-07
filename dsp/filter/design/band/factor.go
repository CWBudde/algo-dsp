package band

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// splitFOSection factors a fourth-order digital section (5-tap numerator b and
// denominator a) into two cascaded biquad sections. It finds the roots of both
// polynomials, pairs them as conjugates, and reconstructs two second-order sections.
// The leading coefficient b[0] is applied as gain to the first biquad.
func splitFOSection(b, a [5]float64) ([]biquad.Coefficients, error) {
	if a[0] == 0 || b[0] == 0 {
		return nil, ErrInvalidParams
	}

	// Find roots of the numerator and denominator polynomials
	// by converting to descending-order form and solving.
	numRoots, err := rootsFromPolyAsc(b)
	if err != nil {
		return nil, err
	}
	denRoots, err := rootsFromPolyAsc(a)
	if err != nil {
		return nil, err
	}

	// Group the four roots into two conjugate pairs each,
	// which will become the two biquad sections.
	numPairs, err := pairConjugates(numRoots)
	if err != nil {
		return nil, err
	}
	denPairs, err := pairConjugates(denRoots)
	if err != nil {
		return nil, err
	}

	// Reconstruct each biquad from its conjugate root pair.
	// The original leading coefficient is folded into the first section.
	sections := make([]biquad.Coefficients, 2)
	scale := b[0]
	for i := 0; i < 2; i++ {
		b0, b1, b2, err := quadFromRoots(numPairs[i])
		if err != nil {
			return nil, err
		}
		a0, a1, a2, err := quadFromRoots(denPairs[i])
		if err != nil {
			return nil, err
		}
		if i == 0 {
			b0 *= scale
			b1 *= scale
			b2 *= scale
		}
		if a0 == 0 {
			return nil, ErrInvalidParams
		}
		// Normalize by a0 so the denominator leading coefficient is 1.
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

// rootsFromPolyAsc finds the roots of a polynomial given in ascending power order
// (c[0] + c[1]*z + c[2]*z^2 + ...). It reverses the coefficients to descending
// order for the Durand-Kerner solver, then inverts the roots to map from the
// reciprocal polynomial back to the original variable.
func rootsFromPolyAsc(c [5]float64) ([]complex128, error) {
	// Reverse to descending power order: c[4]*z^4 + c[3]*z^3 + ...
	coeff := []complex128{
		complex(c[4], 0),
		complex(c[3], 0),
		complex(c[2], 0),
		complex(c[1], 0),
		complex(c[0], 0),
	}
	roots, err := polyRootsDurandKerner(coeff)
	if err != nil {
		return nil, err
	}

	// Invert each root to undo the coefficient reversal:
	// if r is a root of the reversed polynomial, 1/r is a root of the original.
	out := make([]complex128, len(roots))
	for i, x := range roots {
		if x == 0 {
			return nil, ErrInvalidParams
		}
		out[i] = 1 / x
	}
	return out, nil
}

// quadFromRoots expands a conjugate root pair into monic second-order polynomial
// coefficients. Given roots (a+jb) and (a-jb), it returns the coefficients of
// z^2 - 2a*z + (a^2 + b^2) as (b0=1, b1=-2a, b2=a^2+b^2).
func quadFromRoots(pair [2]complex128) (float64, float64, float64, error) {
	r1 := pair[0]
	r2 := pair[1]
	if !isConjugate(r1, r2, conjugateTol) {
		return 0, 0, 0, ErrInvalidParams
	}

	a := real(r1)
	b := math.Abs(imag(r1))

	// (z - (a+jb)) (z - (a-jb)) = z^2 - 2a z + (a^2 + b^2)
	b0 := 1.0
	b1 := -2 * a
	b2 := a*a + b*b
	return b0, b1, b2, nil
}

// pairConjugates groups a slice of complex roots into conjugate pairs.
// For each unused root, it finds the closest match to the expected conjugate
// and validates the pairing within conjugateTol. Returns an error if any
// root cannot be paired (indicating the polynomial has non-conjugate roots).
func pairConjugates(roots []complex128) ([][2]complex128, error) {
	used := make([]bool, len(roots))
	pairs := make([][2]complex128, 0, len(roots)/2)

	for i := range roots {
		if used[i] {
			continue
		}
		// For each unmatched root, search for its conjugate among remaining roots.
		r := roots[i]
		conj := complex(real(r), -imag(r))
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
		// Validate that the closest candidate is actually a conjugate within tolerance.
		if best == -1 || !isConjugate(r, roots[best], conjugateTol) {
			return nil, ErrInvalidParams
		}
		used[i] = true
		used[best] = true
		pairs = append(pairs, [2]complex128{r, roots[best]})
	}
	return pairs, nil
}

// polyRootsDurandKerner finds all roots of a polynomial using the Durand-Kerner
// (Weierstrass) simultaneous iteration method. Coefficients are in descending
// power order: coeff[0]*z^n + coeff[1]*z^(n-1) + ... + coeff[n].
// Returns an error if the polynomial is degenerate or convergence fails.
func polyRootsDurandKerner(coeff []complex128) ([]complex128, error) {
	if len(coeff) < 2 {
		return nil, ErrInvalidParams
	}
	lead := coeff[0]
	if lead == 0 {
		return nil, ErrInvalidParams
	}

	// Normalize to monic form (leading coefficient = 1) for numerical stability.
	n := len(coeff) - 1
	norm := make([]complex128, len(coeff))
	for i := range coeff {
		norm[i] = coeff[i] / lead
	}

	// Cauchy bound: all roots lie within |z| <= max(1, max|a_i|).
	// Use it to set the initial guess radius.
	radius := 0.0
	for i := 1; i <= n; i++ {
		if r := cmplx.Abs(norm[i]); r > radius {
			radius = r
		}
	}
	if radius < 1 {
		radius = 1
	}

	// Spread initial guesses on a circle with slight asymmetry to break
	// symmetry for palindromic polynomials.
	roots := make([]complex128, n)
	for i := 0; i < n; i++ {
		angle := 2*math.Pi*float64(i)/float64(n) + 0.3
		r := radius * (1 + 0.1*float64(i)/float64(n))
		roots[i] = complex(r*math.Cos(angle), r*math.Sin(angle))
	}

	// Iterate: each root is updated by subtracting p(z_i) / prod(z_i - z_j).
	// Converges when the maximum correction falls below tolerance.
	const maxIter = 500
	const tol = 1e-12
	for iter := 0; iter < maxIter; iter++ {
		maxDelta := 0.0
		for i := 0; i < n; i++ {
			// Compute the product denominator: prod_{j!=i} (z_i - z_j).
			den := complex(1, 0)
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				den *= roots[i] - roots[j]
			}
			if cmplx.Abs(den) == 0 {
				// Perturb to escape collision.
				roots[i] += complex(1e-10, 1e-10)
				continue
			}
			// Apply the Durand-Kerner correction step.
			f := polyEval(norm, roots[i])
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

	// Convergence not reached by delta alone.
	// Accept if all residuals are small relative to the polynomial's scale.
	maxResidual := 0.0
	for _, r := range roots {
		res := cmplx.Abs(polyEval(norm, r))
		if res > maxResidual {
			maxResidual = res
		}
	}
	if maxResidual < 1e-6 {
		return roots, nil
	}
	return nil, ErrInvalidParams
}

// polyEval evaluates a polynomial at x using Horner's method.
// Coefficients are in descending power order: coeff[0]*x^n + ... + coeff[n].
func polyEval(coeff []complex128, x complex128) complex128 {
	v := coeff[0]
	for i := 1; i < len(coeff); i++ {
		v = v*x + coeff[i]
	}
	return v
}

// conjugateTol is the relative tolerance for determining whether two complex
// numbers form a conjugate pair (same real part, negated imaginary part).
const conjugateTol = 1e-7

// isConjugate checks whether a and b are complex conjugates within the given
// relative tolerance. It compares real parts for equality and imaginary parts
// for sign-flipped equality, both scaled by the magnitude to handle large values.
func isConjugate(a, b complex128, tol float64) bool {
	if math.Abs(real(a)-real(b)) > tol*math.Max(1, math.Abs(real(a))) {
		return false
	}
	if math.Abs(imag(a)+imag(b)) > tol*math.Max(1, math.Abs(imag(a))) {
		return false
	}
	return true
}
