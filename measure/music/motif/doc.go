// Package motif finds recurring musical figures (motifs, leitmotifs and
// ostinatos) with their transposed and varied returns, from cleaned note
// lines or from beat-level chroma, and ranks them by salience.
//
// # Pipeline
//
// The inputs are monophonic note lines quantised to a [rhythm.Grid] (the
// output of [melody.Clean], typically a lead and a bass line), beat chroma
// for the harmonic fallback, the song's sections and an energy function per
// source:
//
//	chroma, _ := motif.FindChromaMotifs(beatChroma, grid)
//	var all []motif.Motif
//	for _, src := range sources {
//		found, _ := motif.FindNoteMotifs(src.notes, grid, src.name)
//		for i := range found {
//			_ = motif.Corroborate(&found[i], chroma, grid)
//			_ = motif.Score(&found[i], src.notes, sections, src.energy, grid)
//		}
//		all = append(all, found...)
//	}
//	for i := range chroma {
//		_ = motif.Score(&chroma[i], nil, sections, harmonyEnergy, grid)
//	}
//	all = append(all, chroma...)
//	ranked, _ := motif.Rank(all)
//
// The steps:
//
//  1. [FindNoteMotifs] runs two passes over a note line. Grid windows of a
//     bar and half a bar are compared by the Dice overlap of their notes
//     paired by slot position and pitch class under the best transposition,
//     which tolerates the dropped notes and octave errors of a pitch
//     tracker. N-grams of 8, 6 and 4 notes are then compared by a
//     transposition-invariant distance of their intervals (octave-tolerant)
//     and inter-onset ratios. Candidates are clustered greedily into
//     non-overlapping occurrences; rotations of a repeating figure and
//     shorter figures mostly covered by longer ones are suppressed.
//  2. [FindChromaMotifs] groups beat-chroma windows of 8 and 4 beats under
//     the optimal transposition index (the rotation maximising the mean
//     per-beat cosine), for material without reliable notes.
//  3. [Corroborate] marks note motifs whose occurrences coincide with those
//     of one chroma motif at a consistent relative transposition.
//  4. [Score] computes the salience from the occurrence count, the sections
//     visited, the prominence (energy, similarity and note strength), the
//     interval entropy, the span and the chroma confirmation, and assigns
//     the theme or ostinato role.
//  5. [Rank] orders the motifs, assigns IDs and picks the leitmotifs: the
//     best theme, the best ostinato, then the rest by salience, with an
//     overlap limit and a cap per source.
//
// # Parameters and parity
//
// All functions take the same [Option] type; each documents the options it
// reads. The defaults are AudioVisualizer's DefaultMotifParams (the
// PixelParade story analysis), and with them the package reproduces that
// analysis bit for bit: the same motifs, IDs, occurrence order,
// similarities and salience. The one deliberate difference is that [Score]
// sums the entropy terms in a fixed order where the original iterates a Go
// map; this changes the unrounded entropy by at most a few ulps and the
// rounded salience practically never.
//
// Times are in seconds, positions in grid slots (sixteenths by default) and
// pitches in MIDI note numbers. The functions are deterministic and safe for
// concurrent use on distinct motifs.
package motif
