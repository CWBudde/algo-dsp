// Package design provides digital IIR filter coefficient designers.
//
// The functions in this package produce biquad coefficients consumable by
// dsp/filter/biquad for runtime processing. It includes both RBJ-style
// designers (Lowpass, Highpass, Peak, etc.) and Orfanidis-style peaking EQ
// with prescribed DC/Nyquist gain via functional options on [Peak].
//
// The sub-package design/band provides high-order graphic EQ band designers
// (Butterworth, Chebyshev, Elliptic) returning cascaded biquad sections.
//
// # Undesignable filters are transparent, not silent
//
// The designers that return a single biquad.Coefficients value have no way to
// report an error. When a filter cannot be designed for the requested
// parameters — a non-positive or non-finite sample rate, or a frequency
// outside the open interval (0, Nyquist) — they return [biquad.Identity], the
// pass-through section B0 = 1, B1 = B2 = A1 = A2 = 0, so the offending stage
// passes audio through unchanged.
//
// They previously returned the zero Coefficients value. Because that has
// B0 = 0, its transfer function is H(z) = 0: it output silence, and a single
// out-of-range band muted an entire cascade. Callers that need to know whether
// a design succeeded should validate the parameters themselves, or use the
// designers in design/band and design/shelving, which return an explicit
// error.
//
// This contract is chosen for filters used in series, which is what these
// designers are for: a stage that cannot be designed should get out of the way.
// It is deliberately the opposite of the right answer for a parallel filter
// bank whose bands are summed — there, an undesignable band must contribute
// nothing, and a pass-through section would leak the full-band input into the
// sum. Code building summed banks (see dsp/effects.cpgBandpass) therefore
// keeps returning a zero section on purpose; do not "fix" it to match this
// package.
//
// The cascade designers returning []biquad.Coefficients keep their existing
// contract: a nil slice means "no filter", which is unambiguous because an
// empty cascade cannot be mistaken for a mute. When such a designer accepts
// the requested order but not the requested frequency, its sections are
// pass-through for the same reason as above.
package design
