// Package mixedphase designs finite impulse response filters whose delay and
// pre-ringing lie between the minimum- and linear-phase extremes.
//
// The package contains three complementary design methods:
//
//   - [DesignIterative] implements the alternating factorisation proposed by
//     Christian-W. Budde at DAGA 2012. It factors a target magnitude response
//     into a causal minimum-phase part and a short linear-phase residual while
//     repeatedly compensating the truncation error of each part.
//   - [DesignPhaseInterpolation] constructs a complex target response by
//     interpolating between minimum and linear phase, then projects it onto a
//     finite causal support. It is useful as a simple comparison baseline.
//   - [DesignComplexLeastSquares] approximates that same complex target under a
//     frequency weight, optionally reweighted towards the minimax solution.
//
// All three operate on a real prototype impulse response. Its magnitude
// response is the design target; its original phase is not otherwise used.
//
// # Prescribed phase versus optimised phase
//
// The methods differ in what they treat as given. [DesignIterative] derives the
// phase distribution from a latency budget: the caller states a delay, and the
// split between the two factors follows. [DesignPhaseInterpolation] and
// [DesignComplexLeastSquares] instead prescribe the entire complex response and
// then approximate it, so the caller states a phase curve — here parametrised
// by the same mix between minimum and linear phase — and the design only
// controls how the unavoidable approximation error is distributed.
//
// Among the two prescribing methods, phase interpolation is the unweighted
// least-squares solution: on a uniform DFT grid with uniform weights the
// normal equations reduce to the identity, so truncating the inverse transform
// already minimises the mean-square complex error. Both designs then agree
// exactly. A non-uniform weight is what makes the least-squares route
// worthwhile, and Lawson reweighting turns it into a peak-error design.
//
// Because the weighted objective says nothing about bins whose weight is zero,
// the response there is genuinely unconstrained and can diverge by orders of
// magnitude while the weighted band is matched closely. Weight bands down
// rather than out.
//
// Both objectives measure the absolute complex deviation, not a relative one.
// Where the prescribed response is small the absolute error is small too, so an
// unweighted minimax design spends its budget on the passband and lets stopband
// accuracy slip: on the low-pass in the mixedphase example the reweighting
// halves the peak complex error while the dB magnitude error rises from about
// 36 dB to about 60 dB. Supply a weight that rises with the inverse target
// magnitude when stopband depth matters.
//
// # Minimum-phase reconstruction
//
// Both designs rest on a spectral factorisation that turns a sampled magnitude
// response into a causal minimum-phase spectrum. Two implementations are
// available through [MinimumPhaseMethod]:
//
//   - [MethodCepstrum] folds the real cepstrum onto its causal half and
//     exponentiates the complex log spectrum.
//   - [MethodHilbert] evaluates the discrete Hilbert transform of the log
//     magnitude to obtain the phase and pairs it with the target magnitude.
//
// The two are mathematically equivalent. They differ numerically because the
// cepstral route reconstructs the magnitude through the exponential, while the
// Hilbert route carries it through untouched. On a dense grid the Hilbert
// reconstruction therefore reproduces the target magnitude to machine precision
// regardless of the magnitude floor, whereas the cepstral deviation grows with
// the log-domain dynamic range (roughly 1e-10 relative at a floor of 1e-6 and
// 1e-6 at a floor of 1e-12 for a narrow low-pass).
//
// For a single reconstruction that difference is irrelevant in practice:
// truncating the dense response to the tap budget dominates, and both methods
// produce the same FIR to within 1e-10 of its peak. Inside [DesignIterative]
// the difference does matter, because every pass divides by a truncated factor
// and the ill-conditioned nulls of that division amplify it; the two methods
// then converge to designs whose errors differ by a few dB in either direction,
// with neither systematically ahead. Runtime and allocations are equivalent —
// both perform one inverse and one forward transform per reconstruction.
package mixedphase
