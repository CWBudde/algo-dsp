// Package orfanidis provides Orfanidis-style parametric EQ designers.
//
// This package focuses on coefficient design and returns biquad.Coefficients
// for use with dsp/filter/biquad at runtime. It complements the RBJ-style
// designers in dsp/filter/design by supporting explicit DC/Nyquist constraints
// and prescribed band-edge gain.
package orfanidis
