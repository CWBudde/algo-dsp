// Package mixedphase designs finite impulse response filters whose delay and
// pre-ringing lie between the minimum- and linear-phase extremes.
//
// The package contains four complementary design methods:
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
//   - [DesignLowGroupDelay] prescribes no phase at all. It minimises the
//     weighted passband group delay subject to a magnitude tolerance, which is
//     the formulation Wu, Gao and Teo use.
//
// All four operate on a real prototype impulse response. Its magnitude
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
// [DesignLowGroupDelay] gives up the phase entirely and constrains the
// magnitude instead, so the caller states how much magnitude accuracy the delay
// is worth.
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
// # Optimised delay: what the local search can and cannot do
//
// [DesignLowGroupDelay] is a local method on a non-convex problem, and it
// behaves like one. Three properties are worth knowing before relying on it.
//
// It does improve on a truncated minimum-phase design, which is the usual
// answer when low delay is wanted. On the 65-tap low-pass used by the
// mixedphase example the mean passband group delay falls from about 12.70
// samples to 12.62 within a 1 dB tolerance and to 12.08 within 6 dB, and the
// peak passband delay falls from 23.58 to 22.72 and 21.18 respectively. The
// gain is real but modest: minimum phase is already close to delay-optimal for
// a fixed magnitude, and the room the optimiser has is exactly the room the
// tolerance grants it.
//
// The starting point decides which optimum is reached. Started from the
// linear-phase prototype instead, the same 1 dB design ends at a mean delay of
// about 31.5 samples rather than 12.6, and it is a genuine local minimum: it
// improves on its own start and cannot be pushed further. Leaving that basin
// would mean moving a zero across the unit circle, and no descent direction
// does that. Start from a minimum-phase or short-delay design unless there is a
// reason not to.
//
// Convergence is slow rather than sharp. The objective keeps falling with the
// iteration budget instead of settling: at 6 dB tolerance the mean delay
// reaches 12.35 after 50 iterations per penalty stage, 12.08 after 200 and
// 10.79 after 800, while the relative magnitude error climbs from 1.2e-2 to
// 2.5e-2 to 1.4e-1. The budget is therefore a second delay-versus-accuracy
// dial, not a convergence threshold, and results should always be quoted with
// the budget that produced them.
//
// The design also costs far more than the other three. Every objective
// evaluation touches every grid bin and every tap, and there are hundreds of
// evaluations per penalty stage, so a large FFT size is expensive in a way it
// is not for the transform-based methods.
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
