package stft_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/stft"
	"github.com/cwbudde/algo-dsp/internal/testutil"
)

func BenchmarkFrameInto(b *testing.B) {
	s, err := stft.New(2048, 240)
	if err != nil {
		b.Fatal(err)
	}

	x := testutil.DeterministicNoise(1, 1, 24000)
	dst := make([]complex128, s.Bins())
	frames := s.FrameCount(len(x))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		err := s.FrameInto(dst, x, i%frames)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameInto32(b *testing.B) {
	s, err := stft.New32(2048, 240)
	if err != nil {
		b.Fatal(err)
	}

	x64 := testutil.DeterministicNoise(1, 1, 24000)

	x := make([]float32, len(x64))
	for i, v := range x64 {
		x[i] = float32(v)
	}

	dst := make([]complex64, s.Bins())
	frames := s.FrameCount(len(x))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; b.Loop(); i++ {
		err := s.FrameInto(dst, x, i%frames)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInverse(b *testing.B) {
	s, err := stft.New(4096, 1024, stft.WithCenter(stft.PadReflect), stft.WithNormalized())
	if err != nil {
		b.Fatal(err)
	}

	x := testutil.DeterministicNoise(1, 1, 44100)

	spec, err := s.Forward(x)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := s.Inverse(spec, len(x))
		if err != nil {
			b.Fatal(err)
		}
	}
}
