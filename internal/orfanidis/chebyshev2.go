package orfanidis

import "math"

// Chebyshev2Prototype builds the analog prototype sections of a Chebyshev
// Type II parametric equalizer of order Spec.Order.
//
// Type II is equiripple on the reference (G0) side and monotonic on the active
// (G) side, so only Spec.Gs is used; Spec.Gb is ignored. The A and B ellipse
// parameters follow Orfanidis:
//
//	e  = sqrt((G² − Gs²)/(Gs² − G0²))
//	eu = (e + sqrt(1 + e²))^(1/M),        A = (eu − 1/eu)/2
//	ew = (G0·e + Gs·sqrt(1 + e²))^(1/M),  B = (ew − g²/ew)/2
//
// with g = G^(1/M). For u_i = (2i−1)/M, c_i = cos(π·u_i/2), s_i = sin(π·u_i/2)
// and tb = WB, each conjugate pair contributes
//
//	num: B0 = g²·tb²,  B1 = 2·g·B·s_i·tb,  B2 = B² + g²·c_i²
//	den: A0 = tb²,     A1 = 2·A·s_i·tb,    A2 = A² + c_i²
//
// For odd orders the unpaired term has u_i = 1, where c_i = 0 and s_i = 1, so
// both polynomials become perfect squares and the section reduces to its square
// root, (g·tb + B·s)/(tb + A·s).
func Chebyshev2Prototype(s Spec) ([]Section, error) {
	if s.Order < 1 || s.WB <= 0 {
		return nil, ErrInvalidParams
	}

	G0, G, Gs := s.G0, s.G, s.Gs

	den := Gs*Gs - G0*G0
	if IsZero(den) {
		return nil, ErrInvalidParams
	}

	ratio := (G*G - Gs*Gs) / den
	if ratio <= 0 || !isFinite(ratio) {
		return nil, ErrInvalidParams
	}

	m := float64(s.Order)
	e := math.Sqrt(ratio)
	g := math.Pow(G, 1.0/m)
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/m)
	ew := math.Pow(G0*e+Gs*math.Sqrt(1.0+e*e), 1.0/m)

	if IsZero(eu) || IsZero(ew) {
		return nil, ErrInvalidParams
	}

	a := (eu - 1.0/eu) * 0.5
	b := (ew - g*g/ew) * 0.5

	if !isFinite(a) || !isFinite(b) {
		return nil, ErrInvalidParams
	}

	tb := s.WB
	r := s.Order % 2
	L := (s.Order - r) / 2

	secs := make([]Section, 0, L+1)

	if r == 1 {
		secs = append(secs, Section{
			B0: g * tb, B1: b,
			A0: tb, A1: a,
		})
	}

	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / m
		ci := math.Cos(math.Pi * ui * 0.5)
		si := math.Sin(math.Pi * ui * 0.5)

		secs = append(secs, Section{
			B0: g * g * tb * tb, B1: 2 * g * b * si * tb, B2: b*b + g*g*ci*ci,
			A0: tb * tb, A1: 2 * a * si * tb, A2: a*a + ci*ci,
		})
	}

	for _, sec := range secs {
		if !sectionIsFinite(sec) {
			return nil, ErrInvalidParams
		}
	}

	return secs, nil
}
