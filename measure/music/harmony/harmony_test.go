package harmony

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

// triad returns the chroma of a chord on root with the given intervals, the
// root a little stronger than the other tones.
func triad(root int, intervals ...int) [12]float64 {
	var c [12]float64

	for i, iv := range intervals {
		w := 0.8
		if i == 0 {
			w = 1
		}

		c[(root+iv)%12] += w
	}

	return c
}

func addChroma(dst *[12]float64, src [12]float64, weight float64) {
	for i := range dst {
		dst[i] += weight * src[i]
	}
}

// cadence is the chroma of I–IV–V–I in a major key or i–iv–V–i (harmonic
// minor dominant) in a minor key.
func cadence(tonic int, mode Mode) [12]float64 {
	third := 4
	if mode == Minor {
		third = 3
	}

	var c [12]float64

	addChroma(&c, triad(tonic, 0, third, 7), 2)
	addChroma(&c, triad(tonic+5, 0, third, 7), 1)
	addChroma(&c, triad(tonic+7, 0, 4, 7), 1)

	return c
}

func TestEstimateKeyCadenceAllKeys(t *testing.T) {
	t.Parallel()

	for tonic := range 12 {
		for _, mode := range []Mode{Major, Minor} {
			k, err := EstimateKey(cadence(tonic, mode))
			if err != nil {
				t.Fatal(err)
			}

			if int(k.Tonic) != tonic || k.Mode != mode || k.Ambiguous {
				t.Fatalf("%v %v cadence: got %+v", pitch.PitchClass(tonic), mode, k)
			}

			if k.Margin <= 0 || k.Correlation <= k.RunnerUp.Correlation {
				t.Fatalf("%v %v cadence: margin %v", pitch.PitchClass(tonic), mode, k.Margin)
			}

			relTonic, relMode := relativeOf(tonic, mode)
			if int(k.Relative.Tonic) != relTonic || k.Relative.Mode != relMode {
				t.Fatalf("%v %v: relative %v", pitch.PitchClass(tonic), mode, k.Relative)
			}
		}
	}
}

func TestEstimateKeyGMajorScale(t *testing.T) {
	t.Parallel()

	var c [12]float64
	for pc, w := range map[int]float64{7: 3, 9: 1, 11: 2, 0: 1, 2: 2.5, 4: 1, 6: 1} {
		c[pc] = w
	}

	k, err := EstimateKey(c)
	if err != nil {
		t.Fatal(err)
	}

	if k.String() != "G major" || k.Sharps() != 1 || k.Relative.String() != "E minor" {
		t.Fatalf("key %+v", k)
	}
}

func TestEstimateKeyRelativeTie(t *testing.T) {
	t.Parallel()

	// Equal parts G major and E minor: the two relative keys tie.
	var c [12]float64
	for i := range 12 {
		c[(7+i)%12] += kkMajor[i]
		c[(4+i)%12] += kkMinor[i]
	}

	var bassOnE, bassOnG [12]float64

	bassOnE[pitch.PitchClassE], bassOnG[pitch.PitchClassG] = 1, 1

	k, err := EstimateKey(c, WithTonicEvidence(bassOnE))
	if err != nil {
		t.Fatal(err)
	}

	if !k.Ambiguous || k.String() != "E minor" || k.RunnerUp.String() != "G major" || k.Margin >= 0 {
		t.Fatalf("E evidence: %+v", k)
	}

	if k.Tied[0].String() != "G major" || k.Tied[1].String() != "E minor" {
		t.Fatalf("tied pair %v", k.Tied)
	}

	k, err = EstimateKey(c, WithTonicEvidence(bassOnG))
	if err != nil {
		t.Fatal(err)
	}

	if !k.Ambiguous || k.String() != "G major" || k.RunnerUp.String() != "E minor" {
		t.Fatalf("G evidence: %+v", k)
	}

	// Without evidence the correlation order stands; still ambiguous.
	k, err = EstimateKey(c)
	if err != nil {
		t.Fatal(err)
	}

	if !k.Ambiguous || k.String() != "G major" {
		t.Fatalf("no evidence: %+v", k)
	}

	// A zero margin disables tie breaking.
	k, err = EstimateKey(c, WithTieMargin(0), WithTonicEvidence(bassOnE))
	if err != nil {
		t.Fatal(err)
	}

	if k.Ambiguous || k.String() != "G major" || k.Tied != [2]Candidate{} {
		t.Fatalf("zero margin: %+v", k)
	}
}

func TestEstimateKeyErrors(t *testing.T) {
	t.Parallel()

	good := cadence(0, Major)
	nan := good
	nan[3] = math.NaN()

	var evidence [12]float64

	evidence[5] = math.Inf(1)

	cases := []struct {
		name   string
		chroma [12]float64
		opts   []KeyOption
		want   error
	}{
		{"zero", [12]float64{}, nil, ErrFlatChroma},
		{"constant", [12]float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, nil, ErrFlatChroma},
		{"nan", nan, nil, ErrInvalidInput},
		{"nil option", good, []KeyOption{nil}, ErrNilOption},
		{"negative tie", good, []KeyOption{WithTieMargin(-0.1)}, ErrInvalidOption},
		{"nan tie", good, []KeyOption{WithTieMargin(math.NaN())}, ErrInvalidOption},
		{"inf evidence", good, []KeyOption{WithTonicEvidence(evidence)}, ErrInvalidOption},
		{"profile set", good, []KeyOption{WithProfileSet(ProfileSet(9))}, ErrInvalidOption},
	}

	for _, c := range cases {
		_, err := EstimateKey(c.chroma, c.opts...)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}

	k, err := EstimateKey(good, WithProfileSet(KrumhanslKessler))
	if err != nil || k.String() != "C major" {
		t.Fatalf("explicit profile set: %+v, %v", k, err)
	}
}

func TestKeyMethods(t *testing.T) {
	t.Parallel()

	for tonic := range 12 {
		for _, mode := range []Mode{Major, Minor} {
			k := Key{Tonic: pitch.PitchClass(tonic), Mode: mode}
			ref := toRefKey(k)

			if k.Sharps() != ref.Sharps() {
				t.Fatalf("%v: sharps %d, want %d", k, k.Sharps(), ref.Sharps())
			}

			for pc := range 12 {
				if k.Diatonic(pitch.PitchClass(pc)) != ref.Diatonic(pc) {
					t.Fatalf("%v: diatonic %d", k, pc)
				}
			}
		}
	}

	if (Key{Tonic: pitch.PitchClassFSharp}).Sharps() != 6 || (Key{Tonic: pitch.PitchClassD, Mode: Minor}).Sharps() != -1 {
		t.Fatal("sharps")
	}

	bad := []Key{{Tonic: 12}, {Mode: 2}}
	for _, k := range bad {
		if k.Diatonic(pitch.PitchClassC) || !k.Scale().IsZero() || !errors.Is(k.validate(), ErrInvalidInput) {
			t.Fatalf("invalid key %+v", k)
		}
	}

	if Mode(7).String() != "Mode(7)" || ProfileSet(3).String() != "ProfileSet(3)" || KrumhanslKessler.String() != "Krumhansl-Kessler" {
		t.Fatal("strings")
	}

	if _, err := ProfileSet(3).Profile(Major); !errors.Is(err, ErrInvalidOption) {
		t.Fatal(err)
	}

	if _, err := KrumhanslKessler.Profile(Mode(4)); !errors.Is(err, ErrInvalidOption) {
		t.Fatal(err)
	}
}

// chordWindow is a window with the given chroma and bass at -20 dBFS.
func chordWindow(start float64, chroma, bass [12]float64) Window {
	return Window{Start: start, End: start + 1, Chroma: chroma, Bass: bass, LevelDB: -20}
}

func bassOn(pc int) [12]float64 {
	var b [12]float64

	b[pc] = 1

	return b
}

func symbols(chords []Chord) []string {
	out := make([]string, len(chords))
	for i, c := range chords {
		out[i] = c.Symbol
	}

	return out
}

func TestChordsProgressionWithInversion(t *testing.T) {
	t.Parallel()

	// I–vi–IV–V in G major with the IV in first inversion (C/E).
	g := Key{Tonic: pitch.PitchClassG, Mode: Major}
	windows := []Window{
		chordWindow(0, triad(7, 0, 4, 7), bassOn(7)),
		chordWindow(1, triad(4, 0, 3, 7), bassOn(4)),
		chordWindow(2, triad(0, 0, 4, 7), bassOn(4)),
		chordWindow(3, triad(2, 0, 4, 7), bassOn(2)),
	}

	chords, err := Chords(windows, g)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := symbols(chords), []string{"G", "Em", "C/E", "D"}; !slices.Equal(got, want) {
		t.Fatalf("symbols %v, want %v", got, want)
	}

	slashChord := chords[2]
	if slashChord.Root != pitch.PitchClassC || slashChord.Bass != pitch.PitchClassE || slashChord.Quality != "maj" ||
		!slices.Equal(slashChord.Voicing, []int{48, 52, 55}) || slashChord.Margin <= 0 {
		t.Fatalf("slash chord %+v", slashChord)
	}

	if chords[1].Quality != "min" || chords[1].Bass != chords[1].Root {
		t.Fatalf("Em %+v", chords[1])
	}
}

func TestChordsB7OverDSharpIsNotDSharpMinor(t *testing.T) {
	t.Parallel()

	// A B7 in first inversion in E major: D# carries the chroma peak and the
	// bass, the seventh A is clearly present, the root B is weak.
	var c [12]float64

	c[pitch.PitchClassB], c[pitch.PitchClassDSharp], c[pitch.PitchClassFSharp], c[pitch.PitchClassA] = 0.6, 1, 0.9, 0.7
	window := chordWindow(0, c, bassOn(int(pitch.PitchClassDSharp)))
	e := Key{Tonic: pitch.PitchClassE, Mode: Major}

	chords, err := Chords([]Window{window}, e)
	if err != nil {
		t.Fatal(err)
	}

	if chords[0].Symbol != "B7/D#" || chords[0].Root != pitch.PitchClassB || chords[0].Bass != pitch.PitchClassDSharp {
		t.Fatalf("got %+v, want B7/D#", chords[0])
	}

	// With the full bass bonus for the root only (no inversion credit) and
	// a larger weight, the bass on the third would win the root.
	chords, err = Chords([]Window{window}, e, WithBassRootBonus(0.6), WithInversionShare(0))
	if err != nil {
		t.Fatal(err)
	}

	if chords[0].Symbol != "D#m" {
		t.Fatalf("strong root bonus: got %s, want D#m", chords[0].Symbol)
	}
}

func TestChordsSilenceIsN(t *testing.T) {
	t.Parallel()

	const frames = 300

	var chroma [12][]float64
	for pc := range chroma {
		chroma[pc] = make([]float64, frames)
		for i := range frames {
			chroma[pc][i] = float64((pc*7+i)%12) / 11
		}
	}

	rms := make([]float64, frames)
	for i := 100; i < 200; i++ {
		rms[i] = 0.1
	}

	windows, err := Windows(chroma, rms, 100, []Span{{0, 1}, {1, 2}, {2, 3}, {3, 4}})
	if err != nil {
		t.Fatal(err)
	}

	if windows[0].LevelDB != -120 || windows[3].LevelDB != -120 || math.Abs(windows[1].LevelDB-(-20)) > 1e-9 {
		t.Fatalf("levels %v %v %v", windows[0].LevelDB, windows[1].LevelDB, windows[3].LevelDB)
	}

	chords, err := Chords(windows, Key{})
	if err != nil {
		t.Fatal(err)
	}

	if chords[0].Symbol != "N" || !chords[0].NoChord || chords[0].Score != 1 || chords[0].Margin != 0 ||
		chords[0].Voicing != nil || chords[len(chords)-1].Symbol != "N" {
		t.Fatalf("chords %+v", chords)
	}

	// A flat window is N even when loud.
	flat := Window{End: 1, LevelDB: -10, Chroma: [12]float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1.2}}

	chords, err = Chords([]Window{flat}, Key{})
	if err != nil || chords[0].Symbol != "N" {
		t.Fatalf("flat: %+v, %v", chords, err)
	}

	// Disabling the gate lets a quiet chord through.
	quiet := chordWindow(0, triad(0, 0, 4, 7), bassOn(0))
	quiet.LevelDB = -90

	chords, err = Chords([]Window{quiet}, Key{}, WithGateDB(math.Inf(-1)))
	if err != nil || chords[0].Symbol != "C" {
		t.Fatalf("ungated: %+v, %v", chords, err)
	}
}

func TestChordsNoisyWindowDoesNotFlip(t *testing.T) {
	t.Parallel()

	c := triad(0, 0, 4, 7)
	// One window with a stronger B and weaker C: Em scores a little higher
	// on its own.
	noisy := c
	noisy[pitch.PitchClassC], noisy[pitch.PitchClassB] = 0.5, 0.9

	var windows []Window

	for i := range 7 {
		chroma := c
		if i == 3 {
			chroma = noisy
		}

		windows = append(windows, Window{Start: float64(i), End: float64(i + 1), Chroma: chroma, LevelDB: -20})
	}

	alone, err := Chords(windows[3:4], Key{})
	if err != nil || alone[0].Symbol == "C" {
		t.Fatalf("the noisy window alone should not read C: %+v, %v", alone, err)
	}

	chords, err := Chords(windows, Key{})
	if err != nil {
		t.Fatal(err)
	}

	if len(chords) != 1 || chords[0].Symbol != "C" || chords[0].Start != 0 || chords[0].End != 7 {
		t.Fatalf("smoothed chords %+v", chords)
	}

	chords, err = Chords(windows, Key{}, WithSwitchPenalty(0))
	if err != nil || len(chords) != 3 {
		t.Fatalf("unsmoothed chords %+v, %v", chords, err)
	}
}

func TestChordsOptions(t *testing.T) {
	t.Parallel()

	windows := []Window{
		chordWindow(0, triad(0, 0, 5, 7), bassOn(0)),
		chordWindow(1, triad(0, 0, 5, 7), bassOn(0)),
		chordWindow(2, triad(0, 0, 4, 7), bassOn(0)),
	}

	templates := append(DefaultTemplates(), ChordTemplate{Name: "sus4", Suffix: "sus4", Intervals: []int{0, 5, 7}})

	chords, err := Chords(windows, Key{}, WithTemplates(templates...), WithScoreDecimals(-1))
	if err != nil {
		t.Fatal(err)
	}

	if got := symbols(chords); !slices.Equal(got, []string{"Csus4", "C"}) {
		t.Fatalf("symbols %v", got)
	}

	if chords[0].Quality != "sus4" || chords[0].Score < 1.2 {
		t.Fatalf("sus4 score %+v", chords[0])
	}

	rounded, err := Chords(windows, Key{}, WithTemplates(templates...), WithScoreDecimals(1))
	if err != nil || rounded[1].Score != math.Round(chords[1].Score*10)/10 {
		t.Fatalf("rounded %+v, %v", rounded, err)
	}

	// Seventh penalty and in-key bonus shift the scores by their values.
	base, _ := Chords(windows[2:], Key{}, WithInKeyBonus(0), WithScoreDecimals(-1))
	bonus, _ := Chords(windows[2:], Key{}, WithInKeyBonus(0.3), WithSeventhPenalty(0.5), WithMinPeakRatio(1), WithScoreDecimals(-1))

	if math.Abs(bonus[0].Score-base[0].Score-0.3) > 1e-12 {
		t.Fatalf("in-key bonus: %v vs %v", bonus[0].Score, base[0].Score)
	}
}

func TestChordsOptionErrors(t *testing.T) {
	t.Parallel()

	bad := []ChordOption{
		nil,
		WithTemplates(),
		WithTemplates(ChordTemplate{Name: "", Intervals: []int{0, 4}}),
		WithTemplates(ChordTemplate{Name: "N", Intervals: []int{0, 4}}),
		WithTemplates(ChordTemplate{Name: "x", Intervals: []int{0}}),
		WithTemplates(ChordTemplate{Name: "x", Intervals: []int{4, 0}}),
		WithTemplates(ChordTemplate{Name: "x", Intervals: []int{0, 12}}),
		WithTemplates(ChordTemplate{Name: "x", Intervals: []int{0, 4, 4}}),
		WithTemplates(ChordTemplate{Name: "x", Intervals: []int{0, 4}}, ChordTemplate{Name: "y", Intervals: []int{0, 3}}),
		WithBassRootBonus(-1),
		WithInversionShare(1.5),
		WithInversionShare(math.NaN()),
		WithSeventhPenalty(math.Inf(1)),
		WithInKeyBonus(-0.1),
		WithMinPeakRatio(math.NaN()),
		WithGateDB(math.NaN()),
		WithGateDB(math.Inf(1)),
		WithSwitchPenalty(-0.1),
		WithScoreDecimals(16),
	}

	for i, opt := range bad {
		_, err := NewChorder(opt)
		if !errors.Is(err, ErrInvalidOption) && !errors.Is(err, ErrNilOption) {
			t.Errorf("option %d: %v", i, err)
		}

		_, err = Chords(nil, Key{}, opt)
		if err == nil {
			t.Errorf("Chords option %d: no error", i)
		}
	}
}

func TestChordsInputErrors(t *testing.T) {
	t.Parallel()

	good := chordWindow(0, triad(0, 0, 4, 7), bassOn(0))
	mutate := func(f func(w *Window)) []Window {
		w := good
		f(&w)

		return []Window{good, w}
	}

	cases := [][]Window{
		mutate(func(w *Window) { w.Start = math.NaN() }),
		mutate(func(w *Window) { w.End = -1 }),
		mutate(func(w *Window) { w.LevelDB = math.NaN() }),
		mutate(func(w *Window) { w.LevelDB = math.Inf(1) }),
		mutate(func(w *Window) { w.Chroma[3] = -1 }),
		mutate(func(w *Window) { w.Bass[3] = math.Inf(1) }),
	}

	for i, windows := range cases {
		_, err := Chords(windows, Key{})
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("case %d: %v", i, err)
		}
	}

	_, err := Chords([]Window{good}, Key{Mode: 5})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("invalid key: %v", err)
	}

	chords, err := Chords(nil, Key{})
	if err != nil || chords == nil || len(chords) != 0 {
		t.Errorf("empty: %v, %v", chords, err)
	}
}

// TestChorderReuseDoesNotAllocate is not parallel: AllocsPerRun forbids it.
func TestChorderReuseDoesNotAllocate(t *testing.T) {
	windows := toWindows(exactWindows())

	c, err := NewChorder()
	if err != nil {
		t.Fatal(err)
	}

	dst, err := c.Chords(nil, windows, Key{})
	if err != nil {
		t.Fatal(err)
	}

	first := appChords(dst)

	allocs := testing.AllocsPerRun(50, func() {
		dst, err = c.Chords(dst, windows, Key{})
	})
	if err != nil || allocs != 0 {
		t.Fatalf("%v allocs/op, err %v", allocs, err)
	}

	equalChords(t, "reused", appChords(dst), first)

	// Fewer windows shrink the result; more grow the buffers.
	dst, err = c.Chords(dst, windows[:1], Key{})
	if err != nil || len(dst) != 1 || dst[0].Symbol != "C" {
		t.Fatalf("shrunk %+v, %v", dst, err)
	}

	long := slices.Concat(windows, windows, windows)

	dst, err = c.Chords(dst, long, Key{})
	if err != nil || len(dst) < len(first) {
		t.Fatalf("grown %+v, %v", dst, err)
	}
}

func TestWindowsBassAndErrors(t *testing.T) {
	t.Parallel()

	const frames = 200

	var chroma, bass [12][]float64
	for pc := range chroma {
		chroma[pc] = make([]float64, frames)
		bass[pc] = make([]float64, frames)
	}

	rms, voicing := make([]float64, frames), make([]float64, frames)
	for i := range frames {
		rms[i], voicing[i] = 0.5, 1
		chroma[pitch.PitchClassA][i] = 1
		bass[pitch.PitchClassD][i] = 1
	}

	notes := []melody.Note{{Start: 0.5, End: 1.5, MIDI: 40, Strength: 0.5}, {Start: 0, End: 0.1, MIDI: -1, Strength: 1}}

	w, err := Windows(chroma, rms, 100, []Span{{0, 1}}, WithBassChroma(bass, voicing), WithBassNotes(notes), WithLevelFloor(1e-3))
	if err != nil {
		t.Fatal(err)
	}

	if w[0].Chroma[pitch.PitchClassA] != 50 || math.Abs(w[0].Bass[pitch.PitchClassD]-1) > 1e-12 ||
		w[0].Bass[pitch.PitchClassE] != 0.25 || math.Abs(w[0].Bass[pitch.PitchClassB]-0.1) > 1e-12 ||
		math.Abs(w[0].LevelDB-20*math.Log10(0.5)) > 1e-12 {
		t.Fatalf("window %+v", w[0])
	}

	silent, err := Windows(chroma, rms, 100, []Span{{5, 6}}, WithLevelFloor(1e-3))
	if err != nil || silent[0].LevelDB != -60 || silent[0].Bass != [12]float64{} {
		t.Fatalf("floor: %+v, %v", silent, err)
	}

	short := bass
	short[4] = short[4][:10]

	negative := chroma
	negative[2] = slices.Clone(negative[2])
	negative[2][7] = -1

	cases := []struct {
		name string
		err  error
		run  func() error
	}{
		{"nil option", ErrNilOption, func() error { _, err := Windows(chroma, rms, 100, nil, nil); return err }},
		{"frame rate", ErrInvalidInput, func() error { _, err := Windows(chroma, rms, 0, nil); return err }},
		{"length", ErrLengthMismatch, func() error { _, err := Windows(chroma, rms[:5], 100, nil); return err }},
		{"rms", ErrInvalidInput, func() error {
			r := slices.Clone(rms)
			r[3] = math.NaN()
			_, err := Windows(chroma, r, 100, nil)

			return err
		}},
		{"chroma", ErrInvalidInput, func() error { _, err := Windows(negative, rms, 100, nil); return err }},
		{"span", ErrInvalidInput, func() error { _, err := Windows(chroma, rms, 100, []Span{{2, 1}}); return err }},
		{"bass length", ErrLengthMismatch, func() error {
			_, err := Windows(chroma, rms, 100, nil, WithBassChroma(short, voicing))
			return err
		}},
		{"bass voicing", ErrInvalidInput, func() error {
			v := slices.Clone(voicing)
			v[0] = -1
			_, err := Windows(chroma, rms, 100, nil, WithBassChroma(bass, v))

			return err
		}},
		{"bass chroma", ErrInvalidInput, func() error {
			_, err := Windows(chroma, rms, 100, nil, WithBassChroma(negative, rms))
			return err
		}},
		{"note", ErrInvalidOption, func() error {
			_, err := Windows(chroma, rms, 100, nil, WithBassNotes([]melody.Note{{End: math.NaN()}}))
			return err
		}},
		{"floor", ErrInvalidOption, func() error { _, err := Windows(chroma, rms, 100, nil, WithLevelFloor(0)); return err }},
	}

	for _, c := range cases {
		if err := c.run(); !errors.Is(err, c.err) || !strings.HasPrefix(err.Error(), "harmony: ") {
			t.Errorf("%s: got %v, want %v", c.name, err, c.err)
		}
	}

	if Normalize([12]float64{}) != [12]float64{} {
		t.Fatal("normalize zero")
	}
}
