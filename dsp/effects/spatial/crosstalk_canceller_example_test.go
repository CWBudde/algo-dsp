package spatial_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

func ExampleCrosstalkCanceller_ProcessStereo() {
	c, err := spatial.NewCrosstalkCanceller(48000,
		spatial.WithCancellerStages(2),
		spatial.WithCancellerAttenuation(0.7),
	)
	if err != nil {
		panic(err)
	}

	left, right := c.ProcessStereo(0.8, 0.2)
	fmt.Println(left != 0, right != 0)
	// Output: true true
}

func ExampleCrosstalkCanceller_ProcessInPlace() {
	c, err := spatial.NewCrosstalkCanceller(48000)
	if err != nil {
		panic(err)
	}

	left := []float64{0.2, 0.4, 0.6}
	right := []float64{0.6, 0.4, 0.2}

	err = c.ProcessInPlace(left, right)
	if err != nil {
		panic(err)
	}

	fmt.Println(len(left), len(right))
	// Output: 3 3
}
