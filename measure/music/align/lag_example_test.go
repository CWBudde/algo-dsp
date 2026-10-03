package align_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/align"
)

// tones returns n samples of a sum of low sines at 24 kHz, delayed by delay
// samples and scaled by gain.
func tones(n int, delay, gain float64) []float64 {
	x := make([]float64, n)

	for i := range x {
		t := (float64(i) - delay) / 24000
		x[i] = gain * (0.3*math.Sin(2*math.Pi*97*t) + 0.2*math.Sin(2*math.Pi*233*t+1) + 0.1*math.Sin(2*math.Pi*411*t+2))
	}

	return x
}

func ExampleLag() {
	// A rendered copy of the source, 2.5 ms late and 1 dB quieter.
	source := tones(48000, 0, 1)
	render := tones(48000, 60, math.Pow(10, -1.0/20))

	res, err := align.Lag(source, render, 24000)
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %d samples (%.2f ms), correlation %.4f, gain %.2f dB\n",
		res.Samples, res.Seconds*1000, res.Correlation, res.GainDB)
	// Output:
	// lag 60 samples (2.50 ms), correlation 1.0000, gain -1.00 dB
}

func ExampleLagChannels() {
	// A stereo source and a copy whose channels are both 12 samples early.
	source := [][]float64{tones(24000, 0, 1), tones(24000, 5, 0.5)}
	render := [][]float64{tones(24000, -12, 1), tones(24000, -7, 0.5)}

	res, err := align.LagChannels(source, render, 24000)
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %d samples, gain %.2f dB\n", res.Samples, res.GainDB)
	// Output:
	// lag -12 samples, gain 0.00 dB
}

func ExampleLagResult() {
	// A delay of 12.4 samples: the search returns the nearest whole sample
	// and RefinedSamples interpolates between the correlation values.
	res, err := align.Lag(tones(48000, 0, 1), tones(48000, 12.4, 1), 24000)
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %d samples, refined %.1f samples\n", res.Samples, res.RefinedSamples)
	// Output:
	// lag 12 samples, refined 12.4 samples
}

func ExampleLagOption() {
	// The settings of AudioVisualizer's verifyrender at 24 kHz, spelled out:
	// they are the defaults.
	opts := []align.LagOption{
		align.WithLagMaxLag(0.02),
		align.WithLagCoarseStep(16),
		align.WithLagFineRadius(16),
		align.WithLagStride(16),
	}

	res, err := align.Lag(tones(48000, 0, 1), tones(48000, 300, 1), 24000, opts...)
	if err != nil {
		panic(err)
	}

	fmt.Println("lag", res.Samples)
	// Output:
	// lag 300
}

func ExampleWithLagMaxLag() {
	// 30 ms (720 samples) is beyond the default ±20 ms search range.
	source := tones(48000, 0, 1)
	render := tones(48000, 720, 1)

	res, err := align.Lag(source, render, 24000, align.WithLagMaxLag(0.05))
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %.0f ms\n", res.Seconds*1000)
	// Output:
	// lag 30 ms
}

func ExampleWithLagCoarseStep() {
	// A coarse step of 1 searches every lag; the fine search is then moot.
	res, err := align.Lag(tones(24000, 0, 1), tones(24000, 7, 1), 24000,
		align.WithLagCoarseStep(1), align.WithLagFineRadius(0))
	if err != nil {
		panic(err)
	}

	fmt.Println("lag", res.Samples)
	// Output:
	// lag 7
}

func ExampleWithLagFineRadius() {
	// Without a fine search the lag is a multiple of the coarse step.
	res, err := align.Lag(tones(24000, 0, 1), tones(24000, 7, 1), 24000, align.WithLagFineRadius(0))
	if err != nil {
		panic(err)
	}

	fmt.Println("lag", res.Samples)
	// Output:
	// lag 0
}

func ExampleWithLagStride() {
	// Use every sample in the correlation sums.
	res, err := align.Lag(tones(24000, 0, 1), tones(24000, -40, 2), 24000, align.WithLagStride(1))
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %d, gain %.2f dB\n", res.Samples, res.GainDB)
	// Output:
	// lag -40, gain 6.02 dB
}

func ExampleWithLagWindow() {
	// Compare only the second half second of the reference; the render is
	// silent outside it, but the window still has a partner.
	source := tones(24000, 0, 1)
	render := tones(24000, 30, 1)

	for i := range 12000 {
		render[i] = 0
	}

	res, err := align.Lag(source, render, 24000, align.WithLagWindow(0.6, 0.9))
	if err != nil {
		panic(err)
	}

	fmt.Printf("lag %d, correlation %.4f\n", res.Samples, res.Correlation)
	// Output:
	// lag 30, correlation 1.0000
}
