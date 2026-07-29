// Package mixedphase designs finite impulse response filters whose delay and
// pre-ringing lie between the minimum- and linear-phase extremes.
//
// The package contains two complementary design methods:
//
//   - [DesignIterative] implements the alternating factorisation proposed by
//     Christian-W. Budde at DAGA 2012. It factors a target magnitude response
//     into a causal minimum-phase part and a short linear-phase residual while
//     repeatedly compensating the truncation error of each part.
//   - [DesignPhaseInterpolation] constructs a complex target response by
//     interpolating between minimum and linear phase, then projects it onto a
//     finite causal support. It is useful as a simple comparison baseline.
//
// Both methods operate on a real prototype impulse response. Its magnitude
// response is the design target; its original phase is not otherwise used.
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
