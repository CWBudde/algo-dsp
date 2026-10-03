package loudness

import "github.com/cwbudde/algo-dsp/dsp/filter/biquad"

// integratedKWeighting uses the exact 48 kHz coefficients in ITU-R BS.1770-5,
// Annex 1, Tables 1 and 2. Other rates preserve their inverse-bilinear analog
// transfer functions, then apply the bilinear transform at the requested rate.
// This is not a generic RBJ shelf/high-pass or a corner-frequency prewarp.
func integratedKWeighting(rate float64) (biquad.Coefficients, biquad.Coefficients) {
	shelf := biquad.Coefficients{
		B0: 1.53512485958697, B1: -2.69169618940638, B2: 1.19839281085285,
		A1: -1.69065929318241, A2: 0.73248077421585,
	}

	highpass := biquad.Coefficients{
		B0: 1, B1: -2, B2: 1, A1: -1.99004745483398, A2: 0.99007225036621,
	}
	if rate == 48000 {
		return shelf, highpass
	}

	return remapIntegratedFilter(shelf, rate/48000), remapIntegratedFilter(highpass, rate/48000)
}

func remapIntegratedFilter(c biquad.Coefficients, ratio float64) biquad.Coefficients {
	// Substitute q_old = ((1-r)+(1+r)q)/((1+r)+(1-r)q),
	// where q=z^-1 and r=newRate/oldRate, into both polynomials.
	b0, b1, b2 := remapIntegratedPolynomial(c.B0, c.B1, c.B2, ratio)
	a0, a1, a2 := remapIntegratedPolynomial(1, c.A1, c.A2, ratio)

	return biquad.Coefficients{B0: b0 / a0, B1: b1 / a0, B2: b2 / a0, A1: a1 / a0, A2: a2 / a0}
}

func remapIntegratedPolynomial(p0, p1, p2, ratio float64) (float64, float64, float64) {
	d0, d1 := 1+ratio, 1-ratio
	n0, n1 := d1, d0

	return p0*d0*d0 + p1*n0*d0 + p2*n0*n0,
		2*p0*d0*d1 + p1*(n0*d1+n1*d0) + 2*p2*n0*n1,
		p0*d1*d1 + p1*n1*d1 + p2*n1*n1
}
