package motif

import (
	"errors"
	"math"
	randv1 "math/rand"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

func mustGrid(tb testing.TB, bpm float64, duration float64) rhythm.Grid {
	tb.Helper()

	g, _ := newGrid(tb, bpm, 0, 0, duration)

	return g
}

// motifSong places a 5-note motif (intervals +4 +3 -2 +5, IOIs 2 2 4 2) at
// bar 0, bar 4 (+2) and bar 9 (+5, last interval varied) among seeded random
// filler notes. shift moves one note of the second statement by an octave.
// Ported from AudioVisualizer's motif_test.go with the same random source.
func motifSong(tb testing.TB, shift int) ([]melody.CleanNote, rhythm.Grid) {
	tb.Helper()

	g := mustGrid(tb, 120, 40)
	rng := randv1.New(randv1.NewSource(7))

	type statement struct {
		bar, base int
		last      int
	}

	statements := []statement{{0, 60, 5}, {4, 62, 5}, {9, 65, 6}}
	notes := []melody.CleanNote{}
	add := func(slot, midi int) { notes = append(notes, slotNote(g, slot, 1, midi, 0.7)) }
	at := func(bar int) int { return slices.IndexFunc(statements, func(s statement) bool { return s.bar == bar }) }

	for bar := range 14 {
		// Statements are separated from the filler by more than the maximum
		// gap.
		slot, end := 16*bar, 16*(bar+1)
		if at(bar+1) >= 0 {
			end -= 6
		}

		if i := at(bar); i >= 0 {
			s := statements[i]

			pitch := []int{s.base, s.base + 4, s.base + 7, s.base + 5, s.base + 5 + s.last}
			if i == 1 {
				pitch[2] += shift
			}

			for k, at := range []int{0, 2, 4, 8, 10} {
				add(slot+at, pitch[k])
			}

			continue
		}

		for slot < end {
			add(slot, 55+rng.Intn(21))
			slot += 1 + rng.Intn(3)
		}
	}

	return notes, g
}

func ngramOptions() []Option { return []Option{WithGridWindows(), WithLengths(8, 6, 5, 4)} }

func findNotes(tb testing.TB, notes []melody.CleanNote, g rhythm.Grid, opts ...Option) []Motif {
	tb.Helper()

	motifs, err := FindNoteMotifs(notes, g, "lead", opts...)
	if err != nil {
		tb.Fatal(err)
	}

	return motifs
}

func TestNoteMotifRecursTransposedAndVaried(t *testing.T) {
	t.Parallel()

	notes, g := motifSong(t, 0)

	motifs := findNotes(t, notes, g, ngramOptions()...)
	if len(motifs) != 1 {
		t.Fatalf("expected one motif, got %d: %+v", len(motifs), motifs)
	}

	m := motifs[0]
	if len(m.Occurrences) != 3 || len(m.Intervals) != 4 {
		t.Fatalf("motif %+v", m)
	}

	for i, want := range []struct {
		bar, t  int
		variant Variant
	}{{0, 0, ""}, {4, 2, VariantTransposed}, {9, 5, VariantTransposedVaried}} {
		o := m.Occurrences[i]
		if o.Bar != want.bar || o.Transposition != want.t || (want.variant != "" && o.Variant != want.variant) {
			t.Fatalf("occurrence %d: %+v, want %+v", i, o, want)
		}
	}
}

func TestNoteMotifToleratesOctaveError(t *testing.T) {
	t.Parallel()

	notes, g := motifSong(t, 12)

	motifs := findNotes(t, notes, g, ngramOptions()...)
	if len(motifs) != 1 || len(motifs[0].Occurrences) != 3 || motifs[0].Occurrences[1].Transposition != 2 {
		t.Fatalf("octave-displaced statement not matched: %+v", motifs)
	}
}

func TestChromaMotifFindsTransposition(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 30)
	rng := randv1.New(randv1.NewSource(3))

	beats := make([][12]float64, 48)
	for i := range beats {
		beats[i][rng.Intn(12)] = 1
	}

	pattern := [][]int{{0, 4, 7}, {5, 9, 0}, {7, 11, 2}, {0, 4, 7}}
	for _, at := range []struct{ beat, t int }{{0, 0}, {16, 3}, {32, 0}} {
		for k, chord := range pattern {
			beats[at.beat+k] = [12]float64{}
			for _, pc := range chord {
				beats[at.beat+k][(pc+at.t)%12] = 1
			}
		}
	}

	motifs, err := FindChromaMotifs(beats, g, WithChromaWindows(4))
	if err != nil {
		t.Fatal(err)
	}

	if len(motifs) == 0 {
		t.Fatal("no chroma motif")
	}

	got := []int{}
	for _, o := range motifs[0].Occurrences {
		got = append(got, o.Transposition)
	}

	if !slices.Equal(got, []int{0, 3, 0}) || motifs[0].Occurrences[1].Start != g.BeatStart(16) {
		t.Fatalf("occurrences %+v", motifs[0].Occurrences)
	}
}

func TestOstinatoRanksBelowCrossSectionTheme(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 80)
	notes := []melody.CleanNote{}
	occurrence := func(bar int) Occurrence {
		o := Occurrence{Start: g.BarStart(bar), End: g.BarStart(bar) + 1, Bar: bar, Similarity: 0.9}
		for k := range 4 {
			o.NoteIndices = append(o.NoteIndices, len(notes))
			notes = append(notes, melody.CleanNote{Strength: 0.7, MIDI: 60 + 2*k})
		}

		return o
	}

	ostinato := Motif{Source: "lead", Intervals: []int{2, 3, -1}, SpanBeats: 4}
	for bar := 2; bar < 8; bar++ {
		ostinato.Occurrences = append(ostinato.Occurrences, occurrence(bar))
	}

	theme := Motif{Source: "lead", Intervals: []int{4, -2, 5}, SpanBeats: 4}
	for _, bar := range []int{1, 12, 30} {
		theme.Occurrences = append(theme.Occurrences, occurrence(bar))
	}

	sections := []Span{{"intro", 0, 20}, {"verse", 20, 50}, {"outro", 50, 80}}
	energy := func(float64, float64) float64 { return 0.8 }

	for _, m := range []*Motif{&ostinato, &theme} {
		err := Score(m, notes, sections, energy, g)
		if err != nil {
			t.Fatal(err)
		}
	}

	if ostinato.Role != RoleOstinato || theme.Role != RoleTheme {
		t.Fatalf("roles %s %s", ostinato.Role, theme.Role)
	}

	ranked, err := Rank([]Motif{ostinato, theme})
	if err != nil {
		t.Fatal(err)
	}

	if ranked[0].Role != RoleTheme || ranked[0].ID != "M1" || ranked[0].Rank != 1 || ranked[1].Rank != 2 || !ranked[1].Leitmotif {
		t.Fatalf("ranking %+v", ranked)
	}
}

func TestGridMotifToleratesDroppedNotesAndOctaves(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	rng := randv1.New(randv1.NewSource(5))
	pattern := []int{79, 74, 67, 78, 74, 67, 69, 71, 69, 67, 64, 67, 67, 71, 74, 79}
	notes := []melody.CleanNote{}

	for bar := range 8 {
		for k, midi := range pattern {
			slot := 16*bar + k

			switch {
			case bar >= 3 && bar != 6:
				midi = 55 + rng.Intn(24)
			case k == (bar*5)%16:
				continue // dropped by the tracker
			case k == 3 && bar == 1:
				midi -= 12 // octave error
			}

			notes = append(notes, slotNote(g, slot, 1, midi, 0.6))
		}
	}

	motifs := findNotes(t, notes, g, WithLengths())
	if len(motifs) == 0 {
		t.Fatal("no grid motif")
	}

	bars := []int{}

	for _, o := range motifs[0].Occurrences {
		bars = append(bars, o.Bar)
		if o.Transposition != 0 || o.Slot%16 != 0 {
			t.Fatalf("occurrence %+v", o)
		}
	}

	if !slices.Equal(bars, []int{0, 1, 2, 6}) || motifs[0].SpanBeats != 4 {
		t.Fatalf("bars %v span %v", bars, motifs[0].SpanBeats)
	}

	err := Score(&motifs[0], notes, nil, func(float64, float64) float64 { return 1 }, g)
	if err != nil {
		t.Fatal(err)
	}

	if motifs[0].Role != RoleOstinato {
		t.Fatalf("role %s", motifs[0].Role)
	}
}

// TestPlantedMotifFound is the PLAN fixture: an 8-note motif returns
// transposed by +5, varied (one note changed) and with an octave error, and
// every statement is found with the right transposition, in one motif.
func TestPlantedMotifFound(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)

	for _, opts := range [][]Option{nil, {WithGridWindows()}} {
		motifs := findNotes(t, plantedSong(g, 1), g, opts...)
		if len(motifs) == 0 {
			t.Fatal("no motif")
		}

		m := motifs[0]
		if len(m.Occurrences) != len(plantedStatements) {
			t.Fatalf("motif %+v", m)
		}

		for i, s := range plantedStatements {
			o := m.Occurrences[i]
			if o.Bar != s.bar || o.Transposition != s.transpose {
				t.Errorf("statement %d: occurrence %+v, want bar %d transposition %d", i, o, s.bar, s.transpose)
			}
		}

		wantVariants := []Variant{VariantExact, VariantTransposed, VariantVaried, VariantVaried}
		if len(opts) == 0 {
			// The grid pass reports the shape of the whole window.
			wantVariants[0] = m.Occurrences[0].Variant
		}

		for i, v := range wantVariants {
			if m.Occurrences[i].Variant != v {
				t.Errorf("statement %d: variant %s, want %s", i, m.Occurrences[i].Variant, v)
			}
		}

		if len(opts) > 0 && !slices.Equal(m.PrototypeMIDI, plantedPitches) {
			t.Errorf("prototype %v, want %v", m.PrototypeMIDI, plantedPitches)
		}

		for _, other := range motifs[1:] {
			for _, o := range other.Occurrences {
				for _, s := range plantedStatements {
					if o.Bar == s.bar {
						t.Errorf("statement at bar %d also claimed by %+v", s.bar, other)
					}
				}
			}
		}
	}
}

// TestArpeggioIsOneOstinato is the PLAN fixture: a bar-long repeating
// arpeggio is found once, as an ostinato, and not as 16 rotations.
func TestArpeggioIsOneOstinato(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	notes := arpeggioSong(g, 8)

	motifs := findNotes(t, notes, g)
	if len(motifs) != 1 {
		t.Fatalf("expected one motif, got %d: %+v", len(motifs), motifs)
	}

	m := &motifs[0]
	for i, o := range m.Occurrences {
		if o.Bar != i || o.Slot != 16*i || o.Transposition != 0 || o.Variant != VariantExact {
			t.Fatalf("occurrence %d: %+v", i, o)
		}
	}

	if len(m.Occurrences) != 8 || m.SpanBeats != 4 {
		t.Fatalf("motif %+v", m)
	}

	err := Score(m, notes, sectionsOf(g), energyOf("lead"), g)
	if err != nil {
		t.Fatal(err)
	}

	if m.Role != RoleOstinato {
		t.Fatalf("role %s", m.Role)
	}

	// The n-grams alone find one figure as well.
	motifs = findNotes(t, notes, g, WithGridWindows())
	if len(motifs) != 1 {
		t.Fatalf("n-gram pass: expected one motif, got %d", len(motifs))
	}
}

// TestChromaFallbackFindsPlantedMotif is the PLAN fixture: with the notes
// removed, the beat chroma of the planted song still yields the motif with
// the right transpositions.
func TestChromaFallbackFindsPlantedMotif(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	beats := plantedChroma(plantedSong(g, 1), 16, 1)

	motifs, err := FindChromaMotifs(beats, g)
	if err != nil {
		t.Fatal(err)
	}

	for _, m := range motifs {
		bars, trans := []int{}, []int{}
		for _, o := range m.Occurrences {
			bars, trans = append(bars, o.Bar), append(trans, o.Transposition)
		}

		if slices.Equal(bars, []int{0, 4, 8, 12}) {
			if !slices.Equal(trans, []int{0, 5, 0, 0}) || m.SpanBeats != 4 || m.Source != SourceChroma {
				t.Fatalf("motif %+v", m)
			}

			return
		}
	}

	t.Fatalf("planted motif not found: %+v", motifs)
}

// TestNoteMotifsIgnoreInputOrder checks that shuffling the notes changes
// only the note indices, which follow the notes.
func TestNoteMotifsIgnoreInputOrder(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)

	notes := append(plantedSong(g, 3), arpeggioSong(g, 4)...)
	for i := range notes {
		notes[i].RawIndex = i
	}

	want := findNotes(t, notes, g)
	rng := rand.New(rand.NewPCG(50, 50))

	for range 5 {
		perm := rng.Perm(len(notes))
		shuffled := make([]melody.CleanNote, len(notes))

		for to, from := range perm {
			shuffled[to] = notes[from]
		}

		got := findNotes(t, shuffled, g)
		for i := range got {
			for k := range got[i].Occurrences {
				for j, idx := range got[i].Occurrences[k].NoteIndices {
					got[i].Occurrences[k].NoteIndices[j] = perm[idx]
				}
			}
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatal("result depends on the input order")
		}
	}
}

func TestCorroborate(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40) // 0.5 s beats
	occ := func(start float64, t int) Occurrence {
		return Occurrence{Start: start, End: start + 2, Transposition: t}
	}
	note := Motif{Source: "lead", Occurrences: []Occurrence{occ(0, 0), occ(4, 2), occ(8, 5), occ(12, 0), occ(16, 1)}}
	chroma := []Motif{
		{Source: SourceChroma, Occurrences: []Occurrence{occ(0.4, 3), occ(4.2, 5), occ(12, 4)}},
		{Source: SourceChroma, Occurrences: []Occurrence{occ(0.1, 1), occ(4.4, 3), occ(8.2, 6), occ(15.5, 2)}},
	}

	for _, tc := range []struct {
		chroma []Motif
		share  float64
		want   bool
	}{
		{chroma[:1], 0.6, false}, // 2 of 5 at offset 3, 1 at offset 4
		{chroma[:1], 0.4, true},
		{chroma[1:], 0.6, true}, // 4 of 5 at offset 1
		{chroma[1:], 0.9, false},
		{nil, 0, false},
	} {
		m := note
		m.Occurrences = slices.Clone(note.Occurrences)

		err := Corroborate(&m, tc.chroma, g, WithConfirmShare(tc.share))
		if err != nil {
			t.Fatal(err)
		}

		if m.ConfirmedByChroma != tc.want {
			t.Errorf("share %v: confirmed %v, want %v", tc.share, m.ConfirmedByChroma, tc.want)
		}
	}
}

func TestScoreTerms(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	notes := []melody.CleanNote{{Strength: 0.4}, {Strength: 0.8}, {Strength: 0.7}, {Strength: 0.2}, {Strength: 0.6}}
	m := Motif{
		Source: "lead", Intervals: []int{2, 2, -4}, SpanBeats: 2, ConfirmedByChroma: true,
		Occurrences: []Occurrence{
			{Start: 0, End: 1, Similarity: 1, NoteIndices: []int{0, 1}},
			{Start: 10, End: 11, Similarity: 0.8, NoteIndices: []int{1, 2}},
		},
	}
	sections := []Span{{"a", 0, 5}, {"b", 5, 20}, {"a", 20, 30}}

	err := Score(&m, notes, sections, func(t0, _ float64) float64 { return 1 + t0/10 }, g)
	if err != nil {
		t.Fatal(err)
	}

	// Prominence: mean energy 1.5 × mean similarity 0.9 × mean strength
	// 0.675 / median 0.6. Entropy of {2, 2, -4} is 0.918 bits, scaled by 0.3.
	entropy := -(2.0/3*math.Log2(2.0/3) + 1.0/3*math.Log2(1.0/3))
	want := SalienceTerms{
		Count: r3(math.Log(3)), Sections: r3(math.Log(3)), Prominence: r3(1.5 * 0.9 * 0.675 / 0.6), Distinctiveness: r3(0.3 * entropy),
		Span: 0.25, Confirmed: 1.2,
	}

	if m.SalienceTerms != want {
		t.Fatalf("terms %+v, want %+v", m.SalienceTerms, want)
	}

	if m.Role != RoleTheme || m.Salience != r3(math.Log(3)*math.Log(3)*1.5*0.9*0.675/0.6*0.3*entropy*0.25*1.2) {
		t.Fatalf("role %s salience %v", m.Role, m.Salience)
	}

	chroma := Motif{Source: SourceChroma, Notes: []string{"C", "E", "G", "C"}, SpanBeats: 12, Occurrences: []Occurrence{{Start: 1, End: 2, Similarity: 1}}}

	err = Score(&chroma, nil, nil, func(float64, float64) float64 { return 2 }, g)
	if err != nil {
		t.Fatal(err)
	}

	if chroma.SalienceTerms.Distinctiveness != 1 || chroma.SalienceTerms.Span != 1 || chroma.SalienceTerms.Sections != 0 ||
		chroma.SalienceTerms.Prominence != 2 || chroma.Salience != 0 {
		t.Fatalf("chroma terms %+v salience %v", chroma.SalienceTerms, chroma.Salience)
	}
}

func TestRankLimits(t *testing.T) {
	t.Parallel()

	occ := func(start float64) Occurrence { return Occurrence{Start: start, End: start + 2} }
	motifs := []Motif{
		{Source: SourceChroma, Salience: 9, Role: RoleTheme, Occurrences: []Occurrence{occ(50)}},
		{Source: "lead", Salience: 1, Role: RoleTheme, Occurrences: []Occurrence{occ(0), occ(10)}},
		{Source: "keys", Salience: 0.9, Role: RoleTheme, Occurrences: []Occurrence{occ(1), occ(11)}}, // overlaps the first by half
		{Source: "lead", Salience: 0.8, Role: RoleOstinato, Occurrences: []Occurrence{occ(20), occ(22)}},
		{Source: "lead", Salience: 0.7, Role: RoleTheme, Occurrences: []Occurrence{occ(30)}}, // third lead
		{Source: "bass", Salience: 0.6, Role: RoleTheme, Occurrences: []Occurrence{occ(40)}},
		{Source: "bass", Salience: 0.3, Role: RoleTheme, Occurrences: []Occurrence{occ(60)}}, // below the ratio
	}

	ranked, err := Rank(motifs)
	if err != nil {
		t.Fatal(err)
	}

	ids, leit := []string{}, map[string]int{}

	for _, m := range ranked {
		ids = append(ids, m.ID)
		if m.Leitmotif {
			leit[m.ID] = m.Rank
		}

		for _, o := range m.Occurrences {
			if o.MotifID != m.ID {
				t.Fatalf("occurrence id %s in %s", o.MotifID, m.ID)
			}
		}
	}

	if !slices.Equal(ids, []string{"M1", "M2", "M3", "M4", "M5", "M6", "C1"}) {
		t.Fatalf("ids %v", ids)
	}

	if !reflect.DeepEqual(leit, map[string]int{"M1": 1, "M3": 2, "M5": 3}) {
		t.Fatalf("leitmotifs %v", leit)
	}
}

func TestOptionsAndErrors(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	m := Motif{}
	energy := func(float64, float64) float64 { return 1 }

	for i, opt := range []Option{
		nil, WithGridWindows(16, 1), WithGridMinNotes(0), WithGridMinSimilarity(1.5), WithGridTranspositionPenalty(math.NaN()),
		WithLengths(1), WithMaxSpanSlots(0), WithMaxGapSlots(-1), WithMinSimilarity(-0.1), WithMinOccurrences(1),
		WithDistanceWeights(-1, 0.3), WithDistanceWeights(0.7, math.Inf(1)), WithShiftOverlap(2), WithCoveredShare(-1),
		WithChromaWindows(0), WithChromaMinSimilarity(math.NaN()), WithConfirmShare(1.1), WithOstinatoShare(-0.5),
		WithLeitmotifs(-1), WithLeitmotifOverlap(-1), WithLeitmotifMinRatio(math.Inf(1)), WithLeitmotifsPerSource(-1),
	} {
		_, err1 := FindNoteMotifs(nil, g, "lead", opt)
		_, err2 := FindChromaMotifs(nil, g, opt)
		err3 := Corroborate(&m, nil, g, opt)
		err4 := Score(&m, nil, nil, energy, g, opt)
		_, err5 := Rank(nil, opt)

		for _, err := range []error{err1, err2, err3, err4, err5} {
			if !errors.Is(err, ErrInvalidOption) {
				t.Errorf("option %d: error %v, want ErrInvalidOption", i, err)
			}
		}
	}

	var zero rhythm.Grid

	_, err1 := FindNoteMotifs(nil, zero, "lead")
	_, err2 := FindChromaMotifs(nil, zero)
	_, err3 := FindNoteMotifs(nil, g, SourceChroma)
	_, err4 := FindChromaMotifs([][12]float64{{0, math.NaN()}}, g)
	_, err5 := FindNoteMotifs([]melody.CleanNote{{Slot: math.MaxInt / 2}}, g, "lead")
	err6 := Corroborate(nil, nil, g)
	err7 := Corroborate(&m, nil, zero)
	err8 := Score(nil, nil, nil, energy, g)
	err9 := Score(&m, nil, nil, nil, g)
	err10 := Score(&Motif{Occurrences: []Occurrence{{NoteIndices: []int{3}}}}, nil, nil, energy, g)
	err11 := Score(&m, nil, nil, energy, zero)

	for i, err := range []error{err1, err2, err3, err4, err5, err6, err7, err8, err9, err10, err11} {
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("case %d: error %v, want ErrInvalidArgument", i+1, err)
		}
	}

	// Empty input is valid and finds nothing.
	if motifs := findNotes(t, nil, g); len(motifs) != 0 {
		t.Fatalf("motifs from no notes: %+v", motifs)
	}
}

// TestDistanceTables checks the tabulated n-gram distance against the
// direct formula, including values outside the tables.
func TestDistanceTables(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	rng := rand.New(rand.NewPCG(51, 51))

	for _, maxIOI := range []int{8, maxUnitTable + 1} {
		d := newNoteDistance(cfg, maxIOI)

		for range 2000 {
			n := 1 + rng.IntN(7)
			ia, ib, oa, ob := make([]int, n), make([]int, n), make([]int, n), make([]int, n)

			for k := range n {
				ia[k], ib[k] = rng.IntN(61)-30, rng.IntN(61)-30
				oa[k], ob[k] = 1+rng.IntN(maxIOI), 1+rng.IntN(maxIOI)
			}

			p := refDefaultMotifParams()
			if got, want := d.distance(ia, ib, oa, ob), refNoteDistance(ia, ib, oa, ob, p); got != want {
				t.Fatalf("distance %v, want %v", got, want)
			}
		}
	}
}

// TestCompareNotesIsTotal checks the processing order on notes that differ
// in one field only.
func TestCompareNotesIsTotal(t *testing.T) {
	t.Parallel()

	base := melody.CleanNote{Slot: 4, Start: 0.5, End: 0.7, Slots: 2, MIDI: 60, Strength: 0.5, Voice: melody.VoiceArp}
	variants := []func(n *melody.CleanNote){
		func(n *melody.CleanNote) { n.Slot++ }, func(n *melody.CleanNote) { n.Start += 0.01 },
		func(n *melody.CleanNote) { n.End += 0.01 }, func(n *melody.CleanNote) { n.Slots++ },
		func(n *melody.CleanNote) { n.MIDI++ }, func(n *melody.CleanNote) { n.Strength += 0.1 },
		func(n *melody.CleanNote) { n.RawMIDI++ }, func(n *melody.CleanNote) { n.OctaveShift += 12 },
		func(n *melody.CleanNote) { n.Voice = melody.VoiceLead }, func(n *melody.CleanNote) { n.OffsetMS++ },
		func(n *melody.CleanNote) { n.OffGrid = true }, func(n *melody.CleanNote) { n.RawIndex++ },
	}

	for i, change := range variants {
		hi := base
		change(&hi)

		if compareNotes(&base, &hi) >= 0 || compareNotes(&hi, &base) <= 0 || compareNotes(&hi, &hi) != 0 {
			t.Errorf("field %d does not order notes", i)
		}
	}
}

// TestSizeOptionsAreCapped checks that huge sizes are rejected by the
// options instead of panicking in make or running away in the passes.
func TestSizeOptionsAreCapped(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	notes, _ := motifSong(t, 0)

	for i, opt := range []Option{
		WithGridWindows(1 << 60), WithGridWindows(16, MaxSize+1), WithLengths(1 << 60), WithLengths(MaxSize + 1),
		WithMaxSpanSlots(MaxSize + 1), WithMaxSpanSlots(math.MaxInt), WithChromaWindows(1 << 60), WithChromaWindows(MaxSize + 1),
	} {
		_, err := FindNoteMotifs(notes, g, "lead", opt)
		if !errors.Is(err, ErrInvalidOption) {
			t.Errorf("option %d: FindNoteMotifs error %v, want ErrInvalidOption", i, err)
		}

		_, err = FindChromaMotifs([][12]float64{{1}}, g, opt)
		if !errors.Is(err, ErrInvalidOption) {
			t.Errorf("option %d: FindChromaMotifs error %v, want ErrInvalidOption", i, err)
		}
	}

	// The limit itself is accepted.
	_, err := FindNoteMotifs(notes, g, "lead", WithGridWindows(MaxSize), WithLengths(MaxSize), WithMaxSpanSlots(MaxSize))
	if err != nil {
		t.Fatal(err)
	}

	_, err = FindChromaMotifs([][12]float64{{1}}, g, WithChromaWindows(MaxSize))
	if err != nil {
		t.Fatal(err)
	}
}

func TestNilOptionWrapsBothSentinels(t *testing.T) {
	t.Parallel()

	_, err := Rank(nil, WithLeitmotifs(2), nil)
	if !errors.Is(err, ErrNilOption) || !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("error %v, want ErrNilOption and ErrInvalidOption", err)
	}

	_, err = Rank(nil, WithLeitmotifs(-1))
	if errors.Is(err, ErrNilOption) || !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("error %v, want ErrInvalidOption only", err)
	}
}

// TestScoreGuardsNonFinite checks that a zero median strength skips the
// strength factor and that non-finite energies and saliences are rejected.
func TestScoreGuardsNonFinite(t *testing.T) {
	t.Parallel()

	g := mustGrid(t, 120, 40)
	motif := func() Motif {
		return Motif{
			Source: "lead", Intervals: []int{2, -1}, SpanBeats: 8,
			Occurrences: []Occurrence{
				{Start: 0, End: 1, Similarity: 1, NoteIndices: []int{0, 1}},
				{Start: 10, End: 11, Similarity: 1, NoteIndices: []int{2, 3}},
			},
		}
	}
	energy := func(float64, float64) float64 { return 2 }

	// Median strength 0: the strength factor is left out.
	m := motif()

	err := Score(&m, []melody.CleanNote{{Strength: 0}, {Strength: 0}, {Strength: 0.5}, {Strength: 0}}, nil, energy, g)
	if err != nil {
		t.Fatal(err)
	}

	if m.SalienceTerms.Prominence != 2 || !finite(m.Salience) {
		t.Fatalf("zero median: terms %+v salience %v", m.SalienceTerms, m.Salience)
	}

	notes := []melody.CleanNote{{Strength: 0.5}, {Strength: 0.6}, {Strength: 0.7}, {Strength: 0.8}}

	for _, e := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		m := motif()

		err := Score(&m, notes, nil, func(float64, float64) float64 { return e }, g)
		if !errors.Is(err, ErrInvalidArgument) || !reflect.DeepEqual(m, motif()) {
			t.Fatalf("energy %v: error %v, motif %+v", e, err, m)
		}
	}

	m = motif()
	notes[1].Strength = math.Inf(1)

	err = Score(&m, notes, nil, energy, g)
	if !errors.Is(err, ErrInvalidArgument) || !reflect.DeepEqual(m, motif()) {
		t.Fatalf("infinite strength: error %v, motif %+v", err, m)
	}
}

func TestRankRejectsNonFiniteSalience(t *testing.T) {
	t.Parallel()

	for _, s := range []float64{math.NaN(), math.Inf(1)} {
		motifs := []Motif{{Source: "lead", Salience: 1}, {Source: "bass", Salience: s, Leitmotif: true, Rank: 3}}

		_, err := Rank(motifs)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("salience %v: error %v", s, err)
		}

		if motifs[0].ID != "" || !motifs[1].Leitmotif || motifs[1].Rank != 3 {
			t.Fatalf("salience %v: motifs changed: %+v", s, motifs)
		}
	}
}

// TestRankIsIdempotent checks that Rank clears earlier leitmotif marks, so
// a second call (or stale marks) gives the same result as a fresh call.
func TestRankIsIdempotent(t *testing.T) {
	t.Parallel()

	occ := func(start float64) Occurrence { return Occurrence{Start: start, End: start + 2} }
	fresh := func() []Motif {
		return []Motif{
			{Source: "lead", Salience: 1, Role: RoleTheme, Occurrences: []Occurrence{occ(0)}},
			{Source: "lead", Salience: 0.9, Role: RoleTheme, Occurrences: []Occurrence{occ(10)}},
			{Source: "keys", Salience: 0.8, Role: RoleTheme, Occurrences: []Occurrence{occ(20)}},
			{Source: "bass", Salience: 0.2, Role: RoleTheme, Occurrences: []Occurrence{occ(30)}}, // below the ratio
		}
	}

	want, err := Rank(fresh())
	if err != nil {
		t.Fatal(err)
	}

	again, err := Rank(slices.Clone(want))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(again, want) {
		t.Fatalf("second call %+v, want %+v", again, want)
	}

	// Stale marks from elsewhere are cleared.
	stale := fresh()
	stale[3].Leitmotif, stale[3].Rank = true, 7

	got, err := Rank(stale)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stale marks: %+v, want %+v", got, want)
	}

	if got[3].Leitmotif || got[3].Rank != 0 || !got[2].Leitmotif || got[2].Rank != 3 {
		t.Fatalf("leitmotifs %+v", got)
	}
}
