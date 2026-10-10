package cqt_test

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
)

// The CQT front end of spotify/basic-pitch: 2 s minus one hop of 22050 Hz
// audio give 172 frames of 309 bins.
func Example() {
	const sampleRate = 22050

	tr, err := cqt.New(sampleRate,
		cqt.WithHopLength(256),
		cqt.WithFMin(27.5),
		cqt.WithBins(309),
		cqt.WithBinsPerOctave(36),
	)
	if err != nil {
		panic(err)
	}

	// A 440 Hz tone.
	x := make([]float64, 43844)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * 440 * float64(i) / sampleRate)
	}

	// ProcessInto reuses dst and does not allocate once warmed up.
	dst := make([]float64, tr.OutputLen(len(x)))
	if err := tr.ProcessInto(dst, x); err != nil {
		panic(err)
	}

	frames, bins := tr.NumFrames(len(x)), tr.NumBins()

	// Frame-major layout: frame fr, bin b is dst[fr*bins+b].
	mid := dst[(frames/2)*bins : (frames/2+1)*bins]
	peak := 0

	for b, v := range mid {
		if v > mid[peak] {
			peak = b
		}
	}

	fmt.Printf("frames=%d bins=%d octaves=%d nfft=%d hop=%d\n",
		frames, bins, tr.Octaves(), tr.NFFT(), tr.Hop())
	fmt.Printf("peak: bin %d at %.1f Hz\n", peak, tr.Frequencies()[peak])
	// Output:
	// frames=172 bins=309 octaves=9 nfft=256 hop=256
	// peak: bin 144 at 440.0 Hz
}

func ExampleTransform_Frequencies() {
	// nnAudio's defaults: 84 bins, 12 per octave, from 32.70 Hz.
	tr, err := cqt.New(22050)
	if err != nil {
		panic(err)
	}

	f := tr.Frequencies()
	l := tr.Lengths()

	fmt.Printf("%d bins: %.2f Hz .. %.2f Hz\n", len(f), f[0], f[len(f)-1])
	fmt.Printf("A4 is bin 45: %.2f Hz, kernel length %.0f samples\n", f[45], l[45])
	// Output:
	// 84 bins: 32.70 Hz .. 3950.68 Hz
	// A4 is bin 45: 439.96 Hz, kernel length 843 samples
}

func ExampleWithOutput() {
	const sampleRate = 16000

	tr, err := cqt.New(sampleRate,
		cqt.WithHopLength(128),
		cqt.WithFMin(100),
		cqt.WithBins(48),
		cqt.WithBinsPerOctave(12),
		cqt.WithOutput(cqt.OutputComplex),
	)
	if err != nil {
		panic(err)
	}

	// A 400 Hz cosine: bin 24 is two octaves above fmin.
	x := make([]float64, 8000)
	for i := range x {
		x[i] = math.Cos(2 * math.Pi * 400 * float64(i) / sampleRate)
	}

	out, err := tr.Process(x)
	if err != nil {
		panic(err)
	}

	// Complex layout: real and imaginary part interleaved per frame and bin.
	frame, bin := tr.NumFrames(len(x))/2, 24
	i := 2 * (frame*tr.NumBins() + bin)
	re, im := out[i], out[i+1]

	fmt.Printf("%d values, |X| = %.3f\n", len(out), math.Hypot(re, im))
	// Output:
	// 6048 values, |X| = 18.361
}
