package cqt_test

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/cqt"
	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// A generic constant-Q transform with the default options: seven octaves of
// semitone bins from C1 (32.70 Hz). The strongest bin of a 440 Hz tone is
// A4.
func Example() {
	const sampleRate = 22050

	tr, err := cqt.New(sampleRate)
	if err != nil {
		panic(err)
	}

	// One second of a 440 Hz tone.
	x := make([]float64, sampleRate)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * 440 * float64(i) / sampleRate)
	}

	// ProcessInto reuses dst and does not allocate once warmed up.
	dst := make([]float64, tr.OutputLen(len(x)))
	if err := tr.ProcessInto(dst, x); err != nil {
		panic(err)
	}

	frames, bins := tr.FrameCount(len(x)), tr.Bins()

	// Frame-major layout: frame fr, bin b is dst[fr*bins+b].
	mid := dst[(frames/2)*bins : (frames/2+1)*bins]
	peak := 0

	for b, v := range mid {
		if v > mid[peak] {
			peak = b
		}
	}

	fmt.Printf("frames=%d bins=%d octaves=%d padding=%v\n",
		frames, bins, tr.Octaves(), tr.Padding())
	fmt.Printf("peak: bin %d at %.2f Hz\n", peak, tr.Frequencies()[peak])
	// Output:
	// frames=44 bins=84 octaves=7 padding=zero
	// peak: bin 45 at 439.96 Hz
}

// The CQT front end of spotify/basic-pitch: 2 s minus one hop of 22050 Hz
// audio give 172 frames of 309 bins.
func ExampleBasicPitch() {
	tr, err := cqt.New(cqt.BasicPitchSampleRate, cqt.BasicPitch()...)
	if err != nil {
		panic(err)
	}

	// A 440 Hz tone.
	x := make([]float64, 43844)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * 440 * float64(i) / cqt.BasicPitchSampleRate)
	}

	dst := make([]float64, tr.OutputLen(len(x)))
	if err := tr.ProcessInto(dst, x); err != nil {
		panic(err)
	}

	frames, bins := tr.FrameCount(len(x)), tr.Bins()
	mid := dst[(frames/2)*bins : (frames/2+1)*bins]
	peak := 0

	for b, v := range mid {
		if v > mid[peak] {
			peak = b
		}
	}

	fmt.Printf("frames=%d bins=%d octaves=%d nfft=%d hop=%d padding=%v\n",
		frames, bins, tr.Octaves(), tr.NFFT(), tr.Hop(), tr.Padding())
	fmt.Printf("peak: bin %d at %.1f Hz\n", peak, tr.Frequencies()[peak])
	// Output:
	// frames=172 bins=309 octaves=9 nfft=256 hop=256 padding=reflect
	// peak: bin 144 at 440.0 Hz
}

// With fewer bins than one octave, nnAudio places its kernels below the
// frequencies it reports. The NNAudio preset reproduces that; the generic
// transform centres every kernel on its bin.
func ExampleNNAudio() {
	const sampleRate = 22050

	// Six semitone bins from A4: half an octave.
	opts := []cqt.Option{cqt.WithFMin(440), cqt.WithBins(6)}

	generic, err := cqt.New(sampleRate, opts...)
	if err != nil {
		panic(err)
	}

	// Options after the preset override its parameters.
	nn, err := cqt.New(sampleRate, append(cqt.NNAudio(), opts...)...)
	if err != nil {
		panic(err)
	}

	// The centre frequency of a kernel is its phase step per sample.
	kernelHz := func(tr *cqt.Transform, k int) float64 {
		row := tr.Kernels()[k]
		mid := len(row) / 2

		return cmplx.Phase(row[mid+1]/row[mid]) * tr.SampleRate() / (2 * math.Pi)
	}

	fmt.Printf("bin 0:         %.2f Hz\n", generic.Frequencies()[0])
	fmt.Printf("generic kernel %.2f Hz\n", kernelHz(generic, 0))
	fmt.Printf("nnAudio kernel %.2f Hz\n", kernelHz(nn, 0))
	// Output:
	// bin 0:         440.00 Hz
	// generic kernel 440.00 Hz
	// nnAudio kernel 311.13 Hz
}

func ExampleTransform_Frequencies() {
	// The generic defaults: 84 bins, 12 per octave, from 32.70 Hz.
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

// FrameCount follows torch.stft(center=True): n/hop+1 frames. dsp/stft
// counts ceil(n/hop) centred frames, one fewer when the hop divides n.
func ExampleTransform_FrameCount() {
	const sampleRate, hop = 22050, 512

	tr, err := cqt.New(sampleRate, cqt.WithHopLength(hop))
	if err != nil {
		panic(err)
	}

	st, err := stft.New(2048, hop)
	if err != nil {
		panic(err)
	}

	for _, n := range []int{16384, 16000} {
		fmt.Printf("n=%d: cqt %d frames, stft %d frames\n", n, tr.FrameCount(n), st.FrameCount(n))
	}
	// Output:
	// n=16384: cqt 33 frames, stft 32 frames
	// n=16000: cqt 32 frames, stft 32 frames
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
	frame, bin := tr.FrameCount(len(x))/2, 24
	i := 2 * (frame*tr.Bins() + bin)
	re, im := out[i], out[i+1]

	fmt.Printf("%d values, |X| = %.3f\n", len(out), math.Hypot(re, im))
	// Output:
	// 6048 values, |X| = 18.361
}
