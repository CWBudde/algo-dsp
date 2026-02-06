package thd_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/thd"
)

func ExampleAnalyzeSignal() {
	sampleRate := 48000.0
	fftSize := 4096
	fundamentalBin := 64
	fundamental := float64(fundamentalBin) * sampleRate / float64(fftSize)

	signal := make([]float64, fftSize)
	for i := range signal {
		t := float64(i) / sampleRate
		signal[i] = math.Sin(2*math.Pi*fundamental*t) + 0.02*math.Sin(2*math.Pi*2*fundamental*t)
	}

	res := thd.AnalyzeSignal(signal, thd.Config{
		SampleRate:      sampleRate,
		FFTSize:         fftSize,
		FundamentalFreq: fundamental,
	})

	fmt.Printf("THD: %.2f%%\n", res.THD*100)
	fmt.Printf("SINAD: %.2f dB\n", res.SINAD)
	// Output:
	// THD: 2.00%
	// SINAD: 33.94 dB
}
