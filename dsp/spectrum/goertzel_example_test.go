package spectrum_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/spectrum"
)

// ExampleGoertzel estimates the amplitude of a single tone with one Goertzel
// analyzer.
func ExampleGoertzel() {
	const (
		sampleRate = 48000.0
		blockSize  = 480 // exactly 10 cycles of 1 kHz
		amp        = 0.5
	)

	analyzer, err := spectrum.NewGoertzel(1000, sampleRate)
	if err != nil {
		fmt.Println("error")
		return
	}

	buf := make([]float64, blockSize)
	for i := range buf {
		buf[i] = amp * math.Cos(2*math.Pi*1000*float64(i)/sampleRate)
	}

	analyzer.ProcessBlock(buf)

	fmt.Printf("amplitude=%.2f\n", analyzer.NormalizedMagnitude(blockSize))
	// Output:
	// amplitude=0.50
}

// ExampleGoertzelBank decodes a DTMF tone by finding the loudest row and column
// frequency, sharing one input block across all bins.
func ExampleGoertzelBank() {
	const sampleRate = 8000.0

	freqs := []float64{697, 770, 852, 941, 1209, 1336, 1477, 1633}

	bank, err := spectrum.NewGoertzelBank(freqs, sampleRate)
	if err != nil {
		fmt.Println("error")
		return
	}

	// DTMF digit "5": 770 Hz row + 1336 Hz column.
	blockSize := 1024
	buf := make([]float64, blockSize)

	for i := range buf {
		buf[i] = 0.5*math.Sin(2*math.Pi*770*float64(i)/sampleRate) +
			0.5*math.Sin(2*math.Pi*1336*float64(i)/sampleRate)
	}

	bank.ProcessBlock(buf)
	powers := bank.Powers(nil)

	loudest := func(lo, hi int) float64 {
		best := lo
		for i := lo + 1; i < hi; i++ {
			if powers[i] > powers[best] {
				best = i
			}
		}

		return freqs[best]
	}

	fmt.Printf("row=%.0f col=%.0f\n", loudest(0, 4), loudest(4, 8))
	// Output:
	// row=770 col=1336
}

// ExampleAnalyzeBlock measures the power at a single frequency in one call,
// without managing an analyzer's lifecycle.
func ExampleAnalyzeBlock() {
	const (
		sampleRate = 48000.0
		blockSize  = 480 // exactly 10 cycles of 1 kHz
	)

	buf := make([]float64, blockSize)
	for i := range buf {
		buf[i] = 0.5 * math.Cos(2*math.Pi*1000*float64(i)/sampleRate)
	}

	power, err := spectrum.AnalyzeBlock(buf, 1000, sampleRate)
	if err != nil {
		fmt.Println("error")
		return
	}

	fmt.Printf("power>0: %v\n", power > 0)
	// Output:
	// power>0: true
}
