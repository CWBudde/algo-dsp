package rhythm_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

// exampleGrid is a 120 BPM grid (beat 0.5 s, sixteenth 0.125 s, bar 2 s)
// whose first beat is at 0.5 s and whose first downbeat is beat 3 (2.0 s),
// so bar 0 is a pickup bar starting at 0.0 s.
func exampleGrid() rhythm.Grid {
	g, err := rhythm.NewGrid(120, 0.5, 3, 10)
	if err != nil {
		panic(err)
	}

	return g
}

func ExampleNewGrid() {
	// Beat times and downbeat as estimated by this package.
	beats, err := rhythm.BeatGrid(0.5, 120, 10)
	if err != nil {
		panic(err)
	}

	accents := []rhythm.Accent{{Time: 2.0, Weight: 1}, {Time: 4.0, Weight: 1}, {Time: 6.0, Weight: 1}}
	downbeat := rhythm.Downbeat(beats, accents, 4, 0.06)

	g, err := rhythm.NewGrid(120, beats[0], downbeat, 10, rhythm.WithBeats(beats))
	if err != nil {
		panic(err)
	}

	fmt.Printf("downbeat %d, %d beats, bars start at %v\n", downbeat, len(g.Beats()), g.BarStarts())
	// Output:
	// downbeat 3, 19 beats, bars start at [0 2 4 6 8]
}

func ExampleWithBeatsPerBar() {
	// 3/4 at 90 BPM: a bar is three beats of 2/3 s.
	g, err := rhythm.NewGrid(90, 0, 0, 4, rhythm.WithBeatsPerBar(3))
	if err != nil {
		panic(err)
	}

	fmt.Printf("bar %.0f s, %d slots per bar\n", g.BarSeconds(), g.SlotsPerBar())
	// Output:
	// bar 2 s, 12 slots per bar
}

func ExampleWithSubdivisions() {
	// Eighth-note triplets: three slots per beat.
	g, err := rhythm.NewGrid(120, 0, 0, 4, rhythm.WithSubdivisions(3))
	if err != nil {
		panic(err)
	}

	fmt.Printf("slot %.4f s, slot of 1.0 s: %d\n", g.SlotSeconds(), g.Slot(1.0))
	// Output:
	// slot 0.1667 s, slot of 1.0 s: 6
}

func ExampleWithBeats() {
	measured := []float64{0.5, 1.01, 1.49, 2.0}

	g, err := rhythm.NewGrid(120, 0.5, 0, 2.2, rhythm.WithBeats(measured))
	if err != nil {
		panic(err)
	}

	fmt.Println(g.Beats(), g.BeatStart(1))
	// Output:
	// [0.5 1.01 1.49 2] 1
}

func ExampleGrid_Validate() {
	fmt.Println(rhythm.Grid{}.Validate() != nil, exampleGrid().Validate())
	// Output:
	// true <nil>
}

func ExampleGrid_Slot() {
	g := exampleGrid()
	fmt.Println(g.Slot(0.5), g.Slot(0.53), g.Slot(1.0), g.Slot(0.4))
	// Output:
	// 0 0 4 -1
}

func ExampleGrid_SlotTime() {
	fmt.Println(exampleGrid().SlotTime(6))
	// Output:
	// 1.25
}

func ExampleGrid_Bar() {
	g := exampleGrid()
	fmt.Println(g.Bar(0.5), g.Bar(1.99), g.Bar(2.0), g.Bar(-3))
	// Output:
	// 0 0 1 0
}

func ExampleGrid_Bars() {
	fmt.Println(exampleGrid().Bars())
	// Output:
	// 5
}

func ExampleGrid_BarStart() {
	g := exampleGrid()
	fmt.Println(g.BarStart(0), g.BarStart(1))
	// Output:
	// 0 2
}

func ExampleGrid_BarStarts() {
	fmt.Println(exampleGrid().BarStarts())
	// Output:
	// [0 2 4 6 8]
}

func ExampleGrid_BeatStart() {
	fmt.Println(exampleGrid().BeatStart(3))
	// Output:
	// 2
}

func ExampleGrid_Beats() {
	fmt.Println(len(exampleGrid().Beats()), exampleGrid().Beats()[:3])
	// Output:
	// 19 [0.5 1 1.5]
}

func ExampleGrid_BarPosition() {
	fmt.Println(exampleGrid().BarPosition(3))
	// Output:
	// 1.5
}

func ExampleGrid_Tick() {
	g := exampleGrid()
	// 480 ticks per quarter note; audio time 0 or the first beat as tick 0.
	fmt.Println(g.Tick(2.0, 480), g.Tick(2.0-g.Origin(), 480))
	// Output:
	// 1920 1440
}

func ExampleGrid_BPM() {
	g := exampleGrid()
	fmt.Println(g.BPM(), g.Origin(), g.Downbeat(), g.Duration())
	// Output:
	// 120 0.5 3 10
}

func ExampleGrid_Origin() {
	fmt.Println(exampleGrid().Origin())
	// Output:
	// 0.5
}

func ExampleGrid_BeatSeconds() {
	g := exampleGrid()
	fmt.Println(g.BeatSeconds(), g.SlotSeconds(), g.BarSeconds())
	// Output:
	// 0.5 0.125 2
}

func ExampleGrid_SlotSeconds() {
	fmt.Println(exampleGrid().SlotSeconds())
	// Output:
	// 0.125
}

func ExampleGrid_BarSeconds() {
	fmt.Println(exampleGrid().BarSeconds())
	// Output:
	// 2
}

func ExampleGrid_BeatsPerBar() {
	fmt.Println(exampleGrid().BeatsPerBar())
	// Output:
	// 4
}

func ExampleGrid_Subdivisions() {
	fmt.Println(exampleGrid().Subdivisions())
	// Output:
	// 4
}

func ExampleGrid_SlotsPerBar() {
	fmt.Println(exampleGrid().SlotsPerBar())
	// Output:
	// 16
}

func ExampleGrid_Downbeat() {
	fmt.Println(exampleGrid().Downbeat())
	// Output:
	// 3
}

func ExampleGrid_Duration() {
	fmt.Println(exampleGrid().Duration())
	// Output:
	// 10
}
