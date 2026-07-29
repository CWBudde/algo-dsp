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
package mixedphase
