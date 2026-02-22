package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// bilinearK computes the bilinear transform frequency warping factor tan(π*freq/sampleRate).
// Returns (k, true) on success, (0, false) if parameters are invalid.
func bilinearK(freq, sampleRate float64) (float64, bool) {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
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
// Used for odd-order filters.
func butterworthFirstOrderLP(freq, sampleRate float64) biquad.Coefficients {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return biquad.Coefficients{}
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
// Used for odd-order filters.
func butterworthFirstOrderHP(freq, sampleRate float64) biquad.Coefficients {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return biquad.Coefficients{}
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
