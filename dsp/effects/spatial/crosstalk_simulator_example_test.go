package spatial_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

func ExampleCrosstalkSimulator_ProcessStereo() {
	s, err := spatial.NewCrosstalkSimulator(48000,
		spatial.WithSimulatorPreset(spatial.CrosstalkPresetIRCAM),
		spatial.WithSimulatorCrossfeedMix(0.3),
	)
	if err != nil {
		panic(err)
	}

	left, right := s.ProcessStereo(1, 1)
	fmt.Println(left != 0, right != 0)
	// Output: true true
}

func ExampleCrosstalkSimulator_ProcessInPlace() {
	s, err := spatial.NewCrosstalkSimulator(48000)
	if err != nil {
		panic(err)
	}

	left := []float64{0, 1, 0, 0}
	right := []float64{1, 0, 0, 0}

	err = s.ProcessInPlace(left, right)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(left), len(right))
	// Output: 4 4
}
