package onset_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/onset"
)

// clicks returns decaying 1 kHz bursts at the given times (seconds).
func clicks(seconds float64, times ...float64) []float64 {
	const rate = features.DefaultSampleRate

	x := make([]float64, int(seconds*rate))
	for _, t := range times {
		start := int(t * rate)
		for j := 0; j < int(rate/20) && start+j < len(x); j++ {
			x[start+j] = 0.8 * math.Cos(2*math.Pi*1000*float64(j)/rate) * math.Exp(-float64(j)/(rate*0.006))
		}
	}

	return x
}

func ExampleDetect() {
	channels := [][]float64{clicks(2, 0.25, 0.75, 1.25, 1.75)}

	f, err := features.Extract(channels, features.DefaultConfig())
	if err != nil {
		panic(err)
	}

	events, err := onset.Detect(channels, f)
	if err != nil {
		panic(err)
	}

	for _, e := range events {
		fmt.Printf("%.3f s\n", e.Time)
	}
	// Output:
	// 0.250 s
	// 0.750 s
	// 1.250 s
	// 1.750 s
}

func ExampleClassifyDrums() {
	const rate = features.DefaultSampleRate

	// A decaying 60 Hz kick at 0.25 s.
	x := make([]float64, int(rate))
	for j := range int(rate / 3) {
		x[int(0.25*rate)+j] = 0.8 * math.Sin(2*math.Pi*60*float64(j)/rate) * math.Exp(-float64(j)/(rate*0.06))
	}

	channels := [][]float64{x}

	f, err := features.Extract(channels, features.DefaultConfig())
	if err != nil {
		panic(err)
	}

	events, err := onset.Detect(channels, f)
	if err != nil {
		panic(err)
	}

	events, err = onset.ClassifyDrums(events, f)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.2f s %v\n", events[0].Time, events[0].Kind)
	// Output:
	// 0.25 s kick
}

func ExampleKind_String() {
	fmt.Println(onset.KindKick, onset.KindSnare, onset.KindHat, onset.KindUnknown)
	// Output:
	// kick snare hat unknown
}

func ExampleWithDrumRules() {
	// Three-band layout: low, mid, high. One frame, simple shares.
	f := &features.Frames{
		Timing: features.Timing{SampleRate: 100, Hop: 1},
		Bands:  [][]float64{{0.1}, {0.1}, {1}},
	}
	rules := onset.DrumRules{
		Frames: 1, KickBands: []int{0}, KickShare: 0.5,
		HatBands: []int{2}, HatShare: 0.5, BodyBands: []int{0}, BodyMax: 0.2,
	}

	events, err := onset.ClassifyDrums([]onset.Event{{Time: 0, Strength: 1}}, f, onset.WithDrumRules(rules))
	if err != nil {
		panic(err)
	}

	fmt.Println(events[0].Kind)
	// Output:
	// hat
}
