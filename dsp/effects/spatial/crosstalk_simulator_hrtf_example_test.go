package spatial_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

type exampleHRTFProvider struct{}

func (exampleHRTFProvider) ImpulseResponses(sampleRate float64) (spatial.HRTFImpulseResponseSet, error) {
	return spatial.HRTFImpulseResponseSet{
		LeftDirect:  []float64{1.0},
		LeftCross:   []float64{0.15},
		RightDirect: []float64{1.0},
		RightCross:  []float64{0.15},
	}, nil
}

func ExampleHRTFCrosstalkSimulator_ProcessStereo() {
	s, err := spatial.NewHRTFCrosstalkSimulator(48000,
		spatial.WithHRTFProvider(exampleHRTFProvider{}),
		spatial.WithHRTFMode(spatial.HRTFModeComplete),
	)
	if err != nil {
		panic(err)
	}

	left, right := s.ProcessStereo(1, 0)
	fmt.Println(left != 0, right != 0)
	// Output: true true
}

func ExampleHRTFCrosstalkSimulator_ProcessInPlace() {
	s, err := spatial.NewHRTFCrosstalkSimulator(48000,
		spatial.WithHRTFProvider(exampleHRTFProvider{}),
	)
	if err != nil {
		panic(err)
	}

	left := []float64{1, 0, 0}
	right := []float64{0, 1, 0}

	if err := s.ProcessInPlace(left, right); err != nil {
		panic(err)
	}

	fmt.Println(len(left), len(right))
	// Output: 3 3
}
