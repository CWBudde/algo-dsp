package melody_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/measure/music/melody"
)

// tone adds a harmonic tone of the given MIDI note between start and end
// seconds to x.
func tone(x []float64, sampleRate float64, midi int, start, end float64) {
	f0 := pitch.MIDIToFrequency(float64(midi), pitch.DefaultReferenceHz)

	for i := int(start * sampleRate); i < int(end*sampleRate); i++ {
		t := float64(i) / sampleRate
		for h := 1; h <= 4; h++ {
			x[i] += 0.3 / float64(h) * math.Sin(2*math.Pi*f0*float64(h)*t)
		}
	}
}

func ExampleAnalyze() {
	const sampleRate = 24000.0

	x := make([]float64, int(1.2*sampleRate))
	tone(x, sampleRate, 60, 0.1, 0.5) // C4
	tone(x, sampleRate, 67, 0.6, 1.0) // G4

	res, err := melody.Analyze(x, sampleRate, melody.WithOnsets([]float64{0.098, 0.603}))
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d frames at %.0f frames/s\n", len(res.Pitch), res.FrameRate)

	for _, n := range res.Notes {
		fmt.Printf("%.3f-%.2f s: %s%d\n", n.Start, n.End, pitch.PitchClass(n.MIDI%12), n.MIDI/12-1)
	}

	// Output:
	// 120 frames at 100 frames/s
	// 0.098-0.51 s: C4
	// 0.603-1.01 s: G4
}

func ExampleAnalyze_chroma() {
	const sampleRate = 24000.0

	// An A minor triad of pure tones.
	x := make([]float64, int(0.5*sampleRate))

	for _, midi := range []float64{57, 60, 64} {
		f := pitch.MIDIToFrequency(midi, pitch.DefaultReferenceHz)
		for i := range x {
			x[i] += 0.2 * math.Sin(2*math.Pi*f*float64(i)/sampleRate)
		}
	}

	res, err := melody.Analyze(x, sampleRate)
	if err != nil {
		panic(err)
	}

	for c := range res.Chroma {
		if v := res.Chroma[c][25]; v > 0.5 {
			fmt.Printf("%s ", pitch.PitchClass(c))
		}
	}

	fmt.Println()
	// Output:
	// C E A
}

func ExampleSegmentNotes() {
	// A hand-made pitch track at 100 frames/s, for example from a YIN
	// detector converted with pitch.FrequencyToMIDI: 150 ms of a slightly
	// sharp D4, 20 ms of silence (bridged), 100 ms more D4, then 100 ms of F4.
	var pitchTrack, voicing []float64

	add := func(midi float64, frames int) {
		for range frames {
			pitchTrack = append(pitchTrack, midi)
			voicing = append(voicing, map[bool]float64{true: 0.9}[midi != 0])
		}
	}

	add(62.2, 15)
	add(0, 2)
	add(62.1, 10)
	add(65, 10)

	notes, err := melody.SegmentNotes(pitchTrack, voicing, 100)
	if err != nil {
		panic(err)
	}

	for _, n := range notes {
		fmt.Printf("%.2f-%.2f s: MIDI %d, strength %.1f\n", n.Start, n.End, n.MIDI, n.Strength)
	}

	// Output:
	// 0.00-0.27 s: MIDI 62, strength 0.9
	// 0.27-0.37 s: MIDI 65, strength 0.9
}

func ExampleMedianVoiced() {
	// The octave error in frame 2 is removed; the unvoiced frame stays 0.
	smoothed := melody.MedianVoiced([]float64{60, 60, 72, 60, 60, 0, 61}, 1)
	fmt.Println(smoothed)
	// Output:
	// [60 60 60 60 60 0 61]
}

func ExampleDownmix() {
	mono, err := melody.Downmix([][]float64{{0.5, 1, 0}, {0.5, 0, -1}})
	if err != nil {
		panic(err)
	}

	fmt.Println(mono)
	// Output:
	// [0.5 0.5 -0.5]
}
