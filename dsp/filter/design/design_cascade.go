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

// Chebyshev2LP designs a lowpass Chebyshev Type II cascade.
//
// The coefficient formulas are based on mfw legacy MFFilter.pas
// TMFDSPChebyshev2LP.CalculateCoefficients, with a corrected angle term:
// cos((2i+1)*pi/(2N)). The legacy code omits pi in that term.
//
// Future: Add optional Orfanidis-based variant that respects Nyquist gain constraint.
func Chebyshev2LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev2LP(freq, order, rippleDB, sampleRate)
}

// Chebyshev2HP designs a highpass Chebyshev Type II cascade.
//
// The coefficient formulas are ported from mfw legacy MFFilter.pas
// TMFDSPChebyshev2HP.CalculateCoefficients.
//
// Future: Add optional Orfanidis-based variant that respects DC gain constraint.
func Chebyshev2HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients {
	return pass.Chebyshev2HP(freq, order, rippleDB, sampleRate)
}
