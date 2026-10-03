package features_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/measure/music/features"
)

func ExampleExtract() {
	cfg := features.DefaultConfig()

	// One second of a 1 kHz sine at 0.5, left and right in anti-phase.
	left := make([]float64, int(cfg.SampleRate))
	right := make([]float64, len(left))

	for i := range left {
		left[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/cfg.SampleRate)
		right[i] = -left[i]
	}

	f, err := features.Extract([][]float64{left, right}, cfg)
	if err != nil {
		panic(err)
	}

	const i = 50 // frame at 0.5 s

	fmt.Printf("frames=%d t=%.2fs\n", f.Len(), f.FrameTime(i))
	fmt.Printf("rms=%.3f width=%.3f centroid≈%.0f Hz\n", f.RMS[i], f.Width[i], math.Round(f.Centroid[i]/100)*100)
	fmt.Printf("band 400–2000 Hz=%.3f\n", f.Bands[2][i])
	// Output:
	// frames=100 t=0.50s
	// rms=0.354 width=1.000 centroid≈1000 Hz
	// band 400–2000 Hz=0.354
}

func ExampleWithLogSpectrogram() {
	cfg := features.DefaultConfig()

	x := make([]float64, int(cfg.SampleRate))
	for i := range x {
		x[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/cfg.SampleRate)
	}

	f, err := features.Extract([][]float64{x}, cfg, features.WithLogSpectrogram(16, 25, 12000))
	if err != nil {
		panic(err)
	}

	row := f.Spectrogram.Frame(50)
	freqs := f.Spectrogram.BinFrequencies()
	peak := 0

	for b, v := range row {
		if v > row[peak] {
			peak = b
		}
	}

	fmt.Printf("peak row %d centred at %.0f Hz reads %.1f dBFS\n", peak, freqs[peak], row[peak])
	// Output:
	// peak row 9 centred at 977 Hz reads -9.0 dBFS
}

func ExampleNewLogScale() {
	s, err := features.NewLogScale(4, 100, 1600, 24000, 2048)
	if err != nil {
		panic(err)
	}

	fmt.Printf("edges %.0f\n", s.Edges())
	fmt.Printf("centres %.0f\n", s.BinFrequencies())
	// Output:
	// edges [100 200 400 800 1600]
	// centres [141 283 566 1131]
}

func ExampleNormalize() {
	envelope := []float64{0, 0, 1, 1, 1, 0, 0, 0}

	// Instant attack, 10 ms release at 100 frames/s.
	y, err := features.Normalize(envelope, 1e-4, 0, 0.01, 100)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%.3f\n", y)
	// Output:
	// [0.000 0.000 1.000 1.000 1.000 0.368 0.135 0.050]
}

func ExamplePercentile() {
	fmt.Println(features.Percentile([]float64{4, 1, 3, 2, 5}, 0.5))
	// Output:
	// 3
}

func ExampleSilence() {
	const rate = 1000.0

	x := make([]float64, 1000)
	for i := range x {
		if i < 400 || i >= 700 {
			x[i] = 0.5
		}
	}

	intervals, err := features.Silence([][]float64{x}, rate, -45, 0.15)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", intervals)
	// Output:
	// [{Start:0.4 End:0.7}]
}

func ExampleTiming() {
	tm := features.DefaultConfig().Timing()

	fmt.Println(tm.FrameRate(), tm.FrameTime(25), tm.FramePosition(0.25))
	// Output:
	// 100 0.25 25
}

func ExampleFloorSamples() {
	fmt.Println(features.FloorSamples(0.15*24000), features.FloorSamples(0.005*44100))
	// Output:
	// 3600 220
}
