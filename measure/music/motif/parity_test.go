package motif

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

func toRefNotes(notes []melody.CleanNote) []refStoryNote {
	out := make([]refStoryNote, len(notes))
	for i, n := range notes {
		out[i] = refStoryNote{
			Start: n.Start, End: n.End, Slot: n.Slot, Slots: n.Slots, MIDI: n.MIDI, RawMIDI: n.RawMIDI,
			OctaveShift: n.OctaveShift, Strength: n.Strength, Voice: string(n.Voice), OffsetMS: n.OffsetMS,
			OffGrid: n.OffGrid, RawIndex: n.RawIndex,
		}
	}

	return out
}

// toRef converts package motifs to the reference type, keeping nil slices
// nil, so reflect.DeepEqual compares them field by field.
func toRef(motifs []Motif) []refMotif {
	if motifs == nil {
		return nil
	}

	out := make([]refMotif, len(motifs))
	for i, m := range motifs {
		r := refMotif{
			ID: m.ID, Source: m.Source, Role: string(m.Role), Notes: m.Notes, SpanBeats: m.SpanBeats,
			Intervals: m.Intervals, IOI: m.IOI, PrototypeMIDI: m.PrototypeMIDI, Salience: m.Salience,
			SalienceTerms:     refSalienceTerms(m.SalienceTerms),
			ConfirmedByChroma: m.ConfirmedByChroma, Leitmotif: m.Leitmotif, Rank: m.Rank,
		}

		for _, o := range m.Occurrences {
			r.Occurrences = append(r.Occurrences, refMotifOccurrence{
				MotifID: o.MotifID, Start: o.Start, End: o.End, Bar: o.Bar, Slot: o.Slot,
				Transposition: o.Transposition, Similarity: o.Similarity, Variant: string(o.Variant),
				NoteIndices: o.NoteIndices,
			})
		}

		out[i] = r
	}

	return out
}

// optionsOf maps the reference parameters onto options, field by field.
func optionsOf(p refMotifParams) []Option {
	return []Option{
		WithGridWindows(p.GridWindows...),
		WithGridMinNotes(p.GridMinNotes),
		WithGridMinSimilarity(p.GridMinSimilarity),
		WithGridTranspositionPenalty(p.GridTransposeCost),
		WithLengths(p.Lengths...),
		WithMaxSpanSlots(p.MaxSpanSlots),
		WithMaxGapSlots(p.MaxGapSlots),
		WithMinSimilarity(p.MinSimilarity),
		WithMinOccurrences(p.MinOccurrences),
		WithDistanceWeights(p.IntervalWeight, p.IOIWeight),
		WithShiftOverlap(p.ShiftOverlap),
		WithCoveredShare(p.CoveredShare),
		WithChromaWindows(p.ChromaWindowBeats...),
		WithChromaMinSimilarity(p.ChromaMinSimilarity),
		WithConfirmShare(p.ConfirmShare),
		WithOstinatoShare(p.OstinatoShare),
		WithLeitmotifs(p.Leitmotifs),
		WithLeitmotifOverlap(p.LeitmotifOverlap),
		WithLeitmotifMinRatio(p.LeitmotifMinRatio),
		WithLeitmotifsPerSource(p.LeitmotifsPerSource),
	}
}

// randomParams draws a valid parameter set around the defaults.
func randomParams(rng *rand.Rand) refMotifParams {
	pick := func(sets ...[]int) []int { return slices.Clone(sets[rng.IntN(len(sets))]) }
	w := 0.4 + 0.5*rng.Float64()

	return refMotifParams{
		GridWindows:         pick([]int{16, 8}, []int{16}, []int{8, 4}, nil, []int{32, 16, 8}),
		GridMinNotes:        1 + rng.IntN(6),
		GridMinSimilarity:   0.3 + 0.5*rng.Float64(),
		GridTransposeCost:   0.2 * rng.Float64(),
		Lengths:             pick([]int{8, 6, 4}, []int{6, 4, 3}, []int{5}, nil, []int{10, 8, 6, 4, 2}),
		MaxSpanSlots:        8 + rng.IntN(40),
		MaxGapSlots:         rng.IntN(8),
		MinSimilarity:       0.6 + 0.35*rng.Float64(),
		MinOccurrences:      2 + rng.IntN(3),
		IntervalWeight:      w,
		IOIWeight:           1 - w,
		ShiftOverlap:        rng.Float64(),
		CoveredShare:        0.3 + 0.7*rng.Float64(),
		ChromaWindowBeats:   pick([]int{8, 4}, []int{4}, []int{6, 3, 2}, nil),
		ChromaMinSimilarity: 0.6 + 0.35*rng.Float64(),
		ConfirmShare:        rng.Float64(),
		OstinatoShare:       rng.Float64(),
		Leitmotifs:          rng.IntN(6),
		LeitmotifOverlap:    rng.Float64(),
		LeitmotifMinRatio:   rng.Float64(),
		LeitmotifsPerSource: rng.IntN(4),
	}
}

// song is the input of the story motif pipeline.
type song struct {
	name       string
	grid       rhythm.Grid
	ref        refGrid
	lead, bass []melody.CleanNote
	chroma     [][12]float64
	sections   []Span
	energy     func(source string) func(t0, t1 float64) float64
}

// pipeline runs the package the way AudioVisualizer's story motifs() should
// call it: chroma motifs, then per source note motifs corroborated and
// scored, then the chroma motifs scored with the harmony energy, then Rank.
func pipeline(tb testing.TB, s song, opts ...Option) []Motif {
	tb.Helper()

	chroma, err := FindChromaMotifs(s.chroma, s.grid, opts...)
	if err != nil {
		tb.Fatal(err)
	}

	all := []Motif{}

	for _, src := range []struct {
		name  string
		notes []melody.CleanNote
	}{{"lead", s.lead}, {"bass", s.bass}} {
		found, err := FindNoteMotifs(src.notes, s.grid, src.name, opts...)
		if err != nil {
			tb.Fatal(err)
		}

		for i := range found {
			err = Corroborate(&found[i], chroma, s.grid, opts...)
			if err == nil {
				err = Score(&found[i], src.notes, s.sections, s.energy(src.name), s.grid, opts...)
			}

			if err != nil {
				tb.Fatal(err)
			}
		}

		all = append(all, found...)
	}

	for i := range chroma {
		err = Score(&chroma[i], nil, s.sections, s.energy(SourceChroma), s.grid, opts...)
		if err != nil {
			tb.Fatal(err)
		}
	}

	ranked, err := Rank(append(all, chroma...), opts...)
	if err != nil {
		tb.Fatal(err)
	}

	return ranked
}

// refPipeline is AudioVisualizer's story motifs() on the reference copy.
func refPipeline(s song, p refMotifParams) []refMotif {
	spans := []refSpan{}
	for _, c := range s.sections {
		spans = append(spans, refSpan(c))
	}

	chroma := refFindChromaMotifs(s.chroma, s.ref, p)
	all := []refMotif{}

	for _, src := range []struct {
		name  string
		notes []refStoryNote
	}{{"lead", toRefNotes(s.lead)}, {"bass", toRefNotes(s.bass)}} {
		for _, m := range refFindNoteMotifs(src.notes, s.ref, src.name, p) {
			refCorroborate(&m, chroma, s.ref.BeatSeconds, p.ConfirmShare)
			refScore(&m, src.notes, spans, s.energy(src.name), s.ref, p)
			all = append(all, m)
		}
	}

	for _, m := range chroma {
		refScore(&m, nil, spans, s.energy(SourceChroma), s.ref, p)
		all = append(all, m)
	}

	return refRank(all, p)
}

// syntheticSongs are the parity fixtures: the PLAN fixtures and seeded
// phrase songs on grids with and without a pickup bar.
func syntheticSongs(tb testing.TB) []song {
	tb.Helper()

	out := []song{}
	add := func(name string, g rhythm.Grid, r refGrid, lead, bass []melody.CleanNote, chroma [][12]float64) {
		out = append(out, song{name: name, grid: g, ref: r, lead: lead, bass: bass, chroma: chroma, sections: sectionsOf(g), energy: energyOf})
	}

	g, r := newGrid(tb, 120, 0, 0, 40)
	planted := plantedSong(g, 1)
	add("planted", g, r, planted, arpeggioSong(g, 8), plantedChroma(planted, 16, 1))
	add("planted-chroma-only", g, r, nil, nil, plantedChroma(planted, 16, 2))

	for i, grid := range []struct {
		bpm, origin float64
		downbeat    int
	}{{120, 0, 0}, {105, 0.038, 0}, {97, 0.31, 1}, {133, 0.12, 3}, {88, 0.5, 2}, {140, 0, 5}} {
		g, r := newGrid(tb, grid.bpm, grid.origin, grid.downbeat, 90)
		rng := rand.New(rand.NewPCG(45, uint64(i)))
		lead := phraseSong(g, rng, 32)
		bass := phraseSong(g, rng, 24)

		for k := range bass {
			bass[k].MIDI -= 24
		}

		add("phrases", g, r, lead, bass, phraseChroma(g, lead, rng))
	}

	return out
}

// TestParityFindNoteMotifs compares FindNoteMotifs with the verbatim copy of
// AudioVisualizer's FindNoteMotifs bit for bit, with the default options and
// random parameter sets.
func TestParityFindNoteMotifs(t *testing.T) {
	t.Parallel()

	songs := syntheticSongs(t)
	rng := rand.New(rand.NewPCG(47, 47))

	for _, s := range songs {
		for i := range 8 {
			p, opts := refDefaultMotifParams(), []Option(nil)
			if i > 0 {
				p = randomParams(rng)
				opts = optionsOf(p)
			}

			for _, notes := range [][]melody.CleanNote{s.lead, s.bass} {
				got, err := FindNoteMotifs(notes, s.grid, "lead", opts...)
				if err != nil {
					t.Fatal(err)
				}

				want := refFindNoteMotifs(toRefNotes(notes), s.ref, "lead", p)
				if !reflect.DeepEqual(toRef(got), want) {
					t.Fatalf("%s, params %d (%+v): FindNoteMotifs differs from the reference:\ngot  %+v\nwant %+v", s.name, i, p, toRef(got), want)
				}
			}
		}
	}
}

// TestParityFindChromaMotifs compares FindChromaMotifs with the reference.
func TestParityFindChromaMotifs(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(48, 48))

	for _, s := range syntheticSongs(t) {
		for i := range 4 {
			p, opts := refDefaultMotifParams(), []Option(nil)
			if i > 0 {
				p = randomParams(rng)
				opts = optionsOf(p)
			}

			got, err := FindChromaMotifs(s.chroma, s.grid, opts...)
			if err != nil {
				t.Fatal(err)
			}

			want := refFindChromaMotifs(s.chroma, s.ref, p)
			if !reflect.DeepEqual(toRef(got), want) {
				t.Fatalf("%s, params %d: FindChromaMotifs differs from the reference:\ngot  %+v\nwant %+v", s.name, i, toRef(got), want)
			}
		}
	}
}

// TestParityPipeline compares the whole story pipeline (Corroborate, Score,
// Rank included) with the reference.
func TestParityPipeline(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(49, 49))

	for _, s := range syntheticSongs(t) {
		for i := range 4 {
			p, opts := refDefaultMotifParams(), []Option(nil)
			if i > 0 {
				p = randomParams(rng)
				opts = optionsOf(p)
			}

			got := toRef(pipeline(t, s, opts...))
			want := refPipeline(s, p)

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s, params %d: pipeline differs from the reference:\ngot  %+v\nwant %+v", s.name, i, got, want)
			}
		}
	}
}

// TestParityExercisesBranches guards the parity fixtures against becoming
// trivial: with the default options they must produce every variant, both
// roles, confirmed and unconfirmed motifs, leitmotifs and passed-over
// motifs, from note and chroma motifs.
func TestParityExercisesBranches(t *testing.T) {
	t.Parallel()

	variants := map[string]int{}
	roles := map[Role]int{}
	confirmed, leitmotifs, skipped, chroma, notes := 0, 0, 0, 0, 0

	for _, s := range syntheticSongs(t) {
		motifs := pipeline(t, s)

		best := 0.0

		for _, m := range motifs {
			if m.Source != SourceChroma {
				best = max(best, m.Salience)
			}
		}

		for _, m := range motifs {
			roles[m.Role]++

			if m.Source == SourceChroma {
				chroma++
			} else {
				notes++
			}

			if m.ConfirmedByChroma {
				confirmed++
			}

			switch {
			case m.Leitmotif:
				leitmotifs++
			case m.Source != SourceChroma && m.Salience >= DefaultLeitmotifMinRatio*best:
				skipped++ // overlap, per-source cap or count limit
			}

			for _, o := range m.Occurrences {
				variants[m.Source[:1]+string(o.Variant)]++
			}
		}
	}

	t.Logf("variants %v, roles %v, confirmed %d, leitmotifs %d, skipped %d, note motifs %d, chroma motifs %d",
		variants, roles, confirmed, leitmotifs, skipped, notes, chroma)

	for _, v := range []Variant{VariantExact, VariantTransposed, VariantVaried, VariantTransposedVaried} {
		if variants["l"+string(v)] == 0 || variants["c"+string(v)] == 0 {
			t.Errorf("no %s occurrence of a note and a chroma motif", v)
		}
	}

	if roles[RoleTheme] == 0 || roles[RoleOstinato] == 0 || confirmed == 0 || confirmed == notes ||
		leitmotifs == 0 || skipped == 0 || chroma == 0 || notes == 0 {
		t.Fatal("parity fixtures miss a branch")
	}
}
