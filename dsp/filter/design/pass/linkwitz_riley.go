package pass

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// LinkwitzRileyLP designs a lowpass Linkwitz-Riley cascade of the given order.
//
// For even order 2N, the cascade uses two identical Butterworth filters of
// order N. For odd order 2N+1, it uses adjacent Butterworth orders N and N+1.
// In both cases the response is -6.02 dB at the crossover frequency.
//
// The order must be an integer >= 2. Returns nil for invalid parameters
// (order < 2, invalid frequency, etc.).
//
// When used in a crossover with [LinkwitzRileyHP] at the same frequency and
// order, the lowpass and highpass outputs can be summed to produce an allpass
// response only for even orders. For orders divisible by 4 (LR4, LR8, …) the
// outputs are in-phase and sum directly. For orders ≡ 2 mod 4 (LR2, LR6, …)
// the highpass output must be inverted before summing (see
// [LinkwitzRileyHPInverted]). Odd orders do not provide exact allpass summation
// with a polarity flip alone.
func LinkwitzRileyLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	lowOrder, highOrder, ok := linkwitzRileyPrototypeOrders(order)
	if !ok {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	low := ButterworthLP(freq, lowOrder, sampleRate)

	high := ButterworthLP(freq, highOrder, sampleRate)
	if low == nil || high == nil {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, len(low)+len(high))
	sections = append(sections, low...)
	sections = append(sections, high...)

	return sections
}

// LinkwitzRileyHP designs a highpass Linkwitz-Riley cascade of the given order.
//
// For even order 2N, the cascade uses two identical Butterworth filters of
// order N. For odd order 2N+1, it uses adjacent Butterworth orders N and N+1.
// In both cases the response is -6.02 dB at the crossover frequency.
//
// The order must be an integer >= 2. Returns nil for invalid parameters
// (order < 2, invalid frequency, etc.).
//
// For even orders divisible by 4, this output is in-phase with
// [LinkwitzRileyLP] and their sum is allpass. For even orders ≡ 2 mod 4, the
// highpass output is 180° out of phase at the crossover; use
// [LinkwitzRileyHPInverted] or apply a polarity flip when summing. Odd orders
// do not provide exact allpass summation with a polarity flip alone.
func LinkwitzRileyHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	lowOrder, highOrder, ok := linkwitzRileyPrototypeOrders(order)
	if !ok {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	low := ButterworthHP(freq, lowOrder, sampleRate)

	high := ButterworthHP(freq, highOrder, sampleRate)
	if low == nil || high == nil {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, len(low)+len(high))
	sections = append(sections, low...)
	sections = append(sections, high...)

	return sections
}

// LinkwitzRileyHPInverted designs a highpass Linkwitz-Riley cascade with
// inverted polarity. For even orders ≡ 2 mod 4 (LR2, LR6, LR10, …), this
// makes the HP output sum allpass with [LinkwitzRileyLP].
//
// For orders divisible by 4, this function is equivalent to
// [LinkwitzRileyHP] with negated B coefficients (unnecessary in practice).
// For odd orders, this applies a polarity inversion but does not guarantee
// allpass summation with [LinkwitzRileyLP].
func LinkwitzRileyHPInverted(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	sections := LinkwitzRileyHP(freq, order, sampleRate)
	if sections == nil {
		return nil
	}
	// Invert polarity by negating the first section's B coefficients.
	// Negating one section is sufficient since gain is multiplicative.
	sections[0].B0 = -sections[0].B0
	sections[0].B1 = -sections[0].B1
	sections[0].B2 = -sections[0].B2

	return sections
}

// LinkwitzRileyNeedsHPInvert reports whether the given Linkwitz-Riley order
// requires HP polarity inversion for allpass summation. Returns true for
// even orders ≡ 2 mod 4 (LR2, LR6, LR10, …). For odd orders this returns
// false because simple polarity inversion is not sufficient for exact allpass
// summation.
func LinkwitzRileyNeedsHPInvert(order int) bool {
	return order > 0 && order%2 == 0 && order%4 == 2
}

func linkwitzRileyPrototypeOrders(order int) (int, int, bool) {
	if order < 2 {
		return 0, 0, false
	}

	return order / 2, (order + 1) / 2, true
}
