package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// ButterworthLP designs a lowpass Butterworth cascade.
//
// For odd orders, the final section is first-order (B2=A2=0).
//
// It returns nil for order <= 0. For an order it can honour but parameters it
// cannot design for, every section is [biquad.Identity], so the cascade is
// transparent rather than silent.
func ButterworthLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	n2 := order / 2
	for i := n2 - 1; i >= 0; i-- {
		q := butterworthQ(order, i)
		sections = append(sections, LowpassRBJ(freq, q, sampleRate))
	}

	if order%2 != 0 {
		sections = append(sections, butterworthFirstOrderLP(freq, sampleRate))
	}

	return sections
}

// ButterworthHP designs a highpass Butterworth cascade.
//
// For odd orders, the final section is first-order (B2=A2=0).
//
// It returns nil for order <= 0. For an order it can honour but parameters it
// cannot design for, every section is [biquad.Identity], so the cascade is
// transparent rather than silent.
func ButterworthHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	n2 := order / 2
	for i := n2 - 1; i >= 0; i-- {
		q := butterworthQ(order, i)
		sections = append(sections, HighpassRBJ(freq, q, sampleRate))
	}

	if order%2 != 0 {
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}

	return sections
}

// LowpassRBJ designs a lowpass biquad using the RBJ cookbook formula.
//
// For undesignable parameters (freq outside (0, Nyquist), invalid sample
// rate) it returns [biquad.Identity], a pass-through section.
func LowpassRBJ(freq, q, sampleRate float64) biquad.Coefficients {
	if !validPassband(freq, sampleRate) {
		return biquad.Identity()
	}

	q = normalizedQ(q)

	w0 := 2 * math.Pi * freq / sampleRate
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)

	b0 := (1 - cw) / 2
	b1 := 1 - cw
	b2 := (1 - cw) / 2
	a0 := 1 + alpha
	a1 := -2 * cw
	a2 := 1 - alpha

	if a0 == 0 {
		return biquad.Identity()
	}

	return biquad.Coefficients{
		B0: b0 / a0,
		B1: b1 / a0,
		B2: b2 / a0,
		A1: a1 / a0,
		A2: a2 / a0,
	}
}

// HighpassRBJ designs a highpass biquad using the RBJ cookbook formula.
//
// For undesignable parameters (freq outside (0, Nyquist), invalid sample
// rate) it returns [biquad.Identity], a pass-through section.
func HighpassRBJ(freq, q, sampleRate float64) biquad.Coefficients {
	if !validPassband(freq, sampleRate) {
		return biquad.Identity()
	}

	q = normalizedQ(q)

	w0 := 2 * math.Pi * freq / sampleRate
	cw := math.Cos(w0)
	sw := math.Sin(w0)
	alpha := sw / (2 * q)

	b0 := (1 + cw) / 2
	b1 := -(1 + cw)
	b2 := (1 + cw) / 2
	a0 := 1 + alpha
	a1 := -2 * cw
	a2 := 1 - alpha

	if a0 == 0 {
		return biquad.Identity()
	}

	return biquad.Coefficients{
		B0: b0 / a0,
		B1: b1 / a0,
		B2: b2 / a0,
		A1: a1 / a0,
		A2: a2 / a0,
	}
}
