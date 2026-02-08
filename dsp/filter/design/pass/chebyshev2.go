package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// Chebyshev2LP designs a lowpass Chebyshev Type II cascade.
//
// The coefficient formulas are based on mfw legacy MFFilter.pas
// TMFDSPChebyshev2LP.CalculateCoefficients, with a corrected angle term:
// cos((2i+1)*pi/(2N)). The legacy code omits pi in that term.
func Chebyshev2LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	k, ok := bilinearK(freq, sampleRate)
	if !ok {
		return nil
	}
	r0, r1 := cheby2RippleFactors(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)
	k2 := k * k

	for i := (order / 2) - 1; i >= 0; i-- {
		tt := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		c0 := 1 - tt*tt
		c1 := 2 * tt * r1 * k
		t := 1 / (c1 + k2 + r0 + c0)
		sections = append(sections, biquad.Coefficients{
			B0: (k2 + c0) * t,
			B1: 2 * (k2 - c0) * t,
			B2: (k2 + c0) * t,
			A1: 2 * (-k2 + r0 + c0) * t,
			A2: (c1 - k2 - r0 - c0) * t,
		})
	}
	if order%2 != 0 {
		// Legacy code does not implement odd-order Type II sections.
		sections = append(sections, butterworthFirstOrderLP(freq, sampleRate))
	}
	return sections
}

// Chebyshev2HP designs a highpass Chebyshev Type II cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev2HP.CalculateCoefficients.
func Chebyshev2HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}
	k := 1 / math.Tan(math.Pi*freq/sampleRate)
	r0, r1 := cheby2RippleFactors(order, rippleDB)
	sections := make([]biquad.Coefficients, 0, (order+1)/2)
	k2 := k * k

	for i := 0; i < order/2; i++ {
		tt := math.Cos(float64(2*i+1) * math.Pi / (2 * float64(order)))
		c0 := 1 - tt*tt
		c1 := 2 * tt * r1 * k
		t := 1 / (c1 + k2 + r0 + c0)
		sections = append(sections, biquad.Coefficients{
			B0: (c0 + k2) * t,
			B1: 2 * (c0 - k2) * t,
			B2: (c0 + k2) * t,
			A1: 2 * (k2 - r0 - c0) * t,
			A2: (c1 - k2 - r0 - c0) * t,
		})
	}
	if order%2 != 0 {
		// Legacy code does not implement odd-order Type II sections.
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}
	return sections
}
