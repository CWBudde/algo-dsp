package chroma

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/harmony"
	"github.com/cwbudde/algo-dsp/measure/music/motif"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// ramp returns n frames where class pc of frame j is (pc+1)·(j+1).
func ramp(n int) [12][]float64 {
	var c [12][]float64
	for pc := range c {
		c[pc] = make([]float64, n)
		for j := range n {
			c[pc][j] = float64((pc + 1) * (j + 1))
		}
	}

	return c
}

// meanOf returns the expected BeatSync vector of frames [lo, hi) of ramp.
func meanOf(lo, hi int) [12]float64 {
	var v [12]float64
	if hi <= lo {
		return v
	}

	for pc := range v {
		v[pc] = float64(pc+1) * (float64(lo+hi)/2 + 0.5)
	}

	return v
}

func TestProfile(t *testing.T) {
	t.Parallel()

	c := ramp(10)

	tests := []struct {
		name       string
		start, end int
		lo, hi     int
	}{
		{"all", 0, 10, 0, 10},
		{"inner", 2, 5, 2, 5},
		{"clipped both sides", -5, 100, 0, 10},
		{"empty", 4, 4, 0, 0},
		{"reversed", 6, 3, 0, 0},
		{"past the end", 12, 20, 0, 0},
	}

	for _, tt := range tests {
		if got, want := Profile(c, tt.start, tt.end), meanOf(tt.lo, tt.hi); got != want {
			t.Errorf("%s: Profile = %v, want %v", tt.name, got, want)
		}
	}

	// Rows of unequal length clip to the shortest.
	c[5] = c[5][:4]
	if got, want := Profile(c, 0, 10), Profile(ramp(4), 0, 4); got != want {
		t.Errorf("short row: Profile = %v, want %v", got, want)
	}

	if got := Profile([12][]float64{}, 0, 10); got != ([12]float64{}) {
		t.Errorf("no frames: Profile = %v, want zeros", got)
	}
}

func TestBeatSync(t *testing.T) {
	t.Parallel()

	// 4 frames per second, 60 BPM: one beat is 4 frames, frame j is centred
	// on j/4 s.
	const rate = 4.0

	c := ramp(10) // frames at 0 .. 2.25 s

	tests := []struct {
		name   string
		origin float64
		beats  int
		frames [][2]int // expected [lo, hi) per beat
	}{
		// Frames 4 and 8 lie exactly on beat boundaries and go to the
		// later beat.
		{"origin 0", 0, 3, [][2]int{{0, 4}, {4, 8}, {8, 10}}},
		// Beat 0 spans [0.1, 1.1) s: frame 0 (0 s) precedes it, frame 4
		// (1.0 s), whose window straddles the boundary, belongs to it by its
		// centre.
		{"origin 0.1", 0.1, 3, [][2]int{{1, 5}, {5, 9}, {9, 10}}},
		// Beats 0 and 1 lie before the first frame.
		{"negative origin", -2, 4, [][2]int{{0, 0}, {0, 0}, {0, 4}, {4, 8}}},
		// Beats 3 and 4 lie after the last frame.
		{"past the end", 0, 5, [][2]int{{0, 4}, {4, 8}, {8, 10}, {10, 10}, {10, 10}}},
		{"zero beats", 0, 0, nil},
	}

	for _, tt := range tests {
		grid, err := rhythm.NewGrid(60, tt.origin, 0, 3)
		if err != nil {
			t.Fatal(err)
		}

		got, err := BeatSync(c, rate, grid, tt.beats)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}

		if got == nil || len(got) != tt.beats {
			t.Fatalf("%s: %d beats (nil %v), want %d", tt.name, len(got), got == nil, tt.beats)
		}

		for i, fr := range tt.frames {
			if want := meanOf(fr[0], fr[1]); got[i] != want {
				t.Errorf("%s: beat %d = %v, want frames %v = %v", tt.name, i, got[i], fr, want)
			}
		}
	}
}

// TestBeatSyncMatchesWindows checks that BeatSync assigns frames exactly as
// harmony.Windows does for the same spans, at a frame rate whose beat
// boundaries fall between frames.
func TestBeatSyncMatchesWindows(t *testing.T) {
	t.Parallel()

	const (
		rate = 22050.0 / 512
		n    = 400
	)

	c := ramp(n)
	c[11] = slices.Repeat([]float64{1}, n) // counts the frames of a span

	rms := slices.Repeat([]float64{1}, n)

	for _, origin := range []float64{-0.37, 0, 0.123, 0.5} {
		grid, err := rhythm.NewGrid(123, origin, 0, 10)
		if err != nil {
			t.Fatal(err)
		}

		beats := len(grid.Beats()) + 2

		spans := make([]harmony.Span, beats)
		for i := range spans {
			spans[i] = harmony.Span{Start: grid.BeatStart(i), End: grid.BeatStart(i + 1)}
		}

		win, err := harmony.Windows(c, rms, rate, spans)
		if err != nil {
			t.Fatal(err)
		}

		got, err := BeatSync(c, rate, grid, beats)
		if err != nil {
			t.Fatal(err)
		}

		for i, w := range win {
			count := w.Chroma[11]

			for pc := range 11 {
				want := 0.0
				if count > 0 {
					want = w.Chroma[pc] / count
				}

				if math.Abs(got[i][pc]-want) > 1e-9*math.Max(1, want) {
					t.Fatalf("origin %g beat %d class %d: %g, Windows gives %g", origin, i, pc, got[i][pc], want)
				}
			}
		}
	}
}

func TestBeatSyncErrors(t *testing.T) {
	t.Parallel()

	grid, err := rhythm.NewGrid(120, 0, 0, 4)
	if err != nil {
		t.Fatal(err)
	}

	ragged := ramp(8)
	ragged[3] = ragged[3][:7]

	tests := []struct {
		name  string
		c     [12][]float64
		rate  float64
		grid  rhythm.Grid
		beats int
	}{
		{"zero grid", ramp(8), 4, rhythm.Grid{}, 4},
		{"zero frame rate", ramp(8), 0, grid, 4},
		{"negative frame rate", ramp(8), -4, grid, 4},
		{"NaN frame rate", ramp(8), math.NaN(), grid, 4},
		{"infinite frame rate", ramp(8), math.Inf(1), grid, 4},
		{"negative beats", ramp(8), 4, grid, -1},
		{"ragged rows", ragged, 4, grid, 4},
	}

	for _, tt := range tests {
		_, err := BeatSync(tt.c, tt.rate, tt.grid, tt.beats)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: error %v, want ErrInvalidArgument", tt.name, err)
		}
	}

	_, err = BeatSync(ramp(8), 4, rhythm.Grid{}, 4)
	if !errors.Is(err, rhythm.ErrInvalidArgument) {
		t.Errorf("zero grid: error %v does not wrap rhythm.ErrInvalidArgument", err)
	}
}

// TestChromaMotifs synthesizes a chord phrase, its exact repeat and a repeat
// transposed up a whole tone, separated by unrelated chords, and finds them
// with motif.FindChromaMotifs through BeatSync and the default motif
// options.
func TestChromaMotifs(t *testing.T) {
	t.Parallel()

	const (
		sr  = 22050
		bpm = 120
	)

	beat := 60.0 / bpm

	// Two beats per chord: C – Am – F – G, voiced in octave 4 over a bass.
	phrase := [][]float64{{48, 60, 64, 67}, {45, 57, 60, 64}, {41, 57, 60, 65}, {43, 55, 59, 62}}
	// Unrelated material: an augmented and a diminished chord, a cluster
	// and a fourth stack.
	fillers := [][][]float64{
		{{49, 61, 65, 69}, {50, 62, 65, 68}, {52, 63, 64, 66}, {47, 59, 64, 69}},
		{{46, 58, 61, 64}, {44, 56, 60, 64}, {51, 63, 64, 65}, {42, 54, 59, 64}},
	}

	var events []event

	at := 0.0
	add := func(chords [][]float64, transpose float64) {
		for _, ch := range chords {
			notes := make([]float64, len(ch))
			for i, m := range ch {
				notes[i] = m + transpose
			}

			events = append(events, event{at, at + 2*beat, notes})
			at += 2 * beat
		}
	}

	add(phrase, 0)
	add(fillers[0], 0)
	add(phrase, 0)
	add(fillers[1], 0)
	add(phrase, 2)

	x := synth(sr, at, DefaultReferenceHz, events...)

	c, rate, err := Compute(x, sr)
	if err != nil {
		t.Fatal(err)
	}

	grid, err := rhythm.NewGrid(bpm, 0, 0, at)
	if err != nil {
		t.Fatal(err)
	}

	beats, err := BeatSync(c, rate, grid, len(grid.Beats()))
	if err != nil {
		t.Fatal(err)
	}

	motifs, err := motif.FindChromaMotifs(beats, grid)
	if err != nil {
		t.Fatal(err)
	}

	if len(motifs) == 0 {
		t.Fatal("no chroma motif found")
	}

	for _, m := range motifs {
		t.Logf("motif span %g beats, notes %v", m.SpanBeats, m.Notes)

		for _, o := range m.Occurrences {
			t.Logf("  %5.2f..%5.2f s  transposition %+d  similarity %.3f  %s", o.Start, o.End, o.Transposition, o.Similarity, o.Variant)
		}
	}

	// The phrase starts at 0, 8 and 16 s.
	m := motifs[0]
	if m.SpanBeats != 8 || len(m.Occurrences) != 3 {
		t.Fatalf("first motif spans %g beats with %d occurrences, want 8 beats, 3 occurrences", m.SpanBeats, len(m.Occurrences))
	}

	if len(motifs) != 1 {
		t.Errorf("%d motifs, want 1 (the 4-beat windows are covered by the 8-beat motif)", len(motifs))
	}

	wantStart := []float64{0, 8, 16}
	wantTrans := []int{0, 0, 2}
	wantVariant := []motif.Variant{motif.VariantExact, motif.VariantExact, motif.VariantTransposed}

	for i, o := range m.Occurrences {
		if o.Start != wantStart[i] || o.Transposition != wantTrans[i] || o.Variant != wantVariant[i] {
			t.Errorf("occurrence %d at %g s, transposition %d, %s; want %g s, %d, %s",
				i, o.Start, o.Transposition, o.Variant, wantStart[i], wantTrans[i], wantVariant[i])
		}

		if o.Similarity < 0.999 {
			t.Errorf("occurrence %d: similarity %g", i, o.Similarity)
		}
	}

	if !slices.Equal(m.Notes, []string{"C", "C", "A", "A", "F", "F", "G", "G"}) {
		t.Errorf("motif notes %v", m.Notes)
	}
}
