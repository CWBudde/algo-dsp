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

	roots := make([]complex128, n)
	radius := 1.0
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		roots[i] = complex(radius*math.Cos(angle), radius*math.Sin(angle))
	}

	for iter := 0; iter < 200; iter++ {
		maxDelta := 0.0
		for i := 0; i < n; i++ {
			den := complex(1, 0)
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				den *= roots[i] - roots[j]
			}
			if den == 0 {
				return nil, ErrInvalidParams
			}
			f := polyEval(norm, roots[i])
			delta := f / den
			roots[i] -= delta
			if d := cmplx.Abs(delta); d > maxDelta {
				maxDelta = d
			}
		}
		if maxDelta < 1e-12 {
			return roots, nil
		}
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
