package spectrum

import (
	"math"
	"testing"
)

func benchSignal(n int) []float64 {
	buf := make([]float64, n)
	for i := range buf {
		buf[i] = math.Sin(2*math.Pi*1000*float64(i)/8000) * 0.5
	}

	return buf
}

func BenchmarkGoertzelProcessBlock(b *testing.B) {
	analyzer, _ := NewGoertzel(1000, 8000)
	buf := benchSignal(1024)

	b.ReportAllocs()

	for b.Loop() {
		analyzer.Reset()
		analyzer.ProcessBlock(buf)
	}

	_ = analyzer.Power()
}

func BenchmarkGoertzelBankProcessBlock(b *testing.B) {
	freqs := []float64{697, 770, 852, 941, 1209, 1336, 1477, 1633}
	bank, _ := NewGoertzelBank(freqs, 8000)
	buf := benchSignal(1024)
	dst := make([]float64, bank.Len())

	b.ReportAllocs()

	for b.Loop() {
		bank.Reset()
		bank.ProcessBlock(buf)
		bank.Powers(dst)
	}
}
