package motif

import (
	"math/rand/v2"
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/melody"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// benchSong is a seeded phrase song of about n notes at 120 BPM.
func benchSong(b *testing.B, n int) ([]melody.CleanNote, rhythm.Grid) {
	b.Helper()

	g, _ := newGrid(b, 120, 0, 0, float64(n))
	rng := rand.New(rand.NewPCG(52, 52))

	notes := phraseSong(g, rng, n/4)
	for len(notes) > n {
		notes = notes[:n]
	}

	return notes, g
}

// BenchmarkNgramPass is the n-gram pass of FindNoteMotifs (lengths 8, 6
// and 4, grid pass disabled) on 2,000 notes.
func BenchmarkNgramPass(b *testing.B) {
	notes, g := benchSong(b, 2000)
	b.Logf("%d notes", len(notes))
	b.ReportAllocs()

	for b.Loop() {
		_, err := FindNoteMotifs(notes, g, "lead", WithGridWindows())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFindNoteMotifs runs both passes on 2,000 notes.
func BenchmarkFindNoteMotifs(b *testing.B) {
	notes, g := benchSong(b, 2000)
	b.ReportAllocs()

	for b.Loop() {
		_, err := FindNoteMotifs(notes, g, "lead")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFindChromaMotifs runs the chroma pass on the beat chroma of a
// 128-bar song (512 beats).
func BenchmarkFindChromaMotifs(b *testing.B) {
	g, _ := newGrid(b, 120, 0, 0, 256)
	rng := rand.New(rand.NewPCG(53, 53))
	beats := phraseChroma(g, phraseSong(g, rng, 128), rng)

	b.ReportAllocs()

	for b.Loop() {
		_, err := FindChromaMotifs(beats, g)
		if err != nil {
			b.Fatal(err)
		}
	}
}
