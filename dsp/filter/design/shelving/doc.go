// Package shelving provides higher-order parametric shelving filter designers.
//
// Four families are available, in increasing order of transition steepness and
// of ripple accepted in exchange:
//
//   - Butterworth: monotonic in both regions (Holters & Zölzer decomposition).
//   - Chebyshev Type I: equiripple across the shelf, monotonic in the flat
//     region.
//   - Chebyshev Type II: equiripple in the flat region, monotonic across the
//     shelf, which reaches the full gain exactly.
//   - Elliptic (Cauer): equiripple in both regions, giving the steepest
//     transition for a given order.
//
// Chebyshev Type II and elliptic are built on the Orfanidis parametric-equalizer
// prototypes in internal/orfanidis, shared with the band designers.
//
// # Cutoff convention
//
// All four families define freqHz identically, by
//
//	|H(freqHz)|² = (G² + 1)/2,  G = 10^(gainDB/20)
//
// i.e. roughly 3 dB below the shelf gain (Holters & Zölzer, eq. 5), so
// switching family keeps the transition band in place. A consequence noted in
// that paper (§2.3, Fig. 3) is that this convention is asymmetric between boost
// and cut: cascading a +g dB shelf with a −g dB shelf at the same cutoff does
// not give unity through the transition region. Exact boost/cut reciprocity
// would require the alternative sqrt(G) cutoff of their eq. (11), which this
// package does not use.
//
// # Ripple parameters
//
// The Chebyshev I designers take rippleDB, the ripple bound across the shelf.
// The Chebyshev II and elliptic designers take stopbandDB, the ripple bound in
// the flat 0 dB region. The elliptic designers additionally hold the shelf-side
// ripple at a fixed 0.05 dB, matching the band elliptic designer.
//
// Even orders terminate on the ripple bound at DC and Nyquist, odd orders on
// the nominal gain and 0 dB, as is standard for equiripple designs.
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
