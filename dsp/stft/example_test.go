package stft_test

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

func ExampleSTFT_Forward() {
	const (
		sampleRate = 8000.0
		nfft       = 256
		hop        = 64
	)

	// 1 kHz sine, 0.1 s.
	x := make([]float64, 800)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * 1000 * float64(i) / sampleRate)
	}

	s, err := stft.New(nfft, hop)
	if err != nil {
		panic(err)
	}

	spec, err := s.Forward(x)
	if err != nil {
		panic(err)
	}

	frame := spec[5]
	peak := 0

	for k := range frame {
		if cmplx.Abs(frame[k]) > cmplx.Abs(frame[peak]) {
			peak = k
		}
	}

	fmt.Printf("frames=%d bins=%d peak=%.0f Hz\n", len(spec), s.Bins(), float64(peak)*sampleRate/nfft)
	// Output:
	// frames=13 bins=129 peak=1000 Hz
}

func ExampleSTFT_Inverse() {
	x := make([]float64, 1000)
	for i := range x {
		x[i] = math.Sin(0.05*float64(i)) + 0.3*math.Cos(0.31*float64(i))
	}

	// 512/150 is not a COLA pair for Hann; the window-sum-square
	// normalization still reconstructs the input.
	s, err := stft.New(512, 150, stft.WithCenter(stft.PadReflect), stft.WithNormalized())
	if err != nil {
		panic(err)
	}

	spec, err := s.Forward(x)
	if err != nil {
		panic(err)
	}

	y, err := s.Inverse(spec, len(x))
	if err != nil {
		panic(err)
	}

	maxErr := 0.0
	for i := range x {
		maxErr = math.Max(maxErr, math.Abs(y[i]-x[i]))
	}

	fmt.Println(maxErr < 1e-12)
	// Output:
	// true
}

func ExampleSTFT_FrameInto() {
	x := make([]float64, 4800)
	for i := range x {
		x[i] = math.Sin(2 * math.Pi * float64(i) / 32)
	}

	s, err := stft.New(512, 240, stft.WithWindow(window.TypeBlackman))
	if err != nil {
		panic(err)
	}

	// Reuse one buffer for all frames: FrameInto does not allocate.
	dst := make([]complex128, s.Bins())

	for frame := range s.FrameCount(len(x)) {
		if err := s.FrameInto(dst, x, frame); err != nil {
			panic(err)
		}
	}

	fmt.Printf("%d frames, bin 16 = %.1f\n", s.FrameCount(len(x)), cmplx.Abs(dst[16]))
	// Output:
	// 20 frames, bin 16 = 107.5
}
