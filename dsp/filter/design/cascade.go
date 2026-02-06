package design

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ButterworthLP designs a lowpass Butterworth cascade.
//
// For odd orders, the final section is first-order (B2=A2=0).
func ButterworthLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	n2 := order / 2
	for i := n2 - 1; i >= 0; i-- {
		q := butterworthQ(order, i)
		sections = append(sections, Lowpass(freq, q, sampleRate))
	}
	if order%2 != 0 {
		sections = append(sections, butterworthFirstOrderLP(freq, sampleRate))
	}
	return sections
}

// ButterworthHP designs a highpass Butterworth cascade.
//
// For odd orders, the final section is first-order (B2=A2=0).
func ButterworthHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}
	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	n2 := order / 2
	for i := n2 - 1; i >= 0; i-- {
		q := butterworthQ(order, i)
		sections = append(sections, Highpass(freq, q, sampleRate))
	}
	if order%2 != 0 {
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}
	return sections
}

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
		// Legacy code leaves odd-order Chebyshev first-order as TODO.
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
		// Legacy code leaves odd-order Chebyshev first-order as TODO.
		// Use Butterworth first-order section for deterministic behavior.
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}
	return sections
}

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

func butterworthQ(order, index int) float64 {
	theta := math.Pi * float64(2*index+1) / (2 * float64(order))
	s := math.Sin(theta)
	if s == 0 {
		return defaultQ
	}
	return 1 / (2 * s)
}

func bilinearK(freq, sampleRate float64) (float64, bool) {
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return 0, false
	}
	return math.Tan(math.Pi * freq / sampleRate), true
}

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

func cheby2RippleFactors(order int, rippleDB float64) (float64, float64) {
	if order <= 0 {
		return 1, 0
	}
	if rippleDB <= 0 {
		rippleDB = 1
	}
	t := math.Asinh(1/rippleDB) / float64(order)
	r1 := math.Sinh(t)
	r0 := math.Cosh(t)
	return r0 * r0, r1
}

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
