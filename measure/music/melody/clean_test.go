package melody

import (
	"cmp"
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// cleanGrids returns the reference grid and the package grid of r.
func cleanGrids(tb testing.TB, r storyRhythm, duration float64) (storyGrid, rhythm.Grid) {
	tb.Helper()

	g, err := rhythm.NewGrid(r.BPM, r.BeatOrigin, r.Downbeat, duration)
	if err != nil {
		tb.Fatal(err)
	}

	return newStoryGrid(r, duration), g
}

// pixelGrid is the AudioVisualizer test grid: 105 BPM, first beat 38 ms.
func pixelGrid(tb testing.TB) (storyGrid, rhythm.Grid) {
	tb.Helper()

	return cleanGrids(tb, storyRhythm{BPM: 105, BeatOrigin: 0.038}, 30)
}

// gridNote is the AudioVisualizer test helper: a note on slot, slots long
// (ending 10 ms early), with the start moved by jitter seconds.
func gridNote(g rhythm.Grid, slot, slots, midi int, jitter float64) refNote {
	start := g.SlotTime(slot) + jitter

	return refNote{Start: start, End: g.SlotTime(slot+slots) - 0.01, MIDI: midi, Strength: 0.6}
}

func toNotes(in []refNote) []Note {
	out := make([]Note, len(in))
	for i, n := range in {
		out[i] = Note(n)
	}

	return out
}

// optionsOf maps the reference parameters onto clean options.
func optionsOf(p storyCleanParams) []CleanOption {
	return []CleanOption{
		WithFloorMIDI(p.FloorMIDI),
		WithMinStrength(p.MinStrength),
		WithOffGridMS(p.OffGridMS),
		WithOctaveWindow(p.OctaveWindowBars, p.OctaveSpread),
		WithNeighbourSpread(p.NeighbourSpread),
		WithOctavePeriod(p.PeriodSlots, p.PeriodVotes),
		WithArpeggio(p.ArpMinRun, p.ArpMaxGapSlots, p.ArpMaxNoteSlots),
	}
}

func bitsEqual(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

// requireCleanParity runs the reference and Clean and compares them bit
// for bit.
func requireCleanParity(t *testing.T, in []refNote, sg storyGrid, g rhythm.Grid, p storyCleanParams, opts ...CleanOption) ([]CleanNote, []DroppedNote) {
	t.Helper()

	want, raw := storyCleanNotes(in, sg, p)

	got, dropped, err := Clean(toNotes(in), g, opts...)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("%d notes, want %d:\n got %+v\nwant %+v", len(got), len(want), got, want)
	}

	for i, w := range want {
		n := got[i]
		if !bitsEqual(n.Start, w.Start) || !bitsEqual(n.End, w.End) || n.Slot != w.Slot || n.Slots != w.Slots ||
			n.MIDI != w.MIDI || n.RawMIDI != w.RawMIDI || n.OctaveShift != w.OctaveShift ||
			!bitsEqual(n.Strength, w.Strength) || string(n.Voice) != w.Voice || !bitsEqual(n.OffsetMS, w.OffsetMS) ||
			n.OffGrid != w.OffGrid || n.RawIndex != w.RawIndex {
			t.Fatalf("note %d:\n got %+v\nwant %+v", i, n, w)
		}
	}

	var wantDropped []storyRawNote

	for _, r := range raw {
		if r.Dropped != "" {
			wantDropped = append(wantDropped, r)
		}
	}

	if len(dropped) != len(wantDropped) {
		t.Fatalf("%d dropped, want %d", len(dropped), len(wantDropped))
	}

	for i, w := range wantDropped {
		d := dropped[i]
		if string(d.Reason) != w.Dropped || !bitsEqual(d.Start, w.Start) || !bitsEqual(d.End, w.End) ||
			d.MIDI != w.MIDI || !bitsEqual(d.Strength, w.Strength) || raw[d.Index] != w {
			t.Fatalf("dropped %d:\n got %+v\nwant %+v", i, d, w)
		}
	}

	return got, dropped
}

// randomLine returns count tracker-like notes on grid g: a stepwise line with
// planted octave errors, duplicates, floor and weak notes, jitter beyond the
// off-grid threshold, overlaps and a shuffled input order.
func randomLine(rng *rand.Rand, g rhythm.Grid, count, low int) []refNote {
	notes := make([]refNote, 0, count)
	slot, midi := rng.IntN(8)-2, low+18

	for range count {
		slot += rng.IntN(5) // 0 repeats the slot: a duplicate

		midi += rng.IntN(7) - 3
		midi = min(max(midi, low+6), low+30)

		m := midi

		switch r := rng.Float64(); {
		case r < 0.08:
			m += 12
		case r < 0.14:
			m -= 12
		case r < 0.16:
			m += 24
		case r < 0.20:
			m = low - rng.IntN(3) // floor
		}

		length := 1 + rng.IntN(6)
		start := g.SlotTime(slot) + 0.16*g.SlotSeconds()*(2*rng.Float64()-1)*3
		end := g.SlotTime(slot+length) + 0.03*(2*rng.Float64()-1)

		notes = append(notes, refNote{Start: start, End: end, MIDI: m, Strength: 0.2 + 0.8*rng.Float64()})
	}

	rng.Shuffle(len(notes), func(i, j int) { notes[i], notes[j] = notes[j], notes[i] })

	return notes
}

// periodicLine repeats a one-bar figure for bars bars with octave errors
// planted at random positions, so the period vote decides.
func periodicLine(rng *rand.Rand, g rhythm.Grid, bars int) []refNote {
	figure := []struct{ slot, slots, midi int }{
		{0, 2, 79}, {2, 2, 67}, {4, 2, 78}, {6, 2, 74}, {8, 1, 71}, {9, 1, 67}, {10, 2, 74}, {12, 4, 78},
	}

	var notes []refNote

	for bar := range bars {
		for _, f := range figure {
			n := gridNote(g, 16*bar+f.slot, f.slots, f.midi, 0.02*(2*rng.Float64()-1))
			n.Strength = 0.4 + 0.6*rng.Float64()

			switch r := rng.Float64(); {
			case r < 0.1:
				n.MIDI += 12
			case r < 0.2:
				n.MIDI -= 12
			case r < 0.23:
				n.MIDI -= 24
			case r < 0.26:
				n.MIDI += 24
			}

			notes = append(notes, n)
		}
	}

	return notes
}

type cleanCase struct {
	name   string
	rhythm storyRhythm
	notes  func(rng *rand.Rand, g rhythm.Grid) []refNote
}

func cleanCases() []cleanCase {
	pixel := storyRhythm{BPM: 105, BeatOrigin: 0.038}

	return []cleanCase{
		{name: "app-jitter", rhythm: pixel, notes: func(_ *rand.Rand, g rhythm.Grid) []refNote {
			return []refNote{gridNote(g, 4, 2, 67, 0.03), gridNote(g, 8, 2, 69, -0.06)}
		}},
		{name: "app-floor-duplicates", rhythm: pixel, notes: func(_ *rand.Rand, g rhythm.Grid) []refNote {
			weak := gridNote(g, 0, 1, 64, 0.01)
			weak.Strength = 0.4

			return []refNote{gridNote(g, 0, 2, 67, 0), weak, gridNote(g, 2, 1, 52, 0), gridNote(g, 3, 1, 69, 0)}
		}},
		{name: "app-octave-spike", rhythm: pixel, notes: func(_ *rand.Rand, g rhythm.Grid) []refNote {
			var in []refNote
			for i, m := range []int{60, 62, 64, 76, 65, 67} {
				in = append(in, gridNote(g, 2*i, 1, m, 0))
			}

			return in
		}},
		{name: "app-arp-lead", rhythm: pixel, notes: func(_ *rand.Rand, g rhythm.Grid) []refNote {
			var in []refNote
			for i := range 5 {
				in = append(in, gridNote(g, i, 1, 67+i, 0))
			}

			return append(in, gridNote(g, 8, 8, 72, 0), gridNote(g, 20, 1, 67, 0), gridNote(g, 21, 1, 69, 0))
		}},
		{name: "random-pixel", rhythm: pixel, notes: func(rng *rand.Rand, g rhythm.Grid) []refNote {
			return randomLine(rng, g, 400, 52)
		}},
		{name: "random-pickup", rhythm: storyRhythm{BPM: 131.7, BeatOrigin: 0.412, Downbeat: 3}, notes: func(rng *rand.Rand, g rhythm.Grid) []refNote {
			return randomLine(rng, g, 300, 52)
		}},
		{name: "random-dense", rhythm: storyRhythm{BPM: 88.25, BeatOrigin: 1.003, Downbeat: 2}, notes: func(rng *rand.Rand, g rhythm.Grid) []refNote {
			return append(randomLine(rng, g, 250, 52), randomLine(rng, g, 250, 52)...)
		}},
		{name: "periodic", rhythm: pixel, notes: func(rng *rand.Rand, g rhythm.Grid) []refNote {
			return periodicLine(rng, g, 24)
		}},
		{name: "periodic-mixed", rhythm: storyRhythm{BPM: 120, BeatOrigin: 0.25, Downbeat: 1}, notes: func(rng *rand.Rand, g rhythm.Grid) []refNote {
			return append(periodicLine(rng, g, 12), randomLine(rng, g, 100, 52)...)
		}},
		{name: "empty", rhythm: pixel, notes: func(*rand.Rand, rhythm.Grid) []refNote { return nil }},
	}
}

// TestCleanParity compares Clean with the verbatim copy of
// AudioVisualizer's CleanNotes bit for bit: default options against
// LeadCleanParams, BassCleanOptions against BassCleanParams on a bass line,
// and random parameter sets mapped onto options.
func TestCleanParity(t *testing.T) {
	t.Parallel()

	for ci, tc := range cleanCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sg, g := cleanGrids(t, tc.rhythm, 300)
			in := tc.notes(rand.New(rand.NewPCG(45, uint64(ci))), g)

			requireCleanParity(t, in, sg, g, storyLeadCleanParams())

			bass := make([]refNote, len(in))
			for i, n := range in {
				n.MIDI -= 24
				bass[i] = n
			}

			requireCleanParity(t, bass, sg, g, storyBassCleanParams(), BassCleanOptions()...)

			rng := rand.New(rand.NewPCG(46, uint64(ci)))
			for range 20 {
				p := storyCleanParams{
					FloorMIDI:        40 + rng.IntN(20),
					MinStrength:      rng.Float64() * 0.5,
					OffGridMS:        float64(rng.IntN(80)),
					OctaveWindowBars: 4 * rng.Float64(),
					OctaveSpread:     float64(rng.IntN(25)),
					NeighbourSpread:  rng.IntN(6),
					PeriodSlots:      1 + rng.IntN(32),
					PeriodVotes:      1 + rng.IntN(4),
					ArpMinRun:        1 + rng.IntN(6),
					ArpMaxGapSlots:   rng.IntN(4),
					ArpMaxNoteSlots:  1 + rng.IntN(4),
				}
				requireCleanParity(t, in, sg, g, p, optionsOf(p)...)
			}
		})
	}
}

// TestCleanParityExercisesBranches guards the parity fixtures against
// becoming trivial: together they must hit every branch of the reference.
func TestCleanParityExercisesBranches(t *testing.T) {
	t.Parallel()

	reasons := map[DropReason]int{}
	shifts := map[int]int{}
	voices := map[Voice]int{}
	offGrid, onGrid, replaced, kept := 0, 0, 0, 0

	for ci, tc := range cleanCases() {
		_, g := cleanGrids(t, tc.rhythm, 300)
		in := tc.notes(rand.New(rand.NewPCG(45, uint64(ci))), g)

		got, dropped, err := Clean(toNotes(in), g)
		if err != nil {
			t.Fatal(err)
		}

		bySlot := map[int]CleanNote{}

		for _, n := range got {
			shifts[n.OctaveShift]++
			voices[n.Voice]++
			bySlot[n.Slot] = n

			if n.OffGrid {
				offGrid++
			} else {
				onGrid++
			}
		}

		for _, d := range dropped {
			reasons[d.Reason]++

			if d.Reason != DropDuplicate {
				continue
			}

			// Within a slot the order is the input order: a winner with a
			// higher index replaced the dropped note, a lower one kept it.
			if w, ok := bySlot[g.Slot(d.Start)]; ok && w.RawIndex > d.Index {
				replaced++
			} else {
				kept++
			}
		}
	}

	t.Logf("reasons %v, shifts %v, voices %v, off-grid %d/%d, duplicates replaced %d kept %d",
		reasons, shifts, voices, offGrid, onGrid, replaced, kept)

	if reasons[DropFloor] == 0 || reasons[DropWeak] == 0 || reasons[DropDuplicate] == 0 ||
		shifts[12] == 0 || shifts[-12] == 0 || shifts[24] == 0 || shifts[-24] == 0 ||
		voices[VoiceArp] == 0 || voices[VoiceLead] == 0 || offGrid == 0 || onGrid == 0 || replaced == 0 || kept == 0 {
		t.Fatal("parity fixtures miss a branch")
	}
}

// TestCleanStepsAreExercised checks each octave-correction step on its own
// fixture, so every step contributes to the parity cases above.
func TestCleanStepsAreExercised(t *testing.T) {
	t.Parallel()

	_, g := pixelGrid(t)

	// Median step: a note 24 semitones above a steady line has no octave
	// neighbours (the fold needs ±12) and is moved one octave down.
	var line []refNote
	for i, m := range []int{60, 62, 60, 84, 62, 60, 62} {
		line = append(line, gridNote(g, 3*i, 1, m, 0))
	}

	got, _, err := Clean(toNotes(line), g)
	if err != nil {
		t.Fatal(err)
	}

	if got[3].MIDI != 72 || got[3].OctaveShift != -12 {
		t.Fatalf("median step: %+v", got[3])
	}

	// Period vote: a note an octave low in one bar of a repeated figure is
	// raised by the votes of the bars around it.
	var bars []refNote

	for bar := range 5 {
		m := 72
		if bar == 2 {
			m = 60
		}

		bars = append(bars, gridNote(g, 16*bar, 8, m, 0), gridNote(g, 16*bar+8, 8, 67, 0))
	}

	got, _, err = Clean(toNotes(bars), g)
	if err != nil {
		t.Fatal(err)
	}

	for i, n := range got {
		if want := []int{72, 67}[i%2]; n.MIDI != want {
			t.Fatalf("period vote note %d: %+v", i, n)
		}
	}

	if got[4].OctaveShift != 12 {
		t.Fatalf("period vote shift: %+v", got[4])
	}
}

// arpFigure is one bar of eighth notes with genuine leaps: G5 G4 F#5 D5 B4
// G4 D5 F#5.
var arpFigure = []int{79, 67, 78, 74, 71, 67, 74, 78}

// TestCleanArpeggioOctaveErrors plants octave errors in a repeated arpeggio
// and checks that they are corrected while the genuine leaps G5 G4 F#5
// survive (a lone-spike fold alone would flatten G4 onto its neighbours).
func TestCleanArpeggioOctaveErrors(t *testing.T) {
	t.Parallel()

	_, g := pixelGrid(t)

	planted := map[int]int{2*8 + 2: -12, 3*8 + 4: +12, 1*8 + 1: +12, 4*8 + 6: -12}

	var in []refNote

	for bar := range 6 {
		for k, m := range arpFigure {
			in = append(in, gridNote(g, 16*bar+2*k, 2, m+planted[8*bar+k], 0))
		}
	}

	got, dropped, err := Clean(toNotes(in), g)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(in) || len(dropped) != 0 {
		t.Fatalf("%d notes, %d dropped", len(got), len(dropped))
	}

	for i, n := range got {
		if want := arpFigure[i%8]; n.MIDI != want || n.Voice != VoiceArp || n.OctaveShift != -planted[i] {
			t.Fatalf("note %d (bar %d): %+v, want MIDI %d arp shift %d", i, i/8, n, want, -planted[i])
		}
	}
}

// TestCleanJitterIsStable moves every note start by up to ±30 ms: the slots,
// pitches and voices stay the same and no note is off-grid at 50 ms.
func TestCleanJitterIsStable(t *testing.T) {
	t.Parallel()

	_, g := pixelGrid(t)

	var plain []refNote

	for bar := range 6 {
		for k, m := range arpFigure {
			plain = append(plain, gridNote(g, 16*bar+2*k, 2, m, 0))
		}
	}

	want, _, err := Clean(toNotes(plain), g)
	if err != nil {
		t.Fatal(err)
	}

	rng := rand.New(rand.NewPCG(30, 30))

	for trial := range 50 {
		jittered := make([]refNote, len(plain))
		for i, n := range plain {
			n.Start += 0.06*rng.Float64() - 0.03
			jittered[i] = n
		}

		got, _, err := Clean(toNotes(jittered), g)
		if err != nil {
			t.Fatal(err)
		}

		for i, n := range got {
			w := want[i]
			if n.Slot != w.Slot || n.Slots != w.Slots || n.MIDI != w.MIDI || n.Voice != w.Voice || n.OffGrid ||
				math.Abs(n.OffsetMS) > 30.001 {
				t.Fatalf("trial %d note %d: %+v, want %+v", trial, i, n, w)
			}
		}
	}
}

// TestCleanPickupGrid quantises onto a grid with a pickup bar: slots count
// from the first beat, so the bar vote period stays aligned to the beats.
func TestCleanPickupGrid(t *testing.T) {
	t.Parallel()

	g, err := rhythm.NewGrid(120, 0.5, 2, 20)
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := Clean([]Note{{Start: 0.5, End: 0.74, MIDI: 60, Strength: 0.5}, {Start: 1.51, End: 1.74, MIDI: 62, Strength: 0.5}}, g)
	if err != nil {
		t.Fatal(err)
	}

	if got[0].Slot != 0 || got[1].Slot != 8 || g.Bar(got[0].Start) != 0 || g.Bar(got[1].Start) != 1 {
		t.Fatalf("pickup notes %+v", got)
	}
}

func TestCleanOptionsAndErrors(t *testing.T) {
	t.Parallel()

	_, g := pixelGrid(t)
	notes := []Note{{Start: 1, End: 1.2, MIDI: 60, Strength: 0.5}}

	for _, opt := range []CleanOption{
		WithFloorMIDI(-1), WithFloorMIDI(128),
		WithMinStrength(-0.1), WithMinStrength(math.NaN()),
		WithOffGridMS(-1), WithOffGridMS(math.Inf(1)),
		WithOctaveWindow(-1, 19), WithOctaveWindow(1, math.NaN()),
		WithNeighbourSpread(-1),
		WithOctavePeriod(0, 2), WithOctavePeriod(16, 0),
		WithArpeggio(0, 2, 2), WithArpeggio(4, -1, 2), WithArpeggio(4, 2, 0),
	} {
		_, _, err := Clean(notes, g, opt)
		if !errors.Is(err, ErrInvalidOption) {
			t.Errorf("error %v, want ErrInvalidOption", err)
		}
	}

	_, _, err := Clean(notes, g, nil)
	if !errors.Is(err, ErrNilOption) {
		t.Errorf("nil option: %v", err)
	}

	_, _, err = Clean(notes, rhythm.Grid{})
	if !errors.Is(err, rhythm.ErrInvalidArgument) || !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("zero grid: %v, want rhythm.ErrInvalidArgument and ErrInvalidArgument", err)
	}

	for _, bad := range []Note{
		{Start: math.NaN(), End: 1, MIDI: 60, Strength: 1},
		{Start: 0, End: math.Inf(1), MIDI: 60, Strength: 1},
		{Start: 0, End: 1, MIDI: 60, Strength: math.NaN()},
		{Start: 1e300, End: 1e300, MIDI: 60, Strength: 1},
		{Start: 0, End: -1e300, MIDI: 60, Strength: 1},
		{Start: 1, End: 0.5, MIDI: 60, Strength: 1},
	} {
		_, _, err := Clean([]Note{bad}, g)
		if !errors.Is(err, ErrInvalidNote) {
			t.Errorf("note %+v: %v", bad, err)
		}
	}

	got, dropped, err := Clean(nil, g)
	if err != nil || got == nil || dropped == nil || len(got)+len(dropped) != 0 {
		t.Fatalf("empty input: %v %v %v", got, dropped, err)
	}

	// Options change the result: a floor below the note keeps it, a high
	// minimum strength drops it.
	got, _, err = Clean([]Note{{Start: 1, End: 1.2, MIDI: 50, Strength: 0.5}}, g, BassCleanOptions()...)
	if err != nil || len(got) != 1 {
		t.Fatalf("bass floor: %v %v", got, err)
	}

	_, dropped, err = Clean(notes, g, WithMinStrength(0.6))
	if err != nil || len(dropped) != 1 || dropped[0].Reason != DropWeak {
		t.Fatalf("min strength: %v %v", dropped, err)
	}
}

// TestSortPermutationMatchesSortSlice guards the median step: slices.SortFunc
// must order equal pitches exactly like the reference's sort.Slice, because
// that order fixes the floating-point summation of the weights.
func TestSortPermutationMatchesSortSlice(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(7, 7))
	for trial := range 2000 {
		n := 1 + rng.IntN(200)
		a := make([]weightedPitch, n)

		for i := range a {
			a[i] = weightedPitch{float64(40 + rng.IntN(1+trial%24)), float64(i)}
		}

		b := slices.Clone(a)

		sort.Slice(a, func(i, j int) bool { return a[i].midi < a[j].midi })
		slices.SortFunc(b, func(x, y weightedPitch) int { return cmp.Compare(x.midi, y.midi) })

		if !slices.Equal(a, b) {
			t.Fatalf("trial %d: permutations differ", trial)
		}
	}
}

func BenchmarkClean(b *testing.B) {
	_, g := cleanGrids(b, storyRhythm{BPM: 105, BeatOrigin: 0.038}, 600)
	rng := rand.New(rand.NewPCG(1, 2))
	in := toNotes(append(randomLine(rng, g, 1500, 52), periodicLine(rng, g, 60)...))

	b.ReportAllocs()

	for b.Loop() {
		_, _, err := Clean(in, g)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCleanReference is the verbatim AudioVisualizer CleanNotes on the
// same input, for comparison.
func BenchmarkCleanReference(b *testing.B) {
	sg, g := cleanGrids(b, storyRhythm{BPM: 105, BeatOrigin: 0.038}, 600)
	rng := rand.New(rand.NewPCG(1, 2))
	in := append(randomLine(rng, g, 1500, 52), periodicLine(rng, g, 60)...)

	b.ReportAllocs()

	for b.Loop() {
		storyCleanNotes(in, sg, storyLeadCleanParams())
	}
}
