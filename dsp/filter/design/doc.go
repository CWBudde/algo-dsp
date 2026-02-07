// Package design provides digital IIR filter coefficient designers.
//
// The functions in this package produce biquad coefficients consumable by
// dsp/filter/biquad for runtime processing. It includes both RBJ-style
// designers (Lowpass, Highpass, Peak, etc.) and Orfanidis-style peaking EQ
// with prescribed DC/Nyquist gain via functional options on [Peak].
//
// The sub-package design/band provides high-order graphic EQ band designers
// (Butterworth, Chebyshev, Elliptic) returning cascaded biquad sections.
package design
