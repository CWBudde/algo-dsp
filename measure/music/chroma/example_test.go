package chroma_test

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/chroma"
	"github.com/cwbudde/algo-dsp/measure/music/harmony"
	"github.com/cwbudde/algo-dsp/measure/music/motif"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

const sampleRate = 22050

// render plays the chords (MIDI notes) one after another, each for seconds
// seconds: harmonic tones with three partials and 10 ms fades.
func render(seconds float64, chords ...[]float64) []float64 {
	n := int(seconds * sampleRate)
	x := make([]float64, n*len(chords))

	for c, notes := range chords {
		for _, m := range notes {
			f := pitch.MIDIToFrequency(m, pitch.DefaultReferenceHz)

			for h := 1.0; h <= 3; h++ {
				for i := range n {
					fade := math.Min(1, float64(min(i, n-1-i))/(0.01*sampleRate))
					x[c*n+i] += fade * 0.1 / h * math.Sin(2*math.Pi*h*f*float64(i)/sampleRate)
				}
			}
		}
	}

	return x
}

// top names the k strongest pitch classes of v in pitch-class order.
func top(v [12]float64, k int) string {
	idx := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	sort.SliceStable(idx, func(a, b int) bool { return v[idx[a]] > v[idx[b]] })
	idx = idx[:k]
	sort.Ints(idx)

	names := make([]string, k)
	for i := range names {
		names[i] = pitch.PitchClass(idx[i]).String()
	}

	return strings.Join(names, " ")
}

func ExampleCompute() {
	// One second of a D major triad in first inversion (F#3 A3 D4).
	x := render(1, []float64{54, 57, 62})

	c, frameRate, err := chroma.Compute(x, sampleRate)
	if err != nil {
		panic(err)
	}

	mid := len(c[0]) / 2

	var frame [12]float64
	for pc := range frame {
		frame[pc] = c[pc][mid]
	}

	fmt.Printf("%d frames at %.2f frames/s\n", len(c[0]), frameRate)
	fmt.Printf("frame %d: strongest classes %s\n", mid, top(frame, 3))

	// Output:
	// 44 frames at 43.07 frames/s
	// frame 22: strongest classes D F# A
}

func ExampleProfile() {
	// An A minor cadence, Am – Dm – E – Am, one second per chord.
	x := render(1,
		[]float64{45, 57, 60, 64},
		[]float64{50, 62, 65, 69},
		[]float64{40, 56, 59, 64},
		[]float64{45, 57, 60, 64},
	)

	c, _, err := chroma.Compute(x, sampleRate)
	if err != nil {
		panic(err)
	}

	key, err := harmony.EstimateKey(chroma.Profile(c, 0, len(c[0])))
	if err != nil {
		panic(err)
	}

	fmt.Println(key)

	// Output:
	// A minor
}

func ExampleBeatSync() {
	// At 120 BPM, two beats per chord: C – Am – F – G, two unrelated
	// chords, the phrase again, then a whole tone higher. FindChromaMotifs
	// needs three occurrences by default.
	phrase := [][]float64{{48, 60, 64, 67}, {45, 57, 60, 64}, {41, 57, 60, 65}, {43, 55, 59, 62}}

	var chords [][]float64

	chords = append(chords, phrase...)
	chords = append(chords, []float64{49, 61, 65, 69}, []float64{50, 62, 65, 68})
	chords = append(chords, phrase...)

	for _, ch := range phrase {
		up := make([]float64, len(ch))
		for i, m := range ch {
			up[i] = m + 2
		}

		chords = append(chords, up)
	}

	x := render(1, chords...)

	c, frameRate, err := chroma.Compute(x, sampleRate)
	if err != nil {
		panic(err)
	}

	grid, err := rhythm.NewGrid(120, 0, 0, float64(len(chords)))
	if err != nil {
		panic(err)
	}

	beats, err := chroma.BeatSync(c, frameRate, grid, len(grid.Beats()))
	if err != nil {
		panic(err)
	}

	motifs, err := motif.FindChromaMotifs(beats, grid)
	if err != nil {
		panic(err)
	}

	for _, m := range motifs {
		fmt.Printf("%g beats: %s\n", m.SpanBeats, strings.Join(m.Notes, " "))

		for _, o := range m.Occurrences {
			fmt.Printf("  %4.1f s %+d %s\n", o.Start, o.Transposition, o.Variant)
		}
	}

	// Output:
	// 8 beats: C C A A F F G G
	//    0.0 s +0 exact
	//    6.0 s +0 exact
	//   10.0 s +2 transposed
}

// Example_harmonyWindows feeds the chroma to harmony.Windows, which weights
// every frame by its RMS. The RMS is measured over the same frame grid:
// frame i covers samples i·hop − hop to i·hop + hop.
func Example_harmonyWindows() {
	// G – Em – C – D, one second each.
	x := render(1,
		[]float64{43, 55, 59, 62},
		[]float64{40, 55, 59, 64},
		[]float64{48, 55, 60, 64},
		[]float64{50, 57, 62, 66},
	)

	c, frameRate, err := chroma.Compute(x, sampleRate)
	if err != nil {
		panic(err)
	}

	hop := chroma.DefaultHop
	rms := make([]float64, len(c[0]))

	for i := range rms {
		lo, hi := max(0, i*hop-hop), min(len(x), i*hop+hop)

		sum := 0.0
		for _, v := range x[lo:hi] {
			sum += v * v
		}

		rms[i] = math.Sqrt(sum / float64(max(1, hi-lo)))
	}

	spans := make([]harmony.Span, 4)
	for i := range spans {
		spans[i] = harmony.Span{Start: float64(i), End: float64(i + 1)}
	}

	windows, err := harmony.Windows(c, rms, frameRate, spans)
	if err != nil {
		panic(err)
	}

	var profile [12]float64

	for _, w := range windows {
		for pc, v := range w.Chroma {
			profile[pc] += v
		}
	}

	key, err := harmony.EstimateKey(profile)
	if err != nil {
		panic(err)
	}

	chords, err := harmony.Chords(windows, key)
	if err != nil {
		panic(err)
	}

	symbols := make([]string, len(chords))
	for i, ch := range chords {
		symbols[i] = ch.Symbol
	}

	fmt.Println(key, "|", strings.Join(symbols, " "))

	// Output:
	// G major | G Em C D
}
