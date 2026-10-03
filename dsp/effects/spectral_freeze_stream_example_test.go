package effects_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects"
)

func ExampleNewStreamingSpectralFreeze() {
	processor, err := effects.NewSpectralFreeze(48000)
	if err != nil {
		panic(err)
	}

	processor.SetFrozen(true)

	stream, err := effects.NewStreamingSpectralFreeze(processor)
	if err != nil {
		panic(err)
	}

	if err = stream.ProcessInPlace(make([]float64, 128)); err != nil {
		panic(err)
	}

	fmt.Println("Analysis latency:", stream.Latency())
	stream.Reset()
	// Output: Analysis latency: 1024
}
