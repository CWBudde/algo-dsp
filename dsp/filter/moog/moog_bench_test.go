package moog

import (
	"math"
	"testing"
)

func BenchmarkProcessSample(b *testing.B) {
	tests := []struct {
		name    string
		variant Variant
		os      int
	}{
		{name: "classic", variant: VariantClassic, os: 1},
		{name: "classic_lightweight", variant: VariantClassicLightweight, os: 1},
		{name: "improved", variant: VariantImprovedClassic, os: 1},
		{name: "huovilainen", variant: VariantHuovilainen, os: 1},
		{name: "huovilainen_os4", variant: VariantHuovilainen, os: 4},
		{name: "zdf", variant: VariantZDF, os: 1},
		{name: "zdf_os4", variant: VariantZDF, os: 4},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			f, err := New(48000,
				WithVariant(tc.variant),
				WithCutoffHz(1800),
				WithResonance(1.2),
				WithDrive(2.0),
				WithOversampling(tc.os),
			)
			if err != nil {
				b.Fatalf("New() error = %v", err)
			}

			in := 0.0
			step := 2 * math.Pi * 220 / 48000

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = f.ProcessSample(math.Sin(in))
				in += step
			}
		})
	}
}

func BenchmarkProcessInPlace1024(b *testing.B) {
	tests := []struct {
		name    string
		variant Variant
		os      int
	}{
		{name: "classic", variant: VariantClassic, os: 1},
		{name: "classic_lightweight", variant: VariantClassicLightweight, os: 1},
		{name: "huovilainen", variant: VariantHuovilainen, os: 1},
		{name: "huovilainen_os4", variant: VariantHuovilainen, os: 4},
		{name: "zdf", variant: VariantZDF, os: 1},
		{name: "zdf_os4", variant: VariantZDF, os: 4},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			f, err := New(48000,
				WithVariant(tc.variant),
				WithCutoffHz(1400),
				WithResonance(1.0),
				WithDrive(2.5),
				WithOversampling(tc.os),
			)
			if err != nil {
				b.Fatalf("New() error = %v", err)
			}

			buf := make([]float64, 1024)
			for i := range buf {
				buf[i] = 0.7*math.Sin(2*math.Pi*220*float64(i)/48000) + 0.2*math.Sin(2*math.Pi*660*float64(i)/48000)
			}

			b.SetBytes(int64(len(buf) * 8))
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				f.ProcessInPlace(buf)
			}
		})
	}
}
