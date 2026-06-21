package dither

import (
	"math/rand/v2"
	"testing"
)

func BenchmarkQuantizerProcessSample(b *testing.B) {
	quant, _ := NewQuantizer(44100,
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)

	b.ReportAllocs()

	for b.Loop() {
		quant.ProcessSample(0.3)
	}
}

func BenchmarkQuantizerProcessInPlace(b *testing.B) {
	quant, _ := NewQuantizer(44100,
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)

	buf := make([]float64, 1024)
	rng := rand.New(rand.NewPCG(7, 0))

	for idx := range buf {
		buf[idx] = rng.Float64()*2 - 1
	}

	b.ReportAllocs()

	for b.Loop() {
		quant.ProcessInPlace(buf)
	}
}

func BenchmarkQuantizerNoDither(b *testing.B) {
	quant, _ := NewQuantizer(44100,
		WithDitherType(DitherNone),
		WithFIRPreset(PresetNone),
	)

	b.ReportAllocs()

	for b.Loop() {
		quant.ProcessSample(0.3)
	}
}

func BenchmarkQuantizerIIRShelf(b *testing.B) {
	quant, _ := NewQuantizer(44100,
		WithIIRShelf(10000),
		WithRNG(rand.New(rand.NewPCG(42, 0))),
	)

	b.ReportAllocs()

	for b.Loop() {
		quant.ProcessSample(0.3)
	}
}

func BenchmarkQuantizerAllDitherTypes(b *testing.B) {
	types := []struct {
		name       string
		ditherType DitherType
	}{
		{"None", DitherNone},
		{"Rectangular", DitherRectangular},
		{"Triangular", DitherTriangular},
		{"Gaussian", DitherGaussian},
		{"FastGaussian", DitherFastGaussian},
	}

	for _, tt := range types {
		b.Run(tt.name, func(b *testing.B) {
			quant, _ := NewQuantizer(44100,
				WithDitherType(tt.ditherType),
				WithRNG(rand.New(rand.NewPCG(42, 0))),
			)

			b.ReportAllocs()

			for b.Loop() {
				quant.ProcessSample(0.3)
			}
		})
	}
}
