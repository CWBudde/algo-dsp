// Package onset detects note and drum onsets from the frame features of
// measure/music/features and labels drum hits with a spectral-shape
// heuristic.
//
// # Detection
//
// [Detect] picks peaks of the positive spectral flux (features.Frames.Flux)
// with an adaptive threshold. Frame i is a candidate when
//
//   - its RMS is at least the RMS gate (0.001),
//   - Flux[i] >= 0.12·p95, with p95 the 95th percentile of the positive flux
//     values of the whole track,
//   - it is a local maximum (Flux[i] > Flux[i-1] and Flux[i] >= Flux[i+1]),
//   - Flux[i] >= 1.35 × the mean flux over frames i-25..i+25.
//
// The candidate's time is then refined to the start of the 5 ms block with
// the largest positive RMS rise within ±50 ms of the frame centre, which
// moves the event from the centre of a 2048-point analysis window to the
// actual attack. Its strength is min(1, Flux[i]/p95). Finally candidates
// closer than 75 ms are de-duplicated in order of decreasing strength,
// keeping the stronger one, and the result is sorted by time.
//
// Events are observations of spectral novelty, not instrument labels.
//
// # Drum labels
//
// [ClassifyDrums] labels events as kick, snare or hat from the band-energy
// shares of the onset frame and the next two frames. The default rules
// assume the five-band layout of features.DefaultBandEdges and are a
// heuristic for drum stems, not a classifier for full mixes.
package onset
