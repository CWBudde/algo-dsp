// Package spatial provides reusable non-I/O spatial audio effects.
//
// Included processors:
//   - StereoWidener: Mid/side stereo image widening and narrowing.
//   - StereoPanner: Equal-power, linear or compromise panning and stereo
//     balance, with an optional auto-pan LFO.
//   - HaasDelay: Short single-channel delay for precedence-based stereo widening.
//   - CrosstalkCanceller: Staged geometric crosstalk cancellation for speaker playback.
//   - CrosstalkSimulator: Delayed IIR-shaped stereo crossfeed simulation.
//   - HRTFCrosstalkSimulator: FIR HRTF-based direct/crossfeed stereo simulation.
package spatial
