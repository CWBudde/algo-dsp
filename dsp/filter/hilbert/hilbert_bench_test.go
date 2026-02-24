package hilbert

import (
	"math"
	"testing"
)

func BenchmarkProcessSample64(b *testing.B) {
	p, err := New64Default()
	if err != nil {
		b.Fatalf("New64Default() error = %v", err)
	}

	x := 0.0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x = math.Sin(float64(i) * 2 * math.Pi / 97)
		_, _ = p.ProcessSample(x)
	}
}

func BenchmarkProcessBlock64(b *testing.B) {
	p, err := New64Default()
	if err != nil {
		b.Fatalf("New64Default() error = %v", err)
	}

	const n = 1024
	input := make([]float64, n)
	outA := make([]float64, n)
	outB := make([]float64, n)
	for i := range input {
		input[i] = math.Sin(2 * math.Pi * float64(i) / 127)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := p.ProcessBlock(input, outA, outB); err != nil {
			b.Fatalf("ProcessBlock() error = %v", err)
		}
	}
}

func BenchmarkProcessSample32(b *testing.B) {
	p, err := New32Default()
	if err != nil {
		b.Fatalf("New32Default() error = %v", err)
	}

	x := float32(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x = float32(math.Sin(float64(i) * 2 * math.Pi / 97))
		_, _ = p.ProcessSample(x)
	}
}
