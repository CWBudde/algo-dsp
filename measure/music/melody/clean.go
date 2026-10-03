package melody

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// ErrInvalidNote reports a note passed to [Clean] whose times or strength
// are not finite, or whose times lie too far from the grid to be quantised.
var ErrInvalidNote = errors.New("melody: invalid note")

// Default note-cleanup parameters of [Clean]. They are AudioVisualizer's
// lead-line parameters (LeadCleanParams).
const (
	// DefaultCleanFloorMIDI is the default floor: notes at or below this
	// MIDI note (E3, the default lowest pitch candidate of [Analyze]) are
	// the tracker's pinned lowest candidate, bleed or octave errors.
	DefaultCleanFloorMIDI = 52
	// BassFloorMIDI is the floor of [BassCleanOptions] (E1, the lowest
	// candidate of [BassPreset]).
	BassFloorMIDI = 28
	// DefaultMinStrength is the default minimum note strength.
	DefaultMinStrength = 0.35
	// DefaultOffGridMS is the default distance in milliseconds from the
	// nearest slot above which a note is flagged off-grid.
	DefaultOffGridMS = 50.0
	// DefaultOctaveWindowBars is the default half-width in bars of the
	// neighbourhood whose median pitch an octave outlier is moved towards.
	DefaultOctaveWindowBars = 1.0
	// DefaultOctaveSpread is the default distance in semitones from the
	// neighbourhood median beyond which a note is moved by an octave.
	DefaultOctaveSpread = 19.0
	// DefaultNeighbourSpread is the default largest distance in semitones
	// between the two neighbours of a lone octave spike.
	DefaultNeighbourSpread = 2
	// DefaultOctavePeriodSlots is the default period in slots of the
	// octave vote (one 4/4 bar of sixteenths).
	DefaultOctavePeriodSlots = 16
	// DefaultOctavePeriodVotes is the default number of votes needed to
	// move a note by the period vote.
	DefaultOctavePeriodVotes = 2
	// DefaultArpMinRun is the default shortest run of short notes labelled
	// [VoiceArp].
	DefaultArpMinRun = 4
	// DefaultArpMaxGapSlots is the default largest gap in slots between
	// consecutive notes of an arpeggio run.
	DefaultArpMaxGapSlots = 2
	// DefaultArpMaxNoteSlots is the default longest note in slots that
	// counts as short for an arpeggio run.
	DefaultArpMaxNoteSlots = 2
)

// maxCleanSlot bounds the slot index of a note, so slot arithmetic stays
// exact and monotonic in time.
const maxCleanSlot = 1 << 50

// Voice labels a cleaned note as part of the lead line or of an arpeggio.
type Voice string

// Voices assigned by [Clean].
const (
	// VoiceLead marks a note outside an arpeggio run.
	VoiceLead Voice = "lead"
	// VoiceArp marks a note in a run of short, closely spaced notes.
	VoiceArp Voice = "arp"
)

// DropReason says why [Clean] dropped a note.
type DropReason string

// Drop reasons reported by [Clean].
const (
	// DropFloor marks a note at or below the floor MIDI note.
	DropFloor DropReason = "floor"
	// DropWeak marks a note below the minimum strength.
	DropWeak DropReason = "weak"
	// DropDuplicate marks a note that lost its slot to a stronger note.
	DropDuplicate DropReason = "duplicate"
)

// CleanNote is a cleaned note quantised to a [rhythm.Grid]. Start and End
// stay measured times; Slot and Slots are the quantised position.
type CleanNote struct {
	// Start and End are the note boundaries in seconds, rounded to 1 µs.
	// End is cut at the next note's Start.
	Start, End float64
	// Slot is the grid slot nearest to Start.
	Slot int
	// Slots is the length in slots: the slot of End minus Slot, at least 1
	// and cut at the next note's slot.
	Slots int
	// MIDI is the octave-corrected pitch.
	MIDI int
	// RawMIDI is the input pitch.
	RawMIDI int
	// OctaveShift is MIDI - RawMIDI, a multiple of 12.
	OctaveShift int
	// Strength is the input strength rounded to 0.001.
	Strength float64
	// Voice is [VoiceArp] for notes in an arpeggio run, else [VoiceLead].
	Voice Voice
	// OffsetMS is Start minus the time of Slot in milliseconds, rounded to
	// 0.001 ms.
	OffsetMS float64
	// OffGrid reports |OffsetMS| above the off-grid threshold.
	OffGrid bool
	// RawIndex is the index of the note in the input slice.
	RawIndex int
}

// DroppedNote is an input note that [Clean] dropped.
type DroppedNote struct {
	// Note is the input note with Start and End rounded to 1 µs and
	// Strength to 0.001, as [Clean] saw it.
	Note
	// Index is the index of the note in the input slice.
	Index int
	// Reason says why the note was dropped.
	Reason DropReason
}

// CleanOption configures [Clean]. Options return an error wrapping
// [ErrInvalidOption] for out-of-range values.
type CleanOption func(*cleanConfig) error

type cleanConfig struct {
	floorMIDI       int
	minStrength     float64
	offGridMS       float64
	windowBars      float64
	spread          float64
	neighbourSpread int
	periodSlots     int
	periodVotes     int
	arpMinRun       int
	arpMaxGap       int
	arpMaxNote      int
}

func defaultCleanConfig() cleanConfig {
	return cleanConfig{
		floorMIDI:       DefaultCleanFloorMIDI,
		minStrength:     DefaultMinStrength,
		offGridMS:       DefaultOffGridMS,
		windowBars:      DefaultOctaveWindowBars,
		spread:          DefaultOctaveSpread,
		neighbourSpread: DefaultNeighbourSpread,
		periodSlots:     DefaultOctavePeriodSlots,
		periodVotes:     DefaultOctavePeriodVotes,
		arpMinRun:       DefaultArpMinRun,
		arpMaxGap:       DefaultArpMaxGapSlots,
		arpMaxNote:      DefaultArpMaxNoteSlots,
	}
}

// WithFloorMIDI sets the floor MIDI note in [0, 127] (default
// [DefaultCleanFloorMIDI]; [BassFloorMIDI] for a bass line). Notes at or
// below it are dropped with [DropFloor]: a tracker's lowest pitch candidate
// collects bleed and octave errors rather than real notes.
func WithFloorMIDI(midi int) CleanOption {
	return func(cfg *cleanConfig) error {
		if midi < 0 || midi > 127 {
			return fmt.Errorf("%w: floor MIDI note must be in [0, 127], got %d", ErrInvalidOption, midi)
		}

		cfg.floorMIDI = midi

		return nil
	}
}

// WithMinStrength sets the minimum note strength, >= 0 (default
// [DefaultMinStrength]). Weaker notes are dropped with [DropWeak].
func WithMinStrength(strength float64) CleanOption {
	return func(cfg *cleanConfig) error {
		if !finite(strength) || strength < 0 {
			return fmt.Errorf("%w: minimum strength must be >= 0, got %g", ErrInvalidOption, strength)
		}

		cfg.minStrength = strength

		return nil
	}
}

// WithOffGridMS sets the distance in milliseconds from the nearest slot,
// >= 0, above which a note is flagged [CleanNote].OffGrid (default
// [DefaultOffGridMS]).
func WithOffGridMS(ms float64) CleanOption {
	return func(cfg *cleanConfig) error {
		if !finite(ms) || ms < 0 {
			return fmt.Errorf("%w: off-grid threshold must be >= 0 ms, got %g", ErrInvalidOption, ms)
		}

		cfg.offGridMS = ms

		return nil
	}
}

// WithOctaveWindow sets the median step of the octave correction: a note
// further than spread semitones from the duration-weighted median pitch of
// the notes starting within ±bars bars is moved by one octave towards it
// (defaults [DefaultOctaveWindowBars], [DefaultOctaveSpread]). Both must be
// finite and >= 0.
func WithOctaveWindow(bars, spread float64) CleanOption {
	return func(cfg *cleanConfig) error {
		if !finite(bars) || bars < 0 {
			return fmt.Errorf("%w: octave window must be >= 0 bars, got %g", ErrInvalidOption, bars)
		}

		if !finite(spread) || spread < 0 {
			return fmt.Errorf("%w: octave spread must be >= 0 semitones, got %g", ErrInvalidOption, spread)
		}

		cfg.windowBars, cfg.spread = bars, spread

		return nil
	}
}

// WithNeighbourSpread sets the largest distance in semitones, >= 0, between
// the two neighbours of a lone octave spike for the spike to be folded
// (default [DefaultNeighbourSpread]). The spike must lie 11 to 13 semitones
// from both neighbours, so they can never be more than 2 semitones apart and
// larger values act like 2.
func WithNeighbourSpread(semitones int) CleanOption {
	return func(cfg *cleanConfig) error {
		if semitones < 0 {
			return fmt.Errorf("%w: neighbour spread must be >= 0 semitones, got %d", ErrInvalidOption, semitones)
		}

		cfg.neighbourSpread = semitones

		return nil
	}
}

// WithOctavePeriod sets the period vote of the octave correction: notes one
// and two periods of slots away vote on the octave, and a shift needs at
// least minVotes votes (defaults [DefaultOctavePeriodSlots],
// [DefaultOctavePeriodVotes]). Both must be >= 1.
func WithOctavePeriod(slots, minVotes int) CleanOption {
	return func(cfg *cleanConfig) error {
		if slots < 1 || minVotes < 1 {
			return fmt.Errorf("%w: octave period %d slots, %d votes (both must be >= 1)", ErrInvalidOption, slots, minVotes)
		}

		cfg.periodSlots, cfg.periodVotes = slots, minVotes

		return nil
	}
}

// WithArpeggio sets the arpeggio split: a run of at least minRun (>= 1)
// notes, each at most maxNoteSlots (>= 1) slots long and separated by at
// most maxGapSlots (>= 0) slots, is labelled [VoiceArp] (defaults
// [DefaultArpMinRun], [DefaultArpMaxGapSlots], [DefaultArpMaxNoteSlots]).
func WithArpeggio(minRun, maxGapSlots, maxNoteSlots int) CleanOption {
	return func(cfg *cleanConfig) error {
		if minRun < 1 || maxGapSlots < 0 || maxNoteSlots < 1 {
			return fmt.Errorf("%w: arpeggio run %d, gap %d slots, note %d slots", ErrInvalidOption, minRun, maxGapSlots, maxNoteSlots)
		}

		cfg.arpMinRun, cfg.arpMaxGap, cfg.arpMaxNote = minRun, maxGapSlots, maxNoteSlots

		return nil
	}
}

// Clean turns tracker notes (for example [Result].Notes) into a monophonic,
// grid-quantised note list:
//
//  1. Start and End are rounded to 1 µs and Strength to 0.001. Notes at or
//     below the floor ([WithFloorMIDI]) or below the minimum strength
//     ([WithMinStrength]) are dropped.
//  2. Each start is quantised to the nearest slot of grid; notes more than
//     [WithOffGridMS] from it are flagged off-grid.
//  3. One note per slot is kept, the one with the larger strength ×
//     duration (the earlier one on ties); the others are dropped as
//     duplicates. Each note ends no later than the next one starts.
//  4. Octave errors are corrected in three steps. First, repeating material
//     votes: the notes one and two periods away ([WithOctavePeriod]) whose
//     pitch differs by a multiple of an octave (at most two) vote for that
//     difference; the most voted shift is applied if it has the minimum
//     votes, and a note confirmed by the vote (shift 0) is left alone.
//     Second, an unsettled note further than the spread from the
//     duration-weighted median pitch of the notes within the window
//     ([WithOctaveWindow], at least three notes) is moved one octave towards
//     it. Third, a remaining note an octave (±1 semitone) above or below
//     both neighbours, which lie within 4 slots and agree within
//     [WithNeighbourSpread], is folded onto them.
//  5. Runs of short, closely spaced notes are labelled [VoiceArp], the
//     rest [VoiceLead] ([WithArpeggio]).
//
// The cleaned notes are ordered by slot, and the dropped notes by input
// index; both slices are non-nil. Notes with non-finite times or strength,
// or with a slot beyond ±2^50, return an error wrapping [ErrInvalidNote]; an
// invalid grid returns an error wrapping [ErrInvalidArgument] and
// [rhythm.ErrInvalidArgument].
//
// The defaults reproduce AudioVisualizer's CleanNotes with LeadCleanParams
// bit for bit.
func Clean(notes []Note, grid rhythm.Grid, opts ...CleanOption) ([]CleanNote, []DroppedNote, error) {
	cfg := defaultCleanConfig()

	for i, opt := range opts {
		if opt == nil {
			return nil, nil, fmt.Errorf("%w: clean option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, nil, err
		}
	}

	err := grid.Validate()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	}

	for i, n := range notes {
		if !finite(n.Start) || !finite(n.End) || !finite(n.Strength) {
			return nil, nil, fmt.Errorf("%w: note %d %+v", ErrInvalidNote, i, n)
		}

		if n.End < n.Start {
			return nil, nil, fmt.Errorf("%w: note %d ends (%g s) before it starts (%g s)", ErrInvalidNote, i, n.End, n.Start)
		}

		if !onGrid(grid, n.Start) || !onGrid(grid, n.End) {
			return nil, nil, fmt.Errorf("%w: note %d (%g..%g s) is too far from the grid", ErrInvalidNote, i, n.Start, n.End)
		}
	}

	reasons := make([]DroppedNote, len(notes))
	cleaned := make([]CleanNote, 0, len(notes))

	for i, n := range notes {
		n.Start, n.End, n.Strength = round6(n.Start), round6(n.End), round3(n.Strength)
		reasons[i] = DroppedNote{Note: n, Index: i}

		switch {
		case n.MIDI <= cfg.floorMIDI:
			reasons[i].Reason = DropFloor

			continue
		case n.Strength < cfg.minStrength:
			reasons[i].Reason = DropWeak

			continue
		}

		slot := grid.Slot(n.Start)
		offset := (n.Start - grid.SlotTime(slot)) * 1000
		cleaned = append(cleaned, CleanNote{
			Start: n.Start, End: n.End, Slot: slot, Slots: max(1, grid.Slot(n.End)-slot),
			MIDI: n.MIDI, RawMIDI: n.MIDI, Strength: n.Strength,
			OffsetMS: round3(offset), OffGrid: math.Abs(offset) > cfg.offGridMS, RawIndex: i,
		})
	}

	slices.SortStableFunc(cleaned, func(a, b CleanNote) int { return cmp.Compare(a.Slot, b.Slot) })

	mono := monophonic(cleaned, reasons)
	correctOctaves(mono, grid, &cfg)
	splitVoices(mono, &cfg)

	dropped := make([]DroppedNote, 0, len(notes)-len(mono))

	for _, d := range reasons {
		if d.Reason != "" {
			dropped = append(dropped, d)
		}
	}

	return mono, dropped, nil
}

// onGrid reports whether t quantises to a slot within ±maxCleanSlot.
func onGrid(grid rhythm.Grid, t float64) bool {
	return math.Abs((t-grid.Origin())/grid.SlotSeconds()) <= maxCleanSlot
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
func round3(v float64) float64 { return math.Round(v*1e3) / 1e3 }

// monophonic keeps one note per slot of the slot-sorted notes, in place:
// the one with the larger strength × duration, the earlier one on ties. The
// losers are marked as duplicates in reasons. Each kept note then ends no
// later than the next one starts.
func monophonic(notes []CleanNote, reasons []DroppedNote) []CleanNote {
	mono := notes[:0]

	for _, n := range notes {
		if k := len(mono) - 1; k >= 0 && mono[k].Slot == n.Slot {
			if n.Strength*(n.End-n.Start) > mono[k].Strength*(mono[k].End-mono[k].Start) {
				reasons[mono[k].RawIndex].Reason = DropDuplicate
				mono[k] = n
			} else {
				reasons[n.RawIndex].Reason = DropDuplicate
			}

			continue
		}

		mono = append(mono, n)
	}

	for i := 0; i+1 < len(mono); i++ {
		next := mono[i+1]
		mono[i].End = math.Min(mono[i].End, next.Start)
		mono[i].Slots = max(1, min(mono[i].Slots, next.Slot-mono[i].Slot))
	}

	return mono
}

// octaveVote returns the index of the note at slot in the slot-sorted
// notes, or -1.
func octaveVote(notes []CleanNote, slot int) int {
	j, ok := slices.BinarySearchFunc(notes, slot, func(n CleanNote, s int) int { return cmp.Compare(n.Slot, s) })
	if !ok {
		return -1
	}

	return j
}

// weightedPitch is a neighbour pitch weighted by its duration.
type weightedPitch struct{ midi, weight float64 }

// correctOctaves runs the three octave-correction steps of [Clean] on
// monophonic, slot-sorted notes and sets OctaveShift.
func correctOctaves(notes []CleanNote, grid rhythm.Grid, cfg *cleanConfig) {
	pitch := make([]int, len(notes))
	for i, n := range notes {
		pitch[i] = n.MIDI
	}

	settled := periodVote(notes, pitch, cfg)
	medianStep(notes, pitch, settled, cfg.windowBars*grid.BarSeconds(), cfg.spread)

	for i := 1; i+1 < len(notes); i++ {
		prev, n, next := notes[i-1], notes[i], notes[i+1]
		if settled[i] || n.Slot-prev.Slot > 4 || next.Slot-n.Slot > 4 || absInt(prev.MIDI-next.MIDI) > cfg.neighbourSpread {
			continue
		}

		a, b := n.MIDI-prev.MIDI, n.MIDI-next.MIDI
		if absInt(absInt(a)-12) <= 1 && absInt(absInt(b)-12) <= 1 && (a > 0) == (b > 0) {
			notes[i].MIDI -= 12 * signInt(a)
		}
	}

	for i := range notes {
		notes[i].OctaveShift = notes[i].MIDI - notes[i].RawMIDI
	}
}

// periodVote is the first octave step: notes one and two periods away whose
// raw pitch differs by 0, ±12 or ±24 semitones vote for that shift. It
// returns which notes the vote settled.
func periodVote(notes []CleanNote, pitch []int, cfg *cleanConfig) []bool {
	settled := make([]bool, len(notes))

	for i, n := range notes {
		// votes[d/12+2] counts the votes for shift d in -24, -12, 0, 12, 24.
		var votes [5]int

		for _, k := range [...]int{-2, -1, 1, 2} {
			if j := octaveVote(notes, n.Slot+k*cfg.periodSlots); j >= 0 && (pitch[j]-pitch[i])%12 == 0 && absInt(pitch[j]-pitch[i]) <= 24 {
				votes[(pitch[j]-pitch[i])/12+2]++
			}
		}

		best := 0
		for _, o := range [...]int{-24, -12, 12, 24} {
			if votes[o/12+2] > votes[best/12+2] {
				best = o
			}
		}

		if v := votes[best/12+2]; v > 0 && (best == 0 || v >= cfg.periodVotes) {
			notes[i].MIDI += best
			settled[i] = true
		}
	}

	return settled
}

// medianStep is the second octave step: an unsettled note further than
// spread semitones from the duration-weighted median of the raw pitches of
// the other notes starting within ±window seconds (at least three) moves an
// octave towards it, and is settled.
//
// The notes are slot-sorted with distinct slots, and the slot is a
// non-decreasing function of the start time, so the starts are sorted and
// the window is a contiguous index range that only moves forward. The
// neighbours are visited in index order, as in a full scan, so the weighted
// median (and its floating-point sum) is the same.
func medianStep(notes []CleanNote, pitch []int, settled []bool, window, spread float64) {
	around := make([]weightedPitch, 0, 16)
	lo := 0

	for i, n := range notes {
		for lo < i && math.Abs(notes[lo].Start-n.Start) > window {
			lo++
		}

		if settled[i] {
			continue
		}

		around = around[:0]
		total := 0.0

		for j := lo; j < len(notes); j++ {
			m := &notes[j]
			if math.Abs(m.Start-n.Start) > window {
				if j > i {
					break
				}

				continue
			}

			if j != i {
				around = append(around, weightedPitch{float64(pitch[j]), m.End - m.Start})
				total += m.End - m.Start
			}
		}

		if len(around) < 3 {
			continue
		}

		// The reference uses sort.Slice, which is not stable: the order of
		// equal pitches fixes the summation order below. slices.SortFunc is
		// generated from the same pdqsort template (sort/gen_sort_variants.go)
		// and permutes identically, without the reflection allocation
		// (TestSortPermutationMatchesSortSlice guards this).
		slices.SortFunc(around, func(a, b weightedPitch) int { return cmp.Compare(a.midi, b.midi) })

		median, acc := around[len(around)-1].midi, 0.0
		for _, a := range around {
			if acc += a.weight; acc >= total/2 {
				median = a.midi

				break
			}
		}

		if d := float64(n.MIDI) - median; math.Abs(d) > spread {
			notes[i].MIDI -= 12 * int(math.Copysign(1, d))
			settled[i] = true
		}
	}
}

// splitVoices labels runs of at least arpMinRun short, closely spaced notes
// as [VoiceArp] and everything else as [VoiceLead].
func splitVoices(notes []CleanNote, cfg *cleanConfig) {
	short := func(n *CleanNote) bool { return n.Slots <= cfg.arpMaxNote }

	for start := 0; start < len(notes); {
		end := start + 1
		for short(&notes[start]) && end < len(notes) && short(&notes[end]) &&
			notes[end].Slot-(notes[end-1].Slot+notes[end-1].Slots) <= cfg.arpMaxGap {
			end++
		}

		voice := VoiceLead
		if short(&notes[start]) && end-start >= cfg.arpMinRun {
			voice = VoiceArp
		}

		for i := start; i < end; i++ {
			notes[i].Voice = voice
		}

		start = end
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}

	return x
}

func signInt(x int) int {
	if x < 0 {
		return -1
	}

	return 1
}
