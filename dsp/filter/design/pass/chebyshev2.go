package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev2LP designs a lowpass Chebyshev Type II (inverse Chebyshev) cascade.
//
// The design uses the standard analog prototype (inverted Chebyshev Type I poles
// with imaginary-axis zeros) followed by bilinear transform. The passband is
// maximally flat and the stopband exhibits equiripple behavior.
//
// The rippleDB parameter controls the stopband attenuation depth: larger values
// produce deeper stopband notches. It is used identically to Chebyshev Type I's
// ripple parameter (mu = asinh(rippleDB) / order).
func Chebyshev2LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	wc := math.Tan(math.Pi * freq / sampleRate) // pre-warped analog cutoff
	mu := cheby2Mu(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	for i := range order / 2 {
		phi := math.Pi * float64(2*i+1) / float64(2*order)

		// Chebyshev Type I analog prototype pole components
		sigma1 := math.Sinh(mu) * math.Sin(phi)
		omega1 := math.Cosh(mu) * math.Cos(phi)

		// Type II: invert poles (reciprocal of Type I poles)
		poleMagSq := sigma1*sigma1 + omega1*omega1
		sigmaP := sigma1 / poleMagSq
		omegaP := omega1 / poleMagSq

		// Type II zero on imaginary axis
		omegaZ := 1.0 / math.Cos(phi)

		// Scale by pre-warped cutoff
		wpr := wc * sigmaP
		wz := wc * omegaZ
		wp2 := wpr*wpr + (wc*omegaP)*(wc*omegaP)

		// Bilinear transform: s -> (z-1)/(z+1)
		// Numerator from analog (s² + wz²)
		wz2 := wz * wz
		bn0 := 1 + wz2
		bn1 := -2 + 2*wz2
		bn2 := 1 + wz2

		// Denominator from analog (s² + 2·wpr·s + wp2)
		ad0 := 1 + 2*wpr + wp2
		ad1 := -2 + 2*wp2
		ad2 := 1 - 2*wpr + wp2

		// Normalize denominator leading coefficient to 1
		b0 := bn0 / ad0
		b1 := bn1 / ad0
		b2 := bn2 / ad0
		a1 := ad1 / ad0
		a2 := ad2 / ad0

		// Normalize for unity DC gain (z=1)
		dcGain := (b0 + b1 + b2) / (1 + a1 + a2)
		b0 /= dcGain
		b1 /= dcGain
		b2 /= dcGain

		sections = append(sections, biquad.Coefficients{
			B0: b0, B1: b1, B2: b2,
			A1: a1, A2: a2,
		})
	}

	if order%2 != 0 {
		sections = append(sections, cheby2FirstOrderLP(wc, mu))
	}

	return sections
}

// Chebyshev2HP designs a highpass Chebyshev Type II (inverse Chebyshev) cascade.
//
// The design applies an LP-to-HP frequency transformation to the analog prototype
// before the bilinear transform. The passband (above freq) is maximally flat and
// the stopband (below freq) exhibits equiripple behavior.
func Chebyshev2HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	wc := math.Tan(math.Pi * freq / sampleRate)
	mu := cheby2Mu(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	for i := range order / 2 {
		phi := math.Pi * float64(2*i+1) / float64(2*order)

		// Type I prototype components
		sigma1 := math.Sinh(mu) * math.Sin(phi)
		omega1 := math.Cosh(mu) * math.Cos(phi)

		// HP transform: poles become wc·(sigma1, omega1), zeros become wc·cos(phi)
		hpSigma := wc * sigma1
		hpOmega := wc * omega1
		hpWz := wc * math.Cos(phi)

		// Bilinear transform
		hp2 := hpSigma*hpSigma + hpOmega*hpOmega
		wz2 := hpWz * hpWz

		bn0 := 1 + wz2
		bn1 := -2 + 2*wz2
		bn2 := 1 + wz2

		ad0 := 1 + 2*hpSigma + hp2
		ad1 := -2 + 2*hp2
		ad2 := 1 - 2*hpSigma + hp2

		b0 := bn0 / ad0
		b1 := bn1 / ad0
		b2 := bn2 / ad0
		a1 := ad1 / ad0
		a2 := ad2 / ad0

		// Normalize for unity gain at Nyquist (z=-1)
		nyqGain := (b0 - b1 + b2) / (1 - a1 + a2)
		b0 /= nyqGain
		b1 /= nyqGain
		b2 /= nyqGain

		sections = append(sections, biquad.Coefficients{
			B0: b0, B1: b1, B2: b2,
			A1: a1, A2: a2,
		})
	}

	if order%2 != 0 {
		sections = append(sections, cheby2FirstOrderHP(wc, mu))
	}

	return sections
}

// cheby2Mu computes the prototype parameter mu = asinh(ripple) / order.
func cheby2Mu(order int, ripple float64) float64 {
	if ripple <= 0 {
		ripple = 1
	}

	return math.Asinh(ripple) / float64(order)
}

// cheby2FirstOrderLP returns a first-order lowpass section for odd-order Type II.
func cheby2FirstOrderLP(wc, mu float64) biquad.Coefficients {
	sp := wc / math.Sinh(mu) // real pole magnitude
	g := sp / (1 + sp)

	return biquad.Coefficients{
		B0: g,
		B1: g,
		B2: 0,
		A1: (sp - 1) / (1 + sp),
		A2: 0,
	}
}

// cheby2FirstOrderHP returns a first-order highpass section for odd-order Type II.
func cheby2FirstOrderHP(wc, mu float64) biquad.Coefficients {
	sp := wc * math.Sinh(mu) // HP-transformed real pole
	g := 1.0 / (1 + sp)

	return biquad.Coefficients{
		B0: g,
		B1: -g,
		B2: 0,
		A1: (sp - 1) / (1 + sp),
		A2: 0,
	}
}
