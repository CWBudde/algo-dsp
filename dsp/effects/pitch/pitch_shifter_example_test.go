package pitch_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

func ExamplePitchShifter() {
	p, err := pitch.NewPitchShifter(48000)
	if err != nil {
		panic(err)
	}

	_ = p.SetPitchSemitones(7)

	buf := make([]float64, 256)
	for i := range buf {
		buf[i] = 0.5 * math.Sin(2*math.Pi*220*float64(i)/48000.0)
	}

	p.ProcessInPlace(buf)

	fmt.Printf("Ratio: %.3f\n", p.PitchRatio())
	// Output: Ratio: 1.498
}
