// Package geq provides Orfanidis-style high-order graphic EQ band designers.
//
// The designers return cascaded biquad sections as []biquad.Coefficients for
// use with dsp/filter/biquad.Chain at runtime.
package geq
