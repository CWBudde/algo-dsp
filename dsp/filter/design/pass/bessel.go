package pass

import (
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
)

// BesselLP designs a lowpass Bessel (Thomson) cascade.
// The Bessel filter has maximally flat group delay in the passband.
// Supported orders: 1 to 10. Returns nil for unsupported or invalid parameters.
//
// For odd orders, the final section is first-order (B2=A2=0).
func BesselLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 || order > maxBesselOrder {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	wc := math.Tan(math.Pi * freq / sampleRate)
	poles := besselNormPoles(order)

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	for _, p := range poles {
		sigma := -real(p)

		omega := imag(p)
		if omega == 0 {
			sections = append(sections, besselFirstOrderLP(wc, sigma))
		} else {
			sections = append(sections, besselSecondOrderLP(wc, sigma, omega))
		}
	}

	return sections
}

// BesselHP designs a highpass Bessel (Thomson) cascade.
// The Bessel filter has maximally flat group delay in the passband.
// Supported orders: 1 to 10. Returns nil for unsupported or invalid parameters.
//
// For odd orders, the final section is first-order (B2=A2=0).
func BesselHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	if order <= 0 || order > maxBesselOrder {
		return nil
	}

	if sampleRate <= 0 || freq <= 0 || freq >= sampleRate/2 {
		return nil
	}

	wc := math.Tan(math.Pi * freq / sampleRate)
	poles := besselNormPoles(order)

	sections := make([]biquad.Coefficients, 0, (order+1)/2)

	for _, p := range poles {
		sigma := -real(p)

		omega := imag(p)
		if omega == 0 {
			sections = append(sections, besselFirstOrderHP(wc, sigma))
		} else {
			sections = append(sections, besselSecondOrderHP(wc, sigma, omega))
		}
	}

	return sections
}

// besselSecondOrderLP creates a lowpass biquad from a Bessel conjugate pole pair.
// sigma and omega are the -3 dB normalized pole real/imaginary magnitudes (positive).
func besselSecondOrderLP(wc, sigma, omega float64) biquad.Coefficients {
	// Scale analog pole by pre-warped cutoff.
	a := sigma * wc
	b := omega * wc
	p2 := a*a + b*b

	// Bilinear transform: s = (z-1)/(z+1).
	a0 := 1 + 2*a + p2
	a1 := -2 + 2*p2
	a2 := 1 - 2*a + p2

	// Unity DC gain normalization.
	return biquad.Coefficients{
		B0: p2 / a0,
		B1: 2 * p2 / a0,
		B2: p2 / a0,
		A1: a1 / a0,
		A2: a2 / a0,
	}
}

// besselFirstOrderLP creates a first-order lowpass section from a real Bessel pole.
func besselFirstOrderLP(wc, sigma float64) biquad.Coefficients {
	sp := sigma * wc
	norm := 1 / (1 + sp)

	return biquad.Coefficients{
		B0: sp * norm,
		B1: sp * norm,
		A1: (sp - 1) * norm,
	}
}

// besselSecondOrderHP creates a highpass biquad from a Bessel conjugate pole pair.
// sigma and omega are the -3 dB normalized pole real/imaginary magnitudes (positive).
func besselSecondOrderHP(wc, sigma, omega float64) biquad.Coefficients {
	// HP analog: LP-to-HP frequency transformation s → 1/s in normalized domain.
	p2 := sigma*sigma + omega*omega
	wc2 := wc * wc

	a0 := wc2 + 2*sigma*wc + p2
	a1 := 2*wc2 - 2*p2
	a2 := wc2 - 2*sigma*wc + p2

	// Unity Nyquist gain normalization.
	return biquad.Coefficients{
		B0: p2 / a0,
		B1: -2 * p2 / a0,
		B2: p2 / a0,
		A1: a1 / a0,
		A2: a2 / a0,
	}
}

// besselFirstOrderHP creates a first-order highpass section from a real Bessel pole.
func besselFirstOrderHP(wc, sigma float64) biquad.Coefficients {
	norm := 1 / (wc + sigma)

	return biquad.Coefficients{
		B0: sigma * norm,
		B1: -sigma * norm,
		A1: (wc - sigma) * norm,
	}
}

// besselNormPoles returns the -3 dB normalized analog prototype poles for a
// Bessel filter of the given order. Only unique poles are returned: one per
// conjugate pair (positive imaginary part) and the real pole for odd orders.
func besselNormPoles(order int) []complex128 {
	delay := besselDelayPoles[order]
	s := besselScaleFactors[order]

	out := make([]complex128, len(delay))
	for i, p := range delay {
		out[i] = complex(real(p)/s, imag(p)/s)
	}

	return out
}

const maxBesselOrder = 10

// besselDelayPoles contains delay-normalized Bessel filter poles for orders 1–10.
// Only the unique pole from each conjugate pair (positive imaginary part) is stored.
// For odd orders, the real pole (zero imaginary part) is listed last.
//
// Source: C.R. Bond, "Bessel Filter Constants", crbond.com/papers/bsf.pdf.
var besselDelayPoles = [maxBesselOrder + 1][]complex128{
	// order 0: unused
	{},
	// order 1
	{complex(-1.0, 0)},
	// order 2
	{complex(-1.5, 0.8660254038)},
	// order 3
	{complex(-1.8389073227, 1.7543809598), complex(-2.3221853546, 0)},
	// order 4
	{complex(-2.1037893972, 2.6574180419), complex(-2.8962106028, 0.8672341289)},
	// order 5
	{
		complex(-2.3246743032, 3.5710229203),
		complex(-3.3519563992, 1.7426614162),
		complex(-3.6467385953, 0),
	},
	// order 6
	{
		complex(-2.5159322478, 4.4926729537),
		complex(-3.7357083563, 2.6262723114),
		complex(-4.2483593959, 0.8675096732),
	},
	// order 7
	{
		complex(-2.6856768789, 5.4206941307),
		complex(-4.0701391636, 3.5171740477),
		complex(-4.7582905282, 1.7392860613),
		complex(-4.9717868585, 0),
	},
	// order 8
	{
		complex(-2.8389839177, 6.3539112470),
		complex(-4.3682892668, 4.4144425006),
		complex(-5.2048407906, 2.6161751538),
		complex(-5.5878860022, 0.8676144454),
	},
	// order 9
	{
		complex(-2.9792607983, 7.2914651564),
		complex(-4.6384398714, 5.3172716754),
		complex(-5.6044218195, 3.4981415816),
		complex(-6.1293679040, 1.7378483835),
		complex(-6.2970079817, 0),
	},
	// order 10
	{
		complex(-3.1088931555, 8.2324678728),
		complex(-4.8862195924, 6.2249854825),
		complex(-5.9675283089, 4.3849471924),
		complex(-6.6152909655, 2.6115679208),
		complex(-6.9220449048, 0.8676594792),
	},
}

// besselScaleFactors contains the frequency scaling factors to convert from
// delay-normalized to -3 dB normalized Bessel filters.
//
// Source: C.R. Bond, "Bessel Filter Constants", crbond.com/papers/bsf.pdf.
var besselScaleFactors = [maxBesselOrder + 1]float64{
	0, // order 0: unused
	1.0,
	1.36165412871613,
	1.75567236868121,
	2.11391767490422,
	2.42741070215263,
	2.70339506120292,
	2.95172214703872,
	3.17961723751065,
	3.39169313891166,
	3.59098059456916,
}
