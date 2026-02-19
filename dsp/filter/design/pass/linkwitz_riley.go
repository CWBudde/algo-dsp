package pass

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// LinkwitzRileyLP designs a lowpass Linkwitz-Riley cascade of the given order.
//
// A Linkwitz-Riley filter of order 2N is constructed by cascading two
// Butterworth filters of order N. This produces -6.02 dB at the crossover
// frequency and a squared-Butterworth magnitude response.
//
// The order must be a positive even integer (2, 4, 6, 8, …). Returns nil
// for invalid parameters (odd order, order ≤ 0, invalid frequency, etc.).
//
// When used in a crossover with [LinkwitzRileyHP] at the same frequency and
// order, the lowpass and highpass outputs can be summed to produce an allpass
// response. For orders divisible by 4 (LR4, LR8, …) the outputs are in-phase
// and sum directly. For orders ≡ 2 mod 4 (LR2, LR6, …) the highpass output
// must be inverted before summing (see [LinkwitzRileyHPInverted]).
func LinkwitzRileyLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 || order%2 != 0 {
		return nil
	}
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}
	halfOrder := order / 2

	bw := ButterworthLP(freq, halfOrder, sampleRate)
	if bw == nil {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, 2*len(bw))
	sections = append(sections, bw...)
	sections = append(sections, bw...)
	return sections
}

// LinkwitzRileyHP designs a highpass Linkwitz-Riley cascade of the given order.
//
// A Linkwitz-Riley filter of order 2N is constructed by cascading two
// Butterworth filters of order N. This produces -6.02 dB at the crossover
// frequency and a squared-Butterworth magnitude response.
//
// The order must be a positive even integer (2, 4, 6, 8, …). Returns nil
// for invalid parameters (odd order, order ≤ 0, invalid frequency, etc.).
//
// For orders divisible by 4, this output is in-phase with [LinkwitzRileyLP]
// and their sum is allpass. For orders ≡ 2 mod 4, the highpass output is
// 180° out of phase at the crossover; use [LinkwitzRileyHPInverted] or
// apply a polarity flip when summing.
func LinkwitzRileyHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 || order%2 != 0 {
		return nil
	}
	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}
	halfOrder := order / 2

	bw := ButterworthHP(freq, halfOrder, sampleRate)
	if bw == nil {
		return nil
	}

	sections := make([]biquad.Coefficients, 0, 2*len(bw))
	sections = append(sections, bw...)
	sections = append(sections, bw...)
	return sections
}

// LinkwitzRileyHPInverted designs a highpass Linkwitz-Riley cascade with
// inverted polarity. This is useful for orders ≡ 2 mod 4 (LR2, LR6, LR10, …)
// where the standard HP output is 180° out of phase with the LP at the
// crossover frequency. Inverting the HP ensures LP + HP_inv = allpass.
//
// For orders divisible by 4, this function is equivalent to
// [LinkwitzRileyHP] with negated B coefficients (unnecessary in practice).
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
// orders ≡ 2 mod 4 (LR2, LR6, LR10, …).
func LinkwitzRileyNeedsHPInvert(order int) bool {
	return order > 0 && order%4 == 2
}
