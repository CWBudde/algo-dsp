package harmony

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

const parityFrameRate = 100.0 // AudioVisualizer: 24 kHz / 240-sample hop

func sameBits(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

// toRefKey converts a package key into the app's estimate, as far as
// DetectChords needs it.
func toRefKey(k Key) refKeyEstimate {
	return refKeyEstimate{Tonic: int(k.Tonic), Mode: k.Mode.String()}
}

// appKey rebuilds the app's KeyEstimate from the package result, the way
// AudioVisualizer does after switching to the package.
func appKey(k Key, chroma, evidence [12]float64, tie float64) refKeyEstimate {
	out := refKeyEstimate{
		Tonic:               int(k.Tonic),
		Mode:                k.Mode.String(),
		Name:                k.String(),
		Correlation:         refR3(k.Correlation),
		RunnerUp:            k.RunnerUp.String(),
		RunnerUpCorrelation: refR3(k.RunnerUp.Correlation),
		Relative:            k.Relative.String(),
		RelativeCorrelation: refR3(k.Relative.Correlation),
		RelativeAmbiguous:   k.Ambiguous,
	}

	if k.Ambiguous {
		a, b := k.Tied[0], k.Tied[1]
		out.RelativeEvidence = fmt.Sprintf("%s vs %s within %.2f; section-edge bass tonic weight %.3f vs %.3f",
			a, b, tie, evidence[a.Tonic], evidence[b.Tonic])
	}

	for i, v := range Normalize(chroma) {
		if v != 0 {
			out.Profile[i] = refR3(v)
		}
	}

	return out
}

type keyCase struct {
	chroma, evidence [12]float64
	tie              float64
}

func keyCases() []keyCase {
	rng := rand.New(rand.NewPCG(45, 2))
	ties := []float64{0.05, 0, 0.2, 0.6}

	var cases []keyCase

	add := func(c [12]float64) {
		var ev [12]float64
		for i := range ev {
			if rng.IntN(3) == 0 {
				ev[i] = rng.Float64()
			}
		}

		cases = append(cases, keyCase{chroma: c, evidence: ev, tie: ties[len(cases)%len(ties)]})
	}

	for range 300 {
		var c [12]float64
		for i := range c {
			c[i] = rng.Float64()
		}

		add(c)
	}

	// Mixtures of a major key and its relative minor provoke ties.
	for range 300 {
		tonic := rng.IntN(12)
		a, b := rng.Float64(), rng.Float64()

		var c [12]float64
		for i := range 12 {
			c[(tonic+i)%12] += a * refMajorProfile[i]
			c[(tonic+9+i)%12] += b * refMinorProfile[i]
			c[i] += 0.3 * rng.Float64()
		}

		add(c)
	}

	// Exact profiles and sparse inputs.
	for tonic := range 12 {
		var maj, mnr, sparse [12]float64
		for i := range 12 {
			maj[(tonic+i)%12] = refMajorProfile[i]
			mnr[(tonic+i)%12] = refMinorProfile[i]
		}

		sparse[tonic] = 1

		add(maj)
		add(mnr)
		add(sparse)
	}

	return cases
}

// TestParityEstimateKey compares EstimateKey with the verbatim copy of
// AudioVisualizer's EstimateKey bit for bit, after the rounding the app
// applies.
func TestParityEstimateKey(t *testing.T) {
	t.Parallel()

	var ambiguous, swapped, parallel int

	for i, kc := range keyCases() {
		want := refEstimateKey(kc.chroma, kc.evidence, kc.tie)

		k, err := EstimateKey(kc.chroma, WithTonicEvidence(kc.evidence), WithTieMargin(kc.tie))
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}

		got := appKey(k, kc.chroma, kc.evidence, kc.tie)
		if got != want {
			t.Fatalf("case %d:\n got %+v\nwant %+v", i, got, want)
		}

		if k.Sharps() != want.Sharps() {
			t.Fatalf("case %d: sharps %d, want %d", i, k.Sharps(), want.Sharps())
		}

		if !sameBits(k.Margin, k.Correlation-k.RunnerUp.Correlation) {
			t.Fatalf("case %d: margin %v", i, k.Margin)
		}

		if k.Ambiguous {
			ambiguous++

			if k.Tied[0].Tonic != k.Tonic || k.Tied[0].Mode != k.Mode {
				swapped++
			}

			if k.RunnerUp.Tonic == k.Tonic {
				parallel++
			}
		}
	}

	// The cases must reach the tie branches, including the original's
	// parallel-key runner-up after a swap.
	if ambiguous == 0 || swapped == 0 || parallel == 0 {
		t.Fatalf("tie coverage: %d ambiguous, %d swapped, %d parallel", ambiguous, swapped, parallel)
	}
}

type frameData struct {
	rms, voicing        []float64
	chroma, bassChroma  [12][]float64
	notes               []melody.Note
	refNotes            []refStoryNote
	duration, frameRate float64
}

func randomFrames(rng *rand.Rand, frames int, frameRate float64) frameData {
	d := frameData{
		rms:       make([]float64, frames),
		voicing:   make([]float64, frames),
		duration:  float64(frames) / frameRate,
		frameRate: frameRate,
	}

	for pc := range 12 {
		d.chroma[pc] = make([]float64, frames)
		d.bassChroma[pc] = make([]float64, frames)
	}

	for i := range frames {
		if rng.IntN(10) != 0 {
			d.rms[i] = 0.3 * rng.Float64() * rng.Float64()
		}

		if rng.IntN(3) != 0 {
			d.voicing[i] = rng.Float64()
		}

		for pc := range 12 {
			d.chroma[pc][i] = rng.Float64()
			d.bassChroma[pc][i] = rng.Float64() * rng.Float64()
		}

		d.chroma[rng.IntN(12)][i] = 1
	}

	for t := 0.0; t < d.duration; t += 0.05 + 0.4*rng.Float64() {
		n := melody.Note{Start: t, End: t + 0.05 + 0.5*rng.Float64(), MIDI: 28 + rng.IntN(33), Strength: rng.Float64()}
		d.notes = append(d.notes, n)
		d.refNotes = append(d.refNotes, refStoryNote{Start: n.Start, End: n.End, MIDI: n.MIDI, Strength: n.Strength})
	}

	return d
}

// halfBarSpans is the app's chord window loop at 105 BPM (two beats per
// window), with a pickup offset, clipped to the duration.
func halfBarSpans(duration float64) []Span {
	beat := 60.0 / 105
	step := 2 * beat

	var spans []Span

	for t := -0.3; t < duration; t += step {
		spans = append(spans, Span{Start: t, End: math.Min(t+step, duration)})
	}

	return spans
}

// TestParityWindows compares Windows with the app's harmony, bassPC and
// meanDB helpers bit for bit.
func TestParityWindows(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(45, 3))

	for _, rate := range []float64{parityFrameRate, 86.1328125} {
		d := randomFrames(rng, 1200, rate)
		spans := halfBarSpans(d.duration)
		spans = append(spans, Span{0, d.duration}, Span{-5, -1}, Span{d.duration + 1, d.duration + 2},
			Span{1.234, 1.234}, Span{3.3, 1e300}, Span{-1e300, 0.5})

		got, err := Windows(d.chroma, d.rms, rate, spans,
			WithBassChroma(d.bassChroma, d.voicing), WithBassNotes(d.notes))
		if err != nil {
			t.Fatal(err)
		}

		for i, s := range spans {
			wantC := refHarmony(d.rms, d.chroma, rate, s.Start, s.End)
			wantB := refBassPC(d.voicing, d.bassChroma, rate, s.Start, s.End, d.refNotes)
			wantL := refMeanDB(d.rms, rate, s.Start, s.End)
			w := got[i]

			if s.End > 1e6 || s.Start < -1e6 {
				// The original's integer conversion is undefined for these
				// spans; they only check that the package clamps.
				continue
			}

			if w.Start != s.Start || w.End != s.End || !sameBits(w.LevelDB, wantL) {
				t.Fatalf("rate %g span %d: %+v, level want %v", rate, i, w, wantL)
			}

			for pc := range 12 {
				if !sameBits(w.Chroma[pc], wantC[pc]) || !sameBits(w.Bass[pc], wantB[pc]) {
					t.Fatalf("rate %g span %d pc %d: chroma %v/%v bass %v/%v",
						rate, i, pc, w.Chroma[pc], wantC[pc], w.Bass[pc], wantB[pc])
				}
			}

			if n := Normalize(w.Chroma); n != refNormalized(wantC) {
				t.Fatalf("rate %g span %d: normalized %v", rate, i, n)
			}
		}

		whole := got[len(spans)-6]
		if whole.LevelDB <= -120 || got[len(spans)-5].LevelDB != -120 {
			t.Fatalf("levels %v %v", whole.LevelDB, got[len(spans)-5].LevelDB)
		}
	}
}

// chordWindows returns windows of a noisy chord progression in the given key
// with gaps, flat windows and inversions, built with the app's pooling.
func progressionWindows(rng *rand.Rand, tonic int, minor bool, count int) []refChordWindow {
	degrees := [][]int{{0, 4, 7}, {9, 0, 4}, {5, 9, 0}, {7, 11, 2}, {7, 11, 2, 5}, {2, 5, 9}, {4, 7, 11}, {0, 4, 7, 11}}
	if minor {
		degrees = [][]int{{0, 3, 7}, {5, 8, 0}, {7, 11, 2}, {7, 11, 2, 5}, {8, 0, 3}, {3, 7, 10}, {10, 2, 5}, {0, 3, 7, 10}}
	}

	windows := make([]refChordWindow, count)
	chord := degrees[0]

	for w := range windows {
		if rng.IntN(3) == 0 {
			chord = degrees[rng.IntN(len(degrees))]
		}

		win := &windows[w]
		win.Start, win.End = 1.142857*float64(w), 1.142857*float64(w+1)
		win.LevelDB = -20 - 10*rng.Float64()

		for i := range 12 {
			win.Chroma[i] = 0.4 * rng.Float64() * rng.Float64()
		}

		for _, iv := range chord {
			win.Chroma[(tonic+iv)%12] += 0.5 + rng.Float64()
		}

		switch rng.IntN(12) {
		case 0:
			win.LevelDB = -60 // gated
		case 1:
			for i := range 12 {
				win.Chroma[i] = 1 + 0.1*rng.Float64() // flat
			}
		case 2:
			win.Chroma = [12]float64{} // silent
		case 3:
			// no bass evidence
		default:
			bass := chord[0]
			if rng.IntN(4) == 0 {
				bass = chord[1+rng.IntN(len(chord)-1)] // inversion
			}

			win.Bass[(tonic+bass)%12] = 1 + rng.Float64()
			win.Bass[rng.IntN(12)] += 0.5 * rng.Float64()
		}
	}

	return windows
}

func randomWindows(rng *rand.Rand, count int) []refChordWindow {
	windows := make([]refChordWindow, count)
	for w := range windows {
		win := &windows[w]
		win.Start, win.End = 0.5*float64(w), 0.5*float64(w+1)
		win.LevelDB = -70 + 60*rng.Float64()

		for i := range 12 {
			if rng.IntN(2) == 0 {
				win.Chroma[i] = rng.Float64()
			}

			if rng.IntN(4) == 0 {
				win.Bass[i] = rng.Float64()
			}
		}
	}

	return windows
}

// exactWindows has template-exact chroma, so many states tie exactly.
func exactWindows() []refChordWindow {
	pcs := func(classes ...int) [12]float64 {
		var c [12]float64
		for _, pc := range classes {
			c[pc] = 1
		}

		return c
	}

	return []refChordWindow{
		{Start: 0, End: 1, Chroma: pcs(0, 4, 7), Bass: pcs(0), LevelDB: -20},
		{Start: 1, End: 2, Chroma: pcs(9, 0, 4), Bass: pcs(9), LevelDB: -20},
		{Start: 2, End: 3, Chroma: pcs(7, 11, 2, 5), Bass: pcs(7), LevelDB: -20},
		{Start: 3, End: 4, Chroma: pcs(0, 4, 7), Bass: pcs(4), LevelDB: -20},
		{Start: 4, End: 5, Chroma: pcs(0, 4, 7), LevelDB: -70},
		{Start: 5, End: 6, Chroma: pcs(0, 3, 6, 9), LevelDB: -20},
		{Start: 6, End: 7, Chroma: pcs(0, 4, 8), Bass: pcs(0, 4), LevelDB: -20},
		{Start: 7, End: 8, Chroma: pcs(0, 4, 7), Bass: pcs(0), LevelDB: -20},
		{Start: 8, End: 9, Chroma: pcs(0, 4, 7), Bass: pcs(0), LevelDB: -20},
	}
}

func toWindows(ref []refChordWindow) []Window {
	out := make([]Window, len(ref))
	for i, r := range ref {
		out[i] = Window(r)
	}

	return out
}

// appChords maps package chords to the app's Chord fields (without Bar and
// BeatInBar, which the app labels from its grid).
func appChords(chords []Chord) []refChord {
	out := make([]refChord, len(chords))
	for i, c := range chords {
		r := refChord{Start: refR6(c.Start), End: refR6(c.End), Root: -1, Bass: -1, Quality: c.Quality,
			Symbol: c.Symbol, Score: c.Score, Margin: c.Margin}
		if !c.NoChord {
			r.Root, r.Bass, r.Voicing = int(c.Root), int(c.Bass), slices.Clone(c.Voicing)
		}

		out[i] = r
	}

	return out
}

func equalChords(t *testing.T, name string, got, want []refChord) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: %d chords, want %d\n got %+v\nwant %+v", name, len(got), len(want), got, want)
	}

	for i := range want {
		g, w := got[i], want[i]
		if g.Start != w.Start || g.End != w.End || g.Root != w.Root || g.Quality != w.Quality ||
			g.Bass != w.Bass || g.Symbol != w.Symbol || !sameBits(g.Score, w.Score) ||
			!sameBits(g.Margin, w.Margin) || !slices.Equal(g.Voicing, w.Voicing) {
			t.Fatalf("%s: chord %d\n got %+v\nwant %+v", name, i, g, w)
		}
	}
}

// TestParityChords compares Chords with the verbatim copy of AudioVisualizer's
// DetectChords bit for bit, including the reusable Chorder path.
func TestParityChords(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(45, 4))

	chorder, err := NewChorder()
	if err != nil {
		t.Fatal(err)
	}

	var (
		reused                    []Chord
		slash, noChords, sevenths int
	)

	run := func(name string, ref []refChordWindow, key Key) {
		want, wantScores := refDetectChords(ref, toRefKey(key), refDefaultChordParams())

		for _, c := range want {
			switch {
			case c.Root < 0:
				noChords++
			case c.Bass != c.Root:
				slash++
			case len(c.Voicing) == 4:
				sevenths++
			}
		}

		got, err := Chords(toWindows(ref), key)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		equalChords(t, name, appChords(got), want)

		reused, err = chorder.Chords(reused, toWindows(ref), key)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		equalChords(t, name+" (Chorder)", appChords(reused), want)

		// The unrounded window scores must match too.
		ns := len(chorder.states)

		for w, row := range wantScores {
			for s, v := range row {
				if !sameBits(chorder.scores[w*ns+s], v) {
					t.Fatalf("%s: window %d state %d score %v, want %v", name, w, s, chorder.scores[w*ns+s], v)
				}
			}
		}
	}

	for tonic := range 12 {
		for _, mode := range []Mode{Major, Minor} {
			key := Key{Tonic: pitch.PitchClass(tonic), Mode: mode}
			run(fmt.Sprintf("progression %v", key), progressionWindows(rng, tonic, mode == Minor, 40+rng.IntN(80)), key)
			run(fmt.Sprintf("random %v", key), randomWindows(rng, 1+rng.IntN(60)), key)
		}
	}

	run("exact", exactWindows(), Key{Tonic: pitch.PitchClassC, Mode: Major})
	run("exact minor", exactWindows(), Key{Tonic: pitch.PitchClassA, Mode: Minor})
	run("empty", nil, Key{})

	// Full pipeline: pooled frames, the app's key profile, chords.
	d := randomFrames(rng, 3000, parityFrameRate)
	spans := halfBarSpans(d.duration)

	windows, err := Windows(d.chroma, d.rms, parityFrameRate, spans,
		WithBassChroma(d.bassChroma, d.voicing), WithBassNotes(d.notes))
	if err != nil {
		t.Fatal(err)
	}

	ref := make([]refChordWindow, len(spans))
	for i, s := range spans {
		ref[i] = refChordWindow{Start: s.Start, End: s.End,
			Chroma:  refHarmony(d.rms, d.chroma, parityFrameRate, s.Start, s.End),
			Bass:    refBassPC(d.voicing, d.bassChroma, parityFrameRate, s.Start, s.End, d.refNotes),
			LevelDB: refMeanDB(d.rms, parityFrameRate, s.Start, s.End)}
	}

	if !slices.Equal(toWindows(ref), windows) {
		t.Fatal("pipeline windows differ")
	}

	whole := refHarmony(d.rms, d.chroma, parityFrameRate, 0, d.duration)

	key, err := EstimateKey(whole)
	if err != nil {
		t.Fatal(err)
	}

	run("pipeline", ref, key)

	if slash == 0 || noChords == 0 || sevenths == 0 {
		t.Fatalf("branch coverage: %d slash, %d N, %d seventh chords", slash, noChords, sevenths)
	}
}
