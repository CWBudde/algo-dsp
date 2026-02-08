package pass

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
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
		sections = append(sections, design.Lowpass(freq, q, sampleRate))
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
		sections = append(sections, design.Highpass(freq, q, sampleRate))
	}
	if order%2 != 0 {
		sections = append(sections, butterworthFirstOrderHP(freq, sampleRate))
	}
	return sections
}
