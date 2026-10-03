// Package harmony estimates the key and the chord chart of music from chroma
// (pitch-class) features, such as the frame chroma of
// [github.com/cwbudde/algo-dsp/measure/music/melody].
//
// # Pipeline
//
//  1. [Windows] pools frame chroma over time spans (typically beats, half
//     bars or bars of a beat grid). Frame chroma is normalised per frame, so
//     each frame is weighted by its RMS. Optional bass evidence (bass-stem
//     chroma weighted by voicing, and bass notes weighted by overlap and
//     strength) and the span's RMS level complete each [Window].
//  2. [EstimateKey] correlates a chroma profile, typically the pooled chroma
//     of the whole piece (see [Normalize] to mix in bass notes), with the
//     Krumhansl–Kessler major and minor profiles in all 24 rotations. When
//     the best key and its relative major or minor correlate within a margin
//     (0.05), tonic evidence such as the bass at section edges decides, and
//     the key is flagged ambiguous.
//  3. [Chords] scores every window against 12 roots × chord templates
//     (major, minor, dominant, major and minor seventh by default) by cosine
//     similarity, adds a bass bonus for the root (0.2) or, at half weight, for
//     another chord tone (inversions), a bonus for chords in the key (0.05)
//     and a penalty for seventh chords (0.03). Quiet (below −50 dBFS) or flat
//     (max/mean < 1.5) windows are "N". A Viterbi pass with a switch penalty
//     (0.10) smooths the sequence, a bass on a non-root chord tone makes a
//     slash chord, and equal neighbours merge. [Chorder] does the same
//     without allocating once its buffers have grown.
//
// The package has no notion of bars or beats: the caller chooses the spans and
// labels the resulting chords with its own grid.
//
// # Provenance
//
// The defaults reproduce the harmony analysis of the AudioVisualizer project
// (github.com/cwbudde/AudioVisualizer, internal/story) bit for bit, including
// the floating-point operation order and the rounding of chord scores to
// three decimals. Key correlations, chord times and margins are not rounded;
// rounding them to three (six for times) decimals gives the original values.
//
// # Limitations
//
// The key estimate is a global template match: it reports one key for the
// whole profile and does not track modulations. The chord detector knows
// only the templates it is given, treats chroma as octave-free (no voicing or
// register), and its scores are similarity heuristics, not probabilities.
package harmony
