//nolint:all // Verbatim reference copy; kept byte-for-byte close to the original.
package melody

// This file holds a verbatim copy of AudioVisualizer's internal/story/notes.go
// (CleanParams, LeadCleanParams, BassCleanParams, StoryNote, RawNote,
// CleanNotes, correctOctaves, splitVoices) and the parts of
// internal/story/grid.go it uses (Grid, NewGrid, Slot, SlotTime, r6, r3).
// Adapted to standalone test code: audioanalysis.Note is replaced by refNote
// (same fields, from reference_test.go), audioanalysis.Rhythm by its plain
// fields, and identifiers carry a story prefix. The arithmetic and its
// operation order are unchanged; the parity tests compare Clean with this
// copy bit for bit.

import (
	"math"
	"sort"
)

type storyRhythm struct {
	BPM        float64
	BeatOrigin float64
	Beats      []float64
	Downbeat   int
}

type storyGrid struct {
	BPM              float64   `json:"bpm"`
	OriginSeconds    float64   `json:"originSeconds"`
	BeatSeconds      float64   `json:"beatSeconds"`
	SixteenthSeconds float64   `json:"sixteenthSeconds"`
	BarSeconds       float64   `json:"barSeconds"`
	BeatsPerBar      int       `json:"beatsPerBar"`
	Downbeat         int       `json:"downbeatBeatIndex"`
	Duration         float64   `json:"durationSeconds"`
	Beats            []float64 `json:"beatsSeconds"`
	BarStarts        []float64 `json:"barStartsSeconds"`
	barOrigin        float64
}

func newStoryGrid(r storyRhythm, duration float64) storyGrid {
	beat := 60 / r.BPM
	g := storyGrid{BPM: r.BPM, OriginSeconds: r.BeatOrigin, BeatSeconds: beat, SixteenthSeconds: beat / 4, BarSeconds: 4 * beat, BeatsPerBar: 4, Downbeat: r.Downbeat, Duration: duration}
	g.barOrigin = g.OriginSeconds + float64(r.Downbeat%4)*beat
	if r.Downbeat%4 != 0 {
		g.barOrigin -= g.BarSeconds
	}
	g.Beats = append([]float64(nil), r.Beats...)
	for i := 0; len(r.Beats) == 0 && g.BeatStart(i) < duration; i++ {
		g.Beats = append(g.Beats, g.BeatStart(i))
	}
	for i := 0; g.BarStart(i) < duration; i++ {
		g.BarStarts = append(g.BarStarts, g.BarStart(i))
	}
	return g
}

func (g storyGrid) Slot(t float64) int {
	return int(math.Round((t - g.OriginSeconds) / g.SixteenthSeconds))
}
func (g storyGrid) SlotTime(slot int) float64 {
	return g.OriginSeconds + float64(slot)*g.SixteenthSeconds
}
func (g storyGrid) BarStart(n int) float64  { return g.barOrigin + float64(n)*g.BarSeconds }
func (g storyGrid) BeatStart(n int) float64 { return g.OriginSeconds + float64(n)*g.BeatSeconds }

func storyR6(v float64) float64 { return math.Round(v*1e6) / 1e6 }
func storyR3(v float64) float64 { return math.Round(v*1e3) / 1e3 }

// storyCleanParams controls note cleanup. Notes at or below FloorMIDI are the
// tracker's pinned lowest candidate (bleed or octave errors), not pitches.
type storyCleanParams struct {
	FloorMIDI        int     `json:"floorMidi"`
	MinStrength      float64 `json:"minStrength"`
	OffGridMS        float64 `json:"offGridMS"`
	OctaveWindowBars float64 `json:"octaveWindowBars"`
	OctaveSpread     float64 `json:"octaveSpreadSemitones"`
	NeighbourSpread  int     `json:"octaveNeighbourSpreadSemitones"`
	PeriodSlots      int     `json:"octavePeriodSixteenths"`
	PeriodVotes      int     `json:"octavePeriodMinVotes"`
	ArpMinRun        int     `json:"arpMinRun"`
	ArpMaxGapSlots   int     `json:"arpMaxGapSixteenths"`
	ArpMaxNoteSlots  int     `json:"arpMaxNoteSixteenths"`
}

// The spec's ±13 semitone spread and 5 semitone neighbour agreement folded
// genuine arpeggio leaps (G5 G4 F#5) and the two-register breakdown; the
// defaults are wider and a bar-periodic vote runs first.
func storyLeadCleanParams() storyCleanParams {
	return storyCleanParams{52, 0.35, 50, 1, 19, 2, 16, 2, 4, 2, 2}
}
func storyBassCleanParams() storyCleanParams {
	return storyCleanParams{28, 0.35, 50, 1, 19, 2, 16, 2, 4, 2, 2}
}

// storyNote is a cleaned, grid-quantised note. Seconds stay measured; the
// sixteenth fields are the quantised position.
type storyNote struct {
	Start       float64 `json:"startSeconds"`
	End         float64 `json:"endSeconds"`
	Slot        int     `json:"startSixteenth"`
	Slots       int     `json:"sixteenths"`
	MIDI        int     `json:"midi"`
	RawMIDI     int     `json:"rawMidi"`
	OctaveShift int     `json:"octaveShift,omitempty"`
	Strength    float64 `json:"strength"`
	Voice       string  `json:"voice"`
	OffsetMS    float64 `json:"gridOffsetMS"`
	OffGrid     bool    `json:"offGrid,omitempty"`
	RawIndex    int     `json:"rawIndex"`
}

// storyRawNote is a tracker note with the reason it was dropped, if any.
type storyRawNote struct {
	refNote
	Dropped string `json:"dropped,omitempty"`
}

// storyCleanNotes drops floor and weak notes, quantises starts to sixteenths,
// keeps one note per slot, corrects isolated octave errors and splits short
// runs into an "arp" voice. Raw notes are returned with drop reasons.
func storyCleanNotes(in []refNote, g storyGrid, p storyCleanParams) ([]storyNote, []storyRawNote) {
	raw := make([]storyRawNote, len(in))
	notes := []storyNote{}
	for i, n := range in {
		n.Start, n.End, n.Strength = storyR6(n.Start), storyR6(n.End), storyR3(n.Strength)
		raw[i].refNote = n
		switch {
		case n.MIDI <= p.FloorMIDI:
			raw[i].Dropped = "floor"
			continue
		case n.Strength < p.MinStrength:
			raw[i].Dropped = "weak"
			continue
		}
		slot := g.Slot(n.Start)
		offset := (n.Start - g.SlotTime(slot)) * 1000
		notes = append(notes, storyNote{Start: n.Start, End: n.End, Slot: slot, Slots: max(1, g.Slot(n.End)-slot), MIDI: n.MIDI, RawMIDI: n.MIDI,
			Strength: n.Strength, OffsetMS: storyR3(offset), OffGrid: math.Abs(offset) > p.OffGridMS, RawIndex: i})
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].Slot < notes[j].Slot })
	// Monophonic: one note per slot (strongest by strength × duration), and
	// each note ends no later than the next one starts.
	mono := []storyNote{}
	for _, n := range notes {
		if k := len(mono) - 1; k >= 0 && mono[k].Slot == n.Slot {
			if n.Strength*(n.End-n.Start) > mono[k].Strength*(mono[k].End-mono[k].Start) {
				raw[mono[k].RawIndex].Dropped = "duplicate"
				mono[k] = n
			} else {
				raw[n.RawIndex].Dropped = "duplicate"
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
	storyCorrectOctaves(mono, g, p)
	storySplitVoices(mono, p)
	return mono, raw
}

// storyCorrectOctaves first lets repeating material vote: notes of the same pitch
// class at the same position one and two periods away decide the octave, and
// a note they confirm is left alone. Remaining notes are (a) moved by an
// octave towards the duration-weighted median of the surrounding bars when
// more than OctaveSpread semitones away, and (b) folded when they sit an
// octave (±1) from both close neighbours that agree within NeighbourSpread.
func storyCorrectOctaves(notes []storyNote, g storyGrid, p storyCleanParams) {
	pitch := make([]int, len(notes))
	bySlot := map[int]int{}
	for i, n := range notes {
		pitch[i] = n.MIDI
		bySlot[n.Slot] = i
	}
	settled := make([]bool, len(notes))
	for i, n := range notes {
		votes := map[int]int{}
		for _, k := range []int{-2, -1, 1, 2} {
			if j, ok := bySlot[n.Slot+k*p.PeriodSlots]; ok && (pitch[j]-pitch[i])%12 == 0 && storyAbs(pitch[j]-pitch[i]) <= 24 {
				votes[pitch[j]-pitch[i]]++
			}
		}
		best := 0
		for _, o := range []int{-24, -12, 12, 24} {
			if votes[o] > votes[best] {
				best = o
			}
		}
		if votes[best] > 0 && (best == 0 || votes[best] >= p.PeriodVotes) {
			notes[i].MIDI += best
			settled[i] = true
		}
	}
	window := p.OctaveWindowBars * g.BarSeconds
	for i, n := range notes {
		if settled[i] {
			continue
		}
		type wp struct{ midi, weight float64 }
		around := []wp{}
		total := 0.0
		for j, m := range notes {
			if j != i && math.Abs(m.Start-n.Start) <= window {
				around = append(around, wp{float64(pitch[j]), m.End - m.Start})
				total += m.End - m.Start
			}
		}
		if len(around) < 3 {
			continue
		}
		sort.Slice(around, func(a, b int) bool { return around[a].midi < around[b].midi })
		median, acc := around[len(around)-1].midi, 0.0
		for _, a := range around {
			if acc += a.weight; acc >= total/2 {
				median = a.midi
				break
			}
		}
		if d := float64(n.MIDI) - median; math.Abs(d) > p.OctaveSpread {
			notes[i].MIDI -= 12 * int(math.Copysign(1, d))
			settled[i] = true
		}
	}
	for i := 1; i+1 < len(notes); i++ {
		prev, n, next := notes[i-1], notes[i], notes[i+1]
		if settled[i] || n.Slot-prev.Slot > 4 || next.Slot-n.Slot > 4 || storyAbs(prev.MIDI-next.MIDI) > p.NeighbourSpread {
			continue
		}
		a, b := n.MIDI-prev.MIDI, n.MIDI-next.MIDI
		if storyAbs(storyAbs(a)-12) <= 1 && storyAbs(storyAbs(b)-12) <= 1 && (a > 0) == (b > 0) {
			notes[i].MIDI -= 12 * storySign(a)
		}
	}
	for i := range notes {
		notes[i].OctaveShift = notes[i].MIDI - notes[i].RawMIDI
	}
}

// storySplitVoices labels runs of at least ArpMinRun short, closely spaced notes
// as "arp" and everything else as "lead".
func storySplitVoices(notes []storyNote, p storyCleanParams) {
	short := func(n storyNote) bool { return n.Slots <= p.ArpMaxNoteSlots }
	for start := 0; start < len(notes); {
		end := start + 1
		for short(notes[start]) && end < len(notes) && short(notes[end]) && notes[end].Slot-(notes[end-1].Slot+notes[end-1].Slots) <= p.ArpMaxGapSlots {
			end++
		}
		voice := "lead"
		if short(notes[start]) && end-start >= p.ArpMinRun {
			voice = "arp"
		}
		for i := start; i < end; i++ {
			notes[i].Voice = voice
		}
		start = end
	}
}

func storyAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func storySign(x int) int {
	if x < 0 {
		return -1
	}
	return 1
}
