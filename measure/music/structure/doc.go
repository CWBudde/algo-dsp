// Package structure finds the form of a piece of music from a sequence of
// feature vectors: a self-similarity matrix, a novelty curve with section
// boundaries, and letter labels (A, B, A', …) for repeated and varied
// phrases.
//
// The package is feature-agnostic. Callers pass rows, one feature vector per
// beat or bar (chroma, bass chroma, band levels, stem levels, a drum grid,
// …), and the functions work in row indices. Times, bar positions and cue
// names stay with the caller.
//
// # Pipeline
//
//  1. [Blocks] mixes heterogeneous feature groups: each group is z-scored
//     per dimension ([ZScore]; constant dimensions become 0), every row of
//     the group is L2-normalised and scaled by the group weight, and the
//     groups are concatenated. The AudioVisualizer story analysis weights
//     beat chroma 1, bass chroma 0.7, band dB 0.5 and stem dB 0.5, and a
//     bar's 48-value drum grid 0.5.
//  2. [SelfSimilarity] (or [SelfSimilarityInto] without allocation) is the
//     cosine similarity of every pair of rows.
//  3. [FooteNovelty] slides a Gaussian-tapered checkerboard kernel along the
//     diagonal (half-width 8 beats by default; Foote, "Automatic audio
//     segmentation using a measure of audio novelty", ICME 2000). It is high
//     where the rows before t resemble each other, the rows after t resemble
//     each other, and the two groups differ.
//  4. [Peaks] picks boundaries: local maxima within ±radius (4) above
//     mean + σ·sd (σ = 1). [WithMarks] relates each boundary to the nearest
//     reference mark, for example a hand-set cue.
//  5. [Label] groups rows into units (phrases of 4 bars of a bar SSM, or
//     single bars) and gives each unit the letter of its most similar
//     earlier unit when the mean diagonal similarity reaches 0.6, a primed
//     variant from 0.35, and a new letter otherwise.
//
// The defaults ([DefaultHalfWidth], [DefaultPeakRadius], [DefaultPeakSigma],
// [DefaultPhraseRows], [DefaultSame], [DefaultVariant]) reproduce the
// structure analysis of the AudioVisualizer project
// (github.com/cwbudde/AudioVisualizer, internal/story) bit for bit,
// including the floating-point operation order. That analysis rounds its
// outputs to three or six decimals; this package does not.
//
// # Limits
//
// The kernel is truncated at the matrix edges, so the first and last rows
// of a homogeneous passage score up to a quarter of an ideal boundary
// (similarity +1 within and -1 across the sections); in a piece without
// strong boundaries these edge values can pass the peak threshold. Labels
// compare whole units on the diagonal, so a phrase that repeats shifted by a
// few rows, or at another length, is not recognised.
package structure
