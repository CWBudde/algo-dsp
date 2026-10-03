package melody_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// grid120 is a 120 BPM grid: beat 0.5 s, sixteenth slot 0.125 s, bar 2 s.
func grid120() rhythm.Grid {
	g, err := rhythm.NewGrid(120, 0, 0, 60)
	if err != nil {
		panic(err)
	}

	return g
}

// slotNote returns a note on slot of grid120, slots long.
func slotNote(slot, slots, midi int, strength float64) melody.Note {
	return melody.Note{Start: 0.125 * float64(slot), End: 0.125*float64(slot+slots) - 0.01, MIDI: midi, Strength: strength}
}

func ExampleClean() {
	notes := []melody.Note{
		{Start: 0.02, End: 0.24, MIDI: 67, Strength: 0.8}, // 20 ms late
		{Start: 0.01, End: 0.1, MIDI: 74, Strength: 0.4},  // same slot, weaker
		{Start: 0.31, End: 0.49, MIDI: 69, Strength: 0.7}, // 60 ms off the grid
		{Start: 0.5, End: 0.74, MIDI: 81, Strength: 0.8},  // octave spike
		{Start: 0.75, End: 0.99, MIDI: 70, Strength: 0.8},
		{Start: 1.0, End: 1.24, MIDI: 50, Strength: 0.9},  // below the floor
		{Start: 1.25, End: 1.49, MIDI: 71, Strength: 0.2}, // too weak
	}

	clean, dropped, err := melody.Clean(notes, grid120())
	if err != nil {
		panic(err)
	}

	for _, n := range clean {
		fmt.Printf("slot %d+%d MIDI %d (raw %d) offset %+.0f ms off-grid %v %s\n",
			n.Slot, n.Slots, n.MIDI, n.RawMIDI, n.OffsetMS, n.OffGrid, n.Voice)
	}

	for _, d := range dropped {
		fmt.Printf("dropped note %d: %s\n", d.Index, d.Reason)
	}
	// Output:
	// slot 0+2 MIDI 67 (raw 67) offset +20 ms off-grid false arp
	// slot 2+2 MIDI 69 (raw 69) offset +60 ms off-grid true arp
	// slot 4+2 MIDI 69 (raw 81) offset +0 ms off-grid false arp
	// slot 6+2 MIDI 70 (raw 70) offset +0 ms off-grid false arp
	// dropped note 1: duplicate
	// dropped note 5: floor
	// dropped note 6: weak
}

func ExampleCleanNote() {
	clean, _, err := melody.Clean([]melody.Note{{Start: 1.03, End: 1.5, MIDI: 64, Strength: 0.61234}}, grid120())
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", clean[0])
	// Output:
	// {Start:1.03 End:1.5 Slot:8 Slots:4 MIDI:64 RawMIDI:64 OctaveShift:0 Strength:0.612 Voice:lead OffsetMS:30 OffGrid:false RawIndex:0}
}

func ExampleDroppedNote() {
	_, dropped, err := melody.Clean([]melody.Note{{Start: 1, End: 1.5, MIDI: 40, Strength: 0.9}}, grid120())
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", dropped[0])
	// Output:
	// {Note:{Start:1 End:1.5 MIDI:40 Strength:0.9} Index:0 Reason:floor}
}

func ExampleWithFloorMIDI() {
	notes := []melody.Note{{Start: 0, End: 0.4, MIDI: 48, Strength: 0.9}}

	lead, _, _ := melody.Clean(notes, grid120())
	low, _, _ := melody.Clean(notes, grid120(), melody.WithFloorMIDI(40))

	fmt.Println(len(lead), len(low))
	// Output:
	// 0 1
}

func ExampleWithMinStrength() {
	notes := []melody.Note{{Start: 0, End: 0.4, MIDI: 60, Strength: 0.3}}

	_, dropped, _ := melody.Clean(notes, grid120(), melody.WithMinStrength(0.5))
	fmt.Println(dropped[0].Reason)
	// Output:
	// weak
}

func ExampleWithOffGridMS() {
	notes := []melody.Note{{Start: 0.03, End: 0.4, MIDI: 60, Strength: 0.9}}

	clean, _, _ := melody.Clean(notes, grid120(), melody.WithOffGridMS(20))
	fmt.Println(clean[0].OffsetMS, clean[0].OffGrid)
	// Output:
	// 30 true
}

func ExampleWithOctaveWindow() {
	// A note two octaves above a steady line is moved one octave towards
	// the median; a wider spread leaves it alone.
	var notes []melody.Note
	for i, m := range []int{60, 62, 60, 84, 62, 60} {
		notes = append(notes, slotNote(3*i, 1, m, 0.8))
	}

	def, _, _ := melody.Clean(notes, grid120())
	wide, _, _ := melody.Clean(notes, grid120(), melody.WithOctaveWindow(1, 30))

	fmt.Println(def[3].MIDI, wide[3].MIDI)
	// Output:
	// 72 84
}

func ExampleWithNeighbourSpread() {
	// A lone octave spike between neighbours a semitone apart is folded by
	// default; a neighbour spread of 0 demands equal neighbours.
	var notes []melody.Note
	for i, m := range []int{60, 62, 64, 76, 65, 67} {
		notes = append(notes, slotNote(2*i, 1, m, 0.8))
	}

	def, _, _ := melody.Clean(notes, grid120())
	strict, _, _ := melody.Clean(notes, grid120(), melody.WithNeighbourSpread(0))

	fmt.Println(def[3].MIDI, strict[3].MIDI)
	// Output:
	// 64 76
}

func ExampleWithOctavePeriod() {
	// A figure repeated every bar (16 slots) with an octave error in bar 2:
	// the bars around it vote it back.
	var notes []melody.Note

	for bar := range 5 {
		m := 72
		if bar == 2 {
			m = 60
		}

		notes = append(notes, slotNote(16*bar, 8, m, 0.8), slotNote(16*bar+8, 8, 67, 0.8))
	}

	clean, _, _ := melody.Clean(notes, grid120(), melody.WithOctavePeriod(16, 2))
	fmt.Println(clean[4].RawMIDI, clean[4].MIDI, clean[4].OctaveShift)
	// Output:
	// 60 72 12
}

func ExampleWithArpeggio() {
	// Three sixteenths, then a long note: an arpeggio needs a run of four
	// by default, three with the option.
	notes := []melody.Note{slotNote(0, 1, 67, 0.8), slotNote(1, 1, 71, 0.8), slotNote(2, 1, 74, 0.8), slotNote(4, 8, 72, 0.8)}

	def, _, _ := melody.Clean(notes, grid120())
	short, _, _ := melody.Clean(notes, grid120(), melody.WithArpeggio(3, 2, 2))

	fmt.Println(def[0].Voice, short[0].Voice, short[3].Voice)
	// Output:
	// lead arp lead
}

func ExampleBassPreset() {
	const sampleRate = 24000.0

	// One second of A1 (55 Hz) as a sawtooth with 10 harmonics.
	x := make([]float64, int(1.2*sampleRate))
	for i := int(0.1 * sampleRate); i < int(1.1*sampleRate); i++ {
		for h := 1; h <= 10; h++ {
			x[i] += 0.25 / float64(h) * math.Sin(2*math.Pi*55*float64(h)*float64(i)/sampleRate)
		}
	}

	res, err := melody.Analyze(x, sampleRate, melody.BassPreset()...)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(res.Notes), res.Notes[0].MIDI)
	// Output:
	// 1 33
}

func ExampleBassCleanOptions() {
	notes := []melody.Note{{Start: 0, End: 0.4, MIDI: 33, Strength: 0.9}} // A1

	lead, _, _ := melody.Clean(notes, grid120())
	bass, _, _ := melody.Clean(notes, grid120(), melody.BassCleanOptions()...)

	fmt.Println(len(lead), len(bass))
	// Output:
	// 0 1
}
