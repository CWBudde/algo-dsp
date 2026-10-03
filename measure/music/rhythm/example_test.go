package rhythm_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// pulses returns a flux-like curve with unit pulses every period frames,
// starting at frame first.
func pulses(n, first, period int) []float64 {
	x := make([]float64, n)
	for i := first; i < n; i += period {
		x[i] = 1
	}

	return x
}

func ExampleEstimateTempo() {
	tm := features.Timing{SampleRate: 24000, Hop: 240} // 100 frames/s

	flux := pulses(2000, 10, 50) // a pulse every 0.5 s: 120 BPM

	tempo, err := rhythm.EstimateTempo(rhythm.Novelty(flux, rhythm.DefaultNoveltyRadius), tm, rhythm.WithPrior(118))
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.1f BPM, best scan candidate %.1f BPM\n", tempo.BPM, tempo.Candidates[0].BPM)
	// Output:
	// 120.0 BPM, best scan candidate 120.0 BPM
}

func ExampleFitBeatPhase() {
	tm := features.Timing{SampleRate: 24000, Hop: 240}
	low := pulses(2000, 10, 50) // kick energy at 0.1 s, 0.6 s, ...

	phase := rhythm.FitBeatPhase(low, tm, 120, 20)

	beats, err := rhythm.BeatGrid(phase, 120, 20)
	if err != nil {
		panic(err)
	}

	fmt.Printf("phase %.3f s, %d beats, first %.2f s\n", phase, len(beats), beats[0])
	// Output:
	// phase 0.100 s, 40 beats, first 0.10 s
}

func ExampleTempoScore() {
	tm := features.Timing{SampleRate: 24000, Hop: 240}
	x := rhythm.Novelty(pulses(2000, 10, 50), 30)

	fmt.Printf("120 BPM: %.2f, 100 BPM: %.2f\n", rhythm.TempoScore(x, tm, 120), rhythm.TempoScore(x, tm, 100))
	// Output:
	// 120 BPM: 1.00, 100 BPM: 0.00
}

func ExampleGridError() {
	onsets := []float64{0.100, 0.352, 0.598, 0.851}

	// 120 BPM grid at 0.1 s, 16th notes (125 ms).
	fmt.Printf("%.1f ms\n", rhythm.GridError(onsets, 0.1, 120, 4)*1000)
	// Output:
	// 2.0 ms
}

func ExampleBeatGrid() {
	beats, err := rhythm.BeatGrid(0.25, 120, 2)
	if err != nil {
		panic(err)
	}

	fmt.Println(beats)
	// Output:
	// [0.25 0.75 1.25 1.75]
}

func ExampleNovelty() {
	fmt.Println(rhythm.Novelty([]float64{0, 0, 3, 0, 0}, 1))
	// Output:
	// [0 0 2 0 0]
}

func ExampleDownbeat() {
	beats := []float64{0.0, 0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5}
	accents := []rhythm.Accent{
		{Time: 0.51, Weight: 1}, {Time: 2.49, Weight: 1}, // strong kicks on beats 1 and 5
		{Time: 1.0, Weight: 0.3},
	}

	fmt.Println(rhythm.Downbeat(beats, accents, 4, 0.06))
	// Output:
	// 1
}
