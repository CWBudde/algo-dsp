package modulation_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/modulation"
)

func ExampleRotarySpeaker_processStereoInPlace() {
	r, err := modulation.NewRotarySpeaker()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = r.SetSampleRate(44100)
	_ = r.SetMix(1.0)
	_ = r.SetStereoWidth(1.0)
	_ = r.SetDrive(1.1)
	_ = r.SetCrossoverHz(800.0)
	_ = r.SetSpeedMode(modulation.SpeedModeTremolo)

	const n = 512

	left := make([]float64, n)
	right := make([]float64, n)

	for i := range n {
		s := math.Sin(2 * math.Pi * 440 * float64(i) / 44100.0)
		left[i] = s
		right[i] = s
	}

	r.ProcessStereoInPlace(left, right)

	fmt.Printf("processed %d stereo frames\n", n)

	// Output:
	// processed 512 stereo frames
}
