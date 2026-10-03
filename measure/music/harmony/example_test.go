package harmony_test

import (
	"fmt"
	"math"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/harmony"
	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

// chroma returns a chroma vector with weight 1 on the given pitch classes.
func chroma(classes ...pitch.PitchClass) [12]float64 {
	var c [12]float64
	for _, pc := range classes {
		c[pc] = 1
	}

	return c
}

// symbols joins the chord symbols with spaces.
func symbols(chords []harmony.Chord) string {
	out := make([]string, len(chords))
	for i, ch := range chords {
		out[i] = ch.Symbol
	}

	return strings.Join(out, " ")
}

// window is a one-second window of the given chord tones and bass note.
func window(start float64, bass pitch.PitchClass, tones ...pitch.PitchClass) harmony.Window {
	return harmony.Window{Start: start, End: start + 1, Chroma: chroma(tones...), Bass: chroma(bass), LevelDB: -20}
}

const (
	c, d, e, f, g, a, b = pitch.PitchClassC, pitch.PitchClassD, pitch.PitchClassE, pitch.PitchClassF,
		pitch.PitchClassG, pitch.PitchClassA, pitch.PitchClassB
	fSharp = pitch.PitchClassFSharp
)

func ExampleEstimateKey() {
	// Pitch-class durations of a melody in G major.
	var profile [12]float64

	profile[g], profile[a], profile[b], profile[c] = 3, 1, 2, 1
	profile[d], profile[e], profile[fSharp] = 2.5, 1, 1

	key, err := harmony.EstimateKey(profile)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v (r = %.3f), runner-up %v, margin %.3f\n", key, key.Correlation, key.RunnerUp, key.Margin)
	fmt.Printf("relative %v, %d sharp(s)\n", key.Relative, key.Sharps())

	// Output:
	// G major (r = 0.970), runner-up D major, margin 0.267
	// relative E minor, 1 sharp(s)
}

func ExampleWithTonicEvidence() {
	// A profile halfway between C major and A minor.
	major, _ := harmony.KrumhanslKessler.Profile(harmony.Major)
	minor, _ := harmony.KrumhanslKessler.Profile(harmony.Minor)

	var profile [12]float64
	for i := range 12 {
		profile[i] += major[i]
		profile[(9+i)%12] += minor[i]
	}

	// The bass rests on A at the section edges.
	var bass [12]float64

	bass[a] = 1

	key, err := harmony.EstimateKey(profile, harmony.WithTonicEvidence(bass))
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v, ambiguous %v: %v vs %v\n", key, key.Ambiguous, key.Tied[0], key.Tied[1])

	// Output:
	// A minor, ambiguous true: C major vs A minor
}

func ExampleWithTieMargin() {
	major, _ := harmony.KrumhanslKessler.Profile(harmony.Major)
	minor, _ := harmony.KrumhanslKessler.Profile(harmony.Minor)

	var profile [12]float64
	for i := range 12 {
		profile[i] += major[i]
		profile[(9+i)%12] += minor[i]
	}

	key, err := harmony.EstimateKey(profile, harmony.WithTieMargin(0))
	if err != nil {
		panic(err)
	}

	fmt.Println(key, key.Ambiguous)

	// Output:
	// C major false
}

func ExampleWithProfileSet() {
	key, err := harmony.EstimateKey(chroma(d, fSharp, a), harmony.WithProfileSet(harmony.KrumhanslKessler))
	if err != nil {
		panic(err)
	}

	fmt.Println(key)

	// Output:
	// D major
}

func ExampleProfileSet_Profile() {
	major, err := harmony.KrumhanslKessler.Profile(harmony.Major)
	if err != nil {
		panic(err)
	}

	fmt.Println(harmony.KrumhanslKessler, "major tonic, fifth:", major[0], major[7])

	// Output:
	// Krumhansl-Kessler major tonic, fifth: 6.35 5.19
}

func ExampleProfileSet_String() {
	fmt.Println(harmony.KrumhanslKessler)

	// Output:
	// Krumhansl-Kessler
}

func ExampleMode_String() {
	fmt.Println(harmony.Major, harmony.Minor)

	// Output:
	// major minor
}

func ExampleCandidate_String() {
	fmt.Println(harmony.Candidate{Tonic: pitch.PitchClassFSharp, Mode: harmony.Minor})

	// Output:
	// F# minor
}

func ExampleKey_String() {
	fmt.Println(harmony.Key{Tonic: pitch.PitchClassASharp, Mode: harmony.Major})

	// Output:
	// A# major
}

func ExampleKey_Sharps() {
	for _, k := range []harmony.Key{{Tonic: e}, {Tonic: f}, {Tonic: d, Mode: harmony.Minor}} {
		fmt.Printf("%v: %d\n", k, k.Sharps())
	}

	// Output:
	// E major: 4
	// F major: -1
	// D minor: -1
}

func ExampleKey_Diatonic() {
	k := harmony.Key{Tonic: a, Mode: harmony.Minor}
	fmt.Println(k.Diatonic(g), k.Diatonic(pitch.PitchClassGSharp))

	// Output:
	// true false
}

func ExampleKey_Scale() {
	k := harmony.Key{Tonic: a, Mode: harmony.Minor}
	fmt.Println(k.Scale().Degrees())

	// Output:
	// [0 2 3 5 7 8 10]
}

func ExampleWindows() {
	// Two seconds of frames at 100 frames/s: a C major chord, then A minor
	// at half the level.
	const frames = 200

	var frameChroma [12][]float64
	for pc := range frameChroma {
		frameChroma[pc] = make([]float64, frames)
	}

	rms := make([]float64, frames)
	for i := range frames {
		rms[i] = 0.2
		tones := []pitch.PitchClass{c, e, g}

		if i >= 100 {
			rms[i] = 0.1
			tones = []pitch.PitchClass{a, c, e}
		}

		for _, pc := range tones {
			frameChroma[pc][i] = 1
		}
	}

	windows, err := harmony.Windows(frameChroma, rms, 100, []harmony.Span{{Start: 0, End: 1}, {Start: 1, End: 2}})
	if err != nil {
		panic(err)
	}

	for _, w := range windows {
		fmt.Printf("%.0f-%.0f s: C %.0f, E %.0f, A %.0f, level %.1f dBFS\n", w.Start, w.End, w.Chroma[c], w.Chroma[e], w.Chroma[a], w.LevelDB)
	}

	// Output:
	// 0-1 s: C 20, E 20, A 0, level -14.0 dBFS
	// 1-2 s: C 10, E 10, A 10, level -20.0 dBFS
}

func ExampleWithBassChroma() {
	const frames = 100

	var frameChroma, bassChroma [12][]float64
	for pc := range frameChroma {
		frameChroma[pc] = make([]float64, frames)
		bassChroma[pc] = make([]float64, frames)
	}

	rms, voicing := make([]float64, frames), make([]float64, frames)
	for i := range frames {
		rms[i], voicing[i] = 0.1, 0.5
		frameChroma[c][i], bassChroma[e][i] = 1, 1
	}

	windows, err := harmony.Windows(frameChroma, rms, 100, []harmony.Span{{Start: 0, End: 1}},
		harmony.WithBassChroma(bassChroma, voicing))
	if err != nil {
		panic(err)
	}

	fmt.Printf("bass E: %.2f voiced seconds\n", windows[0].Bass[e])

	// Output:
	// bass E: 0.50 voiced seconds
}

func ExampleWithBassNotes() {
	frameChroma := [12][]float64{}
	for pc := range frameChroma {
		frameChroma[pc] = make([]float64, 100)
	}

	notes := []melody.Note{{Start: 0.25, End: 2, MIDI: 43, Strength: 0.8}} // G2

	windows, err := harmony.Windows(frameChroma, make([]float64, 100), 100, []harmony.Span{{Start: 0, End: 1}},
		harmony.WithBassNotes(notes))
	if err != nil {
		panic(err)
	}

	fmt.Printf("bass G: %.2f\n", windows[0].Bass[g])

	// Output:
	// bass G: 0.60
}

func ExampleWithLevelFloor() {
	frameChroma := [12][]float64{}

	windows, err := harmony.Windows(frameChroma, nil, 100, []harmony.Span{{Start: 0, End: 1}},
		harmony.WithLevelFloor(1e-4))
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.1f dBFS\n", windows[0].LevelDB)

	// Output:
	// -80.0 dBFS
}

func ExampleNormalize() {
	fmt.Println(harmony.Normalize([12]float64{2, 0, 0, 0, 1, 0, 0, 1}))

	// Output:
	// [0.5 0 0 0 0.25 0 0 0.25 0 0 0 0]
}

func ExampleChords() {
	key := harmony.Key{Tonic: c, Mode: harmony.Major}
	windows := []harmony.Window{
		window(0, c, c, e, g),
		window(1, c, c, e, g),
		window(2, a, a, c, e),
		window(3, a, f, a, c), // F in first inversion
		window(4, g, g, b, d, f),
		{Start: 5, End: 6, LevelDB: -80}, // silence
	}

	chords, err := harmony.Chords(windows, key)
	if err != nil {
		panic(err)
	}

	for _, ch := range chords {
		fmt.Printf("%.0f-%.0f s %-4s score %.3f voicing %v\n", ch.Start, ch.End, ch.Symbol, ch.Score, ch.Voicing)
	}

	// Output:
	// 0-2 s C    score 1.250 voicing [48 52 55]
	// 2-3 s Am   score 1.250 voicing [57 60 64]
	// 3-4 s F/A  score 1.150 voicing [53 57 60]
	// 4-5 s G7   score 1.220 voicing [55 59 62 65]
	// 5-6 s N    score 1.000 voicing []
}

func ExampleNewChorder() {
	chorder, err := harmony.NewChorder(harmony.WithSwitchPenalty(0.2))
	if err != nil {
		panic(err)
	}

	key := harmony.Key{Tonic: g, Mode: harmony.Major}

	var chords []harmony.Chord

	// Reuse chorder and chords for every block of windows.
	for _, block := range [][]harmony.Window{
		{window(0, g, g, b, d), window(1, d, d, fSharp, a)},
		{window(2, e, e, g, b), window(3, c, c, e, g)},
	} {
		chords, err = chorder.Chords(chords, block, key)
		if err != nil {
			panic(err)
		}

		fmt.Println(symbols(chords))
	}

	// Output:
	// G D
	// Em C
}

func ExampleChorder_Chords() {
	chorder, err := harmony.NewChorder()
	if err != nil {
		panic(err)
	}

	chords, err := chorder.Chords(nil, []harmony.Window{window(0, e, c, e, g)}, harmony.Key{})
	if err != nil {
		panic(err)
	}

	fmt.Println(chords[0].Symbol, chords[0].Root, chords[0].Bass)

	// Output:
	// C/E C E
}

func ExampleDefaultTemplates() {
	for _, t := range harmony.DefaultTemplates() {
		fmt.Printf("%s%q%v ", t.Name, t.Suffix, t.Intervals)
	}

	fmt.Println()

	// Output:
	// maj""[0 4 7] min"m"[0 3 7] 7"7"[0 4 7 10] maj7"maj7"[0 4 7 11] m7"m7"[0 3 7 10]
}

func ExampleWithTemplates() {
	sus4 := harmony.ChordTemplate{Name: "sus4", Suffix: "sus4", Intervals: []int{0, 5, 7}}
	templates := append(harmony.DefaultTemplates(), sus4)

	chords, err := harmony.Chords([]harmony.Window{window(0, d, d, g, a)}, harmony.Key{Tonic: d},
		harmony.WithTemplates(templates...))
	if err != nil {
		panic(err)
	}

	fmt.Println(chords[0].Symbol, chords[0].Quality)

	// Output:
	// Dsus4 sus4
}

func ExampleWithBassRootBonus() {
	// A zero weight drops the bass from the score (1.05 = cosine 1 plus the
	// in-key bonus); the slash still names the strongest bass pitch class.
	chords, err := harmony.Chords([]harmony.Window{window(0, e, c, e, g)}, harmony.Key{},
		harmony.WithBassRootBonus(0))
	if err != nil {
		panic(err)
	}

	fmt.Println(chords[0].Symbol, chords[0].Score)

	// Output:
	// C/E 1.05
}

func ExampleWithInversionShare() {
	for _, share := range []float64{0.5, 0} {
		chords, err := harmony.Chords([]harmony.Window{window(0, e, c, e, g)}, harmony.Key{},
			harmony.WithInversionShare(share))
		if err != nil {
			panic(err)
		}

		fmt.Println(share, chords[0].Symbol, chords[0].Score)
	}

	// Output:
	// 0.5 C/E 1.15
	// 0 C/E 1.05
}

func ExampleWithSeventhPenalty() {
	// C, E, G with a softer B: the triad wins unless sevenths are free.
	w := window(0, c, c, e, g)
	w.Chroma[b] = 0.55

	for _, penalty := range []float64{harmony.DefaultSeventhPenalty, 0} {
		chords, err := harmony.Chords([]harmony.Window{w}, harmony.Key{}, harmony.WithSeventhPenalty(penalty))
		if err != nil {
			panic(err)
		}

		fmt.Println(penalty, chords[0].Symbol)
	}

	// Output:
	// 0.03 C
	// 0 Cmaj7
}

func ExampleWithInKeyBonus() {
	// E, G, B with a softer C: Em and Cmaj7 nearly tie, and only Em is
	// diatonic in B minor.
	w := window(0, e, e, g, b)
	w.Chroma[c] = 0.6
	w.Bass = [12]float64{}

	for _, bonus := range []float64{0, 0.05} {
		chords, err := harmony.Chords([]harmony.Window{w}, harmony.Key{Tonic: b, Mode: harmony.Minor},
			harmony.WithInKeyBonus(bonus))
		if err != nil {
			panic(err)
		}

		fmt.Println(bonus, chords[0].Symbol, chords[0].Score)
	}

	// Output:
	// 0 Cmaj7 0.952
	// 0.05 Em 0.995
}

func ExampleWithMinPeakRatio() {
	// Nearly flat chroma with a slight C major tilt.
	w := harmony.Window{End: 1, LevelDB: -20}
	for i := range w.Chroma {
		w.Chroma[i] = 1
	}

	w.Chroma[c], w.Chroma[e], w.Chroma[g] = 1.7, 1.7, 1.7

	for _, ratio := range []float64{harmony.DefaultMinPeakRatio, 1} {
		chords, err := harmony.Chords([]harmony.Window{w}, harmony.Key{}, harmony.WithMinPeakRatio(ratio))
		if err != nil {
			panic(err)
		}

		fmt.Println(ratio, chords[0].Symbol)
	}

	// Output:
	// 1.5 N
	// 1 C
}

func ExampleWithGateDB() {
	w := window(0, c, c, e, g)
	w.LevelDB = -55

	for _, gate := range []float64{harmony.DefaultGateDB, math.Inf(-1)} {
		chords, err := harmony.Chords([]harmony.Window{w}, harmony.Key{}, harmony.WithGateDB(gate))
		if err != nil {
			panic(err)
		}

		fmt.Println(gate, chords[0].Symbol)
	}

	// Output:
	// -50 N
	// -Inf C
}

func ExampleWithSwitchPenalty() {
	// A short passing E minor between C windows, without bass evidence.
	windows := []harmony.Window{window(0, c, c, e, g), window(1, e, e, g, b), window(2, c, c, e, g)}
	for i := range windows {
		windows[i].Bass = [12]float64{}
	}

	for _, penalty := range []float64{harmony.DefaultSwitchPenalty, 1} {
		chords, err := harmony.Chords(windows, harmony.Key{}, harmony.WithSwitchPenalty(penalty))
		if err != nil {
			panic(err)
		}

		fmt.Println(symbols(chords))
	}

	// Output:
	// C Em C
	// C
}

func ExampleWithScoreDecimals() {
	w := window(0, c, c, e, g)
	w.Chroma[b] = 0.3

	for _, decimals := range []int{harmony.DefaultScoreDecimals, -1} {
		chords, err := harmony.Chords([]harmony.Window{w}, harmony.Key{}, harmony.WithScoreDecimals(decimals))
		if err != nil {
			panic(err)
		}

		fmt.Println(chords[0].Score)
	}

	// Output:
	// 1.235
	// 1.2353292781642933
}
