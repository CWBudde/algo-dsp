package pitch_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

func ExamplePitchProcessor() {
	in := make([]float64, 512)
	for i := range in {
		in[i] = 0.3 * math.Sin(2*math.Pi*220*float64(i)/48000.0)
	}

	td, err := pitch.NewPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	fd, err := pitch.NewSpectralPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	processors := []pitch.PitchProcessor{td, fd}
	for _, p := range processors {
		if err := p.SetPitchSemitones(4); err != nil {
			panic(err)
		}

		out := p.Process(in)
		fmt.Printf("%.3f %d\n", p.PitchRatio(), len(out))
	}

	// Output:
	// 1.260 512
	// 1.260 512
}
