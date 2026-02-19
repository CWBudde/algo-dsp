package biquad

import "math/cmplx"

// PoleZeroPair stores the two poles and two zeros of one biquad section.
// For first-order sections, the second pole/zero is 0.
type PoleZeroPair struct {
	Poles [2]complex128
	Zeros [2]complex128
}

// Poles returns the z-plane poles of the section denominator:
//
//	1 + A1*z^-1 + A2*z^-2 = 0
func (c *Coefficients) Poles() [2]complex128 {
	return quadraticRoots(1, c.A1, c.A2)
}

// Zeros returns the z-plane zeros of the section numerator:
//
//	B0 + B1*z^-1 + B2*z^-2 = 0
func (c *Coefficients) Zeros() [2]complex128 {
	return quadraticRoots(c.B0, c.B1, c.B2)
}

// PoleZeroPair returns both poles and zeros for a single section.
func (c *Coefficients) PoleZeroPair() PoleZeroPair {
	return PoleZeroPair{
		Poles: c.Poles(),
		Zeros: c.Zeros(),
	}
}

// PoleZeroPairs returns one pole/zero pair entry per coefficient set.
func PoleZeroPairs(coeffs []Coefficients) []PoleZeroPair {
	out := make([]PoleZeroPair, len(coeffs))
	for i := range coeffs {
		out[i] = coeffs[i].PoleZeroPair()
	}
	return out
}

// PoleZeroPairs returns one pole/zero pair entry per chain section.
func (c *Chain) PoleZeroPairs() []PoleZeroPair {
	out := make([]PoleZeroPair, len(c.sections))
	for i := range c.sections {
		out[i] = c.sections[i].PoleZeroPair()
	}
	return out
}

func quadraticRoots(a, b, c float64) [2]complex128 {
	if a == 0 {
		if b == 0 {
			return [2]complex128{}
		}
		return [2]complex128{complex(-c/b, 0), 0}
	}

	discriminant := complex(b*b-4*a*c, 0)
	sqrtDiscriminant := cmplx.Sqrt(discriminant)
	den := complex(2*a, 0)
	return [2]complex128{
		(-complex(b, 0) + sqrtDiscriminant) / den,
		(-complex(b, 0) - sqrtDiscriminant) / den,
	}
}
