package motif

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// newGrid builds the package grid and the reference grid from the same
// values.
func newGrid(tb testing.TB, bpm, origin float64, downbeat int, duration float64) (rhythm.Grid, refGrid) {
	tb.Helper()

	g, err := rhythm.NewGrid(bpm, origin, downbeat, duration)
	if err != nil {
		tb.Fatal(err)
	}

	return g, refNewGrid(refRhythm{BPM: bpm, BeatOrigin: origin, Downbeat: downbeat}, duration)
}

// slotNote is a note on slot, slots long, timed on the grid.
func slotNote(g rhythm.Grid, slot, slots, midi int, strength float64) melody.CleanNote {
	return melody.CleanNote{
		Start: g.SlotTime(slot), End: g.SlotTime(slot + slots), Slot: slot, Slots: slots,
		MIDI: midi, RawMIDI: midi, Strength: strength, Voice: melody.VoiceLead,
	}
}

// plantedMotif is the 8-note motif of the PLAN fixture: G4 B4 D5 C5 B4 A4 G4
// D4 with inter-onsets 2 2 2 2 1 1 2 slots, 13 slots long.
var (
	plantedPitches = []int{67, 71, 74, 72, 71, 69, 67, 62}
	plantedOnsets  = []int{0, 2, 4, 6, 8, 9, 10, 12}
)

// statement is one planted statement: its bar, transposition, and an
// optional change of one note (index, semitones).
type statement struct {
	bar, transpose int
	note, change   int
}

// plantedStatements are the original at bar 0, a +5 transposition at bar 4,
// a variant with the sixth note a whole tone higher at bar 8, and an octave
// error on the fourth note at bar 12.
var plantedStatements = []statement{
	{bar: 0, note: -1},
	{bar: 4, transpose: 5, note: -1},
	{bar: 8, note: 5, change: 2},
	{bar: 12, note: 3, change: 12},
}

// plantedSong places plantedStatements in a 16-bar line whose other bars
// are seeded random filler, separated from the statements by more than the
// maximum n-gram gap.
func plantedSong(g rhythm.Grid, seed uint64) []melody.CleanNote {
	rng := rand.New(rand.NewPCG(seed, 45))
	notes := []melody.CleanNote{}

	for bar := range 16 {
		i := slices.IndexFunc(plantedStatements, func(s statement) bool { return s.bar == bar })
		if i >= 0 {
			s := plantedStatements[i]

			for k, at := range plantedOnsets {
				midi := plantedPitches[k] + s.transpose
				if k == s.note {
					midi += s.change
				}

				next := 16
				if k+1 < len(plantedOnsets) {
					next = plantedOnsets[k+1]
				}

				notes = append(notes, slotNote(g, 16*bar+at, min(2, next-at), midi, 0.8))
			}

			continue
		}

		slot, end := 16*bar+6, 16*(bar+1)-6

		for slot < end {
			notes = append(notes, slotNote(g, slot, 1, 55+rng.IntN(24), 0.4+0.4*rng.Float64()))
			slot += 1 + rng.IntN(3)
		}
	}

	return notes
}

// plantedChroma is the beat chroma of plantedSong: each beat adds its notes'
// pitch classes weighted by their length in slots; filler bars get one
// random pitch class per beat.
func plantedChroma(notes []melody.CleanNote, bars int, seed uint64) [][12]float64 {
	rng := rand.New(rand.NewPCG(seed, 46))
	beats := make([][12]float64, 4*bars)
	planted := map[int]bool{}

	for _, s := range plantedStatements {
		planted[s.bar] = true
	}

	for i := range beats {
		if !planted[i/4] {
			beats[i][rng.IntN(12)] = 1
		}
	}

	for _, n := range notes {
		if planted[n.Slot/16] {
			beats[n.Slot/4][n.MIDI%12] += float64(n.Slots)
		}
	}

	return beats
}

// arpeggioSong is a bar-long G-major arpeggio repeated for bars bars.
func arpeggioSong(g rhythm.Grid, bars int) []melody.CleanNote {
	pattern := []int{55, 59, 62, 67, 71, 67, 62, 59, 57, 60, 64, 69, 72, 69, 64, 60}
	notes := []melody.CleanNote{}

	for bar := range bars {
		for k, midi := range pattern {
			notes = append(notes, slotNote(g, 16*bar+k, 1, midi, 0.7))
		}
	}

	return notes
}

// phraseSong is a seeded random song of phrases: a few one- and two-bar
// figures recur transposed, varied and with octave errors between random
// filler, and some notes are dropped. It exercises every branch of the
// motif search.
func phraseSong(g rhythm.Grid, rng *rand.Rand, bars int) []melody.CleanNote {
	type figure struct {
		onsets, pitches, lengths []int
	}

	figures := make([]figure, 3)
	for i := range figures {
		slot := 0
		for slot < 16*(1+i%2) {
			f := &figures[i]
			f.onsets = append(f.onsets, slot)
			f.pitches = append(f.pitches, 60+rng.IntN(16))
			l := 1 + rng.IntN(2)
			f.lengths = append(f.lengths, l)
			slot += l + rng.IntN(2)
		}
	}

	notes := []melody.CleanNote{}
	origin := g.Slot(g.BarStart(0))

	for bar := 0; bar < bars; {
		base := origin + 16*bar

		if rng.IntN(3) == 0 {
			slot := base + rng.IntN(3)
			for slot < base+16 {
				notes = append(notes, slotNote(g, slot, 1, 55+rng.IntN(24), 0.3+0.6*rng.Float64()))
				slot += 1 + rng.IntN(4)
			}

			bar++

			continue
		}

		f := figures[rng.IntN(len(figures))]
		transpose := []int{0, 0, 0, 2, 5, -3, 7}[rng.IntN(7)]

		for k, at := range f.onsets {
			midi := f.pitches[k] + transpose

			switch rng.IntN(20) {
			case 0:
				continue // dropped
			case 1:
				midi += 12 // octave error
			case 2:
				midi -= 1 + rng.IntN(2) // varied
			}

			notes = append(notes, slotNote(g, base+at, f.lengths[k], midi, 0.5+0.4*rng.Float64()))
		}

		bar += 1 + (f.onsets[len(f.onsets)-1]+f.lengths[len(f.lengths)-1]-1)/16
	}

	return notes
}

// phraseChroma is beat chroma of notes plus a little seeded noise; every
// eleventh beat is silent.
func phraseChroma(g rhythm.Grid, notes []melody.CleanNote, rng *rand.Rand) [][12]float64 {
	sub, last := g.Subdivisions(), 0
	for _, n := range notes {
		last = max(last, n.Slot+n.Slots)
	}

	beats := make([][12]float64, last/sub+4)

	for _, n := range notes {
		if b := n.Slot / sub; n.Slot >= 0 && b < len(beats) {
			beats[b][n.MIDI%12] += float64(n.Slots) * n.Strength
		}
	}

	for i := range beats {
		if i%11 == 10 {
			beats[i] = [12]float64{}

			continue
		}

		beats[i][rng.IntN(12)] += 0.2 * rng.Float64()
	}

	return beats
}

// energyOf is a deterministic, source-dependent energy function.
func energyOf(source string) func(t0, t1 float64) float64 {
	phase := float64(len(source))

	return func(t0, t1 float64) float64 {
		return 0.5 + 0.4*math.Sin(1.3*t0+0.7*t1+phase)
	}
}

// sectionsOf splits the grid into named sections of 4 bars.
func sectionsOf(g rhythm.Grid) []Span {
	out := []Span{}
	names := []string{"intro", "verse", "chorus", "verse", "bridge", "chorus", "outro"}

	for b := 0; b < g.Bars(); b += 4 {
		out = append(out, Span{Name: names[(b/4)%len(names)], Start: g.BarStart(b), End: g.BarStart(b + 4)})
	}

	return out
}
