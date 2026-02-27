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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			inData := make([]complex128, testCase.size)
			for i := range inData {
				inData[i] = complex(float64(i)/10.0, float64(testCase.size-i)/10.0)
			}

			b.SetBytes(int64(testCase.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for range b.N {
				_ = Magnitude(inData)
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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			inData := make([]complex128, testCase.size)
			for i := range inData {
				inData[i] = complex(float64(i)/10.0, float64(testCase.size-i)/10.0)
			}

			b.SetBytes(int64(testCase.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for range b.N {
				_ = Power(inData)
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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			re := make([]float64, testCase.size)
			im := make([]float64, testCase.size)
			dst := make([]float64, testCase.size)

			for i := range re {
				re[i] = float64(i) / 10.0
				im[i] = float64(testCase.size-i) / 10.0
			}

			b.SetBytes(int64(testCase.size * 16)) // re+im = 16 bytes per element
			b.ResetTimer()

			for range b.N {
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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			re := make([]float64, testCase.size)
			im := make([]float64, testCase.size)
			dst := make([]float64, testCase.size)

			for i := range re {
				re[i] = float64(i) / 10.0
				im[i] = float64(testCase.size-i) / 10.0
			}

			b.SetBytes(int64(testCase.size * 16)) // re+im = 16 bytes per element
			b.ResetTimer()

			for range b.N {
				PowerFromParts(dst, re, im)
			}
		})
	}
}

// Benchmark the old implementation for comparison.
func magnitudeNaive(inData []complex128) []float64 {
	if len(inData) == 0 {
		return nil
	}

	out := make([]float64, len(inData))
	for i := range out {
		out[i] = cmplx.Abs(inData[i])
	}

	return out
}

func powerNaive(inData []complex128) []float64 {
	if len(inData) == 0 {
		return nil
	}

	out := make([]float64, len(inData))
	for i := range out {
		x := inData[i]
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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			inData := make([]complex128, testCase.size)
			for i := range inData {
				inData[i] = complex(float64(i)/10.0, float64(testCase.size-i)/10.0)
			}

			b.SetBytes(int64(testCase.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for range b.N {
				_ = magnitudeNaive(inData)
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

	for _, testCase := range sizes {
		b.Run(testCase.name, func(b *testing.B) {
			inData := make([]complex128, testCase.size)
			for i := range inData {
				inData[i] = complex(float64(i)/10.0, float64(testCase.size-i)/10.0)
			}

			b.SetBytes(int64(testCase.size * 16)) // complex128 = 16 bytes
			b.ResetTimer()

			for range b.N {
				_ = powerNaive(inData)
			}
		})
	}
}
