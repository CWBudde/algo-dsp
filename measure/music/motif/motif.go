package motif

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// ErrInvalidOption reports an option with an out-of-range value, or a nil
// option. Errors for nil options wrap [ErrNilOption] as well.
var ErrInvalidOption = errors.New("motif: invalid option")

// ErrNilOption reports a nil option. Errors wrapping it also wrap
// [ErrInvalidOption].
var ErrNilOption = errors.New("motif: nil option")

// ErrInvalidArgument reports an invalid argument: a grid not built by
// [rhythm.NewGrid], a nil motif or callback, the reserved source name
// [SourceChroma] for note motifs, note indices outside the note slice, a
// note range too long to scan, a non-finite energy, or a non-finite
// salience.
var ErrInvalidArgument = errors.New("motif: invalid argument")

// SourceChroma is the [Motif].Source of the motifs [FindChromaMotifs] finds.
// [Rank] numbers them separately and never picks them as leitmotifs, so it
// is reserved: [FindNoteMotifs] rejects it as a source name.
const SourceChroma = "chroma"

// Role is the function of a motif in the piece, assigned by [Score].
type Role string

// Roles assigned by [Score].
const (
	// RoleTheme marks a motif whose occurrences are mostly apart.
	RoleTheme Role = "theme"
	// RoleOstinato marks a motif whose occurrences mostly follow each other
	// within a bar: a riff or an accompaniment figure.
	RoleOstinato Role = "ostinato"
)

// Variant says how an occurrence relates to its motif's prototype.
type Variant string

// Variants of an [Occurrence].
const (
	// VariantExact is the prototype's shape at the reference level.
	VariantExact Variant = "exact"
	// VariantTransposed is the prototype's shape at another level.
	VariantTransposed Variant = "transposed"
	// VariantVaried is a changed shape at the reference level.
	VariantVaried Variant = "varied"
	// VariantTransposedVaried is a changed shape at another level.
	VariantTransposedVaried Variant = "transposed+varied"
)

// Occurrence is one statement of a motif.
type Occurrence struct {
	// MotifID is the motif's ID, assigned by [Rank]; empty before.
	MotifID string
	// Start and End are the occurrence's time span in seconds, rounded to
	// 1 µs. Grid windows and chroma windows span their slots or beats;
	// n-grams span the first note's start to the last note's end.
	Start, End float64
	// Bar is the grid bar containing the start.
	Bar int
	// Slot is the grid slot of the start: the window start, the first
	// note's slot, or the first beat's slot.
	Slot int
	// Transposition is the interval in semitones, folded into [-6, 5], from
	// the motif's first occurrence (which is 0) to this one. Folding makes it
	// insensitive to the octave errors of pitch trackers.
	Transposition int
	// Similarity to the cluster's prototype, rounded to 0.001.
	Similarity float64
	// Variant classifies the occurrence against the prototype.
	Variant Variant
	// NoteIndices are the indices of the occurrence's notes in the note
	// slice passed to [FindNoteMotifs], in time order; nil for chroma motifs.
	NoteIndices []int
}

// SalienceTerms are the factors of a motif's salience, each rounded to
// 0.001 (Confirmed is exact). [Motif].Salience is their product.
type SalienceTerms struct {
	// Count is ln(1 + occurrences).
	Count float64
	// Sections is ln(1 + sections visited by an occurrence start).
	Sections float64
	// Prominence is the mean energy over the occurrences × the mean
	// occurrence similarity × (note motifs only) the mean note strength
	// relative to the source's median note strength.
	Prominence float64
	// Distinctiveness is the Shannon entropy in bits of the motif's
	// intervals (note motifs) or pitch classes (chroma motifs), capped at 1
	// and scaled by 0.3 below 1: repeated notes and two-note trills are
	// rarely what a listener remembers.
	Distinctiveness float64
	// Span is the motif span in beats, capped at 8, divided by 8.
	Span float64
	// Confirmed is 1.2 for a motif confirmed by chroma, else 1.
	Confirmed float64
}

// Motif is a recurring figure with its occurrences.
type Motif struct {
	// ID is "M1", "M2", … for note motifs and "C1", … for chroma motifs in
	// order of salience, assigned by [Rank]; empty before.
	ID string
	// Source names the note line the motif was found in (the source passed
	// to [FindNoteMotifs]) or is [SourceChroma].
	Source string
	// Role is assigned by [Score]; empty before.
	Role Role
	// Notes are the prototype's note names (such as "G4", MIDI 60 is C4) at
	// the first occurrence's level and register for note motifs, and the
	// strongest pitch class of each beat (such as "G") for chroma motifs.
	Notes []string
	// SpanBeats is the prototype's span in beats, rounded to 0.001.
	SpanBeats float64
	// Intervals are the prototype's successive intervals in semitones (note
	// motifs only).
	Intervals []int
	// IOI are the prototype's inter-onset intervals in slots, at least 1
	// (note motifs only).
	IOI []int
	// PrototypeMIDI are the MIDI notes named by Notes (note motifs only).
	PrototypeMIDI []int
	// Occurrences are in time order. The first one has Transposition 0.
	Occurrences []Occurrence
	// Salience is the product of SalienceTerms before their rounding,
	// rounded to 0.001; set by [Score].
	Salience float64
	// SalienceTerms are the factors of Salience; set by [Score].
	SalienceTerms SalienceTerms
	// ConfirmedByChroma is set by [Corroborate].
	ConfirmedByChroma bool
	// Leitmotif is set by [Rank] for the picked leitmotifs.
	Leitmotif bool
	// Rank is the 1-based rank of a leitmotif among the leitmotifs, by
	// salience; 0 for other motifs. Set by [Rank].
	Rank int
}

// Span is a named time range in seconds, such as a song section, used by
// [Score] to count the sections a motif visits. Start is inclusive and End
// exclusive.
type Span struct {
	Name       string
	Start, End float64
}

func validGrid(grid rhythm.Grid) error {
	err := grid.Validate()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	return nil
}

// noteName names a MIDI note with sharps and MIDI 60 as C4.
func noteName(midi int) string {
	return fmt.Sprintf("%s%d", pitchClassName(midi), midi/12-1)
}

func pitchClassName(n int) string { return pitch.PitchClass((n%12 + 12) % 12).String() }

// fold maps a transposition to [-6, 5] semitones: the tracker's octave
// errors make octave and direction unreliable.
func fold(t int) int { return ((t%12)+18)%12 - 6 }

func variant(exact bool, t int) Variant {
	switch {
	case exact && t == 0:
		return VariantExact
	case exact:
		return VariantTransposed
	case t == 0:
		return VariantVaried
	}

	return VariantTransposedVaried
}

func r6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func r3(v float64) float64 { return math.Round(v*1e3) / 1e3 }
