package motif_test

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/motif"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// grid120 is a 120 BPM grid in 4/4: beat 0.5 s, sixteenth slot 0.125 s,
// bar 2 s.
func grid120() rhythm.Grid {
	g, err := rhythm.NewGrid(120, 0, 0, 60)
	if err != nil {
		panic(err)
	}

	return g
}

// note is a cleaned note on slot of g, slots long.
func note(g rhythm.Grid, slot, slots, midi int) melody.CleanNote {
	return melody.CleanNote{
		Start: g.SlotTime(slot), End: g.SlotTime(slot + slots), Slot: slot, Slots: slots,
		MIDI: midi, RawMIDI: midi, Strength: 0.8, Voice: melody.VoiceLead,
	}
}

// tune states the figure C4 D4 E4 G4 (onsets 0, 2, 4 and 8 slots) at bar 0,
// a fourth higher at bar 4 and with its third note raised at bar 8.
func tune(g rhythm.Grid) []melody.CleanNote {
	notes := []melody.CleanNote{}

	for _, s := range []struct{ bar, transpose, third int }{{0, 0, 64}, {4, 5, 69}, {8, 0, 65}} {
		for k, at := range []int{0, 2, 4, 8} {
			midi := []int{60, 62, s.third - s.transpose, 67}[k] + s.transpose
			notes = append(notes, note(g, 16*s.bar+at, 2, midi))
		}
	}

	return notes
}

// riff repeats a bar-long figure for bars bars from bar first.
func riff(g rhythm.Grid, first, bars int) []melody.CleanNote {
	notes := []melody.CleanNote{}

	for bar := first; bar < first+bars; bar++ {
		for k, midi := range []int{43, 50, 55, 50} {
			notes = append(notes, note(g, 16*bar+4*k, 4, midi))
		}
	}

	return notes
}

// chords returns beat chroma for the progression C F G C stated with the
// tune: at beats 0, 16 (a fourth higher) and 32, with silence elsewhere.
// Roots weigh 1, fifths 0.8 and thirds 0.6.
func chords(beats int) [][12]float64 {
	out := make([][12]float64, beats)

	for _, at := range []struct{ beat, t int }{{0, 0}, {16, 5}, {32, 0}} {
		for k, root := range []int{0, 5, 7, 0} {
			for i, w := range map[int]float64{0: 1, 4: 0.6, 7: 0.8} {
				out[at.beat+k][(root+i+at.t)%12] = w
			}
		}
	}

	return out
}

func flat(float64, float64) float64 { return 1 }

func printMotif(m motif.Motif) {
	fmt.Printf("%s %s %v span %v beats\n", m.ID, m.Source, m.Notes, m.SpanBeats)

	for _, o := range m.Occurrences {
		fmt.Printf("  bar %d at %.2f s: T%+d %s (%.2f)\n", o.Bar, o.Start, o.Transposition, o.Variant, o.Similarity)
	}
}

// Example runs the whole pipeline on a lead line, a bass riff and beat
// chroma: find, corroborate, score and rank.
func Example() {
	g := grid120()
	sources := []struct {
		name  string
		notes []melody.CleanNote
	}{{"lead", tune(g)}, {"bass", riff(g, 0, 6)}}
	sections := []motif.Span{{Name: "verse", Start: 0, End: 8}, {Name: "chorus", Start: 8, End: 24}}

	chroma, err := motif.FindChromaMotifs(chords(48), g)
	if err != nil {
		panic(err)
	}

	all := []motif.Motif{}

	for _, src := range sources {
		found, err := motif.FindNoteMotifs(src.notes, g, src.name)
		if err != nil {
			panic(err)
		}

		for i := range found {
			err = motif.Corroborate(&found[i], chroma, g)
			if err == nil {
				err = motif.Score(&found[i], src.notes, sections, flat, g)
			}

			if err != nil {
				panic(err)
			}
		}

		all = append(all, found...)
	}

	for i := range chroma {
		err = motif.Score(&chroma[i], nil, sections, flat, g)
		if err != nil {
			panic(err)
		}
	}

	ranked, err := motif.Rank(append(all, chroma...))
	if err != nil {
		panic(err)
	}

	for _, m := range ranked {
		fmt.Printf("%s %-6s %-8s %d× salience %.3f confirmed %-5v leitmotif %-5v rank %d\n",
			m.ID, m.Source, m.Role, len(m.Occurrences), m.Salience, m.ConfirmedByChroma, m.Leitmotif, m.Rank)
	}
	// Output:
	// M1 bass   ostinato 6× salience 1.069 confirmed false leitmotif true  rank 1
	// M2 lead   theme    3× salience 0.222 confirmed true  leitmotif false rank 0
	// C1 chroma theme    3× salience 0.762 confirmed false leitmotif false rank 0
}

func ExampleFindNoteMotifs() {
	g := grid120()

	motifs, err := motif.FindNoteMotifs(tune(g), g, "lead")
	if err != nil {
		panic(err)
	}

	for _, m := range motifs {
		printMotif(m)
	}
	// Output:
	// lead [C4 D4 E4 G4] span 4 beats
	//   bar 0 at 0.00 s: T+0 exact (1.00)
	//   bar 4 at 8.00 s: T+5 transposed (0.90)
	//   bar 8 at 16.00 s: T+0 varied (0.75)
}

func ExampleFindChromaMotifs() {
	g := grid120()

	motifs, err := motif.FindChromaMotifs(chords(48), g)
	if err != nil {
		panic(err)
	}

	for _, m := range motifs {
		printMotif(m)
	}
	// Output:
	// chroma [C F G C] span 4 beats
	//   bar 0 at 0.00 s: T+0 exact (1.00)
	//   bar 4 at 8.00 s: T+5 transposed (1.00)
	//   bar 8 at 16.00 s: T+0 exact (1.00)
}

func ExampleCorroborate() {
	g := grid120()

	chroma, err := motif.FindChromaMotifs(chords(48), g, motif.WithChromaWindows(4))
	if err != nil {
		panic(err)
	}

	// A note motif stated with the chords, transposed with them.
	m := motif.Motif{Source: "lead", Occurrences: []motif.Occurrence{
		{Start: 0, End: 2, Transposition: 0}, {Start: 8, End: 10, Transposition: 5}, {Start: 16, End: 18, Transposition: 0},
	}}

	err = motif.Corroborate(&m, chroma, g)
	if err != nil {
		panic(err)
	}

	fmt.Println("confirmed by chroma:", m.ConfirmedByChroma)
	// Output:
	// confirmed by chroma: true
}

func ExampleScore() {
	g := grid120()
	notes := tune(g)

	motifs, err := motif.FindNoteMotifs(notes, g, "lead")
	if err != nil {
		panic(err)
	}

	m := &motifs[0]
	energy := func(start, _ float64) float64 { return 0.5 + start/20 }

	err = motif.Score(m, notes, []motif.Span{{Name: "A", Start: 0, End: 12}, {Name: "B", Start: 12, End: 24}}, energy, g)
	if err != nil {
		panic(err)
	}

	fmt.Printf("role %s, salience %.3f\n%+v\n", m.Role, m.Salience, m.SalienceTerms)
	// Output:
	// role theme, salience 0.167
	// {Count:1.386 Sections:1.099 Prominence:0.795 Distinctiveness:0.275 Span:0.5 Confirmed:1}
}

func ExampleRank() {
	occ := func(start float64) motif.Occurrence { return motif.Occurrence{Start: start, End: start + 2} }
	motifs := []motif.Motif{
		{Source: "bass", Role: motif.RoleOstinato, Salience: 0.6, Occurrences: []motif.Occurrence{occ(0), occ(2)}},
		{Source: motif.SourceChroma, Role: motif.RoleTheme, Salience: 1.3, Occurrences: []motif.Occurrence{occ(0)}},
		{Source: "lead", Role: motif.RoleTheme, Salience: 0.4, Occurrences: []motif.Occurrence{occ(8)}},
		{Source: "lead", Role: motif.RoleTheme, Salience: 0.2, Occurrences: []motif.Occurrence{occ(12)}},
	}

	ranked, err := motif.Rank(motifs)
	if err != nil {
		panic(err)
	}

	for _, m := range ranked {
		fmt.Printf("%s %-6s salience %.1f leitmotif %-5v rank %d\n", m.ID, m.Source, m.Salience, m.Leitmotif, m.Rank)
	}
	// Output:
	// M1 bass   salience 0.6 leitmotif true  rank 1
	// M2 lead   salience 0.4 leitmotif true  rank 2
	// M3 lead   salience 0.2 leitmotif false rank 0
	// C1 chroma salience 1.3 leitmotif false rank 0
}

func ExampleMotif() {
	g := grid120()

	motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows())
	if err != nil {
		panic(err)
	}

	m := motifs[0]
	fmt.Println("notes:", m.Notes, "MIDI:", m.PrototypeMIDI)
	fmt.Println("intervals:", m.Intervals, "inter-onsets:", m.IOI, "span:", m.SpanBeats, "beats")
	// Output:
	// notes: [C4 D4 E4 G4] MIDI: [60 62 64 67]
	// intervals: [2 2 3] inter-onsets: [2 2 4] span: 2.5 beats
}

func ExampleOccurrence() {
	g := grid120()
	notes := tune(g)

	motifs, err := motif.FindNoteMotifs(notes, g, "lead", motif.WithGridWindows())
	if err != nil {
		panic(err)
	}

	for _, o := range motifs[0].Occurrences {
		first := notes[o.NoteIndices[0]]
		fmt.Printf("slot %d, %.3f–%.3f s, T%+d, first note MIDI %d\n", o.Slot, o.Start, o.End, o.Transposition, first.MIDI)
	}
	// Output:
	// slot 0, 0.000–1.250 s, T+0, first note MIDI 60
	// slot 64, 8.000–9.250 s, T+5, first note MIDI 65
	// slot 128, 16.000–17.250 s, T+0, first note MIDI 60
}

func ExampleSalienceTerms() {
	g := grid120()
	m := motif.Motif{Source: "lead", Intervals: []int{2, 2, 3}, SpanBeats: 2.5, Occurrences: []motif.Occurrence{
		{Start: 0, End: 1.25, Similarity: 1}, {Start: 8, End: 9.25, Similarity: 0.9},
	}}

	err := motif.Score(&m, nil, []motif.Span{{Name: "A", Start: 0, End: 4}, {Name: "B", Start: 4, End: 12}}, flat, g)
	if err != nil {
		panic(err)
	}

	t := m.SalienceTerms
	fmt.Printf("count %.3f × sections %.3f × prominence %.3f × distinctiveness %.3f × span %.3f × confirmed %.1f = %.3f\n",
		t.Count, t.Sections, t.Prominence, t.Distinctiveness, t.Span, t.Confirmed, m.Salience)
	// Output:
	// count 1.099 × sections 1.099 × prominence 0.950 × distinctiveness 0.275 × span 0.313 × confirmed 1.0 = 0.099
}

func ExampleSpan() {
	g := grid120()
	m := motif.Motif{Source: "lead", Intervals: []int{2, 2, 3}, SpanBeats: 4, Occurrences: []motif.Occurrence{
		{Start: 0, End: 2, Similarity: 1}, {Start: 10, End: 12, Similarity: 1}, {Start: 20, End: 22, Similarity: 1},
	}}
	sections := []motif.Span{{Name: "verse", Start: 0, End: 8}, {Name: "chorus", Start: 8, End: 16}, {Name: "verse", Start: 16, End: 24}}

	err := motif.Score(&m, nil, sections, flat, g)
	if err != nil {
		panic(err)
	}

	// Two distinct section names are visited: ln(1 + 2).
	fmt.Printf("sections term %.3f\n", m.SalienceTerms.Sections)
	// Output:
	// sections term 1.099
}

func ExampleRole() {
	g := grid120()
	notes := riff(g, 0, 6)

	motifs, err := motif.FindNoteMotifs(notes, g, "bass")
	if err != nil {
		panic(err)
	}

	err = motif.Score(&motifs[0], notes, nil, flat, g)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(motifs), "motif:", motifs[0].Role, "with", len(motifs[0].Occurrences), "occurrences")
	// Output:
	// 1 motif: ostinato with 6 occurrences
}

func ExampleVariant() {
	g := grid120()

	motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows())
	if err != nil {
		panic(err)
	}

	for _, o := range motifs[0].Occurrences {
		fmt.Println(o.Bar, o.Variant)
	}
	// Output:
	// 0 exact
	// 4 transposed
	// 8 varied
}

func ExampleOption() {
	g := grid120()

	_, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithMinOccurrences(1))
	fmt.Println(errors.Is(err, motif.ErrInvalidOption), err)

	_, err = motif.FindNoteMotifs(tune(g), g, motif.SourceChroma)
	fmt.Println(errors.Is(err, motif.ErrInvalidArgument), err)
	// Output:
	// true motif: invalid option: minimum occurrences must be >= 2, got 1
	// true motif: invalid argument: source "chroma" is reserved for chroma motifs
}

func ExampleDefaultGridWindows() {
	fmt.Println(motif.DefaultGridWindows(), motif.DefaultLengths(), motif.DefaultChromaWindows())
	// Output:
	// [16 8] [8 6 4] [8 4]
}

func ExampleDefaultLengths() {
	g := grid120()

	// Without the grid pass, the default n-gram lengths 8, 6 and 4 find the
	// four-note figure.
	motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithLengths(motif.DefaultLengths()...))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(motifs), "motif of", len(motifs[0].PrototypeMIDI), "notes")
	// Output:
	// 1 motif of 4 notes
}

func ExampleDefaultChromaWindows() {
	g := grid120()

	motifs, err := motif.FindChromaMotifs(chords(48), g, motif.WithChromaWindows(motif.DefaultChromaWindows()...))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(motifs), "chroma motif of", motifs[0].SpanBeats, "beats")
	// Output:
	// 1 chroma motif of 4 beats
}

func ExampleWithGridWindows() {
	g := grid120()

	// Half-bar windows hold two notes of the riff.
	for _, opts := range [][]motif.Option{{motif.WithGridWindows(16)}, {motif.WithGridWindows(8), motif.WithGridMinNotes(2)}} {
		motifs, err := motif.FindNoteMotifs(riff(g, 0, 4), g, "bass", append(opts, motif.WithLengths())...)
		if err != nil {
			panic(err)
		}

		fmt.Println(len(motifs[0].Occurrences), "occurrences of", motifs[0].SpanBeats, "beats")
	}
	// Output:
	// 4 occurrences of 4 beats
	// 8 occurrences of 2 beats
}

func ExampleWithGridMinNotes() {
	g := grid120()

	for _, n := range []int{4, 5} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridMinNotes(n), motif.WithLengths())
		if err != nil {
			panic(err)
		}

		fmt.Printf("at least %d notes per window: %d motifs\n", n, len(motifs))
	}
	// Output:
	// at least 4 notes per window: 1 motifs
	// at least 5 notes per window: 0 motifs
}

func ExampleWithGridMinSimilarity() {
	g := grid120()

	for _, v := range []float64{0.5, 0.8} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridMinSimilarity(v), motif.WithLengths(), motif.WithMinOccurrences(2))
		if err != nil {
			panic(err)
		}

		fmt.Printf("minimum %.1f: %d occurrences\n", v, len(motifs[0].Occurrences))
	}
	// Output:
	// minimum 0.5: 3 occurrences
	// minimum 0.8: 2 occurrences
}

func ExampleWithGridTranspositionPenalty() {
	g := grid120()

	for _, v := range []float64{0, 0.1} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridTranspositionPenalty(v), motif.WithLengths())
		if err != nil {
			panic(err)
		}

		fmt.Printf("penalty %.1f: similarity of the transposed statement %.2f\n", v, motifs[0].Occurrences[1].Similarity)
	}
	// Output:
	// penalty 0.0: similarity of the transposed statement 1.00
	// penalty 0.1: similarity of the transposed statement 0.90
}

func ExampleWithLengths() {
	g := grid120()

	motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithLengths(4, 3))
	if err != nil {
		panic(err)
	}

	for _, m := range motifs {
		fmt.Println(len(m.PrototypeMIDI), "notes,", len(m.Occurrences), "occurrences")
	}
	// Output:
	// 4 notes, 3 occurrences
}

func ExampleWithMaxSpanSlots() {
	g := grid120()

	// The figure spans 10 slots.
	for _, n := range []int{10, 9} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithMaxSpanSlots(n))
		if err != nil {
			panic(err)
		}

		fmt.Printf("span up to %d slots: %d motifs\n", n, len(motifs))
	}
	// Output:
	// span up to 10 slots: 1 motifs
	// span up to 9 slots: 0 motifs
}

func ExampleWithMaxGapSlots() {
	g := grid120()

	// The longest rest in the figure is 2 slots (E4 to G4).
	for _, n := range []int{2, 1} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithMaxGapSlots(n))
		if err != nil {
			panic(err)
		}

		fmt.Printf("rests up to %d slots: %d motifs\n", n, len(motifs))
	}
	// Output:
	// rests up to 2 slots: 1 motifs
	// rests up to 1 slots: 0 motifs
}

func ExampleWithMinSimilarity() {
	g := grid120()

	for _, v := range []float64{0.8, 0.95} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithMinSimilarity(v), motif.WithMinOccurrences(2))
		if err != nil {
			panic(err)
		}

		fmt.Printf("minimum %.2f: %d occurrences\n", v, len(motifs[0].Occurrences))
	}
	// Output:
	// minimum 0.80: 3 occurrences
	// minimum 0.95: 2 occurrences
}

func ExampleWithMinOccurrences() {
	g := grid120()

	for _, n := range []int{3, 4} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithMinOccurrences(n))
		if err != nil {
			panic(err)
		}

		fmt.Printf("at least %d occurrences: %d motifs\n", n, len(motifs))
	}
	// Output:
	// at least 3 occurrences: 1 motifs
	// at least 4 occurrences: 0 motifs
}

func ExampleWithDistanceWeights() {
	g := grid120()

	// Weighting only the rhythm makes the varied statement exact in
	// similarity.
	for _, w := range [][2]float64{{0.7, 0.3}, {0, 1}} {
		motifs, err := motif.FindNoteMotifs(tune(g), g, "lead", motif.WithGridWindows(), motif.WithDistanceWeights(w[0], w[1]))
		if err != nil {
			panic(err)
		}

		fmt.Printf("weights %v: similarity %.2f\n", w, motifs[0].Occurrences[2].Similarity)
	}
	// Output:
	// weights [0.7 0.3]: similarity 0.84
	// weights [0 1]: similarity 1.00
}

func ExampleWithShiftOverlap() {
	g := grid120()

	for _, v := range []float64{0.5, 1} {
		motifs, err := motif.FindNoteMotifs(riff(g, 0, 8), g, "bass", motif.WithGridWindows(), motif.WithLengths(4), motif.WithShiftOverlap(v))
		if err != nil {
			panic(err)
		}

		fmt.Printf("shift overlap %.1f: %d motifs\n", v, len(motifs))
	}
	// Output:
	// shift overlap 0.5: 1 motifs
	// shift overlap 1.0: 2 motifs
}

func ExampleWithCoveredShare() {
	g := grid120()
	notes := []melody.CleanNote{}

	// An 8-note scale figure plus a 2-note tail, three times.
	for bar := range 3 {
		for k, midi := range []int{60, 62, 64, 65, 67, 65, 64, 62, 72, 71} {
			notes = append(notes, note(g, 16*bar+k, 1, midi))
		}
	}

	// The 8-gram covers the figure; the 4-gram of its last two notes and the
	// tail is half covered.
	for _, v := range []float64{0.75, 0.5} {
		motifs, err := motif.FindNoteMotifs(notes, g, "lead", motif.WithGridWindows(), motif.WithLengths(8, 4), motif.WithCoveredShare(v))
		if err != nil {
			panic(err)
		}

		fmt.Printf("covered share %.2f:", v)

		for _, m := range motifs {
			fmt.Print(" ", m.Notes)
		}

		fmt.Println()
	}
	// Output:
	// covered share 0.75: [C4 D4 E4 F4 G4 F4 E4 D4] [E4 D4 C5 B4]
	// covered share 0.50: [C4 D4 E4 F4 G4 F4 E4 D4]
}

func ExampleWithChromaWindows() {
	g := grid120()

	motifs, err := motif.FindChromaMotifs(chords(48), g, motif.WithChromaWindows(2))
	if err != nil {
		panic(err)
	}

	// C F returns as G C: a fifth higher, folded to T-5.
	for _, m := range motifs {
		fmt.Print(m.Notes, " at beats")

		for _, o := range m.Occurrences {
			fmt.Printf(" %d (T%+d)", o.Slot/g.Subdivisions(), o.Transposition)
		}

		fmt.Println()
	}
	// Output:
	// [C F] at beats 0 (T+0) 2 (T-5) 16 (T+5) 18 (T+0) 32 (T+0) 34 (T-5)
}

func ExampleWithChromaMinSimilarity() {
	g := grid120()
	beats := chords(48)
	beats[33][2] = 1 // a passing note in the last statement

	for _, v := range []float64{0.85, 0.99} {
		motifs, err := motif.FindChromaMotifs(beats, g, motif.WithChromaWindows(4), motif.WithChromaMinSimilarity(v))
		if err != nil {
			panic(err)
		}

		fmt.Printf("minimum %.2f: %d motifs\n", v, len(motifs))
	}
	// Output:
	// minimum 0.85: 1 motifs
	// minimum 0.99: 0 motifs
}

func ExampleWithConfirmShare() {
	g := grid120()
	chroma := []motif.Motif{{Source: motif.SourceChroma, Occurrences: []motif.Occurrence{{Start: 0}, {Start: 4}}}}

	for _, v := range []float64{0.6, 0.5} {
		m := motif.Motif{Source: "lead", Occurrences: []motif.Occurrence{{Start: 0}, {Start: 4}, {Start: 8}, {Start: 12}}}

		err := motif.Corroborate(&m, chroma, g, motif.WithConfirmShare(v))
		if err != nil {
			panic(err)
		}

		fmt.Printf("share %.1f: confirmed %v\n", v, m.ConfirmedByChroma)
	}
	// Output:
	// share 0.6: confirmed false
	// share 0.5: confirmed true
}

func ExampleWithOstinatoShare() {
	g := grid120()
	// Three of four occurrences follow each other.
	m := motif.Motif{Source: "bass", Occurrences: []motif.Occurrence{
		{Start: 0, End: 2, Similarity: 1}, {Start: 2, End: 4, Similarity: 1}, {Start: 4, End: 6, Similarity: 1}, {Start: 20, End: 22, Similarity: 1},
	}}

	for _, v := range []float64{0.6, 0.75} {
		err := motif.Score(&m, nil, nil, flat, g, motif.WithOstinatoShare(v))
		if err != nil {
			panic(err)
		}

		fmt.Printf("share %.2f: %s\n", v, m.Role)
	}
	// Output:
	// share 0.60: ostinato
	// share 0.75: theme
}

func rankedLeitmotifs(opts ...motif.Option) []string {
	occ := func(start float64) motif.Occurrence { return motif.Occurrence{Start: start, End: start + 2} }
	motifs := []motif.Motif{
		{Source: "lead", Role: motif.RoleTheme, Salience: 1, Occurrences: []motif.Occurrence{occ(0), occ(10)}},
		{Source: "lead", Role: motif.RoleTheme, Salience: 0.9, Occurrences: []motif.Occurrence{occ(1), occ(20)}},
		{Source: "lead", Role: motif.RoleOstinato, Salience: 0.8, Occurrences: []motif.Occurrence{occ(30), occ(32)}},
		{Source: "bass", Role: motif.RoleTheme, Salience: 0.4, Occurrences: []motif.Occurrence{occ(40)}},
	}

	ranked, err := motif.Rank(motifs, opts...)
	if err != nil {
		panic(err)
	}

	out := []string{}

	for _, m := range ranked {
		if m.Leitmotif {
			out = append(out, m.ID)
		}
	}

	return out
}

func ExampleWithLeitmotifs() {
	fmt.Println(rankedLeitmotifs(), rankedLeitmotifs(motif.WithLeitmotifs(1)))
	// Output:
	// [M1 M3] [M1]
}

func ExampleWithLeitmotifOverlap() {
	// M2 shares a quarter of its time with M1.
	perSource := motif.WithLeitmotifsPerSource(3)
	fmt.Println(rankedLeitmotifs(perSource), rankedLeitmotifs(perSource, motif.WithLeitmotifOverlap(0.2)))
	// Output:
	// [M1 M2 M3] [M1 M3]
}

func ExampleWithLeitmotifMinRatio() {
	fmt.Println(rankedLeitmotifs(motif.WithLeitmotifsPerSource(3)), rankedLeitmotifs(motif.WithLeitmotifsPerSource(3), motif.WithLeitmotifMinRatio(0.3)))
	// Output:
	// [M1 M2 M3] [M1 M2 M3 M4]
}

func ExampleWithLeitmotifsPerSource() {
	fmt.Println(rankedLeitmotifs(), rankedLeitmotifs(motif.WithLeitmotifsPerSource(1)))
	// Output:
	// [M1 M3] [M1]
}
