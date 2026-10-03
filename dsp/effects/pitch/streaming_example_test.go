package pitch_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

func ExampleNewStreamingPitchShifter() {
	processor, err := pitch.NewPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	stream, err := pitch.NewStreamingPitchShifter(processor)
	if err != nil {
		panic(err)
	}

	block := []float64{0.25, -0.5, 0.125}
	if err = stream.ProcessInPlace(block); err != nil {
		panic(err)
	}

	fmt.Println("Identity:", block, "latency:", stream.Latency())
	stream.Reset()
	// Output: Identity: [0.25 -0.5 0.125] latency: 0
}

func ExampleNewStreamingSpectralPitchShifter() {
	processor, err := pitch.NewSpectralPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	if err = processor.SetPitchSemitones(12); err != nil {
		panic(err)
	}

	stream, err := pitch.NewStreamingSpectralPitchShifter(processor)
	if err != nil {
		panic(err)
	}

	block := make([]float64, 128)
	if err = stream.ProcessInPlace(block); err != nil {
		panic(err)
	}

	fmt.Println("Analysis latency:", stream.Latency())
	stream.Reset()
	// Output: Analysis latency: 1024
}
