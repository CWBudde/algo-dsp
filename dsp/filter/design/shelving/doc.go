// Package shelving provides higher-order parametric shelving filter designers.
//
// Butterworth designs use the Holters & Zölzer decomposition, which produces
// maximally-flat magnitude responses. Chebyshev Type I designs extend the same
// framework with elliptical pole placement, trading passband flatness for a
// steeper transition region.
//
// Reference: M. Holters and U. Zölzer, "Parametric Recursive Higher-Order
// Shelving Filters," presented at the 120th AES Convention, Paris, 2006.
//
// The designers return cascaded biquad sections as []biquad.Coefficients for
// use with dsp/filter/biquad.Chain at runtime.
package shelving
