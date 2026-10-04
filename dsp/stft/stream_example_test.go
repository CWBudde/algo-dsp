package stft_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

func ExampleNewStream() {
	stream, _ := stft.NewStream(16, 4)
	frames := 0

	emit := func(_ int64, bins []complex128) error { frames++; return nil }
	if err := stream.Process(make([]float64, 21), emit); err != nil {
		panic(err)
	}

	for {
		done, err := stream.FinishStep(1, emit)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	fmt.Println(frames)
	// Output: 6
}

func ExampleTransform_InverseStream() {
	transform, _ := stft.New(16, 4)
	inverse, _ := transform.InverseStream(21)
	frames, _ := transform.Forward(make([]float64, 21))
	samples := 0

	emit := func(_ int64, block []float64) error { samples += len(block); return nil }
	for _, frame := range frames {
		if err := inverse.ProcessFrame(frame, emit); err != nil {
			panic(err)
		}
	}

	for {
		done, err := inverse.FinishStep(16, emit)
		if err != nil {
			panic(err)
		}

		if done {
			break
		}
	}

	fmt.Println(samples)
	// Output: 21
}
