package spatial_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/effects/spatial"
)

// ExampleHaasDelay_ProcessStereo delays the right channel by 10 ms (480 samples
// at 48 kHz) while the left channel passes through unchanged.
func ExampleHaasDelay_ProcessStereo() {
	h, err := spatial.NewHaasDelay(48000, spatial.WithHaasDelayMs(10))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	const n = 512

	left := make([]float64, n)
	right := make([]float64, n)
	left[0] = 1 // impulse on both channels
	right[0] = 1

	if err := h.ProcessStereoInPlace(left, right); err != nil {
		fmt.Println("error:", err)
		return
	}

	rightIdx := -1

	for i, v := range right {
		if v != 0 {
			rightIdx = i
			break
		}
	}

	fmt.Printf("right channel delayed by %d samples\n", rightIdx)
	// Output:
	// right channel delayed by 480 samples
}
