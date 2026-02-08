package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev1LP designs a lowpass Chebyshev Type I cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev1LP.CalculateCoefficients.
func Chebyshev1LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}
	r0, r1 := cheby1RippleFactors(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)
	k2 := k * k

	for i := (order / 2) - 1; i >= 0; i-- {
		tt := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		b := 1 / (r0 - tt*tt)
		a := k * 2 * b * r1 * tt
		t := 1 / (a + b + k2)
		sections = append(sections, biquad.Coefficients{
			B0: k2 * t,
			B1: 2 * k2 * t,
			B2: k2 * t,
			A1: 2 * (b - k2) * t,
			A2: (a - k2 - b) * t,
		})
	}
	if order%2 != 0 {
		// Legacy code does not implement odd-order Chebyshev first-order sections.
		// Use Butterworth first-order section for deterministic behavior.
		sections = append(sections, butterworthFirstOrderLP(freq, sampleRate))
	}
	return sections
}

// Chebyshev1HP designs a highpass Chebyshev Type I cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev1HP.CalculateCoefficients.
func Chebyshev1HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}
	r0, r1 := cheby1RippleFactors(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)
	k2 := k * k

	for i := (order / 2) - 1; i >= 0; i-- {
		s := math.Sin(float64(2*i+1) * math.Pi / (4 * float64(order)))
		tt := s * s
		a := 1 / (r0 + 4*tt - 4*tt*tt - 1)
		b := 2 * k * a * r1 * (1 - 2*tt)
		t := 1 / (b + 1 + a*k2)
		sections = append(sections, biquad.Coefficients{
			B0: t,
			B1: -2 * t,
			B2: t,
			A1: 2 * (1 - a*k2) * t,
			A2: (b - 1 - a*k2) * t,
		})
	}
	if order%2 != 0 {
		// Legacy code does not implement odd-order Chebyshev first-order sections.
		// Use Butterworth first-order section for deterministic behavior.
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}
	return sections
}
