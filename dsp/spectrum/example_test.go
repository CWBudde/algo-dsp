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

func ExampleGoertzel() {
	fs := 48000.0
	f0 := 1000.0
	g, _ := spectrum.NewGoertzel(f0, fs)

	// Pure sine at 1000 Hz, length 480 (exactly 10 cycles)
	n := 480

	sig := make([]float64, n)
	for i := range sig {
		sig[i] = math.Sin(2 * math.Pi * f0 / fs * float64(i))
	}

	g.ProcessBlock(sig)
	mag := g.Magnitude()

	// Normalize by N/2
	fmt.Printf("Magnitude at 1000Hz: %.1f\n", mag/float64(n/2))

	// Pure sine at 2000 Hz
	g.Reset()

	for i := range sig {
		sig[i] = math.Sin(2 * math.Pi * 2000.0 / fs * float64(i))
	}

	g.ProcessBlock(sig)
	mag2 := g.Magnitude()
	fmt.Printf("Magnitude at 2000Hz: %.1f\n", mag2/float64(n/2))

	// Output:
	// Magnitude at 1000Hz: 1.0
	// Magnitude at 2000Hz: 0.0
}

func ExampleMultiGoertzel() {
	fs := 48000.0
	freqs := []float64{697, 770, 852, 941, 1209, 1336, 1477, 1633} // DTMF frequencies
	mg, _ := spectrum.NewMultiGoertzel(freqs, fs)

	// Generate 697 Hz + 1209 Hz (Digit '1')
	n := 480

	sig := make([]float64, n)
	for i := range sig {
		sig[i] = 0.5*math.Sin(2*math.Pi*697/fs*float64(i)) +
			0.5*math.Sin(2*math.Pi*1209/fs*float64(i))
	}

	mg.ProcessBlock(sig)
	powers := mg.Powers()

	fmt.Println("Detected frequencies:")

	for i, f := range freqs {
		// Thresholding on magnitude (normalized)
		mag := math.Sqrt(powers[i]) / (float64(n) / 2.0)
		if mag > 0.4 {
			fmt.Printf("%.0f Hz (mag: %.1f)\n", f, mag)
		}
	}
	// Output:
	// Detected frequencies:
	// 697 Hz (mag: 0.5)
	// 1209 Hz (mag: 0.5)
}
