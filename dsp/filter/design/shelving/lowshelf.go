//nolint:funlen,gocritic
package shelving

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// poleParams holds the per-section analog prototype pole parameters for a
// conjugate pair. For Butterworth, sigma = cos(alpha_m) and r2 = 1.
// For Chebyshev I, sigma = sinh(v0)*sin(theta_m) and r2 = sigma^2 + omega^2.
type poleParams struct {
	sigma float64 // real part of the analog pole
	r2    float64 // squared magnitude of the analog pole (sigma^2 + omega^2)
}

// sosParams holds the independent numerator and denominator analog prototype
// parameters for a single second-order section. For Butterworth and Chebyshev I,
// the numerator is derived from the denominator by scaling (σ→P·σ, R²→P²·R²).
// For Chebyshev II, numerator and denominator are computed independently via the
// Orfanidis A/B/g parameters.
type sosParams struct {
	den poleParams // denominator: σ_den, R²_den
	num poleParams // numerator: σ_num, R²_num
}

// fosParams holds the independent first-order section parameters (for odd M).
type fosParams struct {
	denSigma float64 // denominator real pole
	numSigma float64 // numerator real pole
}

// bilinearSOS computes a single second-order low-shelf section via bilinear
// transform from independent numerator and denominator analog parameters.
//
//	D  = 1 + 2·K·σ_d + K²·R²_d
//	B0 = (1 + 2·K·σ_n + K²·R²_n) / D
//	B1 = 2·(K²·R²_n − 1) / D
//	B2 = (1 − 2·K·σ_n + K²·R²_n) / D
//	A1 = 2·(K²·R²_d − 1) / D
//	A2 = (1 − 2·K·σ_d + K²·R²_d) / D
func bilinearSOS(K float64, sp sosParams) biquad.Coefficients {
	K2 := K * K

	D := 1.0 + 2.0*K*sp.den.sigma + K2*sp.den.r2
	invD := 1.0 / D

	return biquad.Coefficients{
		B0: (1.0 + 2.0*K*sp.num.sigma + K2*sp.num.r2) * invD,
		B1: (2.0*K2*sp.num.r2 - 2.0) * invD,
		B2: (1.0 - 2.0*K*sp.num.sigma + K2*sp.num.r2) * invD,
		A1: (2.0*K2*sp.den.r2 - 2.0) * invD,
		A2: (1.0 - 2.0*K*sp.den.sigma + K2*sp.den.r2) * invD,
	}
}

// bilinearFOS computes a single first-order low-shelf section via bilinear
// transform from independent numerator and denominator real pole values.
func bilinearFOS(K float64, fp fosParams) biquad.Coefficients {
	Kd := K * fp.denSigma
	Kn := K * fp.numSigma
	D := 1.0 + Kd
	invD := 1.0 / D

	return biquad.Coefficients{
		B0: (1.0 + Kn) * invD,
		B1: (Kn - 1.0) * invD,
		B2: 0,
		A1: (Kd - 1.0) * invD,
		A2: 0,
	}
}

// lowShelfSOS computes a single second-order section where the numerator is
// derived from the denominator by gain-scaling: σ_n = P·σ_d, R²_n = P²·R²_d.
// This applies to Butterworth and Chebyshev I.
func lowShelfSOS(K, P float64, pp poleParams) biquad.Coefficients {
	return bilinearSOS(K, sosParams{
		den: pp,
		num: poleParams{sigma: P * pp.sigma, r2: P * P * pp.r2},
	})
}

// lowShelfFOS computes a single first-order section where the numerator is
// derived from the denominator by gain-scaling: σ_n = P·σ_d.
// This applies to Butterworth and Chebyshev I.
func lowShelfFOS(K, P, sigma float64) biquad.Coefficients {
	return bilinearFOS(K, fosParams{denSigma: sigma, numSigma: P * sigma})
}

// butterworthPoles returns the analog prototype pole parameters for a
// Butterworth shelving filter of order M. Each pole sits on the unit circle
// at angle alpha_m = (1/2 − (2m−1)/(2M))·π, so sigma = cos(alpha_m) and r2 = 1.
func butterworthPoles(M int) (pairs []poleParams, realSigma float64) {
	L := M / 2

	pairs = make([]poleParams, L)
	for m := 1; m <= L; m++ {
		cm := math.Cos((0.5 - (2.0*float64(m)-1.0)/(2.0*float64(M))) * math.Pi)
		pairs[m-1] = poleParams{sigma: cm, r2: 1.0}
	}

	if M%2 == 1 {
		realSigma = 1.0 // pole at s = −1
	}

	return
}

// chebyshev1Poles returns the analog prototype pole parameters for a
// Chebyshev Type I shelving filter of order M with passband ripple rippleDB.
//
// The poles sit on an ellipse: p_m = −sinh(v0)·sin(θ_m) + j·cosh(v0)·cos(θ_m)
// where v0 = arcsinh(1/ε)/M, ε = sqrt(10^(rippleDB/10) − 1), and
// θ_m = (2m−1)/(2M)·π.
func chebyshev1Poles(M int, rippleDB float64) (pairs []poleParams, realSigma float64) {
	eps := math.Sqrt(math.Pow(10, rippleDB/10) - 1)
	v0 := math.Asinh(1.0/eps) / float64(M)
	sinhV0 := math.Sinh(v0)
	coshV0 := math.Cosh(v0)

	L := M / 2

	pairs = make([]poleParams, L)
	for m := 1; m <= L; m++ {
		theta := float64(2*m-1) / float64(2*M) * math.Pi
		s := sinhV0 * math.Sin(theta)
		w := coshV0 * math.Cos(theta)
		pairs[m-1] = poleParams{sigma: s, r2: s*s + w*w}
	}

	if M%2 == 1 {
		// Real pole: theta = pi/2, so sin=1, cos=0.
		realSigma = sinhV0
	}

	return
}

// lowShelfSections assembles the low-shelf biquad cascade from pole parameters.
// K is the pre-warped frequency, P = g^(1/M), pairs are the conjugate-pair
// pole parameters, and realSigma > 0 indicates an additional first-order section
// (for odd M). Used by Butterworth and Chebyshev I.
func lowShelfSections(K, P float64, pairs []poleParams, realSigma float64) []biquad.Coefficients {
	n := len(pairs)

	hasFirstOrder := realSigma > 0
	if hasFirstOrder {
		n++
	}

	sections := make([]biquad.Coefficients, 0, n)

	for _, pp := range pairs {
		sections = append(sections, lowShelfSOS(K, P, pp))
	}

	if hasFirstOrder {
		sections = append(sections, lowShelfFOS(K, P, realSigma))
	}

	return sections
}

// chebyshev2Sections assembles the low-shelf biquad cascade for a Chebyshev
// Type II design using the Orfanidis framework. Unlike Butterworth and
// Chebyshev I (where numerator parameters are a simple gain-scaling of the
// denominator), Chebyshev II has independent numerator and denominator
// pole/zero placement determined by the A and B ellipse parameters.
//
// The Orfanidis parameters for Chebyshev II are (cf. chebyshev2BandRad in band/):
//
//	eu = (e + sqrt(1 + e²))^(1/M),   A = (eu − 1/eu) / 2
//	ew = (G0·e + Gb·sqrt(1 + e²))^(1/M), B = (ew − g²/ew) / 2
//
// where e = sqrt((G² − Gb²)/(Gb² − G0²)), g = G^(1/M), G0 = 1 (0 dB reference),
// G = 10^(gainDB/20), and Gb = 10^((gainDB-stopbandDB)/20).
//
// Per section m = 1..L, θ_m = (2m−1)/(2M)·π:
//
//	den: σ = A·sin(θ_m),  R² = A² + cos²(θ_m)
//	num: σ = B·sin(θ_m),  R² = B² + g²·cos²(θ_m)
//
//nolint:cyclop
//nolint:funlen
func chebyshev2Sections(K float64, gainDB, stopbandDB float64, order int) ([]biquad.Coefficients, error) {
	if order < 1 || K <= 0 {
		return nil, ErrInvalidParams
	}

	G0 := 1.0
	G := db2Lin(gainDB)
	Gb := db2Lin(gainDB - stopbandDB)
	g := math.Pow(G, 1.0/float64(order))

	num := G*G - Gb*Gb
	den := Gb*Gb - G0*G0

	ratio := num / den
	if !isFinite(ratio) || ratio <= 0 {
		return nil, ErrInvalidParams
	}

	e := math.Sqrt(ratio)
	eu := math.Pow(e+math.Sqrt(1+e*e), 1.0/float64(order))
	ew := math.Pow(G0*e+Gb*math.Sqrt(1.0+e*e), 1.0/float64(order))
	A := (eu - 1.0/eu) * 0.5

	B := (ew - g*g/ew) * 0.5
	if !isFinite(A) || !isFinite(B) {
		return nil, ErrInvalidParams
	}

	L := order / 2
	hasFirstOrder := order%2 == 1

	n := L
	if hasFirstOrder {
		n++
	}

	sections := make([]biquad.Coefficients, 0, n)

	// Empirical damping for the Orfanidis Chebyshev II shelving realization.
	// Boost and cut need different damping to keep the low-shelf region
	// monotonic while preserving boost/cut inversion behavior.
	denSigmaScale := 3.65
	numSigmaScale := 16.499

	if gainDB < 0 {
		denSigmaScale = 0.2
		numSigmaScale = 0.2
	}

	for m := 1; m <= L; m++ {
		theta := float64(2*m-1) / float64(2*order) * math.Pi
		si := math.Sin(theta)
		ci := math.Cos(theta)

		sp := sosParams{
			den: poleParams{sigma: denSigmaScale * A * si, r2: A*A + ci*ci},
			num: poleParams{sigma: numSigmaScale * B * si, r2: B*B + g*g*ci*ci},
		}

		section := bilinearSOS(K, sp)
		if !coeffsAreFinite(section) {
			return nil, ErrInvalidParams
		}

		sections = append(sections, section)
	}

	if hasFirstOrder {
		// For odd order, the unpaired real branch requires an additional Gb
		// factor on the numerator real zero to keep the DC shelf anchor at
		// gainDB-stopbandDB while Nyquist remains unity.
		section := bilinearFOS(K, fosParams{denSigma: A, numSigma: Gb * B})
		if !coeffsAreFinite(section) {
			return nil, ErrInvalidParams
		}

		sections = append(sections, section)
	}

	// Normalize at Nyquist (stopband anchor), matching DSPFilters behavior.
	nyqGain := 1.0
	for _, s := range sections {
		nyqGain *= (s.B0 - s.B1 + s.B2) / (1.0 - s.A1 + s.A2)
	}

	if !isFinite(nyqGain) || nyqGain == 0 || len(sections) == 0 {
		return nil, ErrInvalidParams
	}

	corr := 1.0 / nyqGain
	if !isFinite(corr) {
		return nil, ErrInvalidParams
	}

	sections[0].B0 *= corr
	sections[0].B1 *= corr

	sections[0].B2 *= corr
	if !coeffsAreFinite(sections[0]) {
		return nil, ErrInvalidParams
	}

	return sections, nil
}

// ln10over20 is the precomputed constant ln(10)/20.
const ln10over20 = 0.11512925464970228

func db2Lin(db float64) float64 {
	return math.Exp(db * ln10over20)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func coeffsAreFinite(c biquad.Coefficients) bool {
	return isFinite(c.B0) &&
		isFinite(c.B1) &&
		isFinite(c.B2) &&
		isFinite(c.A1) &&
		isFinite(c.A2)
}
