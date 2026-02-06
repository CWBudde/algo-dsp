// Package weighting provides A, B, C, and Z frequency weighting filters
// per IEC 61672.
//
// Frequency weighting curves shape the magnitude response of a signal to
// approximate the frequency-dependent sensitivity of human hearing.
// The standard defines four curves:
//
//   - A-weighting (6th order): approximates the 40-phon equal-loudness contour.
//     Most widely used for noise measurements (e.g., LAeq, LAmax).
//   - B-weighting (5th order): approximates the 70-phon equal-loudness contour.
//     Rarely used in modern practice.
//   - C-weighting (4th order): approximates the 100-phon equal-loudness contour.
//     Used for peak measurements and C-A difference calculations.
//   - Z-weighting (zero-weighting): unity gain at all frequencies, a flat
//     reference defined in IEC 61672:2003 to replace the unweighted "Linear"
//     designation.
//
// All filters are normalized to 0 dB at the 1 kHz reference frequency.
// The returned [biquad.Chain] can be used for both real-time sample-by-sample
// processing and offline block processing.
//
// The implementation uses the bilinear transform of the IEC 61672 analog
// prototype poles to compute digital biquad coefficients. Each weighting
// curve is decomposed into second-order and first-order high-pass sections
// cascaded via [biquad.Chain].
package weighting
