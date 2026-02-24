package loudness_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/loudness"
)

func ExampleMeter() {
	fs := 48000.0
	m := loudness.NewMeter(
		loudness.WithSampleRate(fs),
		loudness.WithChannels(1),
	)

	// Generate 4 seconds of 1000Hz sine at 0.5 amplitude (-6.02 dBFS)
	// mean square = (0.5^2)/2 = 0.125
	// K-weighted mean square (at 1000Hz) approx 0.125 * 1.1668 = 0.14585
	// LUFS = -0.691 + 10*log10(0.14585) = -0.691 - 8.36 = -9.051 LUFS
	n := int(fs * 4)

	sig := make([]float64, n)
	for i := range sig {
		sig[i] = 0.5 * math.Sin(2*math.Pi*1000.0/fs*float64(i))
	}

	m.StartIntegration()
	m.ProcessBlock(sig)

	fmt.Printf("Momentary: %.1f LUFS\n", m.Momentary())
	fmt.Printf("Short-term: %.1f LUFS\n", m.ShortTerm())
	fmt.Printf("Integrated: %.1f LUFS\n", m.Integrated())

	// Output:
	// Momentary: -9.1 LUFS
	// Short-term: -9.1 LUFS
	// Integrated: -9.2 LUFS
}
