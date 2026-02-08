// Package shelving provides higher-order parametric shelving filter designers.
//
// Butterworth designs use the Holters & Zölzer decomposition, which produces
// maximally-flat magnitude responses. Chebyshev Type I designs extend the same
// framework with elliptical pole placement, trading passband flatness for a
// steeper transition region. Chebyshev Type II designs use the Orfanidis
// parametric framework, providing equiripple in the flat (stopband) region
// while maintaining a maximally-flat shelf region.
//
// References:
//   - M. Holters and U. Zölzer, "Parametric Recursive Higher-Order Shelving
//     Filters," 120th AES Convention, Paris, 2006.
//   - S. J. Orfanidis, "High-Order Digital Parametric Equalizer Design,"
//     J. Audio Eng. Soc., vol. 53, no. 11, Nov. 2005.
//
// The designers return cascaded biquad sections as []biquad.Coefficients for
// use with dsp/filter/biquad.Chain at runtime.
package shelving
