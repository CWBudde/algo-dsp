package pass

import (
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// EllipticLP designs a lowpass elliptic (Cauer) filter cascade.
//
// Elliptic filters provide the sharpest transition from passband to stopband
// among classical IIR filter types, at the cost of ripple in both regions.
// The rippleDB parameter controls passband ripple (in dB, typical 0.1-1.0),
// while stopbandDB controls the minimum stopband attenuation (in dB, typical 40-80).
//
// The design uses the standard analog elliptic prototype (poles and zeros
// placed via Jacobi elliptic functions) followed by bilinear transform.
func EllipticLP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}

	// Convert ripple specifications to linear selectivity parameters.
	e := math.Sqrt(math.Pow(10, rippleDB/10) - 1)    // Passband ripple
	es := math.Sqrt(math.Pow(10, stopbandDB/10) - 1) // Stopband selectivity
	k1 := e / es

	// Solve elliptic degree equation to find discrimination parameter.
	kEllip := ellipdeg(order, k1, 1e-9)

	// Compute pole argument for analog prototype.
	v0 := asne(complex(0, 1)/complex(e, 0), k1, 1e-9) / complex(float64(order), 0)

	r := order % 2
	L := (order - r) / 2

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	// First-order section for odd orders.
	if r == 1 {
		// Real pole from cd elliptic function.
		p0 := -real(complex(0, 1) * cde(-1.0+v0, kEllip, 1e-9))
		// Bilinear transform of first-order: H(s) = 1/(s - p0)
		// After bilinear s = k*(z-1)/(z+1): H(z) = (k-p0)/(k+p0) * (1 + z^-1)/(1 - ((k+p0)/(k-p0))z^-1)
		norm := 1 / (k - p0)
		sections = append(sections, biquad.Coefficients{
			B0: k * norm,
			B1: k * norm,
			B2: 0,
			A1: (k + p0) * norm,
			A2: 0,
		})
	}

	// Second-order sections for conjugate pole/zero pairs.
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)

		// Evaluate cd to get the i-th conjugate zero on imaginary axis.
		zi := cde(complex(ui, 0), kEllip, 1e-9)
		// Invert to get normalized zero location.
		invZero := 1.0 / zi
		omegaZ := imag(invZero)

		// Evaluate cd with pole argument to get the i-th pole.
		pi := cde(complex(ui, 0)-v0, kEllip, 1e-9)
		invPole := 1.0 / pi
		sigmaP := -real(invPole)
		omegaP := imag(invPole)

		// Analog prototype second-order section:
		// Numerator: s² + omegaZ²
		// Denominator: s² + 2·sigmaP·s + (sigmaP² + omegaP²)
		zabs2 := omegaZ * omegaZ
		pabs2 := sigmaP*sigmaP + omegaP*omegaP

		// Apply bilinear transform s -> k·(z-1)/(z+1).
		k2 := k * k

		// Numerator after bilinear transform.
		bn0 := k2 + zabs2
		bn1 := 2 * (k2 - zabs2)
		bn2 := k2 + zabs2

		// Denominator after bilinear transform.
		ad0 := k2 + 2*k*sigmaP + pabs2
		ad1 := 2 * (k2 - pabs2)
		ad2 := k2 - 2*k*sigmaP + pabs2

		// Normalize denominator leading coefficient to 1.
		b0 := bn0 / ad0
		b1 := bn1 / ad0
		b2 := bn2 / ad0
		a1 := ad1 / ad0
		a2 := ad2 / ad0

		// Normalize for unity DC gain.
		dcGain := (b0 + b1 + b2) / (1 + a1 + a2)
		b0 /= dcGain
		b1 /= dcGain
		b2 /= dcGain

		sections = append(sections, biquad.Coefficients{
			B0: b0, B1: b1, B2: b2,
			A1: a1, A2: a2,
		})
	}

	return sections
}

// EllipticHP designs a highpass elliptic (Cauer) filter cascade.
//
// Applies an LP-to-HP frequency transformation to the analog elliptic prototype
// before the bilinear transform. The passband (above freq) has controlled ripple,
// and the stopband (below freq) has controlled minimum attenuation.
func EllipticHP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}

	// Convert ripple specifications to linear selectivity parameters.
	e := math.Sqrt(math.Pow(10, rippleDB/10) - 1)
	es := math.Sqrt(math.Pow(10, stopbandDB/10) - 1)
	k1 := e / es

	// Solve elliptic degree equation.
	kEllip := ellipdeg(order, k1, 1e-9)

	// Compute pole argument.
	v0 := asne(complex(0, 1)/complex(e, 0), k1, 1e-9) / complex(float64(order), 0)

	r := order % 2
	L := (order - r) / 2

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	// First-order section for odd orders with LP-to-HP transform.
	if r == 1 {
		// LP pole.
		p0LP := -real(complex(0, 1) * cde(-1.0+v0, kEllip, 1e-9))
		// LP-to-HP: s -> 1/s gives pole at -1/p0LP.
		p0HP := -1.0 / p0LP
		// Bilinear transform of highpass first-order.
		// H(s) = s/(s - p0HP), after bilinear becomes (z-1)/(c(z-1)+(z+1)) where c = k/p0HP
		denom := k - p0HP
		norm := 1 / denom
		sections = append(sections, biquad.Coefficients{
			B0: norm,
			B1: -norm,
			B2: 0,
			A1: (k + p0HP) * norm,
			A2: 0,
		})
	}

	// Second-order sections with LP-to-HP transform.
	for i := 1; i <= L; i++ {
		ui := (2.0*float64(i) - 1.0) / float64(order)

		// Get LP poles (zeros don't affect HP since they're at infinity in analog domain).
		pi := cde(complex(ui, 0)-v0, kEllip, 1e-9)
		invPole := 1.0 / pi
		sigmaPLP := -real(invPole)
		omegaPLP := imag(invPole)

		// LP-to-HP transformation: s -> 1/s.
		// LP zero at s=j·omegaZ becomes HP zero at s=0 (DC).
		// LP pole at s=sigmaP+j·omegaP becomes HP pole at s=1/(sigmaP+j·omegaP).
		// This gives s = sigmaP/(sigmaP²+omegaP²) - j·omegaP/(sigmaP²+omegaP²).
		pabs2LP := sigmaPLP*sigmaPLP + omegaPLP*omegaPLP
		sigmaPHP := sigmaPLP / pabs2LP
		omegaPHP := omegaPLP / pabs2LP

		k2 := k * k

		// HP analog section: H(s) = s² / (s² + 2·sigmaPHP·s + (sigmaPHP²+omegaPHP²))
		// Numerator after bilinear: (z-1)²  = z² - 2z + 1.
		bn0 := 1.0
		bn1 := -2.0
		bn2 := 1.0

		// Denominator: LP pattern with HP pole.
		pabs2HP := sigmaPHP*sigmaPHP + omegaPHP*omegaPHP
		ad0 := k2 + 2*k*sigmaPHP + pabs2HP
		ad1 := 2 * (k2 - pabs2HP)
		ad2 := k2 - 2*k*sigmaPHP + pabs2HP

		// Normalize denominator.
		b0 := bn0 / ad0
		b1 := bn1 / ad0
		b2 := bn2 / ad0
		a1 := ad1 / ad0
		a2 := ad2 / ad0

		// Normalize for unity gain at Nyquist (z=-1).
		nyqGain := (b0 - b1 + b2) / (1 - a1 + a2)
		b0 /= nyqGain
		b1 /= nyqGain
		b2 /= nyqGain

		sections = append(sections, biquad.Coefficients{
			B0: b0, B1: b1, B2: b2,
			A1: a1, A2: a2,
		})
	}

	return sections
}

// Elliptic function helpers (simplified from band package).

// landen computes the Landen sequence of descending moduli.
func landen(k, tol float64) []float64 {
	if k == 0 || k == 1.0 {
		return []float64{k}
	}
	var v []float64
	for k > tol {
		t := k / (1.0 + math.Sqrt((1-k)*(1+k)))
		k = t * t
		v = append(v, k)
	}
	return v
}

// landenK computes K from a precomputed Landen sequence.
func landenK(v []float64) float64 {
	prod := 1.0
	for _, x := range v {
		prod *= 1.0 + x
	}
	return prod * math.Pi * 0.5
}

// srem computes symmetric remainder.
func srem(x, y float64) float64 {
	z := math.Remainder(x, y)
	if math.Abs(z) > y/2.0 {
		z -= y * math.Copysign(1.0, z)
	}
	return z
}

// ellipk computes the complete elliptic integral K(k) and K'(k).
func ellipk(k, tol float64) (float64, float64) {
	kmin := 1e-6
	kmax := math.Sqrt(1 - kmin*kmin)

	var K, Kp float64
	if k == 1.0 {
		K = math.Inf(1)
	} else if k > kmax {
		kp := math.Sqrt((1 - k) * (1 + k))
		L := -math.Log(kp / 4.0)
		K = L + (L-1)*kp*kp/4.0
	} else {
		K = landenK(landen(k, tol))
	}

	if k == 0.0 {
		Kp = math.Inf(1)
	} else if k < kmin {
		L := -math.Log(k / 4.0)
		Kp = L + (L-1.0)*k*k/4.0
	} else {
		kp := math.Sqrt((1 - k) * (1 + k))
		Kp = landenK(landen(kp, tol))
	}

	return K, Kp
}

// acde computes the inverse cd elliptic function.
func acde(w complex128, k, tol float64) complex128 {
	v := landen(k, tol)
	for i := range v {
		v1 := k
		if i > 0 {
			v1 = v[i-1]
		}
		w = w / (1.0 + cmplx.Sqrt(1.0-w*w*complex(v1*v1, 0))) * 2.0 / (1 + complex(v[i], 0))
	}

	u := 2.0 / math.Pi * cmplx.Acos(w)
	K, Kp := ellipk(k, tol)

	return complex(srem(real(u), 4), 0) + complex(0, 1)*complex(srem(imag(u), 2*(Kp/K)), 0)
}

// asne computes the inverse sn elliptic function.
func asne(w complex128, k, tol float64) complex128 {
	return 1.0 - acde(w, k, tol)
}

// cde evaluates the Jacobi cd elliptic function.
func cde(u complex128, k, tol float64) complex128 {
	v := landen(k, tol)
	w := cmplx.Cos(u * math.Pi * 0.5)
	for i := len(v) - 1; i >= 0; i-- {
		w = (1 + complex(v[i], 0)) * w / (1.0 + complex(v[i], 0)*w*w)
	}
	return w
}

// sne evaluates the Jacobi sn elliptic function.
func sne(u []float64, k, tol float64) []float64 {
	v := landen(k, tol)
	w := make([]float64, len(u))
	for i := range u {
		w[i] = math.Sin(u[i] * math.Pi * 0.5)
	}
	for i := len(v) - 1; i >= 0; i-- {
		for j := range w {
			w[j] = ((1 + v[i]) * w[j]) / (1 + v[i]*w[j]*w[j])
		}
	}
	return w
}

// ellipdeg2 computes the elliptic degree equation using nome-based series.
func ellipdeg2(n, k, tol float64) float64 {
	const M = 7
	K, Kp := ellipk(k, tol)
	q := math.Exp(-math.Pi * Kp / K)
	q1 := math.Pow(q, n)

	var s1, s2 float64
	q1sq := q1
	q1pow := q1
	q1gap := q1
	q1_2 := q1 * q1
	for i := 1; i <= M; i++ {
		s2 += q1sq
		s1 += q1sq * q1pow
		q1gap *= q1_2
		q1sq *= q1gap
		q1pow *= q1
	}

	r := (1.0 + s1) / (1.0 + 2*s2)
	return 4 * math.Sqrt(q1) * r * r
}

// ellipdeg solves the degree equation for elliptic filter design.
func ellipdeg(N int, k1, tol float64) float64 {
	L := N / 2
	ui := make([]float64, 0, L)
	for i := 1; i <= L; i++ {
		ui = append(ui, (2.0*float64(i)-1.0)/float64(N))
	}
	kmin := 1e-6
	if k1 < kmin {
		return ellipdeg2(1.0/float64(N), k1, tol)
	}

	kc := math.Sqrt((1 - k1) * (1 + k1))
	w := sne(ui, kc, tol)
	prod := 1.0
	for _, x := range w {
		prod *= x
	}
	kp := math.Pow(kc, float64(N)) * math.Pow(prod, 4)

	return math.Sqrt(1 - kp*kp)
}
