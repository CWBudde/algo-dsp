// Package align checks that a set of parts (for example separated stems)
// sums back to a reference signal (the mix) and is time-aligned with it.
//
// [Check] down-mixes the reference and every part to mono by averaging
// their channels, sums the parts, and reports
//
//   - the normalized correlation of reference and sum at lag 0,
//   - the lag within ±maxLag with the highest correlation (searched on every
//     stride-th sample, 4 by default, which is plenty for a broadband signal
//     and four times cheaper),
//   - the RMS of the residual reference - sum, linear and in dBFS.
//
// A well separated, aligned set has a lag-0 correlation close to 1, a best
// lag of 0 and a residual far below the signal level. Optionally Check
// rejects parts whose length differs from the reference by more than a
// tolerance.
//
// [Lag] and [LagChannels] compare one signal with a reference (for example
// a decoded render with its source) and report the lag found by a coarse
// strided search plus a fine search around it, the normalized correlation
// and the gain in dB at that lag.
package align
