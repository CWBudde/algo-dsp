package design

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design/pass"
)

// High-order pass filter wrappers that delegate to the pass package.
// These provide a stable, high-level API in the design package.
//
// Future work: Add Orfanidis-based variants that respect DC and Nyquist gain constraints.
// TODO: Implement OrfanidisLP/HP variants similar to the Peak filter's Orfanidis option.

// ButterworthLP designs a lowpass Butterworth cascade using the RBJ cookbook approach.
//
// For odd orders, the final section is first-order (B2=A2=0).
//
// Future: Add optional Orfanidis-based variant that respects Nyquist gain constraint.
func ButterworthLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.ButterworthLP(freq, order, sampleRate)
}

// ButterworthHP designs a highpass Butterworth cascade using the RBJ cookbook approach.
//
// For odd orders, the final section is first-order (B2=A2=0).
//
// Future: Add optional Orfanidis-based variant that respects DC gain constraint.
func ButterworthHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.ButterworthHP(freq, order, sampleRate)
}

// Chebyshev1LP designs a lowpass Chebyshev Type I cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev1LP.CalculateCoefficients.
//
// Future: Add optional Orfanidis-based variant that respects Nyquist gain constraint.
func Chebyshev1LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev1LP(freq, order, rippleDB, sampleRate)
}

// Chebyshev1HP designs a highpass Chebyshev Type I cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev1HP.CalculateCoefficients.
//
// Future: Add optional Orfanidis-based variant that respects DC gain constraint.
func Chebyshev1HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev1HP(freq, order, rippleDB, sampleRate)
}

// BesselLP designs a lowpass Bessel (Thomson) cascade.
// The Bessel filter has maximally flat group delay in the passband.
// Supported orders: 1 to 10. Returns nil for unsupported or invalid parameters.
//
// For odd orders, the final section is first-order (B2=A2=0).
func BesselLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.BesselLP(freq, order, sampleRate)
}

// BesselHP designs a highpass Bessel (Thomson) cascade.
// The Bessel filter has maximally flat group delay in the passband.
// Supported orders: 1 to 10. Returns nil for unsupported or invalid parameters.
//
// For odd orders, the final section is first-order (B2=A2=0).
func BesselHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.BesselHP(freq, order, sampleRate)
}

// Chebyshev2LP designs a lowpass Chebyshev Type II (inverse Chebyshev) cascade.
//
// Uses an analog prototype with inverted Chebyshev Type I poles and
// imaginary-axis zeros, followed by bilinear transform. The passband is
// maximally flat and the stopband exhibits equiripple behavior.
func Chebyshev2LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev2LP(freq, order, rippleDB, sampleRate)
}

// Chebyshev2HP designs a highpass Chebyshev Type II (inverse Chebyshev) cascade.
//
// Applies an LP-to-HP frequency transformation to the analog prototype
// before bilinear transform. The passband is maximally flat and the
// stopband exhibits equiripple behavior.
func Chebyshev2HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev2HP(freq, order, rippleDB, sampleRate)
}

// EllipticLP designs a lowpass elliptic (Cauer) cascade.
//
// rippleDB controls passband ripple in dB, and stopbandDB controls minimum
// stopband attenuation in dB.
func EllipticLP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	return pass.EllipticLP(freq, order, rippleDB, stopbandDB, sampleRate)
}

// EllipticHP designs a highpass elliptic (Cauer) cascade.
//
// rippleDB controls passband ripple in dB, and stopbandDB controls minimum
// stopband attenuation in dB.
func EllipticHP(freq float64, order int, rippleDB, stopbandDB, sampleRate float64) []biquad.Coefficients {
	return pass.EllipticHP(freq, order, rippleDB, stopbandDB, sampleRate)
}

// LinkwitzRileyLP designs a lowpass Linkwitz-Riley cascade of the given order.
//
// A Linkwitz-Riley filter of order 2N is constructed by cascading two
// Butterworth filters of order N. At the crossover frequency the magnitude
// is -6.02 dB.
//
// The order must be a positive even integer (2, 4, 6, 8, …). Returns nil
// for invalid parameters.
//
// For orders divisible by 4 (LR4, LR8, …), summing the LP and HP outputs
// directly yields an allpass response. For orders ≡ 2 mod 4 (LR2, LR6, …),
// the HP output must be inverted; use [LinkwitzRileyHPInverted] or the
// crossover package which handles polarity automatically.
func LinkwitzRileyLP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.LinkwitzRileyLP(freq, order, sampleRate)
}

// LinkwitzRileyHP designs a highpass Linkwitz-Riley cascade of the given order.
//
// A Linkwitz-Riley filter of order 2N is constructed by cascading two
// Butterworth filters of order N. At the crossover frequency the magnitude
// is -6.02 dB.
//
// The order must be a positive even integer (2, 4, 6, 8, …). Returns nil
// for invalid parameters.
//
// For orders divisible by 4, this output is in-phase with [LinkwitzRileyLP]
// and their sum is allpass. For orders ≡ 2 mod 4, the highpass is 180° out
// of phase; use [LinkwitzRileyHPInverted] for allpass summation.
func LinkwitzRileyHP(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.LinkwitzRileyHP(freq, order, sampleRate)
}

// LinkwitzRileyHPInverted designs a highpass Linkwitz-Riley cascade with
// inverted polarity for allpass summation with [LinkwitzRileyLP].
//
// For orders ≡ 2 mod 4 (LR2, LR6, LR10, …), the standard HP is 180° out
// of phase with the LP at the crossover. This function returns the HP with
// inverted polarity so that LP + HP_inv = allpass.
//
// For orders divisible by 4, the inversion is unnecessary (the standard HP
// already sums to allpass with LP), but this function still applies it.
func LinkwitzRileyHPInverted(freq float64, order int, sampleRate float64) []biquad.Coefficients {
	return pass.LinkwitzRileyHPInverted(freq, order, sampleRate)
}

// LinkwitzRileyNeedsHPInvert reports whether the given Linkwitz-Riley order
// requires HP polarity inversion for allpass summation. Returns true for
// orders ≡ 2 mod 4 (LR2, LR6, LR10, …).
func LinkwitzRileyNeedsHPInvert(order int) bool {
	return pass.LinkwitzRileyNeedsHPInvert(order)
}
