// Package shelving provides higher-order parametric shelving filter designers
// based on the Holters & Zölzer Butterworth decomposition.
//
// Reference: M. Holters and U. Zölzer, "Parametric Recursive Higher-Order
// Shelving Filters," presented at the 120th AES Convention, Paris, 2006.
//
// The designers return cascaded biquad sections as []biquad.Coefficients for
// use with dsp/filter/biquad.Chain at runtime.
package shelving
