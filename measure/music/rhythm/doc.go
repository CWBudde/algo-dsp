// Package rhythm estimates tempo, beat grid and downbeat from the frame
// features of measure/music/features.
//
// # Tempo
//
// [Novelty] turns the spectral flux into a rhythmic novelty curve: flux
// minus its moving mean over ±radius frames (30 in the AudioVisualizer),
// half-wave rectified. [TempoScore] rates a tempo by the normalized
// autocorrelation of the novelty at 1, 2, 4 and 8 beat periods, with linear
// interpolation for fractional lags. [EstimateTempo] scans a BPM range (60–180
// in 0.5 BPM steps by default), keeps the best-scoring candidates at least 2
// BPM apart (six by default), and refines the tempo in 0.01 BPM steps within
// ±3 BPM of either an explicit prior ([WithPrior]) or the best candidate.
// A prior is always an explicit input; it is never built in.
//
// # Beat grid
//
// [FitBeatPhase] places the beat grid by maximizing, in 1 ms steps, the sum
// of the positive changes of a low-band envelope (kick-like energy) sampled
// at the beat times. [BeatGrid] lists the beat times and [GridError]
// measures how well onsets fit a subdivided grid (median distance).
//
// # Downbeat
//
// [Downbeat] chooses which of the first beatsPerBar beats starts a bar by
// summing accent weights (for example kick and bass onset strengths) that
// fall within a tolerance of each beat.
//
// All functions are deterministic.
package rhythm
