package spectrum

import (
	"math/cmplx"
	"testing"
)

func BenchmarkMagnitude(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			in := make([]complex128, tc.size)
			for i := range in {
				in[i] = complex(float64(i)/10.0, float64(tc.size-i)/10.0)
			}

			b.SetBytes(int64(tc.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = Magnitude(in)
			}
		})
	}
}

func BenchmarkPower(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			in := make([]complex128, tc.size)
			for i := range in {
				in[i] = complex(float64(i)/10.0, float64(tc.size-i)/10.0)
			}

			b.SetBytes(int64(tc.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = Power(in)
			}
		})
	}
}

func BenchmarkMagnitudeFromParts(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			re := make([]float64, tc.size)
			im := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			for i := range re {
				re[i] = float64(i) / 10.0
				im[i] = float64(tc.size-i) / 10.0
			}

			b.SetBytes(int64(tc.size * 16)) // re+im = 16 bytes per element
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				MagnitudeFromParts(dst, re, im)
			}
		})
	}
}

func BenchmarkPowerFromParts(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			re := make([]float64, tc.size)
			im := make([]float64, tc.size)
			dst := make([]float64, tc.size)
			for i := range re {
				re[i] = float64(i) / 10.0
				im[i] = float64(tc.size-i) / 10.0
			}

			b.SetBytes(int64(tc.size * 16)) // re+im = 16 bytes per element
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				PowerFromParts(dst, re, im)
			}
		})
	}
}

// Benchmark the old implementation for comparison
func magnitudeNaive(in []complex128) []float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float64, len(in))
	for i := range out {
		out[i] = cmplx.Abs(in[i])
	}
	return out
}

func powerNaive(in []complex128) []float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float64, len(in))
	for i := range out {
		x := in[i]
		re := real(x)
		im := imag(x)
		out[i] = re*re + im*im
	}
	return out
}

func BenchmarkMagnitudeNaive(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			in := make([]complex128, tc.size)
			for i := range in {
				in[i] = complex(float64(i)/10.0, float64(tc.size-i)/10.0)
			}

			b.SetBytes(int64(tc.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = magnitudeNaive(in)
			}
		})
	}
}

func BenchmarkPowerNaive(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"64", 64},
		{"256", 256},
		{"1K", 1024},
		{"4K", 4096},
		{"16K", 16384},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			in := make([]complex128, tc.size)
			for i := range in {
				in[i] = complex(float64(i)/10.0, float64(tc.size-i)/10.0)
			}

			b.SetBytes(int64(tc.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = powerNaive(in)
			}
		})
	}
}
