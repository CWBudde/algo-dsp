package spectrum_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

func ExampleMagnitude() {
	bins := []complex128{1 + 0i, 0 + 1i, -1 + 0i}
	mag := spectrum.Magnitude(bins)
	fmt.Printf("%.1f %.1f %.1f\n", mag[0], mag[1], mag[2])
	// Output:
	// 1.0 1.0 1.0
}

func ExampleUnwrapPhase() {
	wrapped := []float64{2.8, -2.7, -2.6}
	unwrapped := spectrum.UnwrapPhase(wrapped)
	fmt.Printf("%.3f %.3f %.3f\n", unwrapped[0], unwrapped[1], unwrapped[2])
	// Output:
	// 2.800 3.583 3.683
}

func ExampleSmoothFractionalOctave() {
	freq := []float64{100, 125, 160, 200, 250, 315}
	vals := []float64{1, 1, 9, 1, 1, 1}
	out, _ := spectrum.SmoothFractionalOctave(freq, vals, 3)
	fmt.Printf("%.1f %.1f %.1f\n", out[1], out[2], out[3])
	// Output:
	// 1.0 9.0 1.0
}

func ExampleGroupDelayFromPhase() {
	fftSize := 8
	delay := 1.0
	phase := make([]float64, 4)
	for k := range phase {
		w := 2 * math.Pi * float64(k) / float64(fftSize)
		phase[k] = -w * delay
	}
	gd, _ := spectrum.GroupDelayFromPhase(phase, fftSize)
	fmt.Printf("%.1f %.1f %.1f\n", gd[0], gd[1], gd[2])
	// Output:
	// 1.0 1.0 1.0
}
