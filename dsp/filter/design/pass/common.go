package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// defaultQ is the Butterworth quality factor used when no usable Q is given.
const defaultQ = 1 / math.Sqrt2

// validPassband reports whether freq and sampleRate describe a designable
// corner frequency: both finite, sampleRate positive and freq strictly inside
// (0, Nyquist). Non-finite inputs are rejected explicitly because every
// ordered comparison against NaN is false, which would otherwise let NaN slip
// through into the coefficients.
func validPassband(freq, sampleRate float64) bool {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return false
	}

	if freq <= 0 || freq >= sampleRate/2 || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return false
	}

	return true
}

// normalizedQ clamps an unusable quality factor to the Butterworth default.
func normalizedQ(q float64) float64 {
	if q <= 0 || math.IsNaN(q) || math.IsInf(q, 0) {
		return defaultQ
	}

	return q
}

// bilinearK computes the bilinear transform frequency warping factor tan(π*freq/sampleRate).
// Returns (k, true) on success, (0, false) if parameters are invalid.
func bilinearK(freq, sampleRate float64) (float64, bool) {
	if !validPassband(freq, sampleRate) {
		return 0, false
	}

	return math.Tan(math.Pi * freq / sampleRate), true
}

// butterworthQ returns the quality factor for a Butterworth filter section.
// index ranges from 0 to (order/2 - 1) for the biquad sections.
func butterworthQ(order, index int) float64 {
	theta := math.Pi * float64(2*index+1) / (2 * float64(order))

	s := math.Sin(theta)
	if s == 0 {
		return 1 / math.Sqrt2 // default Q
	}

	return 1 / (2 * s)
}

// butterworthFirstOrderLP designs a first-order lowpass Butterworth section.
// Used for odd-order filters. Returns [biquad.Identity] for undesignable
// parameters, so an odd-order cascade stays transparent instead of muting.
func butterworthFirstOrderLP(freq, sampleRate float64) biquad.Coefficients {
	if !validPassband(freq, sampleRate) {
		return biquad.Identity()
	}

	k := math.Tan(math.Pi * freq / sampleRate)
	norm := 1 / (1 + k)

	return biquad.Coefficients{
		B0: k * norm,
		B1: k * norm,
		B2: 0,
		A1: (k - 1) * norm,
		A2: 0,
	}
}

// butterworthFirstOrderHP designs a first-order highpass Butterworth section.
// Used for odd-order filters. Returns [biquad.Identity] for undesignable
// parameters, so an odd-order cascade stays transparent instead of muting.
func butterworthFirstOrderHP(freq, sampleRate float64) biquad.Coefficients {
	if !validPassband(freq, sampleRate) {
		return biquad.Identity()
	}

	k := math.Tan(math.Pi * freq / sampleRate)
	norm := 1 / (1 + k)

	return biquad.Coefficients{
		B0: norm,
		B1: -norm,
		B2: 0,
		A1: (k - 1) * norm,
		A2: 0,
	}
}

// cheby1RippleFactors computes ripple-dependent factors for Chebyshev Type I filters.
// Returns (r0, r1) where r0 = cosh²(asinh(rippleDB)/order) and r1 = sinh(asinh(rippleDB)/order).
func cheby1RippleFactors(order int, rippleDB float64) (float64, float64) {
	if order <= 0 {
		return 1, 0
	}

	if rippleDB <= 0 {
		rippleDB = 1
	}

	t := math.Asinh(rippleDB) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)

	return r0 * r0, r1
}
