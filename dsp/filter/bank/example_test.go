package bank_test

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/bank"
)

func ExampleOctave() {
	// Build a full-octave filter bank at 48 kHz.
	b := bank.Octave(1, 48000)
	fmt.Printf("Octave bands: %d\n", b.NumBands())
	for _, band := range b.Bands() {
		fmt.Printf("  %.0f Hz (%.0f – %.0f)\n",
			band.CenterFreq, band.LowCutoff, band.HighCutoff)
	}
	// Output:
	// Octave bands: 10
	//   32 Hz (22 – 45)
	//   63 Hz (45 – 89)
	//   126 Hz (89 – 178)
	//   251 Hz (178 – 355)
	//   501 Hz (355 – 708)
	//   1000 Hz (708 – 1413)
	//   1995 Hz (1413 – 2818)
	//   3981 Hz (2818 – 5623)
	//   7943 Hz (5623 – 11220)
	//   15849 Hz (11220 – 22387)
}

func ExampleOctave_thirdOctave() {
	// Build a 1/3-octave bank restricted to 100–10000 Hz.
	b := bank.Octave(3, 48000, bank.WithFrequencyRange(100, 10000))
	fmt.Printf("1/3-octave bands (100–10 kHz): %d\n", b.NumBands())
	// Output:
	// 1/3-octave bands (100–10 kHz): 21
}
