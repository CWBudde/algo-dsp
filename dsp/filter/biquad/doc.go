// Package biquad provides biquad (second-order IIR) filter runtime primitives.
//
// A [Section] implements Direct Form II Transposed processing for a single
// second-order section defined by [Coefficients]. Multiple sections can be
// cascaded via [Chain] for higher-order filters (Butterworth, Chebyshev, etc.).
//
// This package provides the processing runtime only. Coefficient design
// (Butterworth, Chebyshev, parametric EQ, etc.) lives in dsp/filter/design.
package biquad
