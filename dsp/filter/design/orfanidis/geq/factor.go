package geq

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

func splitFOSection(b, a [5]float64) ([]biquad.Coefficients, error) {
	if a[0] == 0 || b[0] == 0 {
		return nil, ErrInvalidParams
	}

	numRoots, err := rootsFromPolyAsc(b)
	if err != nil {
		return nil, err
	}
	denRoots, err := rootsFromPolyAsc(a)
	if err != nil {
		return nil, err
	}

	numPairs, err := pairConjugates(numRoots)
	if err != nil {
		return nil, err
	}
	denPairs, err := pairConjugates(denRoots)
	if err != nil {
		return nil, err
	}

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

func rootsFromPolyAsc(c [5]float64) ([]complex128, error) {
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

	out := make([]complex128, len(roots))
	for i, x := range roots {
		if x == 0 {
			return nil, ErrInvalidParams
		}
		out[i] = 1 / x
	}
	return out, nil
}

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

func pairConjugates(roots []complex128) ([][2]complex128, error) {
	used := make([]bool, len(roots))
	pairs := make([][2]complex128, 0, len(roots)/2)

	for i := range roots {
		if used[i] {
			continue
		}
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
		if best == -1 || !isConjugate(r, roots[best], conjugateTol) {
			return nil, ErrInvalidParams
		}
		used[i] = true
		used[best] = true
		pairs = append(pairs, [2]complex128{r, roots[best]})
	}
	return pairs, nil
}

func polyRootsDurandKerner(coeff []complex128) ([]complex128, error) {
	if len(coeff) < 2 {
		return nil, ErrInvalidParams
	}
	lead := coeff[0]
	if lead == 0 {
		return nil, ErrInvalidParams
	}

	n := len(coeff) - 1
	norm := make([]complex128, len(coeff))
	for i := range coeff {
		norm[i] = coeff[i] / lead
	}

	// Cauchy bound: all roots lie within |z| <= max(1, sum|a_i|).
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

	const maxIter = 500
	const tol = 1e-12
	for iter := 0; iter < maxIter; iter++ {
		maxDelta := 0.0
		for i := 0; i < n; i++ {
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

func polyEval(coeff []complex128, x complex128) complex128 {
	v := coeff[0]
	for i := 1; i < len(coeff); i++ {
		v = v*x + coeff[i]
	}
	return v
}

const conjugateTol = 1e-7

func isConjugate(a, b complex128, tol float64) bool {
	if math.Abs(real(a)-real(b)) > tol*math.Max(1, math.Abs(real(a))) {
		return false
	}
	if math.Abs(imag(a)+imag(b)) > tol*math.Max(1, math.Abs(imag(a))) {
		return false
	}
	return true
}
