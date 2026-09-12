package spectrum

import (
	"math"
	"strconv"
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

// BenchmarkGoertzelBankBins sweeps the bin count across the four-wide grouping's
// boundaries: 1-3 take the scalar remainder path only, 4 and 8 are exact groups,
// and 5/7/9 mix a group with a remainder.
func BenchmarkGoertzelBankBins(b *testing.B) {
	freqs := []float64{110, 220, 440, 697, 770, 852, 941, 1209, 1336, 1477, 1633, 1750, 1900, 2100, 2300, 2500}
	buf := benchSignal(1024)

	for _, bins := range []int{1, 2, 3, 4, 5, 7, 8, 16} {
		bank, err := NewGoertzelBank(freqs[:bins], 8000)
		if err != nil {
			b.Fatalf("NewGoertzelBank: %v", err)
		}

		b.Run("bins="+strconv.Itoa(bins), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				bank.ProcessBlock(buf)
			}
		})
	}
}
